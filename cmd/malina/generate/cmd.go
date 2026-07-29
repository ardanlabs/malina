// Package generate provides single-image text-to-image generation.
package generate

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ardanlabs/malina/sdk/malina"
	"github.com/ardanlabs/malina/sdk/malina/model"
	"github.com/spf13/cobra"
)

// NewCmd constructs the generate command.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate one PNG image",
		Args:  cobra.NoArgs,
		RunE:  run,
	}
	flags := cmd.Flags()
	flags.StringP("model", "m", "", "model checkpoint path")
	flags.StringP("lib", "l", "", "library path (or MALINA_LIB)")
	flags.StringP("prompt", "p", "", "generation prompt")
	flags.String("negative-prompt", "", "negative prompt")
	flags.Int("width", 512, "image width")
	flags.Int("height", 512, "image height")
	flags.Int("steps", 20, "sampling steps")
	flags.Float32("cfg", 7, "CFG scale")
	flags.Int64("seed", -1, "random seed")
	flags.Int("queue-depth", 2, "generation queue depth")
	flags.StringP("output", "o", "output.png", "output PNG path")
	return cmd
}

func run(cmd *cobra.Command, _ []string) (runErr error) {
	modelPath, _ := cmd.Flags().GetString("model")
	prompt, _ := cmd.Flags().GetString("prompt")
	if modelPath == "" {
		return fmt.Errorf("generate: --model is required")
	}
	if prompt == "" {
		return fmt.Errorf("generate: --prompt is required")
	}
	libPath, _ := cmd.Flags().GetString("lib")
	var initOpts []malina.InitOption
	if libPath != "" {
		initOpts = append(initOpts, malina.WithLibPath(libPath))
	}
	if err := malina.Init(initOpts...); err != nil {
		return fmt.Errorf("generate: initializing malina: %w", err)
	}
	queueDepth, _ := cmd.Flags().GetInt("queue-depth")
	m, err := malina.New(model.WithModelPath(modelPath), model.WithQueueDepth(queueDepth))
	if err != nil {
		return fmt.Errorf("generate: loading model: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, m.Unload(context.Background())) }()

	params := model.NewGenerateParams()
	params.Prompt = prompt
	params.NegativePrompt, _ = cmd.Flags().GetString("negative-prompt")
	params.Width, _ = cmd.Flags().GetInt("width")
	params.Height, _ = cmd.Flags().GetInt("height")
	params.Steps, _ = cmd.Flags().GetInt("steps")
	params.CFGScale, _ = cmd.Flags().GetFloat32("cfg")
	params.Seed, _ = cmd.Flags().GetInt64("seed")
	image, err := m.Generate(cmd.Context(), params)
	if err != nil {
		return fmt.Errorf("generate: generating image: %w", err)
	}
	output, _ := cmd.Flags().GetString("output")
	if output == "" {
		return fmt.Errorf("generate: --output is required")
	}
	if err := os.WriteFile(output, image.PNG, 0o644); err != nil {
		return fmt.Errorf("generate: writing PNG: %w", err)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "wrote", output)
	return nil
}
