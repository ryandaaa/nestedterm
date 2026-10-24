package terminal

import (
	"sync"

	uv "github.com/charmbracelet/ultraviolet"
	"tuios.dev/vt"
)

// VTAdapter implements TerminalEmulator using tuios.dev/vt.
type VTAdapter struct {
	mu   sync.RWMutex
	term vt.Terminal
}

// NewAdapter creates a new VTAdapter with the given screen dimensions and scrollback limit.
func NewAdapter(cols, rows, maxScrollback int) *VTAdapter {
	if maxScrollback <= 0 {
		maxScrollback = 5000
	}
	t := vt.NewWithScrollback(cols, rows, maxScrollback)
	return &VTAdapter{
		term: t,
	}
}

// Write feeds bytes from the PTY into the emulator.
func (a *VTAdapter) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.term.Write(p)
}

// TakeResponses drains queued query replies (DA, CPR, etc.) to be sent back to the PTY.
func (a *VTAdapter) TakeResponses() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.term.TakeResponses()
}

// Resize updates the emulator dimensions.
func (a *VTAdapter) Resize(cols, rows int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.term.Resize(cols, rows)
}

// Width returns current width in cells.
func (a *VTAdapter) Width() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.term.Width()
}

// Height returns current height in cells.
func (a *VTAdapter) Height() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.term.Height()
}

// CellAt returns the cell at (x, y) on the active screen.
func (a *VTAdapter) CellAt(x, y int) *TerminalCell {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if x < 0 || x >= a.term.Width() || y < 0 || y >= a.term.Height() {
		return &TerminalCell{Content: " ", Width: 1}
	}

	c := a.term.CellAt(x, y)
	return convertUVCell(c)
}

// CursorPosition returns the active screen cursor coordinates and visibility.
func (a *VTAdapter) CursorPosition() (x, y int, visible bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	pos := a.term.CursorPosition()
	hidden := a.term.IsCursorHidden()
	return pos.X, pos.Y, !hidden
}

// IsAltScreen reports whether the alternate screen buffer is currently active.
func (a *VTAdapter) IsAltScreen() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.term.IsAltScreen()
}

// ScrollbackLen returns the number of lines available in scrollback history.
func (a *VTAdapter) ScrollbackLen() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.term.ScrollbackLen()
}

// ScrollbackCellAt returns the cell at the given scrollback line index (0 = oldest) and column x.
func (a *VTAdapter) ScrollbackCellAt(lineIdx, x int) *TerminalCell {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if lineIdx < 0 || lineIdx >= a.term.ScrollbackLen() {
		return &TerminalCell{Content: " ", Width: 1}
	}

	line := a.term.ScrollbackLine(lineIdx)
	if line == nil || x < 0 || x >= len(line) {
		return &TerminalCell{Content: " ", Width: 1}
	}

	return convertUVCell(line.At(x))
}

// Close cleans up emulator resources.
func (a *VTAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.term.Close()
}

func convertUVCell(c *uv.Cell) *TerminalCell {
	if c == nil {
		return &TerminalCell{Content: " ", Width: 1}
	}

	content := c.Content
	width := c.Width
	if content == "" && width == 0 {
		content = " "
		width = 1
	}

	var attrs uint8
	if c.Style.Attrs&uv.AttrBold != 0 {
		attrs |= AttrBold
	}
	if c.Style.Attrs&uv.AttrFaint != 0 {
		attrs |= AttrDim
	}
	if c.Style.Attrs&uv.AttrItalic != 0 {
		attrs |= AttrItalic
	}
	if c.Style.Attrs&uv.AttrReverse != 0 {
		attrs |= AttrReverse
	}
	if c.Style.Attrs&uv.AttrBlink != 0 {
		attrs |= AttrBlink
	}
	if c.Style.Attrs&uv.AttrStrikethrough != 0 {
		attrs |= AttrStrikethrough
	}
	if c.Style.Underline != uv.UnderlineNone {
		attrs |= AttrUnderline
	}

	return &TerminalCell{
		Content: content,
		Width:   width,
		Fg:      c.Style.Fg,
		Bg:      c.Style.Bg,
		Attrs:   attrs,
	}
}
