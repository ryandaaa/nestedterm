package workspace

import "strings"

import "nestedterm/internal/terminal"

// selection is a text selection in pane-inner coordinates of the visible view.
type selection struct {
	active         bool
	ax, ay, ex, ey int // anchor and current end
}

// ViewRow copies row y of what the pane currently shows (live screen or
// scrollback view) into dst.
func (p *Pane) ViewRow(y int, dst []terminal.TerminalCell) {
	p.mu.RLock()
	term := p.Terminal
	offset := p.ScrollOffset
	inScroll := p.InScrollback
	p.mu.RUnlock()
	if term == nil {
		return
	}

	if inScroll && offset > 0 {
		histLen := term.ScrollbackLen()
		abs := histLen - offset + y
		if abs < histLen {
			term.ReadScrollbackRow(abs, dst)
		} else {
			term.ReadRow(abs-histLen, dst)
		}
		return
	}
	term.ReadRow(y, dst)
}

// BeginSelection anchors a new selection at inner-relative cell (x, y).
func (p *Pane) BeginSelection(x, y int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	x, y = p.clampInner(x, y)
	p.sel = selection{active: true, ax: x, ay: y, ex: x, ey: y}
}

// ExtendSelection moves the free end of the selection to (x, y).
func (p *Pane) ExtendSelection(x, y int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.sel.active {
		return
	}
	p.sel.ex, p.sel.ey = p.clampInner(x, y)
}

// ClearSelection drops any selection.
func (p *Pane) ClearSelection() {
	p.mu.Lock()
	p.sel = selection{}
	p.mu.Unlock()
}

// HasSelection reports whether a non-empty selection exists.
func (p *Pane) HasSelection() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.sel.active && (p.sel.ax != p.sel.ex || p.sel.ay != p.sel.ey)
}

func (p *Pane) clampInner(x, y int) (int, int) {
	return clamp(x, 0, p.InnerRect.W-1), clamp(y, 0, p.InnerRect.H-1)
}

func clamp(v, lo, hi int) int {
	if v > hi {
		v = hi
	}
	if v < lo {
		v = lo
	}
	return v
}

// bounds returns the selection ordered start→end. Callers hold p.mu.
func (p *Pane) bounds() (sx, sy, ex, ey int, ok bool) {
	s := p.sel
	if !s.active || (s.ax == s.ex && s.ay == s.ey) {
		return 0, 0, 0, 0, false
	}
	sx, sy, ex, ey = s.ax, s.ay, s.ex, s.ey
	if sy > ey || (sy == ey && sx > ex) {
		sx, sy, ex, ey = ex, ey, sx, sy
	}
	return sx, sy, ex, ey, true
}

// SelectionSpan returns the selected column range [x0, x1] of inner row y.
func (p *Pane) SelectionSpan(y int) (x0, x1 int, ok bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	sx, sy, ex, ey, ok := p.bounds()
	if !ok || y < sy || y > ey {
		return 0, 0, false
	}
	x0, x1 = 0, p.InnerRect.W-1
	if y == sy {
		x0 = sx
	}
	if y == ey {
		x1 = ex
	}
	return x0, x1, true
}

// SelectedText returns the selected text, one line per row with trailing
// blanks trimmed.
func (p *Pane) SelectedText() string {
	p.mu.RLock()
	_, sy, _, ey, ok := p.bounds()
	w := p.InnerRect.W
	p.mu.RUnlock()
	if !ok || w <= 0 {
		return ""
	}

	row := make([]terminal.TerminalCell, w)
	lines := make([]string, 0, ey-sy+1)
	for y := sy; y <= ey; y++ {
		x0, x1, ok := p.SelectionSpan(y)
		if !ok {
			continue
		}
		p.ViewRow(y, row)
		var b strings.Builder
		for x := x0; x <= x1 && x < w; x++ {
			c := &row[x]
			if c.Content == "" {
				b.WriteByte(' ')
			} else {
				b.WriteString(c.Content)
			}
			if c.Width == 2 {
				x++ // skip the continuation cell
			}
		}
		lines = append(lines, strings.TrimRight(b.String(), " "))
	}
	return strings.Join(lines, "\n")
}
