package render

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"nestedterm/internal/layout"
	"nestedterm/internal/terminal"
	"nestedterm/internal/workspace"
)

// OverlayState holds modal overlays to be rendered on top of the workspace.
type OverlayState struct {
	ShowHelp        bool
	ShowQuitConfirm bool
	HelpText        []string
	BannerMessage   string
}

// Renderer manages compositing workspace panes and modal overlays to the host terminal.
type Renderer struct {
	width       int
	height      int
	borderChars BorderChars
	canvas      *Canvas // scratch canvas composed each frame
	prevCanvas  *Canvas // last frame written to the host terminal; nil forces a full repaint
	rowBuf      []terminal.TerminalCell
	out         []byte // reused frame buffer

	cursorKnown   bool
	lastCursorX   int
	lastCursorY   int
	lastCursorVis bool
}

// NewRenderer creates a new Renderer instance.
func NewRenderer(w, h int, borderStyle string) *Renderer {
	chars := BorderSingle
	switch strings.ToLower(borderStyle) {
	case "rounded":
		chars = BorderRounded
	case "double":
		chars = BorderDouble
	case "heavy":
		chars = BorderHeavy
	}

	return &Renderer{
		width:       w,
		height:      h,
		borderChars: chars,
	}
}

// Resize updates the renderer's viewport dimensions.
func (r *Renderer) Resize(w, h int) {
	r.width = w
	r.height = h
	r.Invalidate()
}

// Invalidate forces the next Render to repaint every cell.
func (r *Renderer) Invalidate() {
	r.prevCanvas = nil
	r.cursorKnown = false
}

// Render composites the current workspace state and writes ANSI sequences to out.
func (r *Renderer) Render(ws *workspace.Workspace, overlays OverlayState, out io.Writer) error {
	if r.width < 2 || r.height < 2 {
		return nil
	}

	if r.canvas == nil || r.canvas.Width != r.width || r.canvas.Height != r.height {
		r.canvas = NewCanvas(r.width, r.height)
	} else {
		r.canvas.Reset()
	}
	canvas := r.canvas
	activePane := ws.GetActivePane()

	// 1. Draw each pane
	for _, p := range ws.AllPanes() {
		isActive := activePane != nil && p.ID == activePane.ID

		// Draw border and title
		canvas.DrawBorder(p.Rect, isActive, p.TitleDisplay(), r.borderChars)

		// Draw inner terminal content
		r.renderPaneContent(canvas, p)
		highlightSelection(canvas, p)
	}

	// 2. Cursor translation
	cursorX := -1
	cursorY := -1
	cursorVisible := false

	if activePane != nil && !activePane.InScrollback && activePane.Terminal != nil {
		cx, cy, vis := activePane.Terminal.CursorPosition()
		inner := activePane.InnerRect
		if vis && cx >= 0 && cx < inner.W && cy >= 0 && cy < inner.H {
			cursorX = inner.X + cx
			cursorY = inner.Y + cy
			cursorVisible = true
		}
	}

	// 3. Render overlays
	if overlays.ShowHelp {
		r.renderHelpOverlay(canvas, overlays.HelpText)
		cursorVisible = false
	} else if overlays.ShowQuitConfirm {
		r.renderQuitConfirmOverlay(canvas)
		cursorVisible = false
	}

	// 4. Render banner notification if present
	if overlays.BannerMessage != "" {
		r.renderBanner(canvas, overlays.BannerMessage)
	}

	canvas.NormalizeWide()

	// 5. Emit only what changed since the previous frame
	return r.emit(canvas, cursorX, cursorY, cursorVisible, out)
}

