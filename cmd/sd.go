package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// SDCmd groups stable-diffusion subcommands under "malina sd". Subcommand
// implementations (txt2img, img2img) land in follow-on milestones.
var SDCmd = &cli.Command{
	Name:  "sd",
	Usage: "Run stable-diffusion.cpp commands (txt2img, img2img)",
	Action: func(c *cli.Context) error {
		return fmt.Errorf("sd: not yet implemented")
	},
}
