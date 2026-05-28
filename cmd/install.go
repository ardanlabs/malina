package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/ardanlabs/malina/pkg/download"
	"github.com/urfave/cli/v2"
)

// InstallCmd installs stable-diffusion.cpp shared libraries into a local
// directory.
var InstallCmd = &cli.Command{
	Name:  "install",
	Usage: "Install stable-diffusion.cpp libraries used by malina",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "version",
			Aliases: []string{"v"},
			Usage:   `version of stable-diffusion.cpp to install (e.g. "master-656-0e4ee04"; default is the malina-pinned version, pass "latest" to query the GitHub releases API)`,
			Value:   "",
		},
		&cli.StringFlag{
			Name:    "lib",
			Aliases: []string{"l"},
			Usage:   "path to stable-diffusion.cpp compiled library files",
			EnvVars: []string{"MALINA_LIB"},
		},
		&cli.StringFlag{
			Name:    "processor",
			Aliases: []string{"p"},
			Usage:   "backend to use (cpu, cuda, metal, vulkan, rocm)",
			Value:   "",
		},
		&cli.StringFlag{
			Name:  "os",
			Usage: "operating system to install for (linux, windows, darwin)",
			Value: runtime.GOOS,
		},
		&cli.BoolFlag{
			Name:    "upgrade",
			Aliases: []string{"u"},
			Usage:   "upgrade existing installation",
			Value:   false,
		},
		&cli.BoolFlag{
			Name:    "quiet",
			Aliases: []string{"q"},
			Usage:   "suppress output during installation",
			Value:   false,
		},
	},
	Action: func(c *cli.Context) error {
		return runInstall(c)
	},
}

func runInstall(c *cli.Context) error {
	libPath := c.String("lib")
	version := c.String("version")
	processor := c.String("processor")
	osInstall := c.String("os")
	upgrade := c.Bool("upgrade")
	quiet := c.Bool("quiet")

	if libPath == "" {
		return fmt.Errorf("missing -lib flag or MALINA_LIB env var")
	}

	if !upgrade && download.AlreadyInstalled(libPath) {
		fmt.Println("stable-diffusion.cpp already installed at", libPath)
		return nil
	}

	switch version {
	case "":
		// Use the malina-pinned default. Avoids hitting the GitHub
		// releases API for first installs and CI runs.
		version = download.DefaultSDVersion
	case "latest":
		v, err := download.SDLatestVersion()
		if err != nil {
			return fmt.Errorf("could not obtain latest version: %w", err)
		}
		version = v
	}

	if !quiet {
		fmt.Println("installing stable-diffusion.cpp", version, "to", libPath)
	} else {
		download.ProgressTracker = nil
	}

	if processor == "" {
		processor = defaultProcessor(osInstall, quiet)
	}

	if err := download.Get(runtime.GOARCH, osInstall, processor, version, libPath); err != nil {
		return fmt.Errorf("failed to download stable-diffusion.cpp: %w", err)
	}

	if !quiet {
		fmt.Println("done.")
		showInstallRequirements(libPath)
	}
	return nil
}

// defaultProcessor picks a sensible backend per OS, matching how
// leejet/stable-diffusion.cpp ships release artifacts. CUDA is auto-
// selected on Windows when nvidia-smi is on PATH; Vulkan and ROCm are
// never picked automatically (users opt in with `-p vulkan|rocm`).
//
// On Linux, CUDA is intentionally NOT auto-selected because upstream
// ships no Linux/CUDA asset; users on a CUDA box should pass `-p vulkan`
// (or `-p rocm` for AMD).
func defaultProcessor(osInstall string, quiet bool) string {
	switch osInstall {
	case "darwin":
		return "metal"
	case "windows":
		if cudaInstalled, cudaVersion := download.HasCUDA(); cudaInstalled {
			if !quiet {
				fmt.Printf("CUDA detected (version %s), using CUDA build\n", cudaVersion)
			}
			return "cuda"
		}
		return "cpu"
	default:
		return "cpu"
	}
}

func showInstallRequirements(libPath string) {
	if os.Getenv("MALINA_LIB") == libPath {
		return
	}
	switch runtime.GOOS {
	case "linux", "darwin":
		fmt.Println(`
You may want to set the MALINA_LIB environment variable to the directory with your stable-diffusion.cpp library files. For example:

    export MALINA_LIB=` + libPath)
	case "windows":
		fmt.Println(`
You may want to set the MALINA_LIB environment variable to the directory with your stable-diffusion.cpp library files. For example:

    set MALINA_LIB=` + libPath)
	}
}
