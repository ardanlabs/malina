// Package system provides native library and host system information.
package system

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/sd"
	"github.com/spf13/cobra"
)

// NewCmd constructs the system command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system",
		Short: "Initialize and show native and host information",
		Args:  cobra.NoArgs,
		RunE:  run,
	}
	cmd.Flags().StringP("lib", "l", "", "library path (or MALINA_LIB)")
	return cmd
}

func run(cmd *cobra.Command, _ []string) error {
	libPath, _ := cmd.Flags().GetString("lib")
	if libPath == "" {
		libPath = os.Getenv("MALINA_LIB")
	}
	if libPath == "" {
		return fmt.Errorf("system: --lib or MALINA_LIB is required")
	}
	if err := malina.Init(malina.WithLibPath(libPath)); err != nil {
		return fmt.Errorf("system: initializing malina: %w", err)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "os: %s\narch: %s\ncpus: %d\nlibrary: %s\nggml-backend-devices: %d\n", runtime.GOOS, runtime.GOARCH, runtime.NumCPU(), libPath, sd.GGMLBackendDeviceCount())
	return nil
}
