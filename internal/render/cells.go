package render

import (
	"image/color"
)

// Cell attributes bitmask matching terminal.TerminalCell
const (
	AttrBold          uint8 = 1 << 0
	AttrDim           uint8 = 1 << 1
	AttrItalic        uint8 = 1 << 2
	AttrUnderline     uint8 = 1 << 3
	AttrBlink         uint8 = 1 << 4
	AttrReverse       uint8 = 1 << 5
	AttrStrikethrough uint8 = 1 << 6
)

// RenderCell represents an on-screen cell in the compositor's canvas.
type RenderCell struct {
	Content      string
	Width        int
	Fg           color.Color
	Bg           color.Color
	Attrs        uint8
	Continuation bool // true if this cell is the trailing cell of a wide character
}

// BlankCell returns an empty blank cell with default attributes.
func BlankCell() RenderCell {
	return RenderCell{
		Content: " ",
		Width:   1,
	}
}

// IsZero reports whether the cell is completely empty.
func (c RenderCell) IsZero() bool {
	return c.Content == "" && c.Fg == nil && c.Bg == nil && c.Attrs == 0 && !c.Continuation
}
