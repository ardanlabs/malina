// controlnet applies Canny edge conditioning to image generation.
//
// Run from the repository root after `malina model pull controlnet-canny-sd1.5`:
//
//	make example-controlnet
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
	inPath := flag.String("in", "samples/adetailer-face.png", "source PNG or JPEG used for Canny edges")
	outPath := flag.String("out", "controlnet.png", "output PNG path")
	prompt := flag.String("prompt", "a detailed astronaut portrait", "generation prompt")
	flag.Parse()

	manifest := loadBundle("controlnet-canny-sd1.5")
	loadLibraries()

	control, err := sd.LoadImage(*inPath)
	if err != nil {
		log.Fatalf("load control image: %v", err)
	}

	cparams := sd.ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	ctx, err := sd.NewContext(cparams)
	if err != nil {
		log.Fatalf("create context: %v", err)
	}
	defer sd.FreeContext(ctx)

	if err := sd.LoadControlNet(ctx, manifest.Files[string(download.RoleControlNet)]); err != nil {
		log.Fatalf("load ControlNet: %v", err)
	}
	if err := sd.PreprocessCanny(control, sd.CannyParams{
		HighThreshold: 0.08,
		LowThreshold:  0.08,
		Weak:          0.8,
		Strong:        1,
	}); err != nil {
		log.Fatalf("preprocess Canny edges: %v", err)
	}

	params := sd.ImgGenParamsInit()
	params.Prompt = *prompt
	params.Width = int32(control.Width)
	params.Height = int32(control.Height)
	params.ControlImage = control
	params.ControlStrength = 1

	image, err := sd.GenerateImage(ctx, params)
	if err != nil {
		log.Fatalf("generate controlled image: %v", err)
	}
	if err := image.SavePNG(*outPath); err != nil {
		log.Fatalf("save output: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d)\n", *outPath, image.Width, image.Height)
}

func loadBundle(name string) download.Manifest {
	dir := filepath.Join(download.DefaultModelsDir(), name)
	manifest, err := download.LoadManifest(dir)
	if err != nil {
		log.Fatalf("load model bundle from %s: %v (did you run `malina model pull %s`?)", dir, err, name)
	}
	return manifest
}

func loadLibraries() {
	dir := download.DefaultLibrariesDir()
	if err := download.VerifyDefaultInstall(context.Background(), dir); err != nil {
		log.Fatalf("verify default libraries: %v (did you run `malina install -u`?)", err)
	}
	if err := sd.Load(dir); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(dir); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}
}
