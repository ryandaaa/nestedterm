package render

import (
	"image/color"
	"strconv"
)

// ANSI sequence constants
const (
	SeqReset      = "\033[0m"
	SeqBold       = "\033[1m"
	SeqDim        = "\033[2m"
	SeqItalic     = "\033[3m"
	SeqUnderline  = "\033[4m"
	SeqBlink      = "\033[5m"
	SeqReverse    = "\033[7m"
	SeqHideCursor = "\033[?25l"
	SeqShowCursor = "\033[?25h"
	SeqHomeCursor = "\033[H"
	SeqClearAll   = "\033[2J"
	SeqSyncBegin  = "\033[?2026h"
	SeqSyncEnd    = "\033[?2026l"
)

// SGRDiff generates minimal ANSI escape sequences needed to transition from prev to next style.
func SGRDiff(prev, next RenderCell) string {
	return string(appendSGRDiff(nil, prev, next))
}

// appendSGRDiff appends the SGR transition from prev to next to dst.
func appendSGRDiff(dst []byte, prev, next RenderCell) []byte {
	if stylesEqual(prev, next) {
		return dst
	}

	// If attributes were turned off, reset all and re-apply
	if (prev.Attrs&^next.Attrs) != 0 || (prev.Fg != nil && next.Fg == nil) || (prev.Bg != nil && next.Bg == nil) {
		dst = append(dst, SeqReset...)
		dst = appendAttrs(dst, next.Attrs)
		dst = appendColor(dst, next.Fg, false)
		dst = appendColor(dst, next.Bg, true)
		return dst
	}

	dst = appendAttrs(dst, next.Attrs&^prev.Attrs)
	if !colorsEqual(prev.Fg, next.Fg) {
		dst = appendColor(dst, next.Fg, false)
	}
	if !colorsEqual(prev.Bg, next.Bg) {
		dst = appendColor(dst, next.Bg, true)
	}
	return dst
}

// cellsEqual reports whether two cells would paint identically.
func cellsEqual(a, b *RenderCell) bool {
	return a.Content == b.Content && a.Width == b.Width && a.Attrs == b.Attrs &&
		a.Continuation == b.Continuation && colorsEqual(a.Fg, b.Fg) && colorsEqual(a.Bg, b.Bg)
}

// appendCursorTo appends a CUP sequence for the 0-indexed cell (x, y).
func appendCursorTo(dst []byte, x, y int) []byte {
	dst = append(dst, "\033["...)
	dst = strconv.AppendInt(dst, int64(y+1), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(x+1), 10)
	return append(dst, 'H')
}

func stylesEqual(a, b RenderCell) bool {
	return a.Attrs == b.Attrs && colorsEqual(a.Fg, b.Fg) && colorsEqual(a.Bg, b.Bg)
}

func colorsEqual(a, b color.Color) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}

func appendAttrs(dst []byte, attrs uint8) []byte {
	if attrs&AttrBold != 0 {
		dst = append(dst, SeqBold...)
	}
	if attrs&AttrDim != 0 {
		dst = append(dst, SeqDim...)
	}
	if attrs&AttrItalic != 0 {
		dst = append(dst, SeqItalic...)
	}
	if attrs&AttrUnderline != 0 {
		dst = append(dst, SeqUnderline...)
	}
	if attrs&AttrBlink != 0 {
		dst = append(dst, SeqBlink...)
	}
	if attrs&AttrReverse != 0 {
		dst = append(dst, SeqReverse...)
	}
	return dst
}

func appendColor(dst []byte, c color.Color, isBg bool) []byte {
	if c == nil {
		if isBg {
			return append(dst, "\033[49m"...)
		}
		return append(dst, "\033[39m"...)
	}

	r, g, b, _ := c.RGBA()
	if isBg {
		dst = append(dst, "\033[48;2;"...)
	} else {
		dst = append(dst, "\033[38;2;"...)
	}
	dst = strconv.AppendUint(dst, uint64(r>>8), 10)
	dst = append(dst, ';')
	dst = strconv.AppendUint(dst, uint64(g>>8), 10)
	dst = append(dst, ';')
	dst = strconv.AppendUint(dst, uint64(b>>8), 10)
	return append(dst, 'm')
}
