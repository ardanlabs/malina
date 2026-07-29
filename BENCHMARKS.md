# Benchmarks

Performance numbers for `sdk/malina/sd` against each of the three model bundles
`malina model pull` knows how to download. Recorded on an Apple M5 Max
(darwin/arm64) with the Metal backend baked into upstream
`stable-diffusion-master-*` releases. The Go benchmarks all run against
the same `lib/libstable-diffusion.dylib`.

Reproduce with:

```
make download-stable-diffusion.cpp   # populates ./lib
make download-models                 # populates ~/models (sd-1.5, sdxl-base-1.0, flux2-klein-9b)
make bench                           # runs all three per-bundle benchmarks
```

## Methodology

- **Drivers**: `BenchmarkGenerateImageSD15`, `BenchmarkGenerateImageSDXL`,
  and `BenchmarkGenerateImageFlux2` in
  [`sdk/malina/sd/benchmark_test.go`](sdk/malina/sd/benchmark_test.go). Each loads its
  bundle's checkpoint(s) once, drops one untimed warm-up iteration (so
  Metal JIT and any first-call library setup do not pollute the
  measurement), and then runs `b.N` `GenerateImage` calls reusing the
  same `Context`.
- **`FreeParamsImmediately = false`** is required for the reuse model:
  the C library's `sd_ctx_params_init` defaults it to `true`, which
  releases the model parameter tensors after the first call and aborts
  the second call with `GGML_ASSERT(buft) failed`. The bench helper
  `benchContextParams` flips it back to `false`.
- **Default shape**: 512x512 with the bundle's natural step count
  (SD 1.5 / SDXL: 20 steps from `sd_img_gen_params_init`; FLUX.2
  [klein]: 4 steps, since the model is 4-step distilled). 512x512 keeps
  per-iteration wall time bounded on Metal — SDXL at its native
  1024x1024 is ~30 s/iter and FLUX.2 at 1024x1024 is well over a
  minute.
- **`BENCHTIME` default `1x`**: a single SD/SDXL/FLUX inference is on
  the order of tens of seconds on Metal, *and* Go's testing framework
  invokes the bench body twice when N>1 (once with N=1 to validate,
  then with N=Nrequested — see
  [`testing/benchmark.go`](https://pkg.go.dev/testing) `launch`),
  which doubles the model-load cost. Override `BENCHTIME=Nx` for
  variance estimates, at the cost of an extra model load + warm-up
  pass.
- **Reported metrics**: `ns/op` (wall time per `GenerateImage` call),
  `s/img` (the same number in seconds for readability), and `px`
  (width × height of the generated image, so the bench output records
  what shape was timed).

## End-to-end text-to-image generation

Recorded on Apple M5 Max, Metal backend, `lib/libstable-diffusion.dylib`
built from upstream stable-diffusion.cpp.

| Bundle           | Shape     | Steps | b.N | ns/op          | s/img | B/op    | allocs/op |
|------------------|-----------|------:|----:|---------------:|------:|--------:|----------:|
| sd-1.5           | 512x512   |    20 |   1 | 18,460,529,666 | 18.46 | 792,408 |       104 |
| sdxl-base-1.0    | 512x512   |    20 |   1 |  8,469,844,708 |  8.47 | 793,176 |       120 |
| flux2-klein-9b   | 512x512   |     4 |   1 | 13,341,297,750 | 13.34 | 793,896 |       120 |

Run commands:

```
make bench-sd-1.5    # BenchmarkGenerateImageSD15
make bench-sdxl      # BenchmarkGenerateImageSDXL
make bench-flux2     # BenchmarkGenerateImageFlux2
make bench           # all three in sequence
```

Each bench skips (rather than fails) when its model env points at a
missing file, so a partial local layout still produces useful output.
The per-bundle env vars are:

| Benchmark                   | Bundle path env (override)   | Smoke-test fallback     |
|-----------------------------|------------------------------|-------------------------|
| `BenchmarkGenerateImageSD15`  | `MALINA_BENCH_MODEL`         | `MALINA_TEST_MODEL`     |
| `BenchmarkGenerateImageSDXL`  | `MALINA_BENCH_SDXL_MODEL`    | `MALINA_SDXL_TEST_MODEL`|
| `BenchmarkGenerateImageFlux2` | `MALINA_BENCH_FLUX2_DIR`     | `MALINA_FLUX2_TEST_DIR` |

The `MALINA_BENCH_*` variants take precedence over the `MALINA_*_TEST_*`
forms so contributors can benchmark a different checkpoint without
disturbing what `make test` exercises.

## Memory & allocations

The `B/op` and `allocs/op` columns above are Go-side only — the bulk of
each generation's heap is in the C library, which Go's `pprof` cannot
see. The per-call Go allocations come from:

- The `cImgGenParams` value populated by `sd_img_gen_params_init` (one
  allocation, reused only as a stack slot).
- C string copies for `Prompt` and `NegativePrompt` (`cStringRefs`
  holds them alive across the FFI call via `runtime.KeepAlive`).
- The decoded `SDImage` struct returned by `GenerateImage`, plus the
  `[]byte` pixel buffer copied out of the C heap.

The numbers are essentially constant across `b.N` for a given bundle,
which confirms the Go marshalling path is not allocating per pixel or
per denoising step.

## Profiling

`make profile-sd-1.5`, `make profile-sdxl`, and `make profile-flux2`
capture CPU + memory profiles for the matching benchmark and write them
to `./profiles/`:

```
make profile-sd-1.5      # BenchmarkGenerateImageSD15 + pprof artifacts
make profile-sdxl        # BenchmarkGenerateImageSDXL + pprof artifacts
make profile-flux2       # BenchmarkGenerateImageFlux2 + pprof artifacts
make profile             # all three, in sequence
```

Override `PROFILE_BENCHTIME` (default `1x`) when you need more samples.
Pprof samples CPU at 10 ms granularity, so a profile from a single
multi-second iteration already contains hundreds of samples; bumping to
`PROFILE_BENCHTIME=3x` gives ~3x more samples at the cost of one extra
model load + warm-up pass (see the BENCHTIME note in the makefile).

Inspect with the standard `go tool pprof` web UI:

```
go tool pprof -http=:0 profiles/sd-1.5.cpu.prof
go tool pprof -http=:0 profiles/sd-1.5.mem.prof
go tool pprof -http=:0 profiles/sdxl.cpu.prof
go tool pprof -http=:0 profiles/sdxl.mem.prof
go tool pprof -http=:0 profiles/flux2.cpu.prof
go tool pprof -http=:0 profiles/flux2.mem.prof
```

What to expect:

- **`*.cpu.prof`** is dominated by `purego.SyscallN` /
  `ffi.Fun.Call` trampolines (almost all real work happens inside the
  loaded `libstable-diffusion.dylib`, which pprof cannot see). The
  Go-side time is the FFI marshalling cost — useful for confirming
  no surprise hot spots have crept into `GenerateImage`'s prep
  (string interning, `cImgGenParams` defaults round-trip, etc.).
- **`*.mem.prof`** is small — `GenerateImage` itself does not allocate
  per denoising step; the only Go allocations per iteration are the
  params struct copy, the `cStringRefs` C string buffers, and the
  decoded `SDImage` pixel buffer copied out of the C heap.

The captured `*.test` binaries and `*.prof` files are gitignored.
