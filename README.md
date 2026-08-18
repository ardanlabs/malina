Copyright 2025-2026 Ardan Labs

hello@ardanlabs.com

# Malina

This project lets you use Go for hardware accelerated local image generation with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) directly integrated into your applications. Malina provides a high-level API that mirrors `stable-diffusion.h` 1-to-1 plus pure-Go PNG/JPEG I/O and Motion-JPEG AVI muxing so you can hand any prompt to a Stable Diffusion model and get an image (or a short video) back.

Malina is the image-generation sibling of [ardanlabs/bucky](https://github.com/ardanlabs/bucky) (which binds whisper.cpp) and [hybridgroup/yzma](https://github.com/hybridgroup/yzma) (which binds llama.cpp). The end goal is to give [Kronk](https://github.com/ardanlabs/kronk) a native, OpenAI-compatible `POST /v1/images/generations` endpoint without the CGo toolchain.

> Malina is the Russian word for "raspberry" — a small, dense, fast-growing
> fruit. Naming a stable-diffusion binding after a fast little thing that
> sprouts colorful pictures is just good taste.

To install malina, fetch the stable-diffusion.cpp shared libraries, and generate the bundled cat sample:

```shell
$ go install github.com/ardanlabs/malina@latest

$ malina install -lib ./lib
$ export MALINA_LIB=$(pwd)/lib

$ malina model pull sd-1.5
$ export MALINA_TEST_MODEL=$HOME/models/sd-1.5/v1-5-pruned-emaonly.safetensors
$ go run ./examples/hello "a lovely cat"
```

## Project Status

[![Go Reference](https://pkg.go.dev/badge/github.com/ardanlabs/malina.svg)](https://pkg.go.dev/github.com/ardanlabs/malina)
[![Go Report Card](https://goreportcard.com/badge/github.com/ardanlabs/malina?style=flat-square)](https://goreportcard.com/report/github.com/ardanlabs/malina)
[![go.mod Go version](https://img.shields.io/github/go-mod/go-version/ardanlabs/malina)](https://github.com/ardanlabs/malina)
[![stable-diffusion.cpp Release](https://img.shields.io/github/v/release/leejet/stable-diffusion.cpp?label=stable-diffusion.cpp)](https://github.com/leejet/stable-diffusion.cpp/releases)

[![Linux](https://github.com/ardanlabs/malina/actions/workflows/linux.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/linux.yml)
[![Windows](https://github.com/ardanlabs/malina/actions/workflows/windows.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/windows.yml)

Sometimes there are breaking changes to stable-diffusion.cpp that require an update to malina. Here are the known compatible versions:

| stable-diffusion.cpp | malina |
| -------------------- | ------ |
| master-820-de298c2   | 1.0.2  |
| master-813-bfbef5b   | 1.0.1  |
| master-669-2d40a8b   | 0.1.x  |

The core FFI binding (context init, `generate_image`, image I/O, log/progress callbacks, GGML backend introspection), pure-Go PNG/JPEG decode + Motion-JPEG AVI mux, CLI (`install`, `system`, `info`, `model list|pull`), and examples (`hello`, `system`, `sd-encode`, `flux2`) have all landed. Kronk integration (an OpenAI-compatible `POST /v1/images/generations` endpoint) lives in the [kronk](https://github.com/ardanlabs/kronk) repo.

## Owner Information

```
Name:     Bill Kennedy
Company:  Ardan Labs
Title:    Managing Partner
Email:    bill@ardanlabs.com
BlueSky:  https://bsky.app/profile/goinggo.net
LinkedIn: www.linkedin.com/in/william-kennedy-5b318778/
Twitter:  https://x.com/goinggodotnet
```

## Install Malina

The fastest way to install on any supported platform is with Go:

```shell
$ go install github.com/ardanlabs/malina@latest

$ malina --help
```

Then fetch the stable-diffusion.cpp shared library bundle (dylib on darwin, DLLs on windows, `.tar.gz` on linux — all sourced from the upstream [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) releases):

```shell
$ malina install -lib ./lib
$ export MALINA_LIB=$(pwd)/lib
$ malina system
```

And pull a model bundle from the bundled catalog:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model info -m ~/models/sd-1.5/v1-5-pruned-emaonly.safetensors
```

## Issues/Features

Here is the existing [Issues/Features](https://github.com/ardanlabs/malina/issues) for the project and the things being worked on or things that would be nice to have.

If you are interested in helping in any way, please send an email to [Bill Kennedy](mailto:bill@ardanlabs.com).

## Architecture

The architecture of malina mirrors bucky and yzma file-for-file so anyone who knows either can drop straight in. There is no CGo: every C call goes through [purego](https://github.com/ebitengine/purego) + [JupiterRider/ffi](https://github.com/JupiterRider/ffi).

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/         malina CLI (install, system, model, sd)       │
├─────────────────────────────────────────────────────────────┤
│  pkg/sd       1-to-1 mirror of stable-diffusion.h           │
│               (context, gen_params, generate, video,        │
│                image I/O, log, system)                      │
│  pkg/download go-getter-driven release-archive resolver +   │
│               bundle catalog (sd-1.5, sdxl, flux2)          │
│  pkg/loader   MALINA_LIB-aware purego library loader        │
│  pkg/utils    cross-platform Go ↔ C string helpers          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
            libstable-diffusion.{dylib|so|dll}
              (stable-diffusion.cpp master-820)
```

## Models

Malina works with any model stable-diffusion.cpp accepts: `.safetensors` and `.gguf` checkpoints for SD 1.x / SD 2.x / SDXL, plus the multi-file FLUX and SD3 layouts (separate diffusion model + VAE + text-encoder files). Recommended hosts are [stable-diffusion-v1-5/stable-diffusion-v1-5](https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5) and the GGUF quants under [city96](https://huggingface.co/city96).

Malina ships a small bundled catalog so you can `malina model pull sd-1.5` instead of pasting URLs:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model pull sdxl-base-1.0
$ malina model pull flux2-klein-9b   # license-gated; export HF_TOKEN first
```

Each bundle drops every required file into `$HOME/models/<bundle>/` along with a `manifest.json` the examples use to resolve paths.

## Support

Malina uses the prebuilt stable-diffusion.cpp release artifacts from [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) directly — there is no companion builder repo. The pinned version is captured in [`pkg/download/install.go`](pkg/download/install.go) as `DefaultSDVersion`.

| OS      | CPU   | GPU          | Source                                                      |
| ------- | ----- | ------------ | ----------------------------------------------------------- |
| Linux   | amd64 | CPU (avx2)   | `sd-master-…-linux-avx2-x64.tar.gz` (upstream)              |
| macOS   | arm64 | Metal        | `sd-master-…-bin-MacOS-arm64.tar.gz` (upstream)             |
| Windows | amd64 | CPU, CUDA 12 | `sd-master-…-bin-win-avx2-x64.zip` / `-cuda12-…` (upstream) |

Whenever there is a new release of stable-diffusion.cpp, the FFI struct mirrors in `pkg/sd` and the version constant in `pkg/download` may need a refresh. Bump `DefaultSDVersion`, regenerate any struct-size assertions in `pkg/sd/*_test.go`, and let CI verify.

## API Examples

There are examples in the [examples/](./examples) directory. Each one
expects `MALINA_LIB` and (for the model-loading examples) `MALINA_TEST_MODEL`
to be set:

```shell
$ export MALINA_LIB=$(pwd)/lib
$ export MALINA_TEST_MODEL=$HOME/models/sd-1.5/v1-5-pruned-emaonly.safetensors
```

[SYSTEM](examples/system/main.go) — the smallest possible malina program: load libstable-diffusion and print the library version, system info, and GGML backend device count. No model required.

```shell
$ make example-system
```

[HELLO](examples/hello/main.go) — load a stable-diffusion model, generate one image from a text prompt, and save it as `hello.png`.

```shell
$ make example-hello
```

[IMG2IMG](examples/img2img/main.go) — image-to-image: load a source PNG or JPEG, hand it to stable-diffusion as the starting latent, and let the prompt repaint it. The default chain consumes `hello.png` written by the previous example.

```shell
$ make example-hello       # writes hello.png
$ make example-img2img     # writes img2img.png in oil-painting style
```

[FLUX2](examples/flux2/main.go) — multi-file FLUX.2 [klein] 9B pipeline using a quantized diffusion model + VAE + Qwen3 LLM text encoder. The example reads the bundle's `manifest.json` to resolve each file's on-disk path.

```shell
$ export HF_TOKEN=hf_...              # FLUX.2 license must be accepted
$ malina model pull flux2-klein-9b    # one-time
$ make example-flux2
```

[SD-ENCODE](examples/sd-encode/main.go) — mux a directory of PNG / JPEG frames into a Motion-JPEG AVI. No model is loaded; this is the pure-Go encoder built on top of `pkg/sd`'s `SaveAVI` helper.

```shell
$ make example-sd-encode
```

## Sample API Program — Hello Example

```go
// hello is the smallest possible malina example: load a stable-diffusion
// model, generate one image from a text prompt, and save it as PNG.
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	prompt := "a lovely cat"
	if len(os.Args) >= 2 {
		prompt = os.Args[1]
	}

	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		log.Fatal("MALINA_LIB must point to the directory containing libstable-diffusion")
	}

	modelPath := os.Getenv("MALINA_TEST_MODEL")
	if modelPath == "" {
		log.Fatal("MALINA_TEST_MODEL must point to a stable-diffusion model file (.gguf or .safetensors)")
	}

	if err := sd.Load(libPath); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(libPath); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	cparams := sd.ContextParamsInit()
	cparams.ModelPath = modelPath

	fmt.Println("loading model from", modelPath, "...")
	ctx, err := sd.NewContext(cparams)
	if err != nil {
		log.Fatalf("sd.NewContext: %v", err)
	}
	defer sd.FreeContext(ctx)

	params := sd.ImgGenParamsInit()
	params.Prompt = prompt

	fmt.Println("generating image for prompt:", prompt)
	start := time.Now()
	img, err := sd.GenerateImage(ctx, params)
	if err != nil {
		log.Fatalf("sd.GenerateImage: %v", err)
	}
	elapsed := time.Since(start)

	const outPath = "hello.png"
	if err := img.SavePNG(outPath); err != nil {
		log.Fatalf("SavePNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d, %d channels) in %s\n", outPath, img.Width, img.Height, img.Channel, elapsed.Round(time.Millisecond))
}
```

This example produces the following output:

```shell
$ make example-hello
go run ./examples/hello "a lovely cat"
loading model from /Users/bill/models/sd-1.5/v1-5-pruned-emaonly.safetensors ...
generating image for prompt: a lovely cat
wrote hello.png (512x512, 3 channels) in 6.842s
```

## License

Apache-2.0 — see [LICENSE](./LICENSE).
