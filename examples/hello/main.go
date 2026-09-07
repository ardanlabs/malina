// hello is the smallest possible malina example: load a stable-diffusion
// model, generate one image from a text prompt, and save it as PNG.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: install the default libraries
//	go run . model pull -y sd-1.5        # one-time: install the default SD 1.5 model
//	make example-hello
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
