STABLE_DIFF_VERSION = master-813-bfbef5b
MALINA_VERSION = v1.0.1

Upgrade this Malina repository to stable-diffusion.cpp
<STABLE_DIFF_VERSION> and prepare Malina release <MALINA_VERSION>.

Use the latest published Malina tag before this work as the release-note
baseline. If <MALINA_VERSION> already exists locally or on the remote, do not
move or replace the tag. Stop and select/recommend the next unused patch
version instead.

Carry the upgrade through upstream API/ABI analysis, implementation, artifact
validation, real model generation, benchmarks, documentation, Malina
versioning, and release notes. Do not only change version strings.

## Preflight

1. Inspect git status and preserve unrelated worktree changes.
2. Inspect local and remote Malina tags.
3. Identify the currently pinned stable-diffusion.cpp release in
   `pkg/download.DefaultSDVersion`.
4. Resolve <STABLE_DIFF_VERSION> to its exact upstream commit and compare it
   with the currently pinned release.
5. Inspect the complete GitHub release asset list for
   leejet/stable-diffusion.cpp <STABLE_DIFF_VERSION>.
6. Confirm that every platform Malina advertises has the required artifacts:
   - macOS Metal arm64
   - Windows CPU amd64
   - Windows CUDA 12 amd64, including any separate CUDA runtime/cuBLAS archive
   - Windows Vulkan amd64
   - Windows ROCm amd64
   - Linux CPU amd64
   - Linux Vulkan amd64
   - Linux ROCm amd64
7. Check whether upstream changed artifact names, archive formats, internal
   directory layouts, dependency packaging, SONAME links, or supported
   platforms. Do not assume the previous regexes still match.
8. If required release assets are unavailable, report the missing artifacts
   and do not update the default yet.

## FFI and ABI audit

Compare leejet/stable-diffusion.cpp's public
`include/stable-diffusion.h` between the currently pinned release and
<STABLE_DIFF_VERSION>. Use the exact commits behind both release tags.

Audit every ABI-sensitive type used by `pkg/sd`, including:

- `sd_ctx_params_t`
- `sd_image_t`
- `sd_slg_params_t`
- `sd_guidance_params_t`
- `sd_sample_params_t`
- `sd_pm_params_t`
- `sd_pulid_params_t`
- `sd_tiling_params_t`
- `sd_cache_params_t`
- `sd_hires_params_t`
- `sd_img_gen_params_t`
- every enum represented as a Go integer
- callback typedefs
- every function prepared through `lib.Prep`

Also inspect changed upstream types and functions that Malina does not
currently bind, such as video, upscaler, cancellation, ControlNet hot-swap,
ADetailer, conversion, device-listing, and result-freeing APIs. Add bindings
only when required to keep Malina's existing API correct or when they replace
unsafe compatibility code. Do not expand the public API merely because
upstream added optional features.

Identify and account for:

- struct fields added, removed, reordered, or retyped
- changed nested structs
- changed enum names, values, and count sentinels
- changed function parameters, return types, and out-parameters
- changed callback signatures
- removed or renamed symbols
- changed allocation and deallocation ownership
- LP64 and LLP64 field offsets, alignment, and total struct sizes
- libffi argument indirection, especially for pointer and out-parameter calls

Use an authoritative C `sizeof`/`offsetof` probe against the target header for
every changed struct. Do not rely only on hand-calculated offsets. Make the
smallest required binding changes and update the struct-size/default-value
tests whenever layouts change.

If upstream removes or retypes a public `ContextParams` or `ImgGenParams`
field, update Malina's public type honestly. Do not retain a dangerous no-op
field. Document any resulting Malina API compatibility impact in the release
notes.

## Pinned version and download matrix

Update all active places that establish, test, or describe the default
stable-diffusion.cpp version, including:

- `pkg/download.DefaultSDVersion`
- downloader asset-selection patterns and platform matrix tests
- installer CLI help
- Makefile examples
- README compatibility and architecture references
- CI comments and behavior that rely on the default
- benchmark provenance when benchmarks are rerun
- relevant comments and examples

Add or update an exact regression test asserting that `DefaultSDVersion`
equals <STABLE_DIFF_VERSION>.

Use the target release's real asset names in regression tests. For releases
that split dependencies into companion archives, ensure the installer
downloads and extracts every required archive from the same release. Preserve
support for explicitly requested older valid tags when their packaging is
self-contained.

Search the entire repository for stale references to the previous active
stable-diffusion.cpp version and removed upstream fields. Distinguish active
defaults from historical statements: do not rewrite historical benchmark
claims unless those measurements are actually rerun.

