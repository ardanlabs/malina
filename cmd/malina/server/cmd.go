// Package server provides the foreground Malina API server command.
package server

import (
	"github.com/ardanlabs/malina/cmd/server/api/services/malina/runner"
	"github.com/spf13/cobra"
)

// NewCmd constructs the server command tree.
func NewCmd() *cobra.Command {
	cfg := runner.DefaultConfig()
	cmd := &cobra.Command{Use: "server", Short: "Run the Malina API server"}
	start := &cobra.Command{Use: "start", Short: "Start the server in the foreground", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return runner.Run(cmd.Context(), cfg) }}
	f := start.Flags()
	f.StringVar(&cfg.Host, "host", cfg.Host, "listen host and port (MALINA_API_HOST)")
	f.StringVar(&cfg.LibPath, "lib", cfg.LibPath, "native library path (MALINA_LIB)")
	f.StringVar(&cfg.ModelPath, "model", cfg.ModelPath, "optional startup model (MALINA_MODEL)")
	f.IntVar(&cfg.QueueDepth, "queue-depth", cfg.QueueDepth, "admitted generation capacity")
	f.DurationVar(&cfg.ReadTimeout, "read-timeout", cfg.ReadTimeout, "HTTP read timeout")
	f.DurationVar(&cfg.WriteTimeout, "write-timeout", cfg.WriteTimeout, "HTTP write timeout")
	f.DurationVar(&cfg.IdleTimeout, "idle-timeout", cfg.IdleTimeout, "HTTP idle timeout")
	f.DurationVar(&cfg.InferenceTimeout, "inference-timeout", cfg.InferenceTimeout, "model load and generation request timeout")
	f.DurationVar(&cfg.ShutdownTimeout, "shutdown-timeout", cfg.ShutdownTimeout, "graceful shutdown timeout")
	f.BoolVar(&cfg.BUI, "bui", cfg.BUI, "serve the administration UI")
	cmd.AddCommand(start)
	return cmd
}
