package config

import (
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	cfg := NewDefaultConfig()
	if cfg.MaxPanes != DefaultMaxPanes {
		t.Fatalf("expected max panes %d, got %d", DefaultMaxPanes, cfg.MaxPanes)
	}
	if cfg.PrefixKey != DefaultPrefixKey {
		t.Fatalf("expected prefix key %s, got %s", DefaultPrefixKey, cfg.PrefixKey)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config validation failed: %v", err)
	}
}

func TestConfigParseFlags(t *testing.T) {
	tmpDir := t.TempDir()
	args := []string{"--cwd", tmpDir, "--max-panes", "8", "--prefix", "ctrl-b"}
	cfg, err := ParseFlags(args)
	if err != nil {
		t.Fatalf("ParseFlags failed: %v", err)
	}
	if cfg.InitialCWD != tmpDir {
		t.Fatalf("expected cwd %s, got %s", tmpDir, cfg.InitialCWD)
	}
	if cfg.MaxPanes != 8 {
		t.Fatalf("expected max panes 8, got %d", cfg.MaxPanes)
	}
	if cfg.PrefixKey != "ctrl-b" {
		t.Fatalf("expected prefix ctrl-b, got %s", cfg.PrefixKey)
	}
}

func TestConfigValidationErrors(t *testing.T) {
	cfg := NewDefaultConfig()
	cfg.MaxPanes = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for max-panes = 0")
	}

	cfg = NewDefaultConfig()
	cfg.Shell = "/nonexistent/binary/path"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for nonexistent shell")
	}

	cfg = NewDefaultConfig()
	cfg.InitialCWD = "/nonexistent/cwd/path/for/test"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for nonexistent cwd")
	}

	cfg = NewDefaultConfig()
	cfg.PrefixKey = "ctrl-z"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unsupported prefix")
	}
}
