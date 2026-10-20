# Benchmarks

Performance numbers for `pkg/sd` against the locally available model bundles.
Recorded on an Apple M5 Max with
128 GiB RAM (macOS 27.0.1, darwin/arm64) using the Metal backend from upstream
stable-diffusion.cpp `master-975-95b3090` artifact
`sd-master-95b3090-bin-Darwin-macOS-26.6.2-arm64.zip` (SHA-256
`2edf5a21c4b73f448c3a02c87925283b77aa94f1859c0d8e649134da68b40813`).
The Go benchmarks all run against the extracted
`./lib/libstable-diffusion.dylib` (SHA-256
`a43c6e892616753685ef06b42dbedf8b271b086d79111a8dff19894428378f31`).
Recorded October 20, 2026, using Go 1.27.3, automatic operator selection,
and Malina's Metal workaround: batched CFG is disabled at context creation.

The official library's Metal group-normalization kernel leaves the second
batch item unwritten. Disabling batched CFG avoids the reproduced all-white
output; these freshly rerun measurements include that correctness workaround.
The target release updates ggml's conv1d implementation, but its Metal
group-normalization kernel is unchanged. A native four-step, 128x128 SD 1.5
img2img probe against this exact dylib still returns 49,152/49,152 white bytes
with batched CFG enabled; disabling it gives a pixel range of 0–255.
The model-backed suite passes with the workaround. Timings alone are not
an image-quality guarantee or evidence about untested models/backends, and
single-iteration measurements do not establish a statistically reliable
speedup over previous releases.

Reproduce with:

```
make download-stable-diffusion.cpp MALINA_LIB="$PWD/lib"
make download-models                 # populates ~/.kronk/malina-models
make bench BENCHTIME=1x MALINA_LIB="$PWD/lib"
```

## Methodology

- **Drivers**: `BenchmarkGenerateImageSD15`, `BenchmarkGenerateImageSDXL`, and
  `BenchmarkGenerateImageImg2ImgSD15` in
  [`pkg/sd/benchmark_test.go`](pkg/sd/benchmark_test.go). Each loads its
  bundle's checkpoint(s) once, drops one untimed warm-up iteration (so
  Metal JIT and any first-call library setup do not pollute the
  measurement), and then runs `b.N` `GenerateImage` calls reusing the
  same `Context`.
- **Default shape**: 512x512 with 20 configured steps from
  `sd_img_gen_params_init`. Img2img retains the
  20-step configuration and the default 0.75 strength, which executes a
  16-step denoising schedule. 512x512 keeps per-iteration wall time bounded
  on Metal — SDXL at its native 1024x1024 is ~30 s/iter.
- **`BENCHTIME` default `1x`**: a single SD/SDXL inference is on
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

## End-to-end generation

Recorded on Apple M5 Max, Metal backend, using the exact upstream artifact
and digest listed above. All measurements used
`make bench BENCHTIME=1x MALINA_LIB="$PWD/lib"`; no benchmarks skipped.

| Workload                | Model/bundle       | Shape   | Steps | b.N | ns/op          | s/img | B/op    | allocs/op |
|-------------------------|--------------------|---------|------:|----:|---------------:|------:|--------:|----------:|
| text-to-image           | sd-1.5             | 512x512 |    20 |   1 | 6,768,739,250 |  6.77 | 798,896 |       253 |
| text-to-image           | sdxl-base-1.0      | 512x512 |    20 |   1 | 8,064,147,041 |  8.06 | 798,896 |       253 |
| image-to-image          | sd-1.5             | 512x512 |    16 |   1 | 6,098,921,084 |  6.10 | 798,840 |       249 |

Actual benchmark output (`b.N=1`, after one untimed warm-up):

```text
BenchmarkGenerateImageSD15-18                  1  6768739250 ns/op  262144 px  6.769 s/img  798896 B/op  253 allocs/op
BenchmarkGenerateImageSDXL-18                  1  8064147041 ns/op  262144 px  8.064 s/img  798896 B/op  253 allocs/op
BenchmarkGenerateImageImg2ImgSD15-18            1  6098921084 ns/op  262144 px  6.099 s/img  798840 B/op  249 allocs/op
```

Run commands:

```
make bench-sd-1.5    # BenchmarkGenerateImageSD15
make bench-sdxl      # BenchmarkGenerateImageSDXL
make bench-img2img-sd-1.5 # BenchmarkGenerateImageImg2ImgSD15
make bench           # all three in sequence
```

Each bench skips (rather than fails) when its model env points at a
missing file, so a partial local layout still produces useful output.
The per-bundle env vars are:

| Benchmark                   | Bundle path env (override)   | Smoke-test fallback     |
|-----------------------------|------------------------------|-------------------------|
| `BenchmarkGenerateImageSD15`  | `MALINA_BENCH_MODEL`         | `MALINA_TEST_MODEL`     |
| `BenchmarkGenerateImageSDXL`  | `MALINA_BENCH_SDXL_MODEL`    | `MALINA_SDXL_TEST_MODEL`|
| `BenchmarkGenerateImageImg2ImgSD15` | `MALINA_BENCH_MODEL` | `MALINA_TEST_MODEL`     |

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

`make profile-sd-1.5`, `make profile-sdxl`, and
`make profile-img2img-sd-1.5` capture CPU + memory profiles for the matching
benchmark and write them to `./profiles/`:

```
make profile-sd-1.5      # BenchmarkGenerateImageSD15 + pprof artifacts
make profile-sdxl        # BenchmarkGenerateImageSDXL + pprof artifacts
make profile-img2img-sd-1.5 # BenchmarkGenerateImageImg2ImgSD15 + pprof artifacts
make profile             # all three in sequence
```

Override `PROFILE_BENCHTIME` (default `10x`) when you need more samples.
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
go tool pprof -http=:0 profiles/img2img-sd-1.5.cpu.prof
go tool pprof -http=:0 profiles/img2img-sd-1.5.mem.prof
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
