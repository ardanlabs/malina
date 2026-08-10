// img2img demonstrates image-to-image generation: load a stable-diffusion
// model, encode a source image (PNG or JPEG) as the starting latent,
// then denoise it using the supplied text prompt and save the result as
// PNG.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: populate ./lib
//	make pull-sd-1.5                     # one-time: download the SD 1.5 bundle into ~/models
//	make example-img2img                 # uses samples/frames/frame_0000.png + a default prompt
//
// Or pass your own image / prompt / strength explicitly:
//
//	go run ./examples/img2img -in samples/frames/frame_0000.png \
//	    -prompt "a watercolor painting of a sunset" -strength 0.6
//
// Strength controls how much of the source survives: lower values
// (0.2-0.4) keep the composition mostly intact, higher values (0.7-0.9)
// let the prompt take over.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	var (
		inPath   = flag.String("in", "", "path to the source image (PNG or JPEG, required)")
		outPath  = flag.String("out", "img2img.png", "output PNG path")
		prompt   = flag.String("prompt", "a watercolor painting", "text prompt that steers the denoising")
		strength = flag.Float64("strength", 0.6, "img2img noise strength (0..1); lower preserves more of the source")
		steps    = flag.Int("steps", 20, "denoising steps")
		seed     = flag.Int64("seed", -1, "RNG seed (-1 = random)")
	)
	flag.Parse()

	if *inPath == "" {
		flag.Usage()
		log.Fatal("-in is required")
	}

	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		log.Fatal("MALINA_LIB must point to the directory containing libstable-diffusion")
	}

	modelPath := os.Getenv("MALINA_TEST_MODEL")
	if modelPath == "" {
		log.Fatal("MALINA_TEST_MODEL must point to a stable-diffusion model file (.gguf or .safetensors)")
	}

	if err := sd.Load(libPath); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}
	if err := sd.Init(libPath); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	fmt.Println("loading source image from", *inPath, "...")
	src, err := sd.LoadImage(*inPath)
	if err != nil {
		log.Fatalf("sd.LoadImage: %v", err)
	}
	fmt.Printf("source image: %dx%d (%d channels)\n", src.Width, src.Height, src.Channel)

	cparams := sd.ContextParamsInit()
	cparams.ModelPath = modelPath

	fmt.Println("loading model from", modelPath, "...")
	ctx, err := sd.NewContext(cparams)
	if err != nil {
		log.Fatalf("sd.NewContext: %v", err)
	}
	defer sd.FreeContext(ctx)

	params := sd.ImgGenParamsInit()
	params.Prompt = *prompt
	params.Width = int32(src.Width)
	params.Height = int32(src.Height)
	params.Steps = int32(*steps)
	params.Strength = float32(*strength)
	params.Seed = *seed
	params.InitImage = src

	fmt.Printf("generating from prompt %q (strength=%.2f, steps=%d) ...\n", *prompt, *strength, *steps)
	start := time.Now()
	img, err := sd.GenerateImage(ctx, params)
	if err != nil {
		log.Fatalf("sd.GenerateImage: %v", err)
	}
	elapsed := time.Since(start)

	if err := img.SavePNG(*outPath); err != nil {
		log.Fatalf("SavePNG: %v", err)
	}
	fmt.Printf("wrote %s (%dx%d, %d channels) in %s\n", *outPath, img.Width, img.Height, img.Channel, elapsed.Round(time.Millisecond))
}
