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

| stable-diffusion.cpp | malina         |
| -------------------- | -------------- |
| master-1052-f5df5f8  | 1.2.x          |
| master-1017-be48649  | 1.1.x (≥1.1.9) |
| master-975-95b3090   | 1.1.8          |

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
The Go API supports image and video generation, image editing, ControlNet,
upscaling, and face refinement. See the [API reference](https://pkg.go.dev/github.com/ardanlabs/malina/pkg/sd)
for integration details.

## Models

Start with a curated bundle: Malina downloads the files needed for each
workflow and keeps them together. Download sizes below are approximate and
do not represent runtime memory requirements.

| Bundle | Use | Download |
| ------ | --- | -------: |
| `sd-1.5` | General-purpose image generation; a good starting point | 4.3 GB |
| `sdxl-base-1.0` | Higher-quality image generation with Stable Diffusion XL | 6.9 GB |
| `z-image-turbo` | Fast text-to-image generation | 6.7 GB |
| `moss-image-sft` | Text-to-image generation with MOSS-Image | 9.4 GB |
| `llada-image-turbo` | Text-to-image generation and image editing | 20.2 GB |
| `flux2-klein-4b` | Compact FLUX image generation; license-gated | 5.3 GB |
| `flux2-klein-9b` | Larger FLUX image generation; license-gated | 16 GB |
| `controlnet-canny-sd1.5` | Guide image generation with Canny edges | 2.3 GB |
| `z-image-turbo-controlnet-union` | Guide Z-Image generation with supplied control maps | 13.4 GB |
| `realesrgan-x4-anime` | Enlarge anime-style images 4× | 18 MB |
| `adetailer-face-yolov8n` | Detect and refine faces | 1.6 GB |
| `animatediff-sd1.5` | Generate video with AnimateDiff | 2.4 GB |
| `wan2.2-s2v-14b` | Audio-conditioned video generation | 18.4 GB |

List the catalog and download a bundle:

```shell
$ malina model list
$ malina model pull sd-1.5
```

Bundles are stored under `~/.kronk/malina-models/<bundle>/`. Review the
license shown by `malina model list` before use. For license-gated bundles,
accept the upstream Hugging Face terms and set `HF_TOKEN` before downloading.
The FLUX Klein bundles carry non-commercial license terms.

You can also supply your own `.safetensors` or `.gguf` models and their
required components through the Go API. Supported families include Stable
Diffusion, SDXL, SD3, and FLUX, among others supported by the compatible
stable-diffusion.cpp library. See the [upstream model guides](https://github.com/leejet/stable-diffusion.cpp/tree/master/docs)
for requirements beyond the curated catalog.

## Support

Malina installs prebuilt libraries from [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases)
for these platforms:

| OS | CPU | Backends |
| --- | --- | -------- |
| macOS | arm64 | Metal |
| Windows | amd64 | CPU, CUDA, Vulkan, ROCm |
| Linux | amd64 | CPU, Vulkan, ROCm |

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
