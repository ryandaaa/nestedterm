package render

import (
	"fmt"
	"image/color"
	"strings"
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
)

// SGRDiff generates minimal ANSI escape sequences needed to transition from prev to next style.
func SGRDiff(prev, next RenderCell) string {
	if stylesEqual(prev, next) {
		return ""
	}

	var sb strings.Builder

	// If attributes were turned off, reset all and re-apply
	if (prev.Attrs&^next.Attrs) != 0 || (prev.Fg != nil && next.Fg == nil) || (prev.Bg != nil && next.Bg == nil) {
		sb.WriteString(SeqReset)
		applyAttrs(&sb, next.Attrs)
		applyColor(&sb, next.Fg, false)
		applyColor(&sb, next.Bg, true)
		return sb.String()
	}

	// Apply new attributes
	newAttrs := next.Attrs &^ prev.Attrs
	applyAttrs(&sb, newAttrs)

	// Apply foreground if changed
	if !colorsEqual(prev.Fg, next.Fg) {
		applyColor(&sb, next.Fg, false)
	}

	// Apply background if changed
	if !colorsEqual(prev.Bg, next.Bg) {
		applyColor(&sb, next.Bg, true)
	}

	return sb.String()
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

func applyAttrs(sb *strings.Builder, attrs uint8) {
	if attrs&AttrBold != 0 {
		sb.WriteString(SeqBold)
	}
	if attrs&AttrDim != 0 {
		sb.WriteString(SeqDim)
	}
	if attrs&AttrItalic != 0 {
		sb.WriteString(SeqItalic)
	}
	if attrs&AttrUnderline != 0 {
		sb.WriteString(SeqUnderline)
	}
	if attrs&AttrBlink != 0 {
		sb.WriteString(SeqBlink)
	}
	if attrs&AttrReverse != 0 {
		sb.WriteString(SeqReverse)
	}
}

func applyColor(sb *strings.Builder, c color.Color, isBg bool) {
	if c == nil {
		if isBg {
			sb.WriteString("\033[49m")
		} else {
			sb.WriteString("\033[39m")
		}
		return
	}

	r, g, b, _ := c.RGBA()
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)

	prefix := 38
	if isBg {
		prefix = 48
	}
	sb.WriteString(fmt.Sprintf("\033[%d;2;%d;%d;%dm", prefix, r8, g8, b8))
}
