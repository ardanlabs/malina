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

$ malina install -u
$ malina model pull sd-1.5
$ go run ./examples/hello "a lovely cat"
```

## Project Status

[![Go Reference](https://pkg.go.dev/badge/github.com/ardanlabs/malina.svg)](https://pkg.go.dev/github.com/ardanlabs/malina)
[![go.mod Go version](https://img.shields.io/github/go-mod/go-version/ardanlabs/malina)](https://github.com/ardanlabs/malina)
[![stable-diffusion.cpp Release](https://img.shields.io/github/v/release/leejet/stable-diffusion.cpp?label=stable-diffusion.cpp)](https://github.com/leejet/stable-diffusion.cpp/releases)

[![Linux](https://github.com/ardanlabs/malina/actions/workflows/linux.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/linux.yml)
[![Windows](https://github.com/ardanlabs/malina/actions/workflows/windows.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/windows.yml)

Sometimes there are breaking changes to stable-diffusion.cpp that require an update to malina. Here are the known compatible versions:

| stable-diffusion.cpp | malina      | Linux prebuilt requirements                                        |
| -------------------- | ----------- | ------------------------------------------------------------------ |
| master-1122-3b12781  | 1.2.4-1.2.5 | CPU/Vulkan: Ubuntu 26.04 (glibc ≥2.43); ROCm archive: Ubuntu 24.04 |
| master-1106-532d2c1  | 1.2.3       | CPU/Vulkan: Ubuntu 26.04 (glibc ≥2.43); ROCm archive: Ubuntu 24.04 |
| master-1088-32d516c  | 1.2.2       | CPU/Vulkan: Ubuntu 26.04 (glibc ≥2.43); ROCm archive: Ubuntu 24.04 |

Malina 1.2.5 requires the DGL-backed opaque-options/tensor/result C API.
Its native library is not interchangeable with 1.2.3 or earlier. Upgrade
Malina and its verified libraries together:

```shell
$ go install github.com/ardanlabs/malina@v1.2.4
$ malina install -u -lib ./lib
```

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

Then fetch the compatible stable-diffusion.cpp libraries:

```shell
$ malina install
$ malina system
```

Malina verifies the downloaded libraries automatically. Use `malina install -u`
to refresh them after upgrading Malina.

And pull a model bundle from the bundled catalog:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model info -m ~/.kronk/malina-models/sd-1.5/v1-5-pruned-emaonly.safetensors
```

## Issues/Features

