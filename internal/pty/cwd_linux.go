package pty

import (
	"fmt"
	"os"
)

// ResolveCWD attempts to inspect the current working directory of the given PID via /proc.
// If the process has exited, permissions are denied, or the symlink is invalid,
// it falls back to the provided fallback path.
func ResolveCWD(pid int, fallback string) string {
	if pid <= 0 {
		return fallback
	}

	target, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return fallback
	}

	stat, err := os.Stat(target)
	if err != nil || !stat.IsDir() {
		return fallback
	}

	return target
}
