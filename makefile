# Get the absolute path of the current Makefile.
MAKEFILE_PATH := $(realpath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR  := $(dir $(MAKEFILE_PATH))
GOOS           = $(shell go env GOOS)
GOARCH         = $(shell go env GOARCH)
MALINA_BACKEND = $(if $(filter darwin,$(GOOS)),metal,cpu)
MALINA_LIB    ?= $(HOME)/.kronk/malina-libraries/$(GOOS)/$(GOARCH)/$(MALINA_BACKEND)
MODELS_DIR    ?= $(HOME)/.kronk/malina-models

# -----------------------------------------------------------------------------
# Bundle downloads. This target invokes `malina model pull` with its default
# model root, creating one subdirectory and manifest per bundle. License-gated
# bundles are intentionally excluded.

download-models:
	go run . model pull -y sd-1.5
	go run . model pull -y controlnet-canny-sd1.5
	go run . model pull -y realesrgan-x4-anime
	go run . model pull -y adetailer-face-yolov8n
	go run . model pull -y animatediff-sd1.5
	go run . model pull -y sdxl-base-1.0


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
# This target always installs download.DefaultSDVersion. There is no separate
# Make setting for the library version used by tests and examples.
download-stable-diffusion.cpp:
	go run . install -lib $(MALINA_LIB) -u

# Regenerate the trusted archive and installed-file hashes when changing
# download.DefaultSDVersion. This downloads every supported upstream artifact.
generate-library-manifest:
	test -n "$(VERSION)" || (echo "VERSION is required" && exit 1)
	cd pkg/download && go run generate_manifest.go -version $(VERSION)

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
# containing the verified download.DefaultSDVersion install. The pkg/sd model
# tests additionally require their model paths; an unset path skips its test.
# GitHub Actions keeps the primary job model-free and runs the advanced model
# tests in separately cached Linux jobs.
#
# Default the per-bundle test env vars to the layout `malina model pull`
# writes under $(MODELS_DIR). When a contributor has downloaded the bundles
# via `make download-models`, `make test` exercises every per-bundle model test
# in pkg/sd. A test skips when its environment variable is unset, but fails
# when a configured file or directory is missing so CI cannot silently pass
# without the requested fixture.
MALINA_TEST_MODEL      ?= $(MODELS_DIR)/sd-1.5/v1-5-pruned-emaonly.safetensors
MALINA_SDXL_TEST_MODEL ?= $(MODELS_DIR)/sdxl-base-1.0/sd_xl_base_1.0.safetensors
MALINA_CONTROLNET_TEST_DIR ?= $(MODELS_DIR)/controlnet-canny-sd1.5
MALINA_UPSCALER_TEST_DIR   ?= $(MODELS_DIR)/realesrgan-x4-anime
MALINA_ADETAILER_TEST_DIR  ?= $(MODELS_DIR)/adetailer-face-yolov8n
MALINA_VIDEO_TEST_DIR      ?= $(MODELS_DIR)/animatediff-sd1.5

test-only:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_TEST_MODEL=$(abspath $(MALINA_TEST_MODEL)) && \
	export MALINA_SDXL_TEST_MODEL=$(abspath $(MALINA_SDXL_TEST_MODEL)) && \
	export MALINA_CONTROLNET_TEST_DIR=$(abspath $(MALINA_CONTROLNET_TEST_DIR)) && \
	export MALINA_UPSCALER_TEST_DIR=$(abspath $(MALINA_UPSCALER_TEST_DIR)) && \
	export MALINA_ADETAILER_TEST_DIR=$(abspath $(MALINA_ADETAILER_TEST_DIR)) && \
	export MALINA_VIDEO_TEST_DIR=$(abspath $(MALINA_VIDEO_TEST_DIR)) && \
	go test -count=1 -tags=malina_model_tests ./...

# test-race re-runs the suite under the race detector. The FFI helpers are
# expected to be called from arbitrary goroutines in production callers
# (kronk, server middlewares), so this catches unsynchronized access to
# the ffi.Fun trampolines and the sync.Once gate in pkg/sd.
test-race:
	export MALINA_LIB=$(abspath $(MALINA_LIB)) && \
	export MALINA_TEST_MODEL=$(abspath $(MALINA_TEST_MODEL)) && \
	export MALINA_SDXL_TEST_MODEL=$(abspath $(MALINA_SDXL_TEST_MODEL)) && \
	export MALINA_CONTROLNET_TEST_DIR=$(abspath $(MALINA_CONTROLNET_TEST_DIR)) && \
	export MALINA_UPSCALER_TEST_DIR=$(abspath $(MALINA_UPSCALER_TEST_DIR)) && \
	export MALINA_ADETAILER_TEST_DIR=$(abspath $(MALINA_ADETAILER_TEST_DIR)) && \
	export MALINA_VIDEO_TEST_DIR=$(abspath $(MALINA_VIDEO_TEST_DIR)) && \
	go test -count=1 -race -tags=malina_model_tests ./...

test: test-only lint vuln-check diff

# pull-test-assets downloads everything `make test` needs to exercise the
# end-to-end paths: the stable-diffusion shared libraries and every ungated
# bundle used by pkg/sd tests. The installer always refreshes
# download.DefaultSDVersion, while pkg/download.GetBundle skips fully-present
# files and HTTP-Range-resumes partial ones.
pull-test-assets:
	go run . install -lib $(MALINA_LIB) -u
	$(MAKE) download-models

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
bench: bench-sd-1.5 bench-sdxl bench-img2img-sd-1.5

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

# make profile runs every profiler in sequence.
profile: profile-sd-1.5 profile-sdxl profile-img2img-sd-1.5

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
# Pattern targets: report-sd-1.5, report-sdxl, and report-img2img-sd-1.5.

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
report: report-sd-1.5 report-sdxl report-img2img-sd-1.5

# -----------------------------------------------------------------------------
# Example runners use Malina's default library and model locations directly.

example-system:
	go run ./examples/system

# example-hello requires the default sd-1.5 bundle.
example-hello:
	go run ./examples/hello "a lovely cat"

# example-concurrent measures serial and concurrent generation using two
# independent native contexts. Each context loads its own model weights.
example-concurrent:
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
	go run ./examples/img2img \
	    -in $(IMG2IMG_IN) \
	    -out $(IMG2IMG_OUT) \
	    -prompt "$(IMG2IMG_PROMPT)" \
	    -strength $(IMG2IMG_STRENGTH)

# Advanced examples each use the smallest catalog bundle for that feature.
example-controlnet:
	go run ./examples/controlnet

example-upscale:
	go run ./examples/upscale

example-adetailer:
	go run ./examples/adetailer

example-animatediff:
	go run ./examples/animatediff

# example-sd-encode demonstrates encoding a directory of PNG frames into a
# Motion-JPEG AVI. No model is loaded; this is a pure-Go encoder. Override
# FRAMES_DIR / FPS / OUT to point at your own frames.
FRAMES_DIR ?= samples/frames
FPS        ?= 24
SECS       ?= 1
OUT        ?= output.avi
example-sd-encode:
	go run ./examples/sd-encode -i $(FRAMES_DIR) -fps $(FPS) -secs $(SECS) -o $(OUT)
