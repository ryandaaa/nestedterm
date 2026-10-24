package config

import (
	"time"
)

const (
	DefaultMaxPanes      = 12
	DefaultMinCols       = 12
	DefaultMinRows       = 4
	DefaultGraceInterval = 750 * time.Millisecond
	DefaultPrefixKey     = "ctrl-space"
	DefaultBorderStyle   = "single"
	Version              = "0.1.0"
)
