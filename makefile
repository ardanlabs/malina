# Get the absolute path of the current Makefile.
MAKEFILE_PATH := $(realpath $(lastword $(MAKEFILE_LIST)))
MAKEFILE_DIR  := $(dir $(MAKEFILE_PATH))
MALINA_LIB    ?= $(MAKEFILE_DIR)lib
MODELS_DIR    ?= $(HOME)/models

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
#   make download-stable-diffusion.cpp VERSION=master-656-0e4ee04
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

# make test runs all package tests. The pkg/sd tests require MALINA_LIB and
# MALINA_TEST_MODEL to be set; without them they skip.
test-only:
	export MALINA_LIB=$(MALINA_LIB) && \
	go test -count=1 ./...

test: test-only lint vuln-check diff

tidy:
	go mod tidy

deps-upgrade:
	go get -u -v ./...
	go mod tidy

# -----------------------------------------------------------------------------
# Bundle downloads. Each target invokes `malina model pull` which downloads
# every file in the named bundle into $(MODELS_DIR)/<bundle>/ along with a
# manifest.json. The flux bundle is license-gated; set HF_TOKEN first.

pull-sd-1.5:
	go run . model pull -y -o $(MODELS_DIR) sd-1.5

pull-sdxl-base-1.0:
	go run . model pull -y -o $(MODELS_DIR) sdxl-base-1.0

pull-flux2-klein-9b:
	go run . model pull -y -o $(MODELS_DIR) flux2-klein-9b

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
