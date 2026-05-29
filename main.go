package main

import (
	"fmt"
	"os"

	"github.com/ardanlabs/malina/cmd"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:     "malina",
		Usage:    "Malina command line tool",
		Commands: buildCommands(),
	}

	err := app.Run(os.Args)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func buildCommands() []*cli.Command {
	return []*cli.Command{
		cmd.InstallCmd,
		cmd.SystemCmd,
		cmd.ModelCmd,
		cmd.SDCmd,
		versionCmd,
		infoCmd,
	}
}

var versionCmd = &cli.Command{
	Name:  "version",
	Usage: "Show malina version",
	Action: func(c *cli.Context) error {
		return runShowVersion()
	},
}

func runShowVersion() error {
	return showMalinaVersion()
}

func showMalinaVersion() error {
	fmt.Printf("malina version %s\n", Version())
	return nil
}

func runShowInfo(c *cli.Context) error {
	cmd.ShowInfo(c)
	return showMalinaVersion()
}

var infoCmd = &cli.Command{
	Name:  "info",
	Usage: "Show malina version",
	Action: func(c *cli.Context) error {
		return runShowInfo(c)
	},
}
