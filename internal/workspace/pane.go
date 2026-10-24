package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nestedterm/internal/layout"
	"nestedterm/internal/pty"
	"nestedterm/internal/terminal"
)

// PaneID represents a unique identifier for a pane.
type PaneID string

// Pane represents a single managed terminal pane.
type Pane struct {
	ID           PaneID
	Index        int
	Title        string
	InitialCWD   string
	LastKnownCWD string

	mu       sync.RWMutex
	Status   PaneStatus
	ExitCode *int

	Session  *pty.PTYSession
	Terminal terminal.TerminalEmulator

	Rect      layout.Rect // outer bounds including borders
	InnerRect layout.Rect // inner bounds excluding borders

	InScrollback bool
	ScrollOffset int // 0 = at bottom (live view), > 0 = lines scrolled up
}

// Write writes bytes into the pane's child PTY process.
func (p *Pane) Write(data []byte) (int, error) {
	p.mu.RLock()
	session := p.Session
	p.mu.RUnlock()

	if session == nil {
		return 0, fmt.Errorf("pane %s has no active session", p.ID)
	}
	return session.Write(data)
}

// Resize updates the pane geometry and propagates dimensions to the emulator and PTY.
func (p *Pane) Resize(outer layout.Rect) error {
	p.mu.Lock()
	p.Rect = outer
	p.InnerRect = outer.Inner(1)
	cols := p.InnerRect.W
	rows := p.InnerRect.H
	term := p.Terminal
	session := p.Session
	p.mu.Unlock()

	if term != nil {
		term.Resize(cols, rows)
	}
	if session != nil {
		return session.Resize(cols, rows)
	}
	return nil
}

// GetCWD dynamically updates and returns the working directory.
func (p *Pane) GetCWD() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Session != nil {
		current := p.Session.GetCWD()
		p.LastKnownCWD = current
		return current
	}
	return p.LastKnownCWD
}

// TitleDisplay returns the formatted label to be shown on the pane's top border.
func (p *Pane) TitleDisplay() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dir := p.LastKnownCWD
	home := os.Getenv("HOME")
	if home != "" && strings.HasPrefix(dir, home) {
		dir = "~" + strings.TrimPrefix(dir, home)
	} else if len(dir) > 20 {
		dir = "..." + filepath.Base(dir)
	}

	shellName := "shell"
	if p.Title != "" {
		shellName = p.Title
	}

	var statusExtra string
	if p.Status == PaneExited {
		code := 0
		if p.ExitCode != nil {
			code = *p.ExitCode
		}
		statusExtra = fmt.Sprintf(" (exited: %d)", code)
	} else if p.InScrollback {
		statusExtra = fmt.Sprintf(" [SCROLL: +%d]", p.ScrollOffset)
	}

	return fmt.Sprintf(" [%d] %s │ %s%s ", p.Index, shellName, dir, statusExtra)
}

// EnterScrollback activates scrollback browsing mode.
func (p *Pane) EnterScrollback() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.InScrollback = true
	if p.Terminal != nil {
		maxLines := p.Terminal.ScrollbackLen()
		if maxLines > 0 {
			p.ScrollOffset = 1
		}
	}
}

// ExitScrollback exits scrollback browsing mode.
func (p *Pane) ExitScrollback() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.InScrollback = false
	p.ScrollOffset = 0
}

// ScrollUp increments the scrollback offset up by lines.
func (p *Pane) ScrollUp(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.Terminal == nil {
		return
	}
	maxLines := p.Terminal.ScrollbackLen()
	p.ScrollOffset += lines
	if p.ScrollOffset > maxLines {
		p.ScrollOffset = maxLines
	}
	if p.ScrollOffset > 0 {
		p.InScrollback = true
	}
}

// ScrollDown decrements the scrollback offset down by lines.
func (p *Pane) ScrollDown(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.ScrollOffset -= lines
	if p.ScrollOffset <= 0 {
		p.ScrollOffset = 0
		p.InScrollback = false
	}
}

// ResetScroll resets scrollback to live view.
func (p *Pane) ResetScroll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.ScrollOffset = 0
	p.InScrollback = false
}

// Close gracefully terminates the pane session and closes resources.
func (p *Pane) Close(gracePeriod time.Duration) error {
	p.mu.Lock()
	p.Status = PaneClosing
	sess := p.Session
	term := p.Terminal
	p.mu.Unlock()

	var err error
	if sess != nil {
		err = sess.Close(gracePeriod)
	}
	if term != nil {
		_ = term.Close()
	}
	return err
}