// emit diffs canvas against the previous frame and writes the minimal update to out.
func (r *Renderer) emit(canvas *Canvas, cursorX, cursorY int, cursorVisible bool, out io.Writer) error {
	prev := r.prevCanvas
	if prev != nil && (prev.Width != canvas.Width || prev.Height != canvas.Height) {
		prev = nil
	}

	buf := r.out[:0]
	var curStyle RenderCell
	curX, curY := -1, -1
	changed := false

	for y := 0; y < canvas.Height; y++ {
		row := canvas.Cells[y*canvas.Width : (y+1)*canvas.Width]
		var prevRow []RenderCell
		if prev != nil {
			prevRow = prev.Cells[y*canvas.Width : (y+1)*canvas.Width]
		}
		for x := range row {
			c := &row[x]
			if c.Continuation {
				continue
			}
			if prevRow != nil && cellsEqual(c, &prevRow[x]) {
				continue
			}

			if !changed {
				changed = true
				buf = append(buf, SeqSyncBegin...)
				buf = append(buf, SeqHideCursor...)
				buf = append(buf, SeqReset...)
			}
			if curX != x || curY != y {
				buf = appendCursorTo(buf, x, y)
			}
			buf = appendSGRDiff(buf, curStyle, *c)
			curStyle = *c

			if c.Content != "" {
				buf = append(buf, c.Content...)
			} else {
				buf = append(buf, ' ')
			}
			curY = y
			curX = x + 1
			if c.Width == 2 {
				curX = x + 2
			}
		}
	}

	if changed {
		buf = append(buf, SeqReset...)
	}

	cursorMoved := !r.cursorKnown || cursorVisible != r.lastCursorVis ||
		(cursorVisible && (cursorX != r.lastCursorX || cursorY != r.lastCursorY))
	if changed || cursorMoved {
		if cursorVisible && cursorX >= 0 && cursorY >= 0 {
			buf = appendCursorTo(buf, cursorX, cursorY)
			buf = append(buf, SeqShowCursor...)
		} else {
			buf = append(buf, SeqHideCursor...)
		}
		if changed {
			buf = append(buf, SeqSyncEnd...)
		}
	}
	r.cursorKnown = true
	r.lastCursorX, r.lastCursorY, r.lastCursorVis = cursorX, cursorY, cursorVisible

	// Swap buffers: the frame just composed becomes the baseline.
	if r.prevCanvas != nil && r.prevCanvas.Width == canvas.Width && r.prevCanvas.Height == canvas.Height {
		r.canvas, r.prevCanvas = r.prevCanvas, canvas
	} else {
		r.canvas, r.prevCanvas = nil, canvas
	}
	r.out = buf

	if len(buf) == 0 {
		return nil
	}
	_, err := out.Write(buf)
	return err
}

func (r *Renderer) renderPaneContent(canvas *Canvas, p *workspace.Pane) {
	if p.Terminal == nil {
		return
	}

	inner := p.InnerRect
	if inner.W <= 0 || inner.H <= 0 {
		return
	}

	if cap(r.rowBuf) < inner.W {
		r.rowBuf = make([]terminal.TerminalCell, inner.W)
	}
	row := r.rowBuf[:inner.W]

	if p.InScrollback && p.ScrollOffset > 0 {
		// Render from scrollback history
		for y := 0; y < inner.H; y++ {
			p.ViewRow(y, row)
			for col := range row {
				drawTerminalCell(canvas, inner.X+col, inner.Y+y, &row[col])
			}
		}
		return
	}

	// Render live screen
	for y := 0; y < inner.H; y++ {
		p.Terminal.ReadRow(y, row)
		for col := 0; col < len(row); col++ {
			tc := &row[col]
			drawTerminalCell(canvas, inner.X+col, inner.Y+y, tc)
			if tc.Width == 2 && col+1 < len(row) {
				// Mark next column as wide continuation
				canvas.SetCell(inner.X+col+1, inner.Y+y, RenderCell{
					Continuation: true,
					Fg:           tc.Fg,
					Bg:           tc.Bg,
				})
				col++
			}
		}
	}
}

func drawTerminalCell(canvas *Canvas, x, y int, tc *terminal.TerminalCell) {
	if tc == nil {
		canvas.SetCell(x, y, BlankCell())
		return
	}
	content := tc.Content
	if content == "" {
		content = " "
	}
	canvas.SetCell(x, y, RenderCell{
		Content: content,
		Width:   tc.Width,
		Fg:      tc.Fg,
		Bg:      tc.Bg,
		Attrs:   tc.Attrs,
	})
}

