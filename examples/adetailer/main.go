// adetailer detects faces and refines them with an inpainting pass.
//
// Run from the repository root after `malina model pull adetailer-face-yolov8n`:
//
//	make example-adetailer
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
	inPath := flag.String("in", "samples/adetailer-face.png", "source portrait PNG or JPEG")
	outPath := flag.String("out", "adetailer.png", "output PNG path")
	prompt := flag.String("prompt", "a detailed portrait photo", "face refinement prompt")
	flag.Parse()

	bundleDir := filepath.Join(download.DefaultModelsDir(), "adetailer-face-yolov8n")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load model bundle from %s: %v (did you run `malina model pull adetailer-face-yolov8n`?)", bundleDir, err)
	}

	libDir := download.DefaultLibrariesDir()
	if err := download.VerifyDefaultInstall(context.Background(), libDir); err != nil {
		log.Fatalf("verify default libraries: %v (did you run `malina install -u`?)", err)
	}
	if err := sd.Load(libDir); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(libDir); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	input, err := sd.LoadImage(*inPath)
	if err != nil {
		log.Fatalf("load input image: %v", err)
	}
	cparams := sd.ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	ctx, err := sd.NewContext(cparams)
	if err != nil {
		log.Fatalf("create generation context: %v", err)
	}
	defer sd.FreeContext(ctx)

	detailer, err := sd.NewADetailerContext(manifest.Files[string(download.RoleADetailer)], sd.NumPhysicalCores(), "cpu", "")
	if err != nil {
		log.Fatalf("create ADetailer context: %v", err)
	}
	defer sd.FreeADetailerContext(detailer)

	inpaint := sd.ImgGenParamsInit()
	inpaint.Prompt = *prompt
	inpaint.Width = int32(input.Width)
	inpaint.Height = int32(input.Height)
	images, err := sd.ADetailImage(detailer, ctx, input, sd.ADetailerParams{
		Prompt:    *prompt,
		ExtraArgs: "input_size=640,confidence=0.3,inpaint_width=64,inpaint_height=64",
	}, inpaint)
	if err != nil {
		log.Fatalf("refine image: %v", err)
	}
	result := images[len(images)-1]
	if err := result.SavePNG(*outPath); err != nil {
		log.Fatalf("save output: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d)\n", *outPath, result.Width, result.Height)
}
