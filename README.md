Copyright 2025-2026 Ardan Labs

hello@ardanlabs.com

# Malina

This project provides hardware-accelerated local image generation with [stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp). The importable `sdk/malina` package owns model lifecycle, bounded request admission, serialized generation, and PNG output; `sdk/malina/sd` remains an implementation and advanced FFI layer not intended for application code.

Malina is the image-generation sibling of [ardanlabs/bucky](https://github.com/ardanlabs/bucky) (which binds whisper.cpp) and [hybridgroup/yzma](https://github.com/hybridgroup/yzma) (which binds llama.cpp). It owns its OpenAI-compatible `POST /v1/images/generations` endpoint; future [Kronk](https://github.com/ardanlabs/kronk) integration will use Malina over HTTP rather than importing its native bindings in-process.

> Malina is the Russian word for "raspberry" — a small, dense, fast-growing
> fruit. Naming a stable-diffusion binding after a fast little thing that
> sprouts colorful pictures is just good taste.

To install malina, fetch the stable-diffusion.cpp shared libraries, and generate the bundled cat sample:

```shell
$ go install github.com/ardanlabs/malina/cmd/malina@latest

$ malina libs pull -lib ./lib
$ export MALINA_LIB=$(pwd)/lib

$ malina model pull sd-1.5
$ export MALINA_TEST_MODEL=$HOME/models/sd-1.5/v1-5-pruned-emaonly.safetensors
$ go run ./examples/hello "a lovely cat"
```

## Project Status

[![Go Reference](https://pkg.go.dev/badge/github.com/ardanlabs/malina/sdk/malina.svg)](https://pkg.go.dev/github.com/ardanlabs/malina/sdk/malina)
[![Go Report Card](https://goreportcard.com/badge/github.com/ardanlabs/malina?style=flat-square)](https://goreportcard.com/report/github.com/ardanlabs/malina)
[![go.mod Go version](https://img.shields.io/github/go-mod/go-version/ardanlabs/malina)](https://github.com/ardanlabs/malina)
[![stable-diffusion.cpp Release](https://img.shields.io/github/v/release/leejet/stable-diffusion.cpp?label=stable-diffusion.cpp)](https://github.com/leejet/stable-diffusion.cpp/releases)

[![Linux](https://github.com/ardanlabs/malina/actions/workflows/linux.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/linux.yml)
[![Windows](https://github.com/ardanlabs/malina/actions/workflows/windows.yml/badge.svg)](https://github.com/ardanlabs/malina/actions/workflows/windows.yml)

Sometimes there are breaking changes to stable-diffusion.cpp that require an update to malina. Here are the known compatible versions:

| stable-diffusion.cpp | malina |
| -------------------- | ------ |
| master-669-2d40a8b   | 0.1.x  |

The `sdk/malina` SDK, reusable `sdk/malina/model` package, CLI, one-model HTTP server, OpenAI-compatible image endpoint, and embedded management BUI have landed. Kronk integration is a later external HTTP adapter; Malina does not import or run Kronk in-process.

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
$ go install github.com/ardanlabs/malina/cmd/malina@latest

$ malina --help
```

GitHub release archives contain the thin `malina` executable, not the native
stable-diffusion.cpp libraries or a model. Whether installing from a release
archive or with `go install`, fetch a native library bundle separately as
shown below.

Then fetch the stable-diffusion.cpp shared library bundle (dylib on darwin, DLLs on windows, `.tar.gz` on linux — all sourced from the upstream [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) releases):

```shell
$ malina libs pull -lib ./lib
$ export MALINA_LIB=$(pwd)/lib
$ malina system
```

And pull a model bundle from the bundled catalog:

```shell
$ malina model list
$ malina model pull sd-1.5
$ malina model info sd-1.5
```

### Development environment

With Nix and flakes enabled, enter the CPU development shell from the
repository root and build the BUI and CLI:

```shell
$ nix develop ./zarf/nix#cpu
$ (cd cmd/server/api/frontends/bui && npm ci && npm run build)
$ go build ./cmd/malina
```

The repository's pre-commit hook is optional. It rebuilds generated files and
checks whitespace; enable it for this clone with:

```shell
$ git config core.hooksPath .githooks
```

### Linux CPU container

Pushes to `main` and version tags publish the Linux amd64 CPU image to GHCR
after its exact digest passes a vulnerability scan and is signed. It contains
the CLI, embedded BUI, and a checksum-verified copy of Malina's pinned CPU
native-library bundle, but no model. The upstream CPU bundle requires an
AVX2-capable x86-64 host. For example, after an image has been published:

```shell
$ docker pull ghcr.io/ardanlabs/malina:main
$ docker run --rm -p 127.0.0.1:8080:8080 \
    -v "$HOME/models:/models:ro" ghcr.io/ardanlabs/malina:main
```

Open `http://127.0.0.1:8080/admin/` and load a checkpoint under `/models`, or
set `MALINA_MODEL=/models/<checkpoint>` when starting the container. The
initial container supports **Linux amd64 CPU with AVX2 only**; it does not
provide GPU backends, baseline pre-AVX2 x86-64, or arm64 images.

Malina's HTTP server has no built-in authentication or TLS. Do not publish it
on an untrusted interface; bind the host port to loopback as above, or place
it behind an appropriately secured reverse proxy.

## Issues/Features

Here is the existing [Issues/Features](https://github.com/ardanlabs/malina/issues) for the project and the things being worked on or things that would be nice to have.

If you are interested in helping in any way, please send an email to [Bill Kennedy](mailto:bill@ardanlabs.com).

## Architecture

Malina follows the service and CLI conventions of the sibling projects where useful while keeping native ownership behind its own high-level SDK. There is no CGo: every C call goes through [purego](https://github.com/ebitengine/purego) + [JupiterRider/ffi](https://github.com/JupiterRider/ffi).

```
┌─────────────────────────────────────────────────────────────┐
│  cmd/malina   CLI (libs, system, model, generate, server)   │
│  cmd/server   HTTP domain, one-model service, embedded BUI  │
├─────────────────────────────────────────────────────────────┤
│  sdk/malina       concurrency-safe lifecycle and queue      │
│  sdk/malina/model reusable-context config and PNG output    │
├─────────────────────────────────────────────────────────────┤
│  sdk/malina/sd internal/advanced stable-diffusion.h FFI     │
│                (context, generation, image, video, system)  │
│  sdk/tools/libs   native library installation               │
│  sdk/tools/models curated model catalog and downloads       │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
            libstable-diffusion.{dylib|so|dll}
              (stable-diffusion.cpp master-669)
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

Malina uses the prebuilt stable-diffusion.cpp release artifacts from [leejet/stable-diffusion.cpp](https://github.com/leejet/stable-diffusion.cpp/releases) directly — there is no companion builder repo. The pinned version is captured in [`sdk/tools/libs/libs.go`](sdk/tools/libs/libs.go) as `DefaultSDVersion`.

| OS      | CPU   | GPU          | Source                                                      |
| ------- | ----- | ------------ | ----------------------------------------------------------- |
| Linux   | amd64 | CPU (avx2)   | `sd-master-…-linux-avx2-x64.tar.gz` (upstream)              |
| macOS   | arm64 | Metal        | `sd-master-…-bin-MacOS-arm64.tar.gz` (upstream)             |
| Windows | amd64 | CPU, CUDA 12 | `sd-master-…-bin-win-avx2-x64.zip` / `-cuda12-…` (upstream) |

Whenever there is a new release of stable-diffusion.cpp, the FFI struct mirrors in `sdk/malina/sd` and the version constant in `sdk/tools/libs` may need a refresh. Bump `DefaultSDVersion`, regenerate any struct-size assertions in `sdk/malina/sd/*_test.go`, and let CI verify.

## API Examples

Use `sdk/malina` when embedding Malina. The handle owns one reusable native context and serializes calls to it, matching Kronk's `sdk/kronk` and `sdk/kronk/model` package layout:

```go
import (
	"context"
	"log"
	"os"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

if err := malina.Init(malina.WithLibPath(os.Getenv("MALINA_LIB"))); err != nil {
	log.Fatal(err)
}

engine, err := malina.New(model.WithModelPath("model.safetensors"))
if err != nil {
	log.Fatal(err)
}
defer engine.Unload(context.Background())

params := model.NewGenerateParams()
params.Prompt = "a raspberry spaceship"
image, err := engine.Generate(context.Background(), params)
if err != nil {
	log.Fatal(err)
}
if err := os.WriteFile("output.png", image.PNG, 0o644); err != nil {
	log.Fatal(err)
}
```

Start in degraded mode (health is available and readiness is false), then load a model from the BUI at `http://127.0.0.1:8080/admin/`:

```shell
$ MALINA_LIB=$(pwd)/lib malina server start
```

Use `--model` to load at startup. Configuration also accepts `MALINA_API_HOST`, `MALINA_MODEL`, `MALINA_QUEUE_DEPTH`, `MALINA_READ_TIMEOUT`, `MALINA_WRITE_TIMEOUT`, `MALINA_IDLE_TIMEOUT`, `MALINA_INFERENCE_TIMEOUT`, `MALINA_SHUTDOWN_TIMEOUT`, and `MALINA_BUI`.

```shell
$ curl -X POST localhost:8080/v1/malina/models/load -H 'content-type: application/json' -d '{"model_path":"/models/model.safetensors"}'
$ curl -X POST localhost:8080/v1/images/generations -H 'content-type: application/json' -d '{"prompt":"a raspberry spaceship","size":"512x512","n":1,"response_format":"b64_json"}'
```

Routes are `GET /healthz`, `GET /readyz`, `POST /v1/images/generations`, `GET /v1/models`, `GET /v1/malina/models`, `GET /v1/malina/models/ps`, `POST /v1/malina/models/load`, and `POST /v1/malina/models/unload`.

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

[SD-ENCODE](examples/sd-encode/main.go) — mux a directory of PNG / JPEG frames into a Motion-JPEG AVI. No model is loaded; this is the pure-Go `model.SaveAVI` encoder accepting standard Go images.

```shell
$ make example-sd-encode
```

## SDK Program — Hello Example

```go
// hello is the smallest possible malina example: load a stable-diffusion
// model, generate one image from a text prompt, and save it as PNG.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
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

	if err := malina.Init(malina.WithLibPath(libPath)); err != nil {
		log.Fatalf("malina.Init: %v", err)
	}

	fmt.Println("loading model from", modelPath, "...")
	engine, err := malina.New(model.WithModelPath(modelPath))
	if err != nil {
		log.Fatalf("malina.New: %v", err)
	}
	defer engine.Unload(context.Background())

	params := model.NewGenerateParams()
	params.Prompt = prompt

	fmt.Println("generating image for prompt:", prompt)
	start := time.Now()
	img, err := engine.Generate(context.Background(), params)
	if err != nil {
		log.Fatalf("malina.Generate: %v", err)
	}
	elapsed := time.Since(start)

	const outPath = "hello.png"
	if err := os.WriteFile(outPath, img.PNG, 0o644); err != nil {
		log.Fatalf("write PNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d) in %s\n", outPath, img.Width, img.Height, elapsed.Round(time.Millisecond))
}
```

This example produces the following output:

```shell
$ make example-hello
go run ./examples/hello "a lovely cat"
loading model from /Users/bill/models/sd-1.5/v1-5-pruned-emaonly.safetensors ...
generating image for prompt: a lovely cat
wrote hello.png (512x512) in 6.842s
```

## License

Apache-2.0 — see [LICENSE](./LICENSE).
