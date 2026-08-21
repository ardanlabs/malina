Copyright 2025-2026 Ardan Labs

hello@ardanlabs.com

# Malina

This project lets you use Go for hardware accelerated local image and video generation with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp) directly integrated into your applications. Malina maps the safe public `stable-diffusion.h` API plus pure-Go PNG/JPEG I/O and Motion-JPEG AVI muxing.

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
| master-827-97d2990   | 1.0.x (default in 1.0.4+) |
| master-820-de298c2   | 1.0.x  |
| master-813-bfbef5b   | 1.0.1  |
| master-669-2d40a8b   | 0.1.x  |

The FFI binding includes image and native video generation, upscaling, ADetailer, ControlNet hot-swap, conversion, Canny preprocessing, cancellation, preview/backend callbacks, device listing, and every generation parameter in the target header. Pure-Go PNG/JPEG decode + Motion-JPEG AVI mux, the CLI (`install`, `system`, `info`, `model list|pull`), and examples (`hello`, `system`, `sd-encode`, `flux2`) have also landed. Kronk integration (an OpenAI-compatible `POST /v1/images/generations` endpoint) lives in the [kronk](https://github.com/ardanlabs/kronk) repo.

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

Then fetch the stable-diffusion.cpp shared library bundle (dylib on macOS, DLLs on Windows, and `.so` files on Linux, all distributed in ZIP archives from the upstream [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) releases):

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
│  pkg/sd       safe stable-diffusion.h FFI surface           │
│               (image/video generation, upscaler, callbacks, │
│                conversion, image I/O, log, system)          │
│  pkg/download go-getter-driven release-archive resolver +   │
│               bundle catalog (sd-1.5, sdxl, flux2)          │
│  pkg/loader   MALINA_LIB-aware purego library loader        │
│  pkg/utils    cross-platform Go ↔ C string helpers          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
            libstable-diffusion.{dylib|so|dll}
              (stable-diffusion.cpp master-827)
```

### FFI API coverage

`pkg/sd` prepares 59 of the 62 functions exported by the pinned
`stable-diffusion.h`. This includes all functions with a safe ownership
contract. Newer optional symbols are resolved at load time so an explicitly
requested older compatible library can still load; calling an unavailable
feature returns `sd.ErrUnsupportedAPI`.

The only functions intentionally not called are `sd_ctx_params_to_str`,
`sd_sample_params_to_str`, and `sd_img_gen_params_to_str`. Each returns a
newly allocated `char *`, but upstream provides no matching public deallocator.
Freeing those pointers from Go would be unsafe across Windows CRT boundaries,
and not freeing them would leak. Malina will bind them when upstream exposes a
matched free API. The exported `sample_method_to_str` and `scheduler_to_str`
data arrays are represented by the safe name and parse functions instead of
directly exposing C global memory.

## Models

Malina works with any model stable-diffusion.cpp accepts: `.safetensors` and `.gguf` checkpoints for SD 1.x / SD 2.x / SDXL, plus the multi-file FLUX and SD3 layouts (separate diffusion model + VAE + text-encoder files). Recommended hosts are [stable-diffusion-v1-5/stable-diffusion-v1-5](https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5) and the GGUF quants under [city96](https://huggingface.co/city96).

Malina ships a small bundled catalog so you can `malina model pull sd-1.5` instead of pasting URLs:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model pull sdxl-base-1.0
$ malina model pull flux2-klein-4b   # license-gated; export HF_TOKEN first
$ malina model pull flux2-klein-9b   # license-gated; export HF_TOKEN first
```

Each bundle drops every required file into `$HOME/models/<bundle>/` along with a `manifest.json` the examples use to resolve paths.

## Support

Malina uses the prebuilt stable-diffusion.cpp release artifacts from [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) directly — there is no companion builder repo. The pinned version is captured in [`pkg/download/install.go`](pkg/download/install.go) as `DefaultSDVersion`.

| OS      | CPU   | Backend           | Upstream artifact pattern                                      |
| ------- | ----- | ----------------- | -------------------------------------------------------------- |
| macOS   | arm64 | Metal             | `sd-master-…-bin-Darwin-macOS-…-arm64.zip`                     |
| Windows | amd64 | CPU               | `sd-master-…-bin-win-cpu-x64.zip`                              |
| Windows | amd64 | CUDA 12           | `sd-master-…-bin-win-cuda12-x64.zip` plus `cudart-…-cu12-…zip` |
| Windows | amd64 | Vulkan            | `sd-master-…-bin-win-vulkan-x64.zip`                           |
| Windows | amd64 | ROCm              | `sd-master-…-bin-win-rocm-…-x64.zip`                           |
| Linux   | amd64 | CPU               | `sd-master-…-bin-Linux-Ubuntu-…-x86_64.zip`                    |
| Linux   | amd64 | Vulkan / ROCm     | CPU pattern plus `-vulkan.zip` or `-rocm-….zip`                |

Whenever there is a new release of stable-diffusion.cpp, the FFI struct mirrors in `pkg/sd` and the version constant in `pkg/download` may need a refresh. Bump `DefaultSDVersion`, regenerate any struct-size assertions in `pkg/sd/*_test.go`, and let CI verify.

The `malina_model_tests` suite exercises the standard SD 1.5, SDXL, and
FLUX.2 fixtures configured by the Makefile. Additional wrapped APIs have
fixture-gated smoke tests; set the applicable model path before `make test`:

| Environment variable | Smoke test |
| -------------------- | ---------- |
| `MALINA_CONTROLNET_TEST_MODEL` | ControlNet load, query, and unload |
| `MALINA_UPSCALER_TEST_MODEL` | ESRGAN context, factor query, and upscale |
| `MALINA_ADETAILER_TEST_MODEL` | Detector context and ADetailer pass |
| `MALINA_VIDEO_TEST_MODEL` | Native video frame and audio generation |

Each advanced test reports the exact missing variable and skips when its
fixture is unavailable.

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

[CONCURRENT](examples/concurrent/main.go) — compare serial generation with concurrent generation on independent native contexts. Contexts cannot be shared concurrently, and each independent context loads another copy of the model weights.

```shell
$ make example-concurrent
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
