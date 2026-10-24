package render

import (
	"image/color"

	"nestedterm/internal/layout"
)

// BorderChars contains glyphs for drawing borders.
type BorderChars struct {
	TopLeft     string
	TopRight    string
	BottomLeft  string
	BottomRight string
	Horizontal  string
	Vertical    string
	TeeDown     string
	TeeUp       string
	TeeRight    string
	TeeLeft     string
	Cross       string
}

var (
	BorderSingle = BorderChars{
		TopLeft: "┌", TopRight: "┐", BottomLeft: "└", BottomRight: "┘",
		Horizontal: "─", Vertical: "│",
		TeeDown: "┬", TeeUp: "┴", TeeRight: "├", TeeLeft: "┤", Cross: "┼",
	}
	BorderRounded = BorderChars{
		TopLeft: "╭", TopRight: "╮", BottomLeft: "╰", BottomRight: "╯",
		Horizontal: "─", Vertical: "│",
		TeeDown: "┬", TeeUp: "┴", TeeRight: "├", TeeLeft: "┤", Cross: "┼",
	}
	BorderDouble = BorderChars{
		TopLeft: "╔", TopRight: "╗", BottomLeft: "╚", BottomRight: "╝",
		Horizontal: "═", Vertical: "║",
		TeeDown: "╦", TeeUp: "╩", TeeRight: "╠", TeeLeft: "╣", Cross: "╬",
	}
	BorderHeavy = BorderChars{
		TopLeft: "┏", TopRight: "┓", BottomLeft: "┗", BottomRight: "┛",
		Horizontal: "━", Vertical: "┃",
		TeeDown: "┳", TeeUp: "┻", TeeRight: "┣", TeeLeft: "┫", Cross: "╋",
	}
)

var (
	ColorActiveBorder   = color.RGBA{R: 64, G: 212, B: 244, A: 255} // Vibrant modern turquoise
	ColorInactiveBorder = color.RGBA{R: 70, G: 75, B: 90, A: 255}   // Subtle slate
	ColorTitleActive    = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	ColorTitleInactive  = color.RGBA{R: 140, G: 145, B: 160, A: 255}
	ColorOverlayBg      = color.RGBA{R: 22, G: 24, B: 32, A: 255}   // Modern charcoal
	ColorOverlayFg      = color.RGBA{R: 225, G: 230, B: 240, A: 255}
	ColorKeyHighlight   = color.RGBA{R: 255, G: 210, B: 90, A: 255}  // Warm amber gold
	ColorSectionHeader  = color.RGBA{R: 90, G: 210, B: 245, A: 255}  // Cool vibrant cyan
)

// Canvas represents a 2D grid of screen cells.
type Canvas struct {
	Width  int
	Height int
	Cells  []RenderCell
}

// NewCanvas creates a Canvas with dimensions w x h filled with blank cells.
func NewCanvas(w, h int) *Canvas {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	size := w * h
	cells := make([]RenderCell, size)
	for i := range cells {
		cells[i] = BlankCell()
	}
	return &Canvas{
		Width:  w,
		Height: h,
		Cells:  cells,
	}
}

// SetCell puts a cell at (x, y) if within canvas bounds.
func (c *Canvas) SetCell(x, y int, cell RenderCell) {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}
	c.Cells[(y*c.Width)+x] = cell
}

// GetCell returns the cell at (x, y) or BlankCell if out of bounds.
func (c *Canvas) GetCell(x, y int) RenderCell {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return BlankCell()
	}
	return c.Cells[(y*c.Width)+x]
}

// Fill fills the given rectangle with the specified cell.
func (c *Canvas) Fill(r layout.Rect, cell RenderCell) {
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			c.SetCell(x, y, cell)
		}
	}
}

// DrawString draws a line of text starting at (x, y).
func (c *Canvas) DrawString(x, y int, str string, fg, bg color.Color, attrs uint8) {
	c.DrawStringClipped(x, y, str, fg, bg, attrs, c.Width)
}