## Local runtime validation

The repository's `lib/` directory is ignored and may be upgraded as part of
this task. Install the exact target release through Malina's installer, using
the appropriate processor for the current host. On Apple Silicon, for example:

    go run . install \
        -lib "$PWD/lib" \
        -p metal \
        -v <STABLE_DIFF_VERSION> \
        -u \
        -q

Verify:

- the selected asset belongs to <STABLE_DIFF_VERSION>
- the local library and all sibling dependencies were replaced
- all required symbols resolve through `sd.Load`
- `malina system` succeeds against the installed library
- FFI struct-size and default-parameter tests pass against that library
- an empty-context error path returns safely rather than crashing
- at least one real SD 1.5 generation completes through `GenerateImage`
- returned pixels survive the C-to-Go copy and PNG round trip
- result memory is released using the target API's matched allocator

Use the standard local model locations when available:

    export MALINA_LIB="$PWD/lib"
    export MALINA_TEST_MODEL="$HOME/models/sd-1.5/v1-5-pruned-emaonly.safetensors"
    export MALINA_SDXL_TEST_MODEL="$HOME/models/sdxl-base-1.0/sd_xl_base_1.0.safetensors"
    export MALINA_FLUX2_TEST_DIR="$HOME/models/flux2-klein-9b"

Run the SD 1.5, SDXL, FLUX.2, and img2img smoke tests when their fixtures are
available. If models are missing, report exactly which tests skipped. Do not
claim runtime compatibility based only on struct-size tests or a differently
packaged Homebrew library when the upstream release can be installed.

## Benchmarks

Rerun every benchmark target included by `make bench` against the exact
<STABLE_DIFF_VERSION> library installed in `./lib`:

    make bench BENCHTIME=1x

This currently includes:

- `BenchmarkGenerateImageSD15`
- `BenchmarkGenerateImageSDXL`
- `BenchmarkGenerateImageFlux2`
- `BenchmarkGenerateImageImg2ImgSD15`

Use a larger `BENCHTIME` only when practical; model loading and image
generation are expensive. Record the exact commands and any skipped benchmark
caused by a missing model.

Update `BENCHMARKS.md` with the actual output from the target library:

- stable-diffusion.cpp release and artifact provenance
- host and backend
- model/bundle and image shape
- denoising step count and `b.N`
- `ns/op` and `s/img`
- `B/op` and `allocs/op`
- img2img results when that benchmark is part of `make bench`

Keep or improve exact reproduction instructions. Never change only the
stable-diffusion.cpp version label while retaining measurements produced by
the previous library.

## Malina release version

Set Malina's reported version to <MALINA_VERSION> without the leading `v` in
`version.go`.

Update `version_test.go` to assert the exact expected value rather than merely
checking that `Version()` is non-empty.

Do not create, move, force-update, or push a Git tag without explicit
approval.

## Verification

After all changes, run:

    gofmt -s -w <changed Go files>
    go vet ./...
    staticcheck ./...
    go build ./...
    go test ./...
    gopls check <changed Go files>
    git diff --check

Also run `pkg/sd` tests with `MALINA_LIB="$PWD/lib"` and the available model
environment variables so they exercise the newly installed target library.
Run a focused real-generation smoke test with `-count=1` so test caching cannot
hide an FFI failure.

Do not suppress diagnostics. Report failed checks, skipped tests, missing
models, unavailable platform hardware, and any validation that could not be
performed locally.

## Release notes

Produce Markdown release notes for <MALINA_VERSION> using the latest published
Malina tag before this work as the baseline. Summarize the complete changes
since that tag, not merely the immediately preceding commit.

Include, when applicable:

- the stable-diffusion.cpp upgrade
- platform artifact and installer changes
- FFI/API/ABI changes or explicit compatibility confirmation
- memory-ownership changes
- dependency upgrades
- model smoke-test results
- benchmark results
- regression coverage
- Malina version reporting
- public API compatibility or breaking changes
- upgrade commands
- a full changelog URL

Use this upgrade command:

    go install github.com/ardanlabs/malina@<MALINA_VERSION>
    malina install -u -lib ./lib

Copy the final Markdown release notes into the macOS clipboard with `pbcopy`
when available and verify the clipboard heading. If `pbcopy` is unavailable,
write the notes to a clearly identified temporary Markdown file and report its
path.

## Final report

Summarize:

- files changed
- upstream commit comparison and whether FFI changes were required
- changed public API, if any
- target artifacts validated and any platform limitations
- installed local library path and release
- real model-generation tests and skipped fixtures
- benchmark highlights
- verification results
- release/tag state
- where the release notes were placed