func (r *Renderer) renderHelpOverlay(canvas *Canvas, helpLines []string) {
	maxLineLen := 0
	for _, line := range helpLines {
		runes := []rune(line)
		if len(runes) > maxLineLen {
			maxLineLen = len(runes)
		}
	}

	w := maxLineLen + 6
	if w < 56 {
		w = 56
	}
	if w > canvas.Width-4 {
		w = canvas.Width - 4
	}
	h := len(helpLines) + 4
	if h > canvas.Height-2 {
		h = canvas.Height - 2
	}
	x := (canvas.Width - w) / 2
	y := (canvas.Height - h) / 2
	box := layout.Rect{X: x, Y: y, W: w, H: h}

	// 1. Solid modern dark background
	canvas.Fill(box, RenderCell{
		Content: " ",
		Bg:      ColorOverlayBg,
	})

	// 2. Beautiful frame
	canvas.DrawBorder(box, true, " NestedTerm Keybindings ", r.borderChars)

	// 3. Render content with strict clipping to right boundary
	rightLimit := x + w - 2
	for i, line := range helpLines {
		row := y + 1 + i
		if row >= y+h-2 {
			break
		}

		if strings.HasPrefix(line, "FAST ") || strings.HasPrefix(line, "PREFIX ") || strings.HasPrefix(line, "SCROLLBACK ") {
			// Section Header: Bold Vibrant Cyan
			canvas.DrawStringClipped(x+3, row, line, ColorSectionHeader, ColorOverlayBg, AttrBold, rightLimit)
		} else if strings.HasPrefix(line, "  ") && len(line) > 18 {
			// Two-column layout: Shortcut in Amber Gold, description in soft white
			keyPart := line[:20]
			descPart := line[20:]
			canvas.DrawStringClipped(x+3, row, keyPart, ColorKeyHighlight, ColorOverlayBg, AttrBold, rightLimit)
			canvas.DrawStringClipped(x+23, row, descPart, ColorOverlayFg, ColorOverlayBg, 0, rightLimit)
		} else {
			canvas.DrawStringClipped(x+3, row, line, ColorOverlayFg, ColorOverlayBg, 0, rightLimit)
		}
	}

	// 4. Centered Footer
	footer := "Press any key or Esc to close"
	footerX := x + (w-len(footer))/2
	if footerX < x+2 {
		footerX = x + 2
	}
	canvas.DrawStringClipped(footerX, y+h-2, footer, ColorActiveBorder, ColorOverlayBg, AttrBold, rightLimit)
}

func (r *Renderer) renderQuitConfirmOverlay(canvas *Canvas) {
	w := 54
	h := 7
	if w > canvas.Width-4 {
		w = canvas.Width - 4
	}
	x := (canvas.Width - w) / 2
	y := (canvas.Height - h) / 2
	box := layout.Rect{X: x, Y: y, W: w, H: h}

	canvas.Fill(box, RenderCell{
		Content: " ",
		Bg:      ColorOverlayBg,
	})
	canvas.DrawBorder(box, true, " Confirm Quit ", r.borderChars)

	msg1 := "Active terminal sessions are currently running."
	msg2 := "Are you sure you want to terminate all and exit?"
	msg3 := "[Y] Kill all and quit    [N / Esc] Cancel"

	rightLimit := x + w - 2
	canvas.DrawStringClipped(x+4, y+2, msg1, ColorOverlayFg, ColorOverlayBg, AttrBold, rightLimit)
	canvas.DrawStringClipped(x+4, y+3, msg2, ColorTitleInactive, ColorOverlayBg, 0, rightLimit)
	canvas.DrawStringClipped(x+6, y+5, msg3, ColorKeyHighlight, ColorOverlayBg, AttrBold, rightLimit)
}

func (r *Renderer) renderBanner(canvas *Canvas, msg string) {
	if msg == "" || canvas.Height < 1 {
		return
	}
	y := canvas.Height - 1
	trimmed := strings.TrimSpace(msg)
	badge := fmt.Sprintf(" %s ", trimmed)
	runes := []rune(badge)
	lenMsg := len(runes)
	if lenMsg > canvas.Width-2 {
		lenMsg = canvas.Width - 2
		runes = runes[:lenMsg]
	}
	x := canvas.Width - lenMsg - 1

	fg := ColorActiveBorder
	bg := ColorOverlayBg
	if strings.Contains(strings.ToLower(trimmed), "error") ||
		strings.Contains(strings.ToLower(trimmed), "too small") ||
		strings.Contains(strings.ToLower(trimmed), "cannot") {
		fg = color.RGBA{R: 255, G: 110, B: 90, A: 255} // Alert coral red
	} else if strings.HasPrefix(trimmed, "[") {
		fg = ColorKeyHighlight // Warm amber gold for prefix badge
	}

	canvas.DrawString(x, y, string(runes), fg, bg, AttrBold)
}

// highlightSelection inverts the cells of the pane's mouse selection.
func highlightSelection(canvas *Canvas, p *workspace.Pane) {
	inner := p.InnerRect
	for y := 0; y < inner.H; y++ {
		x0, x1, ok := p.SelectionSpan(y)
		if !ok {
			continue
		}
		for x := x0; x <= x1 && x < inner.W; x++ {
			c := canvas.GetCell(inner.X+x, inner.Y+y)
			c.Attrs ^= AttrReverse
			canvas.SetCell(inner.X+x, inner.Y+y, c)
		}
	}
}
