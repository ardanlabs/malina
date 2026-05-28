// system is the smallest possible malina example: load libstable-diffusion
// and print the library version and system info.
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
