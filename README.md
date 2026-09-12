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

| stable-diffusion.cpp | malina      |
| -------------------- | ----------- |
| master-859-7f410a3   | 1.1.x       |
| master-849-d04e895   | 1.0.10      |
| master-846-d8fb10c   | 1.0.9       |

The FFI binding includes image and native video generation, upscaling, ADetailer, ControlNet hot-swap, conversion, Canny preprocessing, cancellation, preview/backend callbacks, device and loaded-model identification, and every generation parameter in the target header. Pure-Go PNG/JPEG decode + Motion-JPEG AVI mux, the CLI (`install`, `system`, `info`, `model list|pull`), and runnable examples for the generation APIs have also landed. Kronk integration (an OpenAI-compatible `POST /v1/images/generations` endpoint) lives in the [kronk](https://github.com/ardanlabs/kronk) repo.

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
$ malina install
$ malina system
```

Malina verifies an upstream asset's GitHub SHA-256 digest before extracting
it. `DefaultSDVersion` includes the SHA-256 of the embedded trusted manifest,
which authenticates the asset ID, size, archive digest, and every installed
shared library for Malina's pinned stable-diffusion.cpp release. Installs
created before this verification metadata was introduced must be refreshed
once with `malina install --upgrade`.

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

The architecture of malina mirrors bucky and yzma file-for-file so anyone who knows either can drop straight in. There is no CGo: every C call goes through [purego](https://github.com/ebitengine/purego) + [JupiterRider/ffi](https://github.com/JupiterRider/ffi).

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/         malina CLI (install, system, model, sd)       │
├─────────────────────────────────────────────────────────────┤
│  pkg/sd       safe stable-diffusion.h FFI surface           │
│               (image/video generation, upscaler, callbacks, │
│                conversion, image I/O, log, system)          │
│  pkg/download go-getter-driven release-archive resolver +   │
│               curated generation and tool-model catalog     │
│  pkg/loader   MALINA_LIB-aware purego library loader        │
│  pkg/utils    cross-platform Go ↔ C string helpers          │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
            libstable-diffusion.{dylib|so|dll}
              (stable-diffusion.cpp master-859)
```

### FFI API coverage

`pkg/sd` prepares 60 of the 63 functions exported by the pinned
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

The `master-846-d8fb10c` ABI replaces `ContextParams.StreamLayers` with
`ContextParams.DisablePrefetch`, adds `ContextParams.DisableSegmentedCompute`,
and inserts the `LogVerbose` level. Code setting `StreamLayers` must migrate to
the new controls; the old field's storage now has the opposite meaning.

## Models

Malina works with any model stable-diffusion.cpp accepts: `.safetensors` and `.gguf` checkpoints for SD 1.x / SD 2.x / SDXL, plus the multi-file FLUX and SD3 layouts (separate diffusion model + VAE + text-encoder files). Recommended hosts are [stable-diffusion-v1-5/stable-diffusion-v1-5](https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5) and the GGUF quants under [city96](https://huggingface.co/city96).

The target library also supports SenseNova U1.5 directories. It is not in the curated catalog because upstream requires a complete repository directory with eight weight shards plus tokenizer and configuration files, while catalog roles currently resolve to individual files.

Malina ships a curated catalog so you can pull complete generation workflows
and standalone tool models instead of pasting URLs:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model pull controlnet-canny-sd1.5
$ malina model pull realesrgan-x4-anime
$ malina model pull adetailer-face-yolov8n
$ malina model pull animatediff-sd1.5
$ malina model pull sdxl-base-1.0
$ malina model pull flux2-klein-4b   # license-gated; export HF_TOKEN first
$ malina model pull flux2-klein-9b   # license-gated; export HF_TOKEN first
```

Each bundle drops every required file into `$HOME/.kronk/malina-models/<bundle>/` along with a `manifest.json` the examples use to resolve paths.

## Support

Malina uses the prebuilt stable-diffusion.cpp release artifacts from [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) directly — there is no companion builder repo. The pinned version is captured in [`pkg/download/install.go`](pkg/download/install.go) as `DefaultSDVersion`; its trusted release metadata, archive hashes, installed-file hashes, and symlink targets are captured in [`pkg/download/library_manifest.json`](pkg/download/library_manifest.json). A dynamically selected release such as `-v latest` still receives archive-level verification from GitHub. Malina saves that GitHub Release API response and the resulting extracted-file hashes beside the installed libraries, so later offline checks can detect changed metadata or local corruption. Only the pinned release has an authenticated post-install baseline embedded in the Malina binary.

| OS      | CPU   | Backend       | Upstream artifact pattern                                      |
| ------- | ----- | ------------- | -------------------------------------------------------------- |
| macOS   | arm64 | Metal         | `sd-master-…-bin-Darwin-macOS-…-arm64.zip`                     |
| Windows | amd64 | CPU           | `sd-master-…-bin-win-cpu-x64.zip`                              |
| Windows | amd64 | CUDA 12       | `sd-master-…-bin-win-cuda12-x64.zip` plus `cudart-…-cu12-…zip` |
| Windows | amd64 | Vulkan        | `sd-master-…-bin-win-vulkan-x64.zip`                           |
| Windows | amd64 | ROCm          | `sd-master-…-bin-win-rocm-…-x64.zip`                           |
| Linux   | amd64 | CPU           | `sd-master-…-bin-Linux-Ubuntu-…-x86_64.zip`                    |
| Linux   | amd64 | Vulkan / ROCm | CPU pattern plus `-vulkan.zip` or `-rocm-….zip`                |

Whenever there is a new release of stable-diffusion.cpp, the FFI struct mirrors in `pkg/sd` and the version constant in `pkg/download` may need a refresh. Generate and review the new trusted manifest with `make generate-library-manifest VERSION=master-N-shortsha`, bump `DefaultSDVersion`, regenerate any struct-size assertions in `pkg/sd/*_test.go`, and let CI verify. Manifest generation downloads and hashes every supported release asset, so it can take several minutes and several gigabytes of transfer.

The `malina_model_tests` suite exercises SD 1.5, SDXL, and the advanced APIs
against real models configured by the Makefile. The license-gated FLUX.2 Klein
bundles remain available as opt-in catalog entries, but are deliberately not
used by tests, benchmarks, examples, or `make download-models`.

| Environment variable         | Catalog bundle / functional test                     |
| ---------------------------- | ---------------------------------------------------- |
| `MALINA_CONTROLNET_TEST_DIR` | `controlnet-canny-sd1.5` controlled image generation |
| `MALINA_UPSCALER_TEST_DIR`   | `realesrgan-x4-anime` 4× image upscaling             |
| `MALINA_ADETAILER_TEST_DIR`  | `adetailer-face-yolov8n` face detection/refinement   |
| `MALINA_VIDEO_TEST_DIR`      | `animatediff-sd1.5` multi-frame video generation     |

Each advanced test skips when its fixture variable is unset and fails when a
configured fixture is missing. GitHub Actions downloads and caches each
advanced bundle before running its corresponding native functional test on
Linux.

## API Examples

There are examples in the [examples/](./examples) directory. They always load
libraries and models from Malina's default locations under `~/.kronk`; no
library or model path configuration is required or accepted. Before loading,
each example verifies that the installed libraries exactly match the
authenticated `download.DefaultSDVersion` pin:

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

[UPSCALE](examples/upscale/main.go) — enlarge an image 4× with the compact Real-ESRGAN anime model.

```shell
$ malina model pull realesrgan-x4-anime
$ make example-upscale
```

[ADETAILER](examples/adetailer/main.go) — detect faces with YOLOv8n and refine each detected region with Stable Diffusion inpainting.

```shell
$ malina model pull adetailer-face-yolov8n
$ make example-adetailer
```

[ANIMATEDIFF](examples/animatediff/main.go) — generate a temporally conditioned sequence with an AnimateDiff motion module and save it as an AVI.

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
