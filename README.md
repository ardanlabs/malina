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
| master-1017-be48649  | 1.1.x (≥1.1.9) |
| master-975-95b3090   | 1.1.8       |
| master-969-8c442fb   | 1.1.6-1.1.7 |

The FFI binding includes image and native video generation, upscaling, ADetailer, ControlNet hot-swap, conversion, Canny preprocessing, cancellation, preview/backend callbacks, device and loaded-model identification, and every generation parameter in the target header. Pure-Go PNG/JPEG decode + Motion-JPEG AVI mux, the CLI (`install`, `system`, `info`, `model list|pull`), and runnable examples for the generation APIs have also landed. Kronk integration (an OpenAI-compatible `POST /v1/images/generations` endpoint) lives in the [kronk](https://github.com/ardanlabs/kronk) repo.

The `master-1017-be48649` upgrade changes the native result ABI for image,
video, upscaler, and ADetailer operations. Malina 1.1.9 requires this new ABI;
1.1.8 and earlier bindings must not load it. Upgrade Malina and reinstall its
pinned library together. Existing Go generation signatures are preserved.
`SDImage.GenerationInfo` exposes each image's resolved `sdcpp.generation/v1`
JSON (including its actual batch seed), and `GenerateVideoWithInfo` returns
frames, audio, effective FPS, and sequence metadata as a `VideoGenerationResult`.
Pixels, audio, and metadata are copied before the matched native result
destructor runs. PNG/JPEG loading and saving remain pixel-only operations.
`VideoGenParams.RefImageArgs`, the FLUX.3/H3 AV/Ming prediction values, and
`AddRPCServers` (before `ListDevices`) expose the other header additions.
Batched CFG remains disabled for Metal; other backends keep their existing
behavior.

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
              (stable-diffusion.cpp master-1017)
```

## Models

Malina works with any model stable-diffusion.cpp accepts: `.safetensors` and `.gguf` checkpoints for SD 1.x / SD 2.x / SDXL, plus the multi-file FLUX and SD3 layouts (separate diffusion model + VAE + text-encoder files). Recommended hosts are [stable-diffusion-v1-5/stable-diffusion-v1-5](https://huggingface.co/stable-diffusion-v1-5/stable-diffusion-v1-5) and the GGUF quants under [city96](https://huggingface.co/city96).

The target library also supports SenseNova U1.5 directories. It is not in the curated catalog because upstream requires a complete repository directory with eight weight shards plus tokenizer and configuration files, while catalog roles currently resolve to individual files.

The target library adds Wan2.2 S2V 14B audio-conditioned video support and
requires external tokenizer JSON files for PiD and Lens. The catalog includes
the complete Wan S2V video/audio workflow. PiD is not curated because its
official weights are non-commercial, and Lens requires multiple large model
components plus an external tokenizer.

The `master-908-88411ef` release also supports Qwen Image 2.1 and LLaDA-Image.
Qwen Image 2.1 is not curated because its license restricts use to
non-commercial research and evaluation. The catalog includes the Apache-2.0
LLaDA-Image-Turbo model for 4-step text-to-image generation and image editing.

The `master-929-3f8527a` release adds PixArt-α/Σ and Ming-Image Design.
Neither is curated yet: the official PixArt distribution is a multi-file
transformer/text-encoder/VAE layout and upstream does not yet apply PixArt-α's
resolution micro-conditioning, while Ming-Image requires a 6B diffusion model,
a BF16 Ling-mini-2.0 text encoder, a VAE, and an external tokenizer. Those
official workflows are impractically large for the curated catalog without
stable compact variants, though callers can supply their files directly.

The `master-969-8c442fb` release adds Iris-3B, Z-Image L2P, FLUX.3 Image
Klein/Klein Base, and MoSR upscalers. These are not curated yet:
Iris's official F32 diffusion weights are approximately 12 GB plus a Qwen3-VL
encoder, and L2P's diffusion download is approximately 18.6 GB plus a Qwen3
encoder. Both pixel-space workflows need no VAE but lack verified compact
bundles. Official FLUX.3 weights are gated. The linked 17 MB MoSR checkpoint
[`4xNomos2_hq_mosr.pth`](https://github.com/Phhofm/models/releases/tag/4xNomos2_hq_mosr)
fits the upscaler role, but its weight release states the ambiguous license
`CC-BY-0.4`; curation is deferred until the weight license is clarified.
Existing ungated bundles remain the recommended validation fixtures.

The `master-1017-be48649` release adds Fibo 2.0 Dev/Edit Turbo support,
Z-Image ControlNet, and compact GGUF links for the already-supported MOSS-Image
and FLUX.3 families. The catalog now includes **MOSS-Image-SFT**: ungated
Apache-2.0 Q4_0 diffusion, Qwen3.5-9B Q4_K_M LLM, full FLUX.2 VAE, and the
mandatory external tokenizer (four immutable-revision files, about 9.4 GB).
Map its roles to `DiffusionModelPath`, `LLMPath`, `VAEPath`, and `Tokenizer`;
see the [target MOSS-Image guide](https://github.com/leejet/stable-diffusion.cpp/blob/master-1017-be48649/docs/moss_image.md).
MiniMax-H3 LoRAs continue using existing LoRA parameters.

**Z-Image-Turbo-ControlNet-Union** combines the existing Z-Image bundle with
the upstream-tested 2601 Union 2.1 ControlNet weights (about 13.4 GB total,
Apache-2.0). Supply a control map through `ControlImage` and the control model
through `ControlNetPath`; disable VAE encode tiling as recommended upstream.
Pose/depth preprocessing and simultaneous reference-image editing are not
provided by this bundle. These two new bundles are opt-in; their pinned file
availability and role coverage are checked, but they are not local inference
validation fixtures.

Fibo 2.0 is not curated: the original weights are gated, and the ungated
mirror retains CC BY-NC 4.0 restrictions (commercial use requires a separate
agreement). FLUX.3 remains deferred because its gated custom
`flux-kommunity-license` has not been cleared for this catalog; an ungated
GGUF mirror does not remove that requirement. Neither is used in validation.

The catalog also includes **Z-Image-Turbo**: an ungated Apache-2.0 bundle
with Q4_K diffusion weights, a Q4_K_M Qwen3-4B-Instruct-2507 text encoder,
and a VAE (three revision-pinned files, approximately 6.7 GB total).
Use 8 steps and CFG 1 for its distilled text-to-image workflow. Resolve the
bundle's diffusion, LLM, and VAE roles to `ContextParams.DiffusionModelPath`,
`LLMPath`, and `VAEPath`; no additional tokenizer or vision projector is needed.

Malina ships a curated catalog so you can pull complete generation workflows
and standalone tool models instead of pasting URLs:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model pull controlnet-canny-sd1.5
$ malina model pull realesrgan-x4-anime
$ malina model pull adetailer-face-yolov8n
$ malina model pull animatediff-sd1.5
$ malina model pull wan2.2-s2v-14b # four files; approximately 18.4 GB
$ malina model pull sdxl-base-1.0
$ malina model pull llada-image-turbo # five files; approximately 20.2 GB
$ malina model pull z-image-turbo   # three files; approximately 6.7 GB; ungated
$ malina model pull moss-image-sft  # four files; approximately 9.4 GB; ungated
$ malina model pull z-image-turbo-controlnet-union # four files; approximately 13.4 GB
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
bundles, the approximately 20.2 GB LLaDA-Image-Turbo bundle, and the
approximately 18.4 GB Wan2.2 S2V bundle remain available as opt-in catalog
entries, but are deliberately not used by tests, benchmarks, examples, or
`make download-models`.

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
