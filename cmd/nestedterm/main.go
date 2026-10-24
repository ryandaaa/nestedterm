package main

import (
	"fmt"
	"os"

	"nestedterm/internal/app"
	"nestedterm/internal/config"
	"nestedterm/internal/keyboard"
)

func main() {
	cfg, err := config.ParseFlags(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n\nRun 'nestedterm --help' for usage instructions.\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Printf("NestedTerm version %s (Linux/Debian)\n", config.Version)
		os.Exit(0)
	}

	if cfg.ShowHelp {
		printHelp()
		os.Exit(0)
	}

	application := app.NewApp(cfg)
	if err := application.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "nestedterm: %v\n", err)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Printf(`NestedTerm v%s - Linux terminal multiplexer with embedded terminal emulation and automatic tiling

USAGE:
  nestedterm [flags]

FLAGS:
  --shell <path>       Default interactive shell executable (default: $SHELL or /bin/bash)
  --cwd <path>         Initial working directory (default: current directory)
  --max-panes <n>      Maximum number of simultaneous panes (default: 12)
  --prefix <key>       Prefix key: ctrl-space, ctrl-b, ctrl-a (default: ctrl-space)
  --border <style>     Border style: single, rounded, double, heavy (default: single)
  --log <file>         Path to optional debug log file
  -h, --help           Show this help text
  -v, --version        Show version information

DEFAULT KEYBINDINGS (Prefix: Ctrl+Space):
`, config.Version)

	for _, line := range keyboard.BuildHelpLines(0x00) {
		fmt.Println(" " + line)
	}
	fmt.Println()
}
