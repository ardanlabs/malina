# Benchmarks

Performance numbers for `pkg/sd` against the locally available model bundles.
Recorded on an Apple M5 Max with
128 GiB RAM (macOS 27.0.1, darwin/arm64) using the Metal backend from upstream
stable-diffusion.cpp `master-1106-532d2c1` artifact
`sd-master-532d2c1-bin-Darwin-macOS-26.7.2-arm64.zip` (SHA-256
`8146d6ffc75f8ff8f64b62a80fffee8fb714ab65e368e202084135fa07cc6f36`,
GitHub asset ID 753442992, 38,331,254 bytes).
The Go benchmarks all run against the extracted
`./lib/libstable-diffusion.dylib` (SHA-256
`81de32ea476979c40367661705c6767a6adc296cb7599f4e075f8196f93989ca`).
Recorded November 16, 2026, for Malina 1.2.3 using Go 1.27.4, automatic operator selection,
and Malina's Metal workaround: batched CFG is disabled at context creation.

The Metal batched-CFG workaround is retained from Malina 1.1.8. The earlier
`master-975-95b3090` four-step 128x128 img2img probe returned 49,152/49,152
white bytes with batched CFG enabled and a range of 0–255 with it disabled.
That diagnostic was not rerun here; these new measurements use disabled
batched CFG and retain native generation metadata and result copying.
The model-backed suite passes with that configuration. Timings alone are not
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
  `ImgGenParamsInit` and the native opaque-options defaults. Img2img retains the
  20-step configuration and the default 0.75 strength, which executes a
  16-step denoising schedule. 512x512 keeps per-iteration wall time bounded
  on Metal — SDXL at its native 1024x1024 is ~30 s/iter.
- **`BENCHTIME` default `1x`**: a single SD/SDXL inference is on
  the order of tens of seconds on Metal, _and_ Go's testing framework
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

| Workload       | Model/bundle  | Shape   | Steps | b.N |         ns/op | s/img |    B/op | allocs/op |
| -------------- | ------------- | ------- | ----: | --: | ------------: | ----: | ------: | --------: |
| text-to-image  | sd-1.5        | 512x512 |    20 |   1 | 6,632,107,542 |  6.63 | 800,400 |       308 |
| text-to-image  | sdxl-base-1.0 | 512x512 |    20 |   1 | 7,211,255,292 |  7.21 | 800,400 |       308 |
| image-to-image | sd-1.5        | 512x512 |    16 |   1 | 5,968,779,500 |  5.97 | 800,912 |       328 |

Actual benchmark output (`b.N=1`, after one untimed warm-up):

```text
BenchmarkGenerateImageSD15-18                  1  6632107542 ns/op  262144 px  6.632 s/img  800400 B/op  308 allocs/op
BenchmarkGenerateImageSDXL-18                  1  7211255292 ns/op  262144 px  7.211 s/img  800400 B/op  308 allocs/op
BenchmarkGenerateImageImg2ImgSD15-18            1  5968779500 ns/op  262144 px  5.969 s/img  800912 B/op  328 allocs/op
```

All three targets completed without skips. These one-iteration runs do not
establish a statistically reliable performance regression or speedup.

## Version-over-version comparison

### Malina 1.2.0 → 1.2.2

Previous measurements come from `git show v1.2.0:BENCHMARKS.md`, recorded
November 5, 2026 with stable-diffusion.cpp `master-1052-f5df5f8` and Go
1.27.3. The Malina 1.2.2 measurements were recorded November 11 with
`master-1088-32d516c` and Go 1.27.4. Both used the same Apple M5 Max,
128 GiB RAM, macOS 27.0.1, Metal backend, model bundles, 512x512 shapes,
step counts, disabled batched CFG, and `BENCHTIME=1x` methodology.
The Go toolchain also changed, so differences cannot be attributed solely
to the native-library upgrade.

Time change is (current ns/op ÷ previous ns/op − 1) × 100, calculated from
the unrounded measurements. **Negative means less time per image (faster);
positive means more time per image (slower).**

| Workload / sample                    | Previous s/img | Current s/img | Change, seconds | Time change |
| ------------------------------------ | -------------: | ------------: | --------------: | ----------: |
| SD 1.5 text-to-image                 |          6.583 |         6.577 |          −0.006 |      −0.09% |
| SDXL text-to-image                   |          7.127 |         7.241 |          +0.113 |      +1.59% |
| SD 1.5 img2img, previous initial run |         14.915 |         5.922 |          −8.993 |     −60.30% |
| SD 1.5 img2img, previous repeat      |          6.252 |         5.922 |          −0.330 |      −5.28% |

