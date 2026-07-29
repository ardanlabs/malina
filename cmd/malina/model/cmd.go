// Package model provides commands for managing curated model bundles.
package model

import (
	"fmt"
	"maps"
	"slices"

	"github.com/ardanlabs/malina/sdk/tools/models"
	"github.com/spf13/cobra"
)

// NewCmd constructs the model command tree.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "model",
		Short: "Manage stable-diffusion model bundles",
	}
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List curated model bundles",
		Args:  cobra.NoArgs,
		RunE:  runList,
	}
	infoCmd := &cobra.Command{
		Use:   "info <bundle>",
		Short: "Show model bundle details",
		Args:  cobra.ExactArgs(1),
		RunE:  runInfo,
	}
	pullCmd := &cobra.Command{
		Use:   "pull <bundle>",
		Short: "Download a model bundle",
		Args:  cobra.ExactArgs(1),
		RunE:  runPull,
	}
	pullCmd.Flags().StringP("output", "o", models.DefaultModelsDir(), "model bundles directory")
	pullCmd.Flags().BoolP("yes", "y", false, "accepted for non-interactive scripts")
	cmd.AddCommand(listCmd, infoCmd, pullCmd)
	return cmd
}

func runList(cmd *cobra.Command, _ []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-7s %s\n", "NAME", "GATED", "DESCRIPTION")
	for _, bundle := range models.Catalog() {
		fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-7t %s\n", bundle.Name, bundle.Gated, bundle.Description)
	}
	return nil
}

func runInfo(cmd *cobra.Command, args []string) error {
	bundle, ok := models.BundleByName(args[0])
	if !ok {
		return fmt.Errorf("model info: unknown bundle %q", args[0])
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "name:        %s\ndescription: %s\nlicense:     %s\ngated:       %t\nfiles:\n", bundle.Name, bundle.Description, bundle.License, bundle.Gated)
	for _, file := range bundle.Files {
		fmt.Fprintf(out, "  %-12s %-8s %s\n", file.Role, file.Size, file.Filename)
	}
	return nil
}

func runPull(cmd *cobra.Command, args []string) error {
	if _, ok := models.BundleByName(args[0]); !ok {
		return fmt.Errorf("model pull: unknown bundle %q", args[0])
	}
	output, _ := cmd.Flags().GetString("output")
	manifest, err := models.GetBundle(cmd.Context(), args[0], output)
	if err != nil {
		return fmt.Errorf("model pull: downloading bundle: %w", err)
	}
	roles := slices.Sorted(maps.Keys(manifest.Files))
	for _, role := range roles {
		fmt.Fprintf(cmd.OutOrStdout(), "%-12s %s\n", role, manifest.Files[role])
	}
	return nil
}
