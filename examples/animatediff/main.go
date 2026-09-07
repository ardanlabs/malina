// animatediff generates multiple temporally conditioned frames and saves them
// as a Motion-JPEG AVI.
//
// Run from the repository root after `malina model pull animatediff-sd1.5`:
//
//	make example-animatediff
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
	outPath := flag.String("out", "animatediff.avi", "output AVI path")
	prompt := flag.String("prompt", "a cat walking through a garden", "video prompt")
	flag.Parse()

	bundleDir := filepath.Join(download.DefaultModelsDir(), "animatediff-sd1.5")
	manifest, err := download.LoadManifest(bundleDir)
	if err != nil {
		log.Fatalf("load model bundle from %s: %v (did you run `malina model pull animatediff-sd1.5`?)", bundleDir, err)
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

	cparams := sd.ContextParamsInit()
	cparams.ModelPath = manifest.Files[string(download.RoleModel)]
	cparams.MotionModulePath = manifest.Files[string(download.RoleMotionModule)]
	ctx, err := sd.NewContext(cparams)
	if err != nil {
		log.Fatalf("create video context: %v", err)
	}
	defer sd.FreeContext(ctx)

	params, err := sd.VideoGenParamsInit()
	if err != nil {
		log.Fatalf("initialize video parameters: %v", err)
	}
	params.Prompt = *prompt
	params.Width = 128
	params.Height = 128
	params.Sample.Steps = 4
	params.VideoFrames = 4
	params.FPS = 1

	frames, _, err := sd.GenerateVideo(ctx, params)
	if err != nil {
		log.Fatalf("generate video: %v", err)
	}
	if err := sd.SaveAVI(*outPath, frames, int(params.FPS), 90); err != nil {
		log.Fatalf("save AVI: %v", err)
	}
	fmt.Printf("wrote %s (%d frames, %dx%d @ %d fps)\n", *outPath, len(frames), params.Width, params.Height, params.FPS)
}
