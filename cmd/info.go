package cmd

import (
	"fmt"

	"github.com/urfave/cli/v2"
)

// ShowInfo prints the malina banner and a short tagline.
func ShowInfo(c *cli.Context) error {
	fmt.Println(logo)
	fmt.Println()
	fmt.Println("Local text-to-image in Go using stable-diffusion.cpp with hardware acceleration")

	return nil
}

const logo = `
 ___ ___   ____  _      ____  ____    ____ 
|   |   | /    || |    |    ||    \  /    |
| _   _ ||  o  || |     |  | |  _  ||  o  |
|  \_/  ||     || |___  |  | |  |  ||     |
|   |   ||  _  ||     | |  | |  |  ||  _  |
|   |   ||  |  ||     | |  | |  |  ||  |  |
|___|___||__|__||_____||____||__|__||__|__|`
