// flux2 mirrors gosd's flagship example: a multi-file FLUX.2 [klein] 9B
// pipeline using a quantized diffusion model + VAE + Qwen3 LLM text
// encoder.
//
// The example reads the bundle's manifest.json to resolve each file's
// on-disk path, so the bundle directory location is the only thing the
// user needs to configure (MALINA_FLUX2_DIR; default ~/models/flux2-klein-9b).
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: populate ./lib
//	export HF_TOKEN=hf_...               # FLUX.2-dev license must be accepted
//	make pull-flux2-klein-9b             # one-time: download the bundle into ~/models
//	make example-flux2
//
// The makefile target wires MALINA_LIB to ./lib and MALINA_FLUX2_DIR to
// ~/models/flux2-klein-9b, then invokes
// `go run ./examples/flux2 "An orange cat on palm beach playing with oranges."`.
// Pass a custom prompt by running `go run ./examples/flux2 "your prompt"`
// directly after the environment variables are set.
package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	prompt := "An orange cat on palm beach playing with oranges."
	if len(os.Args) >= 2 {
		prompt = os.Args[1]
	}

	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		log.Fatal("MALINA_LIB must point to the directory containing libstable-diffusion")
	}

	bundleDir := os.Getenv("MALINA_FLUX2_DIR")
	if bundleDir == "" {
		bundleDir = filepath.Join(download.DefaultModelsDir(), "flux2-klein-9b")
	}

	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load bundle manifest from %s: %v (did you run `malina model pull flux2-klein-9b`?)", bundleDir, err)
	}

	// Load the dynamic libraries.
	if err := sd.Load(libPath); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(libPath); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	// Create and configure the inference context.
	ctxParams := sd.ContextParamsInit()

	// Declare models (FLUX.2 ships as three files: a quantized diffusion
	// transformer, an autoencoder, and a Qwen3 LLM text encoder).
	ctxParams.DiffusionModelPath = manifest.Files[string(download.RoleDiffusion)]
	ctxParams.VAEPath = manifest.Files[string(download.RoleVAE)]
	ctxParams.LLMPath = manifest.Files[string(download.RoleLLM)]

	fmt.Println("diffusion:", ctxParams.DiffusionModelPath)
	fmt.Println("vae:      ", ctxParams.VAEPath)
	fmt.Println("llm:      ", ctxParams.LLMPath)
	fmt.Println("loading context ...")

	ctx, err := sd.NewContext(ctxParams)
	if err != nil {
		log.Fatalf("sd.NewContext: %v", err)
	}
	defer sd.FreeContext(ctx)

	// Initialize image generation parameters.
	imgParams := sd.ImgGenParamsInit()

	// Prompts.
	imgParams.Prompt = prompt
	imgParams.NegativePrompt = "mascots, watermark, signature"

	fmt.Println("generating image for prompt:", prompt)
	start := time.Now()
	img, err := sd.GenerateImage(ctx, imgParams)
	if err != nil {
		log.Fatalf("sd.GenerateImage: %v", err)
	}
	elapsed := time.Since(start)

	const outPath = "output.png"
	if err := img.SavePNG(outPath); err != nil {
		log.Fatalf("SavePNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d, %d channels) in %s\n", outPath, img.Width, img.Height, img.Channel, elapsed.Round(time.Millisecond))
}
