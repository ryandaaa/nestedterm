package keyboard

import (
	"strings"
)

// ResolvePrefixByte converts a human-readable prefix name into its control byte.
func ResolvePrefixByte(name string) byte {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "ctrl-space", "ctrl+space", "ctrl-@", "ctrl+@":
		return 0x00 // NUL
	case "ctrl-b", "ctrl+b":
		return 0x02 // STX
	case "ctrl-a", "ctrl+a":
		return 0x01 // SOH
	default:
		return 0x00
	}
}

// PrefixDisplayName returns a user-friendly label for the prefix key.
func PrefixDisplayName(prefix byte) string {
	switch prefix {
	case 0x00:
		return "Ctrl+Space"
	case 0x02:
		return "Ctrl+B"
	case 0x01:
		return "Ctrl+A"
	default:
		return "Prefix"
	}
}
