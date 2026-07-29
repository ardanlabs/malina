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

	"github.com/ardanlabs/malina/sdk/malina"
)

func main() {
	libPath := os.Getenv("MALINA_LIB")
	if libPath == "" {
		log.Fatal("MALINA_LIB must point to the directory containing libstable-diffusion")
	}

	if err := malina.Init(malina.WithLibPath(libPath)); err != nil {
		log.Fatalf("malina.Init: %v", err)
	}
	info, err := malina.SystemInfo()
	if err != nil {
		log.Fatalf("malina.SystemInfo: %v", err)
	}

	fmt.Println("-- stable-diffusion.cpp --")
	fmt.Println("version:                ", info.NativeVersion)
	fmt.Println("physical-cores:         ", info.PhysicalCores)
	fmt.Println("ggml-backend-devices:   ", info.BackendDeviceCount)
	fmt.Println()
	fmt.Println("-- System info --")
	fmt.Println(info.Description)
}
