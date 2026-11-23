// moss-image-edit edits a reference image using Malina's embedded MOSS HQ workflow.
//
// Run from the repository root after `malina model pull moss-image-edit-hq`:
//
//	make example-moss-image-edit
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	inPath := flag.String("in", "samples/adetailer-face.png", "reference PNG or JPEG")
	outPath := flag.String("out", "moss-image-edit.png", "output PNG path")
	prompt := flag.String("prompt", "Change the person's hair to bright blue, keeping everything else the same.", "editing instruction")
	libDir := flag.String("lib", download.DefaultLibrariesDir(), "verified native library directory")
	width := flag.Int("width", 512, "output width (multiple of 16)")
	height := flag.Int("height", 512, "output height (multiple of 16)")
	steps := flag.Int("steps", 20, "sampling steps")
	seed := flag.Int64("seed", -1, "random seed; negative selects a random seed")
	flag.Parse()

	bundleDir := filepath.Join(download.DefaultModelsDir(), "moss-image-edit-hq")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load model bundle: %v (run `malina model pull moss-image-edit-hq` first)", err)
	}
	if err := download.VerifyDefaultInstall(context.Background(), *libDir); err != nil {
		log.Fatalf("verify native libraries: %v", err)
	}
	if err := sd.Load(*libDir); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(*libDir); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}
	input, err := sd.LoadImage(*inPath)
	if err != nil {
		log.Fatalf("load reference image: %v", err)
	}
	ctx, err := sd.NewContext(sd.ContextParamsInit())
	if err != nil {
		log.Fatalf("create editing context: %v", err)
	}
	defer sd.FreeContext(ctx)

	// EditMOSSImage fills and executes Malina's embedded DGL template.
	// No external .dgl file or LoRA prompt tag is required.
	params := sd.MOSSImageEditParamsInit()
	params.DiffusionModelPath = manifest.Files[string(download.RoleDiffusion)]
	params.LLMPath = manifest.Files[string(download.RoleLLM)]
	params.LLMVisionPath = manifest.Files[string(download.RoleLLMVision)]
	params.TokenizerPath = manifest.Files[string(download.RoleTokenizer)]
	params.VAEPath = manifest.Files[string(download.RoleVAE)]
	params.LoraPath = manifest.Files[string(download.RoleLora)]
	params.Prompt = *prompt
	params.Width, params.Height = int32(*width), int32(*height)
	params.Steps, params.Seed = int32(*steps), *seed
	result, err := sd.EditMOSSImage(ctx, input, params)
	if err != nil {
		log.Fatalf("edit image: %v", err)
	}
	if err := result.SavePNG(*outPath); err != nil {
		log.Fatalf("save output: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d)\n", *outPath, result.Width, result.Height)
}
