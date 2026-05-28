package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ardanlabs/malina/pkg/sd"
	"github.com/urfave/cli/v2"
)

// SDCmd groups stable-diffusion subcommands under "malina sd". Subcommand
// implementations for generation (txt2img, img2img) land in follow-on
// milestones.
var SDCmd = &cli.Command{
	Name:  "sd",
	Usage: "Run stable-diffusion.cpp commands",
	Subcommands: []*cli.Command{
		sdEncodeCmd,
	},
}

var sdEncodeCmd = &cli.Command{
	Name:      "encode",
	Usage:     "Encode a directory of PNG frames into a Motion-JPEG AVI video",
	ArgsUsage: " ",
	Description: `Encode reads every PNG file under --input (sorted lexicographically) and
muxes them into a single Motion-JPEG AVI at the specified --fps. No model
is loaded; this is a pure-Go encoder.

Examples:
  malina sd encode --input frames/ --fps 24 -o out.avi
  malina sd encode --input frames/ --fps 30 --quality 85 -o clip.avi`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "input",
			Aliases:  []string{"i"},
			Usage:    "directory containing the source PNG frames",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "output",
			Aliases:  []string{"o"},
			Usage:    "output AVI file path",
			Required: true,
		},
		&cli.IntFlag{
			Name:  "fps",
			Usage: "frames per second",
			Value: 24,
		},
		&cli.IntFlag{
			Name:  "quality",
			Usage: "JPEG quality (1-100)",
			Value: 90,
		},
	},
	Action: func(c *cli.Context) error {
		return runSDEncode(c)
	},
}

func runSDEncode(c *cli.Context) error {
	inputDir := c.String("input")
	outPath := c.String("output")
	fps := c.Int("fps")
	quality := c.Int("quality")

	pngs, err := listPNGs(inputDir)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if len(pngs) == 0 {
		return fmt.Errorf("encode: no .png files found in %s", inputDir)
	}

	frames := make([]*sd.SDImage, 0, len(pngs))
	for _, p := range pngs {
		img, err := sd.LoadPNG(p)
		if err != nil {
			return fmt.Errorf("encode: load %s: %w", p, err)
		}
		frames = append(frames, img)
	}

	if err := sd.SaveAVI(outPath, frames, fps, quality); err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	fmt.Printf("wrote %s (%d frames, %dx%d, %d fps)\n",
		outPath, len(frames), frames[0].Width, frames[0].Height, fps)
	return nil
}

// listPNGs returns the sorted absolute paths of all *.png files directly
// inside dir (non-recursive).
func listPNGs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read directory %s: %w", dir, err)
	}

	var pngs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			pngs = append(pngs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(pngs)
	return pngs, nil
}
