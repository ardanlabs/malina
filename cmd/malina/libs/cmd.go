// Package libs provides commands for installing stable-diffusion.cpp libraries.
package libs

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ardanlabs/malina/sdk/tools/downloader"
	toollibs "github.com/ardanlabs/malina/sdk/tools/libs"
	"github.com/spf13/cobra"
)

// NewCmd constructs the libs command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "libs",
		Short: "Install or upgrade stable-diffusion.cpp libraries",
	}
	pullCmd := &cobra.Command{
		Use:   "pull",
		Short: "Download stable-diffusion.cpp libraries",
		Args:  cobra.NoArgs,
		RunE:  runPull,
	}
	pullCmd.Flags().StringP("lib", "l", "", "library destination (or MALINA_LIB)")
	pullCmd.Flags().StringP("version", "v", "", "release version, or latest")
	pullCmd.Flags().StringP("processor", "p", "", "backend: cpu, cuda, metal, vulkan, or rocm")
	pullCmd.Flags().String("os", runtime.GOOS, "target operating system")
	pullCmd.Flags().BoolP("upgrade", "u", false, "replace an existing installation")
	pullCmd.Flags().BoolP("quiet", "q", false, "suppress download progress")
	cmd.AddCommand(pullCmd)
	return cmd
}

func runPull(cmd *cobra.Command, _ []string) error {
	libPath, _ := cmd.Flags().GetString("lib")
	if libPath == "" {
		libPath = os.Getenv("MALINA_LIB")
	}
	if libPath == "" {
		return fmt.Errorf("libs pull: --lib or MALINA_LIB is required")
	}

	upgrade, _ := cmd.Flags().GetBool("upgrade")
	if !upgrade && toollibs.AlreadyInstalled(libPath) {
		fmt.Fprintln(cmd.OutOrStdout(), "stable-diffusion.cpp already installed at", libPath)
		return nil
	}
	version, _ := cmd.Flags().GetString("version")
	if version == "" {
		version = toollibs.DefaultSDVersion
	} else if version == "latest" {
		latest, err := toollibs.SDLatestVersion()
		if err != nil {
			return fmt.Errorf("libs pull: obtaining latest version: %w", err)
		}
		version = latest
	}
	opSys, _ := cmd.Flags().GetString("os")
	processor, _ := cmd.Flags().GetString("processor")
	quiet, _ := cmd.Flags().GetBool("quiet")
	if processor == "" {
		processor = defaultProcessor(opSys)
	}
	var progress = downloader.DefaultProgressTracker()
	if quiet {
		progress = nil
	}
	if err := toollibs.GetWithContext(cmd.Context(), runtime.GOARCH, opSys, processor, version, libPath, progress); err != nil {
		return fmt.Errorf("libs pull: downloading stable-diffusion.cpp: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "installed stable-diffusion.cpp", version, "at", libPath)
	return nil
}

func defaultProcessor(opSys string) string {
	if opSys == "darwin" {
		return "metal"
	}
	if opSys == "windows" {
		if installed, _ := toollibs.HasCUDA(); installed {
			return "cuda"
		}
	}
	return "cpu"
}
