package render

import (
	"fmt"
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
	prevCanvas  *Canvas
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
	r.prevCanvas = nil
}

// Render composites the current workspace state and writes ANSI sequences to out.
func (r *Renderer) Render(ws *workspace.Workspace, overlays OverlayState, out io.Writer) error {
	if r.width < 2 || r.height < 2 {
		return nil
	}

	canvas := NewCanvas(r.width, r.height)
	activePane := ws.GetActivePane()

	// 1. Draw each pane
	for _, p := range ws.AllPanes() {
		isActive := activePane != nil && p.ID == activePane.ID

		// Draw border and title
		canvas.DrawBorder(p.Rect, isActive, p.TitleDisplay(), r.borderChars)

		// Draw inner terminal content
		r.renderPaneContent(canvas, p)
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

	// 5. Emit ANSI sequence to outer terminal
	var sb strings.Builder
	// Move cursor to top-left and hide cursor during paint
	sb.WriteString(SeqHideCursor)
	sb.WriteString(SeqHomeCursor)

	var currentStyle RenderCell
	for y := 0; y < canvas.Height; y++ {
		for x := 0; x < canvas.Width; x++ {
			c := canvas.GetCell(x, y)
			if c.Continuation {
				continue
			}

			// Apply style diff
			diff := SGRDiff(currentStyle, c)
			if diff != "" {
				sb.WriteString(diff)
				currentStyle = c
			}

			if c.Content != "" {
				sb.WriteString(c.Content)
			} else {
				sb.WriteString(" ")
			}
		}
		if y < canvas.Height-1 {
			sb.WriteString("\r\n")
		}
	}

	// Reset styling at end
	sb.WriteString(SeqReset)

	// Position and show physical cursor
	if cursorVisible && cursorX >= 0 && cursorY >= 0 {
		sb.WriteString(fmt.Sprintf("\033[%d;%dH", cursorY+1, cursorX+1))
		sb.WriteString(SeqShowCursor)
	} else {
		sb.WriteString(SeqHideCursor)
	}

	r.prevCanvas = canvas
	_, err := out.Write([]byte(sb.String()))
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

	if p.InScrollback && p.ScrollOffset > 0 {
		// Render from scrollback history
		histLen := p.Terminal.ScrollbackLen()
		totalRows := histLen + inner.H
		targetEnd := totalRows - p.ScrollOffset
		targetStart := targetEnd - inner.H

		for row := 0; row < inner.H; row++ {
			absRow := targetStart + row
			for col := 0; col < inner.W; col++ {
				var tc *terminal.TerminalCell
				if absRow < histLen {
					tc = p.Terminal.ScrollbackCellAt(absRow, col)
				} else {
					tc = p.Terminal.CellAt(col, absRow-histLen)
				}
				drawTerminalCell(canvas, inner.X+col, inner.Y+row, tc)
			}
		}
	} else {
		// Render live screen
		for row := 0; row < inner.H; row++ {
			for col := 0; col < inner.W; col++ {
				tc := p.Terminal.CellAt(col, row)
				drawTerminalCell(canvas, inner.X+col, inner.Y+row, tc)
				if tc != nil && tc.Width == 2 && col+1 < inner.W {
					// Mark next column as wide continuation
					canvas.SetCell(inner.X+col+1, inner.Y+row, RenderCell{
						Continuation: true,
						Fg:           tc.Fg,
						Bg:           tc.Bg,
					})
					col++
				}
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
	w := 54
	h := len(helpLines) + 4
	if w > canvas.Width-4 {
		w = canvas.Width - 4
	}
	if h > canvas.Height-2 {
		h = canvas.Height - 2
	}
	x := (canvas.Width - w) / 2
	y := (canvas.Height - h) / 2
	box := layout.Rect{X: x, Y: y, W: w, H: h}

	// Fill background
	canvas.Fill(box, RenderCell{
		Content: " ",
		Bg:      ColorOverlayBg,
	})
	// Draw border
	canvas.DrawBorder(box, true, " NestedTerm Keybindings ", r.borderChars)

	// Draw lines
	for i, line := range helpLines {
		if i >= h-3 {
			break
		}
		canvas.DrawString(x+2, y+1+i, line, ColorOverlayFg, ColorOverlayBg, 0)
	}

	footer := "Press any key to dismiss"
	canvas.DrawString(x+(w-len(footer))/2, y+h-2, footer, ColorActiveBorder, ColorOverlayBg, AttrBold)
}

func (r *Renderer) renderQuitConfirmOverlay(canvas *Canvas) {
	w := 50
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

	msg1 := "Managed terminal sessions are still running."
	msg2 := "Are you sure you want to terminate all and exit?"
	msg3 := "[Y] Yes, kill all and quit    [N / Esc] Cancel"

	canvas.DrawString(x+3, y+2, msg1, ColorOverlayFg, ColorOverlayBg, AttrBold)
	canvas.DrawString(x+3, y+3, msg2, ColorTitleInactive, ColorOverlayBg, 0)
	canvas.DrawString(x+3, y+5, msg3, ColorActiveBorder, ColorOverlayBg, AttrBold)
}

func (r *Renderer) renderBanner(canvas *Canvas, msg string) {
	if msg == "" || canvas.Height < 1 {
		return
	}
	y := canvas.Height - 1
	runes := []rune(msg)
	lenMsg := len(runes)
	if lenMsg > canvas.Width-4 {
		lenMsg = canvas.Width - 4
		runes = runes[:lenMsg]
	}
	x := canvas.Width - lenMsg - 2
	canvas.DrawString(x, y, fmt.Sprintf(" %s ", string(runes)), ColorActiveBorder, ColorOverlayBg, AttrBold)
}
