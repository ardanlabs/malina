package main

import (
	"os"

	"github.com/ardanlabs/malina/cmd/malina/generate"
	"github.com/ardanlabs/malina/cmd/malina/libs"
	"github.com/ardanlabs/malina/cmd/malina/model"
	"github.com/ardanlabs/malina/cmd/malina/server"
	"github.com/ardanlabs/malina/cmd/malina/system"
	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/spf13/cobra"
)

func main() {
	cmd := cobra.Command{
		Use:     "malina",
		Short:   "Local text-to-image generation using stable-diffusion.cpp",
		Version: malina.Version,
	}
	cmd.AddCommand(libs.NewCmd(), model.NewCmd(), system.NewCmd(), generate.NewCmd(), server.NewCmd())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
