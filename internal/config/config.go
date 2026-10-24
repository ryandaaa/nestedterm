package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Config represents runtime configuration for NestedTerm.
type Config struct {
	Shell         string
	InitialCWD    string
	MaxPanes      int
	PrefixKey     string
	GraceInterval time.Duration
	LogFile       string
	BorderStyle   string
	ShowHelp      bool
	ShowVersion   bool
}

// NewDefaultConfig returns a configuration with sensible defaults.
func NewDefaultConfig() *Config {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = os.Getenv("HOME")
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	return &Config{
		Shell:         shell,
		InitialCWD:    cwd,
		MaxPanes:      DefaultMaxPanes,
		PrefixKey:     DefaultPrefixKey,
		GraceInterval: DefaultGraceInterval,
		BorderStyle:   DefaultBorderStyle,
	}
}

// ParseFlags parses command line arguments and populates Config.
func ParseFlags(args []string) (*Config, error) {
	cfg := NewDefaultConfig()

	fs := flag.NewFlagSet("nestedterm", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	fs.StringVar(&cfg.Shell, "shell", cfg.Shell, "Default interactive shell executable")
	fs.StringVar(&cfg.InitialCWD, "cwd", cfg.InitialCWD, "Initial working directory")
	fs.IntVar(&cfg.MaxPanes, "max-panes", cfg.MaxPanes, "Maximum number of simultaneous panes")
	fs.StringVar(&cfg.PrefixKey, "prefix", cfg.PrefixKey, "Prefix key (e.g. ctrl-space, ctrl-b, ctrl-a)")
	fs.StringVar(&cfg.LogFile, "log", "", "Path to debug log file")
	fs.StringVar(&cfg.BorderStyle, "border", cfg.BorderStyle, "Border style: single, rounded, double, or heavy")
	fs.BoolVar(&cfg.ShowHelp, "help", false, "Show help and exit")
	fs.BoolVar(&cfg.ShowHelp, "h", false, "Show help (shorthand)")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "Show version and exit")
	fs.BoolVar(&cfg.ShowVersion, "v", false, "Show version (shorthand)")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if cfg.ShowHelp || cfg.ShowVersion {
		return cfg, nil
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks configuration values for validity.
func (c *Config) Validate() error {
	if c.MaxPanes < 1 {
		return errors.New("max-panes must be at least 1")
	}
	if c.MaxPanes > 64 {
		return errors.New("max-panes cannot exceed 64")
	}

	// Validate shell path
	resolvedShell, err := exec.LookPath(c.Shell)
	if err != nil {
		return fmt.Errorf("shell executable %q not found or not executable: %w", c.Shell, err)
	}
	c.Shell = resolvedShell

	// Validate CWD
	if c.InitialCWD == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("unable to determine current working directory: %w", err)
		}
		c.InitialCWD = cwd
	}
	absCWD, err := filepath.Abs(c.InitialCWD)
	if err != nil {
		return fmt.Errorf("invalid working directory %q: %w", c.InitialCWD, err)
	}
	stat, err := os.Stat(absCWD)
	if err != nil {
		return fmt.Errorf("working directory %q does not exist: %w", absCWD, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("working directory %q is not a directory", absCWD)
	}
	c.InitialCWD = absCWD

	// Validate prefix
	p := strings.ToLower(strings.TrimSpace(c.PrefixKey))
	switch p {
	case "ctrl-space", "ctrl+space", "ctrl-@", "ctrl+@":
		c.PrefixKey = "ctrl-space"
	case "ctrl-b", "ctrl+b":
		c.PrefixKey = "ctrl-b"
	case "ctrl-a", "ctrl+a":
		c.PrefixKey = "ctrl-a"
	default:
		return fmt.Errorf("unsupported prefix key %q (supported: ctrl-space, ctrl-b, ctrl-a)", c.PrefixKey)
	}

	return nil
}
