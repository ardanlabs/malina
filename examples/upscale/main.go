// upscale enlarges an image with Real-ESRGAN.
//
// Run from the repository root after `malina model pull realesrgan-x4-anime`:
//
//	make example-upscale
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
	inPath := flag.String("in", "samples/adetailer-face.png", "source PNG or JPEG")
	outPath := flag.String("out", "upscaled.png", "output PNG path")
	flag.Parse()

	bundleDir := filepath.Join(download.DefaultModelsDir(), "realesrgan-x4-anime")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load model bundle from %s: %v (did you run `malina model pull realesrgan-x4-anime`?)", bundleDir, err)
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
	ctx, err := sd.NewUpscalerContext(manifest.Files[string(download.RoleUpscaler)], false, sd.NumPhysicalCores(), 0, "", "")
	if err != nil {
		log.Fatalf("create upscaler context: %v", err)
	}
	defer sd.FreeUpscalerContext(ctx)

	factor, err := sd.GetUpscaleFactor(ctx)
	if err != nil {
		log.Fatalf("get upscale factor: %v", err)
	}
	images, err := sd.Upscale(ctx, input, uint32(factor))
	if err != nil {
		log.Fatalf("upscale image: %v", err)
	}
	if err := images[0].SavePNG(*outPath); err != nil {
		log.Fatalf("save output: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d, %dx upscale)\n", *outPath, images[0].Width, images[0].Height, factor)
}
