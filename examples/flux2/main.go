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
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
	"github.com/ardanlabs/malina/sdk/tools/models"
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
		bundleDir = filepath.Join(models.DefaultModelsDir(), "flux2-klein-9b")
	}
	manifest, err := models.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load bundle manifest from %s: %v", bundleDir, err)
	}
	diffusion := manifest.Files[string(models.RoleDiffusion)]
	vae := manifest.Files[string(models.RoleVAE)]
	llm := manifest.Files[string(models.RoleLLM)]
	fmt.Printf("diffusion: %s\nvae:       %s\nllm:       %s\n", diffusion, vae, llm)
	if err := malina.Init(malina.WithLibPath(libPath)); err != nil {
		log.Fatalf("malina.Init: %v", err)
	}
	engine, err := malina.New(model.WithDiffusionModelPath(diffusion), model.WithVAEPath(vae), model.WithLLMPath(llm))
	if err != nil {
		log.Fatalf("malina.New: %v", err)
	}
	defer func() {
		if err := engine.Unload(context.Background()); err != nil {
			log.Printf("unload: %v", err)
		}
	}()
	params := model.NewGenerateParams()
	params.Prompt = prompt
	params.NegativePrompt = "mascots, watermark, signature"
	params.Steps = 4
	start := time.Now()
	img, err := engine.Generate(context.Background(), params)
	if err != nil {
		log.Fatalf("Generate: %v", err)
	}
	const outPath = "output.png"
	if err := os.WriteFile(outPath, img.PNG, 0o644); err != nil {
		log.Fatalf("write PNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d) in %s\n", outPath, img.Width, img.Height, time.Since(start).Round(time.Millisecond))
}