Here is the existing [Issues/Features](https://github.com/ardanlabs/malina/issues) for the project and the things being worked on or things that would be nice to have.

If you are interested in helping in any way, please send an email to [Bill Kennedy](mailto:bill@ardanlabs.com).

## Architecture

Malina calls stable-diffusion.cpp through [purego](https://github.com/ebitengine/purego)
and [JupiterRider/ffi](https://github.com/JupiterRider/ffi), without CGo.
Native options copy strings and arrays and retain input tensors. Malina copies
result pixels, audio, and generation metadata into Go-owned values before
releasing the native result with its matching library allocator.
The Go API supports image and video generation, image editing, ControlNet,
upscaling, and explicit DGL workflows for face refinement and AnimateDiff.
See the [API reference](https://pkg.go.dev/github.com/ardanlabs/malina/pkg/sd)
for integration details.

## Models

Start with a curated bundle: Malina downloads the files needed for each
workflow and keeps them together. Download sizes below are approximate and
do not represent runtime memory requirements.

| Bundle                           | Use                                                                        | Download |
| -------------------------------- | -------------------------------------------------------------------------- | -------: |
| `sd-1.5`                         | General-purpose image generation; a good starting point                    |   4.3 GB |
| `sdxl-base-1.0`                  | Higher-quality image generation with Stable Diffusion XL                   |   6.9 GB |
| `controlnet-union-sdxl`          | SDXL base with multi-task ControlNet Union; set `ImgGenParams.ControlType` |   9.5 GB |
| `z-image-turbo`                  | Fast text-to-image generation                                              |   6.7 GB |
| `moss-image-sft`                 | Text-to-image generation with MOSS-Image                                   |   9.4 GB |
| `moss-image-edit-hq`             | Instruction-based MOSS SFT image editing with armV5 LoRA and vision        |  10.8 GB |
| `llada-image-turbo`              | Text-to-image generation and image editing                                 |  20.2 GB |
| `qwen-image-2.1`                 | Generation and reference-image editing; research/evaluation license        |  11.2 GB |
| `flux2-klein-base-4b`            | Undistilled FLUX generation and reference-image editing; Apache-2.0        |   5.3 GB |
| `flux2-klein-4b`                 | Compact FLUX image generation; license-gated                               |   5.3 GB |
| `flux2-klein-9b`                 | Larger FLUX image generation; license-gated                                |    16 GB |
| `controlnet-canny-sd1.5`         | Guide image generation with Canny edges                                    |   2.3 GB |
| `z-image-turbo-controlnet-union` | Guide Z-Image generation with supplied control maps                        |  13.4 GB |
| `realesrgan-x4-anime`            | Enlarge anime-style images 4×                                              |    18 MB |
| `adetailer-face-yolov8n`         | Detect and refine faces                                                    |   1.6 GB |
| `animatediff-sd1.5`              | Generate video with AnimateDiff                                            |   2.4 GB |
| `wan2.2-s2v-14b`                 | Audio-conditioned video generation                                         |  18.4 GB |

List the catalog and download a bundle:

```shell
$ malina model list
$ malina model pull sd-1.5
```

Bundles are stored under `~/.kronk/malina-models/<bundle>/`. Review the
license shown by `malina model list` before use. For license-gated bundles,
accept the upstream Hugging Face terms and set `HF_TOKEN` before downloading.
Download gating and license restrictions are separate: the new
`flux2-klein-base-4b` and `qwen-image-2.1` bundles are both ungated, but only
the former is Apache-2.0. Qwen Image 2.1's [Qwen Research license](https://huggingface.co/Qwen/Qwen-Image-2.1/blob/main/LICENSE)
permits research/evaluation only; commercial use requires a separate license.
The existing `flux2-klein-9b` model carries non-commercial terms.

Both new bundles pin every component to an immutable Hugging Face revision
and are opt-in (not included in `make download-models`):

```shell
$ malina model pull flux2-klein-base-4b
$ malina model pull qwen-image-2.1
```

Load each manifest's `diffusion`, `llm`, and `vae` files into the matching
`sd.ContextParams` paths. For Qwen editing, also set `LLMVisionPath` from
`llm_vision`; the bundle includes the vision projector. Pass reference images
through `sd.ImgGenParams.RefImages` rather than `InitImage` for instruction-based
editing. Start with 20 steps and CFG 4 for Klein Base (not the distilled
model's 4-step settings), or 20 Euler steps and CFG 6 for Qwen Image 2.1.
Qwen output dimensions must be divisible by 32, and its dedicated 2.1 VAE
cannot be replaced with the older Qwen Image VAE. These bundles use the existing
generation API; they do not add new embedded DGL workflows or example programs.

You can also supply your own `.safetensors` or `.gguf` models and their
required components through the Go API. Supported families include Stable
Diffusion, SDXL, SD3, and FLUX, among others supported by the compatible
stable-diffusion.cpp library. See the [upstream model guides](https://github.com/leejet/stable-diffusion.cpp/tree/master/docs)
for requirements beyond the curated catalog.

## Support

Malina installs prebuilt libraries from [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases)
for these platforms:

| OS      | CPU   | Backends                |
| ------- | ----- | ----------------------- |
| macOS   | arm64 | Metal                   |
| Windows | amd64 | CPU, CUDA, Vulkan, ROCm |
| Linux   | amd64 | CPU, Vulkan, ROCm       |

Use the compatible library listed in the grid above. The installer selects
the default for your Malina installation; you do not need to choose an
upstream archive manually.

## API Examples

There are examples in the [examples/](./examples) directory. They load
verified libraries and models from Malina's default locations under
`~/.kronk`. Install the libraries and the required bundle before running:

```shell
$ malina install -u
$ malina model pull sd-1.5
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

[CONTROLNET](examples/controlnet/main.go) — derive Canny edges from an input image and use them to constrain the generated image's composition.

```shell
$ malina model pull controlnet-canny-sd1.5
$ make example-controlnet
```

[MOSS IMAGE EDIT HQ](examples/moss-image-edit/main.go) — edit a reference PNG/JPEG
with an instruction. The bundle includes all six pinned files (Apache-2.0,
ungated); no external DGL file or LoRA prompt tag is needed.

```shell
$ malina model pull moss-image-edit-hq
$ make example-moss-image-edit
$ go run ./examples/moss-image-edit -in photo.png -out edited.png -prompt "Make the sky orange."
```

The Go API is `sd.EditMOSSImage(ctx, image, params)`, starting with
`sd.MOSSImageEditParamsInit()`. Defaults are 512×512, 20 Euler steps, CFG 3,
LoRA weight 1, and a random seed. Supply the six component paths from the bundle
manifest, including `download.RoleLora` and `download.RoleLLMVision`. Output
dimensions must be positive multiples of 16. The reference latent is resized
to a maximum 1024-pixel long edge and aligned to multiples of 16; input pixels
are not changed. This wrapper accepts one reference and retains replayable
source in `GenerationInfo`. It supports the same context-local progress and
cooperative cancellation as the other bundled workflows. Download size is not runtime RAM.

The large bundle is opt-in, not part of `make download-models`. To run its
model-backed regression test after downloading:

```shell
$ MALINA_LIB="$PWD/lib" MALINA_MOSS_EDIT_TEST_DIR="$HOME/.kronk/malina-models/moss-image-edit-hq" go test -count=1 -tags=malina_model_tests ./pkg/sd -run '^TestEditMOSSImage$'
```

[UPSCALE](examples/upscale/main.go) — enlarge an image 4× with the compact Real-ESRGAN anime model.

```shell
$ malina model pull realesrgan-x4-anime
$ make example-upscale
```

[ADETAILER](examples/adetailer/main.go) — detect faces with YOLOv8n and refine each detected region with an explicit DGL inpainting program.

```shell
$ malina model pull adetailer-face-yolov8n
$ make example-adetailer
```

[ANIMATEDIFF](examples/animatediff/main.go) — run an explicit DGL program with an AnimateDiff motion module and save its temporally conditioned frames as an AVI.

```shell
$ malina model pull animatediff-sd1.5
$ make example-animatediff
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
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	prompt := "a lovely cat"
	if len(os.Args) >= 2 {
		prompt = os.Args[1]
	}

	libPath := download.DefaultLibrariesDir()
	bundleDir := filepath.Join(download.DefaultModelsDir(), "sd-1.5")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load model bundle from %s: %v (did you run `malina model pull sd-1.5`?)", bundleDir, err)
	}
	modelPath := manifest.Files[string(download.RoleModel)]

	if err := download.VerifyDefaultInstall(context.Background(), libPath); err != nil {
		log.Fatalf("verify default libraries: %v (did you run `malina install -u`?)", err)
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
loading model from /Users/bill/.kronk/malina-models/sd-1.5/v1-5-pruned-emaonly.safetensors ...
generating image for prompt: a lovely cat
wrote hello.png (512x512, 3 channels) in 6.842s
```

## License

Apache-2.0 — see [LICENSE](./LICENSE).
