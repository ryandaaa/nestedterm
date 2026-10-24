package terminal

import (
	"image/color"
)

// Cell attributes bitmask
const (
	AttrBold          uint8 = 1 << 0
	AttrDim           uint8 = 1 << 1
	AttrItalic        uint8 = 1 << 2
	AttrUnderline     uint8 = 1 << 3
	AttrBlink         uint8 = 1 << 4
	AttrReverse       uint8 = 1 << 5
	AttrStrikethrough uint8 = 1 << 6
)

// TerminalCell represents a single screen or scrollback cell.
type TerminalCell struct {
	Content string
	Width   int // 1 = regular, 2 = wide lead, 0 = continuation or empty
	Fg      color.Color
	Bg      color.Color
	Attrs   uint8
}

// TerminalEmulator defines the interface required by NestedTerm to manage
// in-process terminal emulation for a pane.
type TerminalEmulator interface {
	// Write feeds bytes from the PTY into the emulator.
	Write(p []byte) (int, error)

	// TakeResponses drains queued query replies (DA, CPR, etc.) to be sent back to the PTY.
	TakeResponses() []byte

	// Resize updates the emulator dimensions.
	Resize(cols, rows int)

	// Width returns current width in cells.
	Width() int

	// Height returns current height in cells.
	Height() int

	// CellAt returns the cell at (x, y) on the active screen (0-indexed).
	CellAt(x, y int) *TerminalCell

	// CursorPosition returns the active screen cursor coordinates and visibility.
	CursorPosition() (x, y int, visible bool)

	// IsAltScreen reports whether the alternate screen buffer is currently active.
	IsAltScreen() bool

	// ScrollbackLen returns the number of lines available in scrollback history.
	ScrollbackLen() int

	// ScrollbackCellAt returns the cell at the given scrollback line index (0 = oldest) and column x.
	ScrollbackCellAt(lineIdx, x int) *TerminalCell

	// ReadRow copies row y of the active screen into dst (one cell per element)
	// under a single lock. Out-of-range cells are filled with blanks.
	ReadRow(y int, dst []TerminalCell)

	// ReadScrollbackRow copies scrollback line lineIdx (0 = oldest) into dst.
	// Out-of-range cells are filled with blanks.
	ReadScrollbackRow(lineIdx int, dst []TerminalCell)

	// Close cleans up emulator resources.
	Close() error
}
