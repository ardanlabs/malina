package cmd

import (
	"fmt"
	"runtime"

	"github.com/ardanlabs/malina/pkg/sd"
	"github.com/urfave/cli/v2"
)

// SystemCmd shows information about the host environment and the loaded
// stable-diffusion.cpp library.
var SystemCmd = &cli.Command{
	Name:  "system",
	Usage: "Show stable-diffusion.cpp / system information",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "lib",
			Aliases: []string{"l"},
			Usage:   "path to stable-diffusion.cpp compiled library files",
			EnvVars: []string{"MALINA_LIB"},
		},
	},
	Action: func(c *cli.Context) error {
		return runSystemInfo(c)
	},
}

func runSystemInfo(c *cli.Context) error {
	libPath := c.String("lib")

	fmt.Println("-- Host --")
	fmt.Printf("os:   %s\n", runtime.GOOS)
	fmt.Printf("arch: %s\n", runtime.GOARCH)
	fmt.Printf("cpus: %d\n", runtime.NumCPU())
	fmt.Println()

	fmt.Println("-- Library --")
	if libPath == "" {
		fmt.Println("MALINA_LIB not set; pass -lib or set the env var")
		return nil
	}
	fmt.Println("path:", libPath)

	if err := sd.Load(libPath); err != nil {
		return fmt.Errorf("failed to load stable-diffusion.cpp from %s: %w", libPath, err)
	}

	if err := sd.Init(libPath); err != nil {
		return fmt.Errorf("failed to init stable-diffusion.cpp from %s: %w", libPath, err)
	}

	fmt.Println("ggml-backend-devices:", sd.GGMLBackendDeviceCount())
	return nil
}
