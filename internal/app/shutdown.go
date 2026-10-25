package app

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// TerminalRestorer manages host terminal raw mode and ensures clean restoration.
type TerminalRestorer struct {
	fd            int
	originalState *term.State
	restoreOnce   sync.Once
	restored      bool
}

// EnterRawMode takes over the terminal: saves original state, enables raw mode,
// and enters the alternate screen buffer.
func EnterRawMode(fd int) (*TerminalRestorer, error) {
	if !term.IsTerminal(fd) {
		return nil, fmt.Errorf("file descriptor %d is not an interactive terminal", fd)
	}

	state, err := term.MakeRaw(fd)
	if err != nil {
		return nil, fmt.Errorf("failed to enable raw mode: %w", err)
	}

	r := &TerminalRestorer{
		fd:            fd,
		originalState: state,
	}

	// Enter alternate screen buffer and hide cursor initially
	_, _ = os.Stdout.WriteString("\033[?1049h\033[?25l\033[?1000h\033[?1002h\033[?1006h")
	return r, nil
}

// Restore restores original terminal state, cursor visibility, and exits alternate screen.
// Safe to call multiple times (idempotent).
func (r *TerminalRestorer) Restore() {
	if r == nil {
		return
	}
	r.restoreOnce.Do(func() {
		// Exit alternate screen, reset attributes, show cursor
		_, _ = os.Stdout.WriteString("\033[?1006l\033[?1002l\033[?1000l\033[?25h\033[0m\033[?1049l")
		if r.originalState != nil {
			_ = term.Restore(r.fd, r.originalState)
		}
		r.restored = true
	})
}
