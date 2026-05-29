// system is the smallest possible malina example: load libstable-diffusion
// and print the library version and system info. No model is loaded.
//
// Run it from the repo root with:
//
//	make download-stable-diffusion.cpp   # one-time: populate ./lib
//	make example-system
//
// The makefile target wires MALINA_LIB to ./lib before invoking
// `go run ./examples/system`.
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/ardanlabs/malina/pkg/sd"
)

func main() {
	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		log.Fatal("MALINA_LIB must point to the directory containing libstable-diffusion")
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