The two img2img rows compare the **same current sample** against both
previous samples, not two new runs. The previous 14.915-second initial
sample did not persist in its 6.252-second repeat; its cause remains
unknown. Showing only the initial sample would exaggerate the apparent
improvement. SD 1.5 text-to-image was nearly unchanged in these samples,
SDXL took slightly longer, and img2img took less time than either previous
sample. None of these single-iteration comparisons establishes a reliable
speedup or regression; repeated samples are needed to distinguish a trend
from run-to-run variation.

For that comparison, Go allocation counts remain 266/image for all workloads. Text-to-image
memory rises from 800,112 to 800,144 B/op (+32). Img2img rises from
800,224 B/op in the previous initial run or 800,240 B/op in its repeat to
800,288 B/op (+64 or +48 respectively). These exclude native allocations.

### Malina 1.2.2 → 1.2.3

The baseline is the latest published tag before this upgrade, `v1.2.2`:
`git show v1.2.2:BENCHMARKS.md`. Its measurements were recorded November 11,
2026 with `master-1088-32d516c`; the current measurements were recorded
November 16, 2026 with `master-1106-532d2c1`. Both use Go 1.27.4 on the same
Apple M5 Max, 128 GiB RAM, macOS 27.0.1, Metal backend, model bundles,
512x512 shapes, step counts, disabled batched CFG, automatic sampler/scheduler,
and one warm-up followed by `BENCHTIME=1x`. The benchmark source and model
fixtures are unchanged; Malina's marshalling now uses opaque native options
and copied tensors instead of public C parameter structs. These measurements
therefore include both the native-library and binding changes. The benchmark
leaves the native seed default (-1, random) unchanged in both versions.

| Workload | Previous s/img | Current s/img | Change, seconds | Time change | B/op, previous → current (delta) | allocs/op, previous → current (delta) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| SD 1.5 text-to-image | 6.577103 | 6.632108 | +0.055004 | +0.84% | 800,144 → 800,400 (+256) | 266 → 308 (+42) |
| SDXL text-to-image | 7.240571 | 7.211255 | −0.029316 | −0.40% | 800,144 → 800,400 (+256) | 266 → 308 (+42) |
| SD 1.5 img2img | 5.921618 | 5.968780 | +0.047162 | +0.80% | 800,288 → 800,912 (+624) | 266 → 328 (+62) |

Percentages use the unrounded baseline ns/op values 6,577,103,166,
7,240,571,042, and 5,921,617,708, and the current ns/op values in the raw
output above. Negative time change means faster; positive means slower.
All three workloads are comparable under the methodology above. These are
single samples, not statistically established speedups or regressions.
No repeats or outliers have been excluded; the older comparison's two
img2img baseline samples remain recorded above.

## Benchmark commands

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

| Benchmark                           | Bundle path env (override) | Smoke-test fallback      |
| ----------------------------------- | -------------------------- | ------------------------ |
| `BenchmarkGenerateImageSD15`        | `MALINA_BENCH_MODEL`       | `MALINA_TEST_MODEL`      |
| `BenchmarkGenerateImageSDXL`        | `MALINA_BENCH_SDXL_MODEL`  | `MALINA_SDXL_TEST_MODEL` |
| `BenchmarkGenerateImageImg2ImgSD15` | `MALINA_BENCH_MODEL`       | `MALINA_TEST_MODEL`      |

The `MALINA_BENCH_*` variants take precedence over the `MALINA_*_TEST_*`
forms so contributors can benchmark a different checkpoint without
disturbing what `make test` exercises.

## Memory & allocations

The `B/op` and `allocs/op` columns above are Go-side only — the bulk of
each generation's heap is in the C library, which Go's `pprof` cannot
see. The per-call Go allocations come from:

- Compatibility-field inspection and typed native-option setter arguments,
  including C string copies for keys, prompts, and explicit mode values.
- Input-tensor preparation for img2img; the native tensor constructor copies
  the input pixels and the option object retains its storage.
- The decoded `SDImage` struct returned by `GenerateImage`, plus the
  `[]byte` pixel buffer copied out of the C heap.
- Each result's native `sdcpp.generation/v1` JSON copied into `GenerationInfo`.

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
  (compatibility-field inspection and native-option setters, etc.).
- **`*.mem.prof`** is small — `GenerateImage` itself does not allocate
  per denoising step; the only Go allocations per iteration are the
  option setter arguments, C string buffers, and the
  decoded `SDImage` pixel buffer copied out of the C heap.

The captured `*.test` binaries and `*.prof` files are gitignored.
