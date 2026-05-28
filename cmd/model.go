package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/urfave/cli/v2"
)

// ModelCmd manages stable-diffusion model bundles (list / info / pull).
//
// A "bundle" is the set of files (diffusion model, VAE, text encoders, etc.)
// that stable-diffusion.cpp needs to construct a single Context. Curation
// lives in pkg/download/bundle.go; kronk and other downstream consumers may
// layer their own catalog on top of this one.
var ModelCmd = &cli.Command{
	Name:  "model",
	Usage: "Manage stable-diffusion model bundles",
	Subcommands: []*cli.Command{
		modelListCmd,
		modelInfoCmd,
		modelPullCmd,
	},
}

var modelListCmd = &cli.Command{
	Name:  "list",
	Usage: "List the bundled catalog of curated stable-diffusion model bundles",
	Action: func(c *cli.Context) error {
		return runModelList(c)
	},
}

func runModelList(_ *cli.Context) error {
	fmt.Printf("%-18s %-7s %s\n", "NAME", "GATED", "DESCRIPTION")
	for _, b := range download.Catalog() {
		gated := "no"
		if b.Gated {
			gated = "YES"
		}
		fmt.Printf("%-18s %-7s %s\n", b.Name, gated, b.Description)
	}
	fmt.Println()
	fmt.Println("Use `malina model pull <name>` to download into ~/models (override with -o).")
	fmt.Println("Use `malina model info <name>` to see the file list for a bundle.")
	return nil
}

var modelInfoCmd = &cli.Command{
	Name:      "info",
	Usage:     "Show the file list and license for a bundle",
	ArgsUsage: "<bundle-name>",
	Action: func(c *cli.Context) error {
		return runModelInfo(c)
	},
}

func runModelInfo(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("info: provide a bundle name (see `malina model list`)")
	}
	name := c.Args().First()
	b, ok := download.BundleByName(name)
	if !ok {
		return fmt.Errorf("info: unknown bundle %q (see `malina model list`)", name)
	}

	fmt.Printf("name:        %s\n", b.Name)
	fmt.Printf("description: %s\n", b.Description)
	fmt.Printf("license:     %s\n", b.License)
	fmt.Printf("gated:       %t\n", b.Gated)
	fmt.Println("files:")
	for _, f := range b.Files {
		fmt.Printf("  - role=%-12s size=%-8s file=%s\n", f.Role, f.Size, f.Filename)
		fmt.Printf("    url=%s\n", f.URL)
	}
	if b.Gated {
		fmt.Println()
		fmt.Println("This bundle is license-gated. Accept the license on the upstream")
		fmt.Println("Hugging Face page, then export HF_TOKEN with a read-access token.")
	}
	return nil
}

var modelPullCmd = &cli.Command{
	Name:      "pull",
	Usage:     "Download every file in a curated bundle",
	ArgsUsage: "<bundle-name>",
	Description: `Download every file in a curated bundle into <output>/<bundle-name>/.

A manifest.json is written alongside the files mapping each role
(diffusion, vae, llm, ...) to its absolute on-disk path so downstream
tooling (kronk, examples) can resolve paths without consulting the
catalog directly.

Examples:
  malina model pull sd-1.5
  malina model pull -o /tmp/models sdxl-base-1.0
  HF_TOKEN=hf_xxx malina model pull flux2-klein-9b`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:        "output",
			Aliases:     []string{"o"},
			Usage:       "directory to save the bundle into (a subdir per bundle is created)",
			Value:       download.DefaultModelsDir(),
			DefaultText: "~/models",
		},
		&cli.BoolFlag{
			Name:    "yes",
			Aliases: []string{"y"},
			Usage:   "create the output directory without prompting",
			Value:   false,
		},
	},
	Action: func(c *cli.Context) error {
		return runModelPull(c)
	},
}

func runModelPull(c *cli.Context) error {
	if c.NArg() < 1 {
		return fmt.Errorf("pull: provide a bundle name (see `malina model list`)")
	}
	name := c.Args().First()
	if _, ok := download.BundleByName(name); !ok {
		return fmt.Errorf("pull: unknown bundle %q (see `malina model list`)", name)
	}

	output := c.String("output")
	autoYes := c.Bool("yes")

	if _, err := os.Stat(output); os.IsNotExist(err) {
		if !autoYes {
			fmt.Printf("Directory %s does not exist.\n", output)
			fmt.Print("Would you like to create it? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			response = strings.ToLower(strings.TrimSpace(response))
			if response != "y" && response != "yes" {
				fmt.Println("Download cancelled.")
				return nil
			}
		}
		if err := os.MkdirAll(output, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
		fmt.Printf("Created directory %s\n", output)
	}

	fmt.Printf("Downloading bundle %q into %s ...\n", name, output)

	m, err := download.GetBundle(c.Context, name, output)
	if err != nil {
		return fmt.Errorf("download bundle: %w", err)
	}

	fmt.Println("Download completed successfully.")
	fmt.Println("Bundle manifest:")
	for role, path := range m.Files {
		fmt.Printf("  %-12s %s\n", role, path)
	}
	return nil
}
