# Get the absolute path of the current Makefile.
MAKEFILE_PATH := $(realpath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR  := $(dir $(MAKEFILE_PATH))
MALINA_LIB    ?= $(MAKEFILE_DIR)lib
MODELS_DIR    ?= $(HOME)/models

# -----------------------------------------------------------------------------
# Bundle downloads. Each target invokes `malina model pull` which downloads
# every file in the named bundle into $(MODELS_DIR)/<bundle>/ along with a
# manifest.json. The FLUX bundles are license-gated; set HF_TOKEN first.

download-models:
	go run . model pull -y -o $(MODELS_DIR) sd-1.5
	go run . model pull -y -o $(MODELS_DIR) sdxl-base-1.0
	go run . model pull -y -o $(MODELS_DIR) flux2-klein-4b
	go run . model pull -y -o $(MODELS_DIR) flux2-klein-9b	


clean-stable-diffusion.cpp:
	rm -rf $(MALINA_LIB)/*

# Install stable-diffusion.cpp shared libraries into $(MALINA_LIB).
#
# leejet/stable-diffusion.cpp publishes two flavors of GitHub Releases:
#   - semver tags such as v0.9.0 (stable releases)
#   - rolling master builds tagged master-<N>-<shortsha>, auto-published by
#     CI for every commit on master. These are real Releases with binaries
#     attached, which is why "master-N-shortsha" can be queried and
#     downloaded just like a versioned tag.
#
# This target always passes -u (upgrade) so an existing install in
# $(MALINA_LIB) is replaced rather than silently skipped.
#
#   make download-stable-diffusion.cpp                          # malina-pinned version (see pkg/download.DefaultSDVersion)
#   make download-stable-diffusion.cpp VERSION=latest           # whatever leejet/stable-diffusion.cpp /releases/latest returns
#   make download-stable-diffusion.cpp VERSION=master-820-de298c2
#   make download-stable-diffusion.cpp VERSION=v0.9.0
download-stable-diffusion.cpp:
	go run . install -lib $(MALINA_LIB) -u $(if $(VERSION),-v $(VERSION))

install:
	go install .

lint:
	go vet ./...
	staticcheck -checks=all ./...

vuln-check:
	govulncheck ./...

diff:
	go fix -diff ./...

# make test runs all package tests, including the model-backed tests guarded
# by the malina_model_tests build tag. MALINA_LIB must point at a directory
# with libstable-diffusion (see download-stable-diffusion.cpp). The pkg/sd
# end-to-end smoke test additionally requires MALINA_TEST_MODEL to point at
# a stable-diffusion checkpoint; when unset it is skipped, not failed. GitHub
# Actions intentionally runs go test without this tag so no model is needed.
#
# Default the per-bundle test env vars to the layout `malina model pull`
# writes under $(MODELS_DIR). When a contributor has downloaded all three
# bundles via `make download-models` (or `make pull-test-assets` for just
# sd-1.5), `make test` exercises every per-bundle smoke test in pkg/sd.
# Each test independently skips (not fails) when its env var is unset or
# the file/directory is missing, so contributors who only have a subset
# of the bundles never see false failures.
MALINA_TEST_MODEL      ?= $(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors
MALINA_SDXL_TEST_MODEL ?= $(MODELS_DIR)/sdxl-base-1.0/sd_xl_base_1.0.safetensors
MALINA_FLUX2_TEST_DIR  ?= $(MODELS_DIR)/flux2-klein-9b

test-only:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_TEST_MODEL=$(abspath $(MALINA_TEST_MODEL)) && \
	export MALINA_SDXL_TEST_MODEL=$(abspath $(MALINA_SDXL_TEST_MODEL)) && \
	export MALINA_FLUX2_TEST_DIR=$(abspath $(MALINA_FLUX2_TEST_DIR)) && \
	go test -count=1 -tags=malina_model_tests ./...

# test-race re-runs the suite under the race detector. The FFI helpers are
# expected to be called from arbitrary goroutines in production callers
# (kronk, server middlewares), so this catches unsynchronized access to
# the ffi.Fun trampolines and the sync.Once gate in pkg/sd.
test-race:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_TEST_MODEL=$(abspath $(MALINA_TEST_MODEL)) && \
	export MALINA_SDXL_TEST_MODEL=$(abspath $(MALINA_SDXL_TEST_MODEL)) && \
	export MALINA_FLUX2_TEST_DIR=$(abspath $(MALINA_FLUX2_TEST_DIR)) && \
	go test -count=1 -race -tags=malina_model_tests ./...

test: test-only lint vuln-check diff

# pull-test-assets downloads everything `make test` needs to exercise the
# end-to-end paths: the stable-diffusion shared libraries and the SD 1.5
# bundle the pkg/sd smoke test loads. Both halves are idempotent — the
# install command skips when libstable-diffusion is already present in
# $(MALINA_LIB), and pkg/download.GetBundle skips fully-present files and
# HTTP-Range-resumes partial ones.
pull-test-assets:
	go run . install -lib $(MALINA_LIB)
	go run . model pull -y -o $(MODELS_DIR) sd-1.5

tidy:
	go mod tidy

deps-upgrade:
	go get -u -v ./...
	go mod tidy

# ==============================================================================
# Benchmarks and profiles
#
# Each bundle has its own benchmark in pkg/sd/benchmark_test.go that runs a
# single text-to-image generation per iteration against a real checkpoint.
# Model loading happens once outside the timed loop and a warm-up iteration
# is dropped so Metal/CUDA JIT does not pollute the steady-state measurement.
# See BENCHMARKS.md for the methodology and recorded numbers.
#
# BENCHTIME controls iteration count for the bench targets (default 1x);
# PROFILE_BENCHTIME mirrors it for the profile targets but defaults to
# 10x so the captured profiles are dominated by steady-state work rather
# than one-shot setup. benchSetup loads the .dylib once per process and
# runGenerateBench drops one warm-up GenerateImage before the timed loop,
# so additional iterations only re-run the steady-state path that we
# actually want to profile. Note: Go's testing framework runs the bench
# body twice when N>1 (once with N=1, then with N=Nrequested — see
# testing/benchmark.go:340), but with PROFILE_BENCHTIME=Nx that initial
# N=1 pass is amortized across the larger sample. Override BENCHTIME=Nx
# on the bench targets to get repeated samples for benchstat variance.

BENCHTIME               ?= 1x
PROFILE_BENCHTIME       ?= 10x
MALINA_BENCH_MODEL      ?= $(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors
MALINA_BENCH_SDXL_MODEL ?= $(MODELS_DIR)/sdxl-base-1.0/sd_xl_base_1.0.safetensors
MALINA_BENCH_FLUX2_DIR  ?= $(MODELS_DIR)/flux2-klein-9b

# make bench-sd-1.5 runs BenchmarkGenerateImageSD15 against MALINA_BENCH_MODEL.
# Override with `make bench-sd-1.5 MALINA_BENCH_MODEL=...` to benchmark a
# different SD 1.5 checkpoint; pass BENCHTIME=Nx to control iteration count.
bench-sd-1.5:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_MODEL=$(abspath $(MALINA_BENCH_MODEL)) && \
	go test -bench=BenchmarkGenerateImageSD15 -benchtime=$(BENCHTIME) -benchmem -run='^$$' ./pkg/sd/

# make bench-sdxl runs BenchmarkGenerateImageSDXL against MALINA_BENCH_SDXL_MODEL.
bench-sdxl:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_SDXL_MODEL=$(abspath $(MALINA_BENCH_SDXL_MODEL)) && \
	go test -bench=BenchmarkGenerateImageSDXL -benchtime=$(BENCHTIME) -benchmem -run='^$$' ./pkg/sd/

# make bench-flux2 runs BenchmarkGenerateImageFlux2 against MALINA_BENCH_FLUX2_DIR.
bench-flux2:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_FLUX2_DIR=$(abspath $(MALINA_BENCH_FLUX2_DIR)) && \
	go test -bench=BenchmarkGenerateImageFlux2 -benchtime=$(BENCHTIME) -benchmem -run='^$$' ./pkg/sd/

# make bench-img2img-sd-1.5 runs BenchmarkGenerateImageImg2ImgSD15 against
# MALINA_BENCH_MODEL. The benchmark uses an in-process synthesized 512x512
# init image (no file I/O) so it has the same env requirements as
# bench-sd-1.5 — just MALINA_LIB + a model path.
bench-img2img-sd-1.5:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_MODEL=$(abspath $(MALINA_BENCH_MODEL)) && \
	go test -bench=BenchmarkGenerateImageImg2ImgSD15 -benchtime=$(BENCHTIME) -benchmem -run='^$$' ./pkg/sd/

# make bench runs every per-bundle benchmark. Each one skips (not fails)
# when its model env points at a missing file, so a partial local layout
# still produces useful output.
bench: bench-sd-1.5 bench-sdxl bench-flux2 bench-img2img-sd-1.5

# make profile-sd-1.5 captures CPU + memory profiles for the SD 1.5 bench
# and writes them to ./profiles/. The Go-side profile is dominated by
# purego/ffi trampolines because almost all real work happens inside
# libstable-diffusion.dylib (which pprof cannot see), but the memory
# profile is useful for spotting per-call allocations on the marshalling
# path. Inspect with:
#
#   go tool pprof -text profiles/sd-1.5.cpu.prof
#   go tool pprof -text profiles/sd-1.5.mem.prof
profile-sd-1.5:
	mkdir -p profiles
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_MODEL=$(abspath $(MALINA_BENCH_MODEL)) && \
	go test -bench=BenchmarkGenerateImageSD15 -benchtime=$(PROFILE_BENCHTIME) -run='^$$' \
	    -cpuprofile=profiles/sd-1.5.cpu.prof \
	    -memprofile=profiles/sd-1.5.mem.prof \
	    -benchmem \
	    -o profiles/sd-1.5.test \
	    ./pkg/sd/
	@echo
	@echo "Profiles written to ./profiles/. Inspect with:"
	@echo "  go tool pprof -text profiles/sd-1.5.cpu.prof"
	@echo "  go tool pprof -text profiles/sd-1.5.mem.prof"

# make profile-sdxl captures CPU + memory profiles for the SDXL bench.
profile-sdxl:
	mkdir -p profiles
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_SDXL_MODEL=$(abspath $(MALINA_BENCH_SDXL_MODEL)) && \
	go test -bench=BenchmarkGenerateImageSDXL -benchtime=$(PROFILE_BENCHTIME) -run='^$$' \
	    -cpuprofile=profiles/sdxl.cpu.prof \
	    -memprofile=profiles/sdxl.mem.prof \
	    -benchmem \
	    -o profiles/sdxl.test \
	    ./pkg/sd/
	@echo
	@echo "Profiles written to ./profiles/. Inspect with:"
	@echo "  go tool pprof -text profiles/sdxl.cpu.prof"
	@echo "  go tool pprof -text profiles/sdxl.mem.prof"

# make profile-img2img-sd-1.5 captures CPU + memory profiles for the
# img2img benchmark. Useful for spotting allocations in the InitImage
# binding path (`bindCImage`, `runtime.KeepAlive`) and confirming the
# VAE encode pass shows up in the cycles breakdown alongside the
# diffusion steps that dominate txt2img.
profile-img2img-sd-1.5:
	mkdir -p profiles
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_MODEL=$(abspath $(MALINA_BENCH_MODEL)) && \
	go test -bench=BenchmarkGenerateImageImg2ImgSD15 -benchtime=$(PROFILE_BENCHTIME) -run='^$$' \
	    -cpuprofile=profiles/img2img-sd-1.5.cpu.prof \
	    -memprofile=profiles/img2img-sd-1.5.mem.prof \
	    -benchmem \
	    -o profiles/img2img-sd-1.5.test \
	    ./pkg/sd/
	@echo
	@echo "Profiles written to ./profiles/. Inspect with:"
	@echo "  go tool pprof -text profiles/img2img-sd-1.5.cpu.prof"
	@echo "  go tool pprof -text profiles/img2img-sd-1.5.mem.prof"

# make profile-flux2 captures CPU + memory profiles for the FLUX.2 bench.
profile-flux2:
	mkdir -p profiles
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_BENCH_FLUX2_DIR=$(abspath $(MALINA_BENCH_FLUX2_DIR)) && \
	go test -bench=BenchmarkGenerateImageFlux2 -benchtime=$(PROFILE_BENCHTIME) -run='^$$' \
	    -cpuprofile=profiles/flux2.cpu.prof \
	    -memprofile=profiles/flux2.mem.prof \
	    -benchmem \
	    -o profiles/flux2.test \
	    ./pkg/sd/
	@echo
	@echo "Profiles written to ./profiles/. Inspect with:"
	@echo "  go tool pprof -text profiles/flux2.cpu.prof"
	@echo "  go tool pprof -text profiles/flux2.mem.prof"

# make profile runs every profiler in sequence.
profile: profile-sd-1.5 profile-sdxl profile-flux2 profile-img2img-sd-1.5

# -----------------------------------------------------------------------------
# Text profile reports
#
# `make report-<bench>` runs the matching profile target and then dumps the
# CPU + memory profiles to a single text file at profiles/<bench>.report.txt.
# Useful for sharing with humans (or LLMs) without standing up the pprof
# browser. The report contains four sections:
#
#   1. CPU profile, top entries by flat time
#   2. CPU profile, top entries by cumulative time
#   3. Memory profile, top entries by allocated space
#   4. Memory profile, top entries by allocated object count
#
# Override REPORT_NODES to widen or narrow the entry count (default 60).
# Pattern targets: report-sd-1.5, report-sdxl, report-flux2,
# report-img2img-sd-1.5.

REPORT_NODES ?= 60

report-%: profile-%
	@printf "Writing report to profiles/$*.report.txt ..."
	@{ \
	    echo "================================================================"; \
	    echo "CPU profile — top $(REPORT_NODES) by flat time"; \
	    echo "================================================================"; \
	    go tool pprof -text -nodecount=$(REPORT_NODES) \
	        profiles/$*.test profiles/$*.cpu.prof 2>&1; \
	    echo; \
	    echo "================================================================"; \
	    echo "CPU profile — top $(REPORT_NODES) by cumulative time"; \
	    echo "================================================================"; \
	    go tool pprof -text -cum -nodecount=$(REPORT_NODES) \
	        profiles/$*.test profiles/$*.cpu.prof 2>&1; \
	    echo; \
	    echo "================================================================"; \
	    echo "Memory profile — top $(REPORT_NODES) by allocated space"; \
	    echo "================================================================"; \
	    go tool pprof -text -alloc_space -nodecount=$(REPORT_NODES) \
	        profiles/$*.test profiles/$*.mem.prof 2>&1; \
	    echo; \
	    echo "================================================================"; \
	    echo "Memory profile — top $(REPORT_NODES) by allocated objects"; \
	    echo "================================================================"; \
	    go tool pprof -text -alloc_objects -nodecount=$(REPORT_NODES) \
	        profiles/$*.test profiles/$*.mem.prof 2>&1; \
	} > profiles/$*.report.txt
	@echo " done"
	@echo "Share with: cat profiles/$*.report.txt"

# make report runs every text reporter in sequence.
report: report-sd-1.5 report-sdxl report-flux2 report-img2img-sd-1.5

# -----------------------------------------------------------------------------
# Example runners. Each target wires up MALINA_LIB + the model paths the
# example needs and invokes `go run`.

example-system:
	export MALINA_LIB=$(MALINA_LIB) && \
	go run ./examples/system

# example-hello requires the sd-1.5 bundle (make pull-sd-1.5).
example-hello:
	export MALINA_LIB=$(MALINA_LIB) && \
	export MALINA_TEST_MODEL=$(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors && \
	go run ./examples/hello "a lovely cat"

# example-concurrent measures serial and concurrent generation using two
# independent native contexts. Each context loads its own model weights.
example-concurrent:
	export MALINA_LIB=$(MALINA_LIB) && \
	export MALINA_TEST_MODEL=$(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors && \
	go run ./examples/concurrent

# example-img2img requires the sd-1.5 bundle and a source PNG. By default
# it consumes hello.png produced by `make example-hello`, so the natural
# flow is:
#
#   make example-hello                       # writes hello.png
#   make example-img2img                     # rewrites hello.png in oil-painting style
#
# Override IMG2IMG_IN / IMG2IMG_PROMPT / IMG2IMG_STRENGTH to point at your
# own source image and steer the result. Strength runs 0..1; lower values
# preserve more of the source.
IMG2IMG_IN       ?= samples/frames/image1.jpg
IMG2IMG_OUT      ?= img2img.png
IMG2IMG_PROMPT   ?= produce an oil painting of the fields you see in the provided image.
IMG2IMG_STRENGTH ?= 0.6
example-img2img:
	export MALINA_LIB=$(MALINA_LIB) && \
	export MALINA_TEST_MODEL=$(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors && \
	go run ./examples/img2img \
	    -in $(IMG2IMG_IN) \
	    -out $(IMG2IMG_OUT) \
	    -prompt "$(IMG2IMG_PROMPT)" \
	    -strength $(IMG2IMG_STRENGTH)

# example-flux2 requires the flux2-klein-9b bundle (make pull-flux2-klein-9b).
# The example reads $(MODELS_DIR)/flux2-klein-9b/manifest.json for paths.
example-flux2:
	export MALINA_LIB=$(MALINA_LIB) && \
	export MALINA_FLUX2_DIR=$(MODELS_DIR)/flux2-klein-9b && \
	go run ./examples/flux2 "An orange cat on palm beach playing with oranges."

# example-sd-encode demonstrates encoding a directory of PNG frames into a
# Motion-JPEG AVI. No model is loaded; this is a pure-Go encoder. Override
# FRAMES_DIR / FPS / OUT to point at your own frames.
FRAMES_DIR ?= samples/frames
FPS        ?= 24
SECS       ?= 1
OUT        ?= output.avi
example-sd-encode:
	go run ./examples/sd-encode -i $(FRAMES_DIR) -fps $(FPS) -secs $(SECS) -o $(OUT)
