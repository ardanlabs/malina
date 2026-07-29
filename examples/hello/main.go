// hello is the smallest possible malina example: load a stable-diffusion
// model, generate one image from a text prompt, and save it as PNG.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: populate ./lib
//	make pull-sd-1.5                     # one-time: download the SD 1.5 bundle into ~/models
//	make example-hello
//
// The makefile target wires MALINA_LIB to ./lib and MALINA_TEST_MODEL to
// ~/models/sd-1.5/v1-5-pruned-emaonly.safetensors, then invokes
// `go run ./examples/hello "a lovely cat"`. Pass a custom prompt by
// running `go run ./examples/hello "your prompt"` directly after the
// environment variables are set.
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
	defer func() {
		if err := engine.Unload(context.Background()); err != nil {
			log.Printf("malina.Unload: %v", err)
		}
	}()

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