// DrawStringClipped draws a line of text starting at (x, y), strictly bound by maxCol (exclusive).
func (c *Canvas) DrawStringClipped(x, y int, str string, fg, bg color.Color, attrs uint8, maxCol int) {
	runes := []rune(str)
	for i, r := range runes {
		col := x + i
		if col >= c.Width || col >= maxCol {
			break
		}
		c.SetCell(col, y, RenderCell{
			Content: string(r),
			Width:   1,
			Fg:      fg,
			Bg:      bg,
			Attrs:   attrs,
		})
	}
}

// DrawBorder draws a border frame around r with an optional title on top.
func (c *Canvas) DrawBorder(r layout.Rect, active bool, title string, chars BorderChars) {
	if r.W < 2 || r.H < 2 {
		return
	}

	borderColor := ColorInactiveBorder
	var borderAttrs uint8
	if active {
		borderColor = ColorActiveBorder
		borderAttrs = AttrBold
	}

	right := r.X + r.W - 1
	bottom := r.Y + r.H - 1

	// Corners
	c.setOrMergeBorder(r.X, r.Y, chars.TopLeft, borderColor, borderAttrs, chars)
	c.setOrMergeBorder(right, r.Y, chars.TopRight, borderColor, borderAttrs, chars)
	c.setOrMergeBorder(r.X, bottom, chars.BottomLeft, borderColor, borderAttrs, chars)
	c.setOrMergeBorder(right, bottom, chars.BottomRight, borderColor, borderAttrs, chars)

	// Horizontal edges
	for x := r.X + 1; x < right; x++ {
		c.setOrMergeBorder(x, r.Y, chars.Horizontal, borderColor, borderAttrs, chars)
		c.setOrMergeBorder(x, bottom, chars.Horizontal, borderColor, borderAttrs, chars)
	}

	// Vertical edges
	for y := r.Y + 1; y < bottom; y++ {
		c.setOrMergeBorder(r.X, y, chars.Vertical, borderColor, borderAttrs, chars)
		c.setOrMergeBorder(right, y, chars.Vertical, borderColor, borderAttrs, chars)
	}

	// Title
	if title != "" && r.W > 4 {
		titleFg := ColorTitleInactive
		titleAttrs := uint8(0)
		if active {
			titleFg = ColorTitleActive
			titleAttrs = AttrBold
		}

		maxLen := r.W - 4
		runes := []rune(title)
		if len(runes) > maxLen {
			runes = runes[:maxLen]
		}
		c.DrawString(r.X+2, r.Y, string(runes), titleFg, nil, titleAttrs)
	}
}

func (c *Canvas) setOrMergeBorder(x, y int, glyph string, fg color.Color, attrs uint8, b BorderChars) {
	if x < 0 || x >= c.Width || y < 0 || y >= c.Height {
		return
	}

	existing := c.GetCell(x, y)
	newGlyph := mergeBorderGlyph(existing.Content, glyph, b)

	// If active, overwrite color/attributes
	finalFg := fg
	finalAttrs := attrs
	if existing.Attrs&AttrBold != 0 {
		finalAttrs |= AttrBold
		finalFg = ColorActiveBorder
	}

	c.SetCell(x, y, RenderCell{
		Content: newGlyph,
		Width:   1,
		Fg:      finalFg,
		Attrs:   finalAttrs,
	})
}

func mergeBorderGlyph(existing, incoming string, b BorderChars) string {
	if existing == "" || existing == " " {
		return incoming
	}
	if existing == incoming {
		return incoming
	}

	// Handle cross / tee intersections
	if (existing == b.Vertical && incoming == b.Horizontal) || (existing == b.Horizontal && incoming == b.Vertical) {
		return b.Cross
	}
	if existing == b.TopLeft && incoming == b.Horizontal {
		return b.TeeDown
	}
	if existing == b.BottomLeft && incoming == b.Horizontal {
		return b.TeeUp
	}
	if existing == b.TopRight && incoming == b.Horizontal {
		return b.TeeDown
	}
	if existing == b.BottomRight && incoming == b.Horizontal {
		return b.TeeUp
	}
	if (existing == b.Horizontal && incoming == b.Vertical) || (existing == b.Vertical && incoming == b.Horizontal) {
		return b.Cross
	}

	return incoming
}
