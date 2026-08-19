// concurrent measures image generation on independent stable-diffusion
// contexts. It intentionally does not call GenerateImage concurrently on one
// context because the native context contains mutable, unsynchronized state.
//
// Run it from the repo root with:
//
//	make example-concurrent
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ardanlabs/malina/pkg/sd"
)

type generation struct {
	image   *sd.SDImage
	elapsed time.Duration
	err     error
}

func main() {
	workers := flag.Int("workers", 2, "number of independent native contexts")
	steps := flag.Int("steps", 10, "sampling steps per measured image")
	size := flag.Int("size", 512, "square output dimension")
	prompt := flag.String("prompt", "two red sailboats crossing a calm mountain lake at sunrise", "generation prompt")
	output := flag.String("output", "concurrent-output", "output directory")
	flag.Parse()

	if *workers < 2 {
		log.Fatal("workers must be at least 2 to test concurrency")
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
	sd.SetProgressCallback(func(int, int, float32) {})
	if err := sd.Init(libPath); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	fmt.Printf("loading %d independent contexts (model memory scales with this count)\n", *workers)
	contexts := make([]sd.Context, *workers)
	for i := range contexts {
		params := sd.ContextParamsInit()
		params.ModelPath = modelPath

		started := time.Now()
		ctx, err := sd.NewContext(params)
		if err != nil {
			log.Fatalf("sd.NewContext %d: %v", i, err)
		}
		contexts[i] = ctx
		defer sd.FreeContext(ctx)
		fmt.Printf("context %d loaded in %s\n", i, time.Since(started).Round(time.Millisecond))
	}

	warmParams := sd.ImgGenParamsInit()
	warmParams.Prompt = *prompt
	warmParams.Width = 64
	warmParams.Height = 64
	warmParams.Steps = 1
	for i, ctx := range contexts {
		fmt.Printf("warming context %d\n", i)
		if _, err := sd.GenerateImage(ctx, warmParams); err != nil {
			log.Fatalf("warm context %d: %v", i, err)
		}
	}

	params := sd.ImgGenParamsInit()
	params.Prompt = *prompt
	params.Width = int32(*size)
	params.Height = int32(*size)
	params.Steps = int32(*steps)
	params.Seed = 42

	serial, serialWall := runBatch(contexts, params, false)
	parallel, parallelWall := runBatch(contexts, params, true)
	if err := os.MkdirAll(*output, 0o755); err != nil {
		log.Fatalf("create output directory: %v", err)
	}
	writeResults(*output, "serial", serial)
	writeResults(*output, "parallel", parallel)

	fmt.Printf("\nserial wall time:   %s\n", serialWall.Round(time.Millisecond))
	printDurations(serial)
	fmt.Printf("parallel wall time: %s\n", parallelWall.Round(time.Millisecond))
	printDurations(parallel)
	fmt.Printf("wall-time speedup:  %.2fx\n", serialWall.Seconds()/parallelWall.Seconds())
	fmt.Println("Both independent-context calls were in flight together and completed successfully.")
	fmt.Println("A speedup above 1x indicates the backend executed useful work concurrently; a lower speedup may indicate GPU saturation or internal serialization.")
}

func runBatch(contexts []sd.Context, params sd.ImgGenParams, parallel bool) ([]generation, time.Duration) {
	results := make([]generation, len(contexts))
	started := time.Now()
	if !parallel {
		for i, ctx := range contexts {
			results[i] = generate(ctx, params, i)
		}
		return results, time.Since(started)
	}

	start := make(chan struct{})
	var ready sync.WaitGroup
	var done sync.WaitGroup
	ready.Add(len(contexts))
	done.Add(len(contexts))
	for i, ctx := range contexts {
		go func() {
			defer done.Done()
			ready.Done()
			<-start
			results[i] = generate(ctx, params, i)
		}()
	}
	ready.Wait()
	started = time.Now()
	close(start)
	done.Wait()

	return results, time.Since(started)
}

func generate(ctx sd.Context, params sd.ImgGenParams, worker int) generation {
	params.Seed += int64(worker)
	started := time.Now()
	image, err := sd.GenerateImage(ctx, params)

	return generation{
		image:   image,
		elapsed: time.Since(started),
		err:     err,
	}
}

func writeResults(output string, batch string, results []generation) {
	for i, result := range results {
		if result.err != nil {
			log.Fatalf("%s generation %d: %v", batch, i, result.err)
		}
		path := filepath.Join(output, fmt.Sprintf("%s-%02d.png", batch, i))
		if err := result.image.SavePNG(path); err != nil {
			log.Fatalf("save %s: %v", path, err)
		}
	}
}

func printDurations(results []generation) {
	for i, result := range results {
		fmt.Printf("  context %d: %s\n", i, result.elapsed.Round(time.Millisecond))
	}
}
