// img2img demonstrates image-to-image generation: load a stable-diffusion
// model, encode a source image (PNG or JPEG) as the starting latent,
// then denoise it using the supplied text prompt and save the result as
// PNG.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: populate ./lib
//	make pull-sd-1.5                     # one-time: download the SD 1.5 bundle into ~/models
//	make example-img2img                 # uses examples/samples/frames/image1.jpg + a default prompt
//
// Or pass your own image / prompt / strength explicitly:
//
//	go run ./examples/img2img -in examples/samples/frames/image1.jpg \
//	    -prompt "a watercolor painting of a sunset" -strength 0.6
//
// Strength controls how much of the source survives: lower values
// (0.2-0.4) keep the composition mostly intact, higher values (0.7-0.9)
// let the prompt take over.
package main

import (
	"context"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"os"
	"time"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
)

func main() {
	inPath := flag.String("in", "", "path to source image")
	outPath := flag.String("out", "img2img.png", "output PNG path")
	prompt := flag.String("prompt", "a watercolor painting", "generation prompt")
	strength := flag.Float64("strength", 0.6, "img2img strength")
	steps := flag.Int("steps", 20, "denoising steps")
	seed := flag.Int64("seed", -1, "RNG seed")
	flag.Parse()
	if *inPath == "" {
		log.Fatal("-in is required")
	}
	file, err := os.Open(*inPath)
	if err != nil {
		log.Fatalf("open source: %v", err)
	}
	src, _, err := image.Decode(file)
	closeErr := file.Close()
	if err != nil {
		log.Fatalf("decode source: %v", err)
	}
	if closeErr != nil {
		log.Fatalf("close source: %v", closeErr)
	}
	libPath, modelPath := os.Getenv("MALINA_LIB"), os.Getenv("MALINA_TEST_MODEL")
	if libPath == "" || modelPath == "" {
		log.Fatal("MALINA_LIB and MALINA_TEST_MODEL are required")
	}
	if err := malina.Init(malina.WithLibPath(libPath)); err != nil {
		log.Fatalf("malina.Init: %v", err)
	}
	engine, err := malina.New(model.WithModelPath(modelPath), model.WithVAEEncoder())
	if err != nil {
		log.Fatalf("malina.New: %v", err)
	}
	defer func() {
		if err := engine.Unload(context.Background()); err != nil {
			log.Printf("unload: %v", err)
		}
	}()
	params := model.NewGenerateParams()
	params.Prompt, params.InitImage = *prompt, src
	params.Strength, params.Steps, params.Seed = float32(*strength), *steps, *seed
	start := time.Now()
	img, err := engine.Generate(context.Background(), params)
	if err != nil {
		log.Fatalf("Generate: %v", err)
	}
	if err := os.WriteFile(*outPath, img.PNG, 0o644); err != nil {
		log.Fatalf("write PNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d) in %s\n", *outPath, img.Width, img.Height, time.Since(start).Round(time.Millisecond))
}
