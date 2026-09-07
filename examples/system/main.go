// system is the smallest possible malina example: load libstable-diffusion
// and print the library version and system info. No model is loaded.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: install the default libraries
//	make example-system
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	libPath := download.DefaultLibrariesDir()

	if err := download.VerifyDefaultInstall(context.Background(), libPath); err != nil {
		log.Fatalf("verify default libraries: %v (did you run `malina install -u`?)", err)
	}
	if err := sd.Load(libPath); err != nil {
		log.Fatalf("sd.Load: %v", err)
	}

	if err := sd.Init(libPath); err != nil {
		log.Fatalf("sd.Init: %v", err)
	}

	fmt.Println("-- stable-diffusion.cpp --")
	fmt.Println("version:                ", sd.Version())
	fmt.Println("physical-cores:         ", sd.NumPhysicalCores())
	fmt.Println("ggml-backend-devices:   ", sd.GGMLBackendDeviceCount())
	fmt.Println()
	fmt.Println("-- System info --")
	fmt.Println(sd.SystemInfo())
}
