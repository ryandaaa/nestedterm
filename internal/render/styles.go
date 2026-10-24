package render

import (
	"image/color"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
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
	SeqStrike     = "\033[9m"
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
		// Reset already restored the default colors, so only set explicit ones.
		if next.Fg != nil {
			dst = appendColor(dst, next.Fg, false)
		}
		if next.Bg != nil {
			dst = appendColor(dst, next.Bg, true)
		}
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

// colorKind distinguishes how a color is encoded on the wire. Palette colors
// are sent as indices so the host terminal's own theme applies; two colors
// that happen to share an RGB value but differ in kind must not be treated as
// equal, or the style diff would carry the wrong encoding forward.
type colorKind uint8

const (
	kindDefault colorKind = iota
	kindBasic             // ansi.BasicColor, SGR 30-37/90-97
	kindIndexed           // ansi.IndexedColor, SGR 38;5;n
	kindRGB
)

func colorKey(c color.Color) (colorKind, uint64) {
	switch v := c.(type) {
	case nil:
		return kindDefault, 0
	case ansi.BasicColor:
		return kindBasic, uint64(v)
	case ansi.IndexedColor:
		return kindIndexed, uint64(v)
	}
	r, g, b, a := c.RGBA()
	return kindRGB, uint64(r>>8)<<24 | uint64(g>>8)<<16 | uint64(b>>8)<<8 | uint64(a>>8)
}

func colorsEqual(a, b color.Color) bool {
	// Fast paths for the common cases; they never report a false "equal".
	switch av := a.(type) {
	case nil:
		return b == nil
	case ansi.BasicColor:
		bv, ok := b.(ansi.BasicColor)
		return ok && av == bv
	case ansi.IndexedColor:
		bv, ok := b.(ansi.IndexedColor)
		return ok && av == bv
	case color.RGBA:
		if bv, ok := b.(color.RGBA); ok && av == bv {
			return true
		}
	}

	ka, va := colorKey(a)
	kb, vb := colorKey(b)
	return ka == kb && va == vb
}

// trueColor controls whether RGB colors are sent as 24-bit SGR. When false they
// are approximated with the nearest xterm-256 color.
var trueColor = true

// SetTrueColor selects 24-bit (true) or 256-color (false) output for RGB colors.
func SetTrueColor(on bool) { trueColor = on }

// DetectTrueColor reports whether the host terminal advertises 24-bit color.
// NESTEDTERM_TRUECOLOR=0|1 overrides detection.
func DetectTrueColor() bool {
	switch os.Getenv("NESTEDTERM_TRUECOLOR") {
	case "1":
		return true
	case "0":
		return false
	}
	switch strings.ToLower(os.Getenv("COLORTERM")) {
	case "truecolor", "24bit":
		return true
	}
	term := strings.ToLower(os.Getenv("TERM"))
	return strings.Contains(term, "truecolor") || strings.Contains(term, "-direct")
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
	if attrs&AttrStrikethrough != 0 {
		dst = append(dst, SeqStrike...)
	}
	return dst
}

func appendColor(dst []byte, c color.Color, isBg bool) []byte {
	switch v := c.(type) {
	case nil:
		if isBg {
			return append(dst, "\033[49m"...)
		}
		return append(dst, "\033[39m"...)

	case ansi.BasicColor:
		n := int(v)
		base := 30
		if isBg {
			base = 40
		}
		if n >= 8 {
			base += 60
			n -= 8
		}
		dst = append(dst, "\033["...)
		dst = strconv.AppendInt(dst, int64(base+n), 10)
		return append(dst, 'm')

	case ansi.IndexedColor:
		return appendIndexed(dst, uint8(v), isBg)
	}

	if !trueColor {
		return appendIndexed(dst, uint8(ansi.Convert256(c)), isBg)
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

func appendIndexed(dst []byte, n uint8, isBg bool) []byte {
	if isBg {
		dst = append(dst, "\033[48;5;"...)
	} else {
		dst = append(dst, "\033[38;5;"...)
	}
	dst = strconv.AppendUint(dst, uint64(n), 10)
	return append(dst, 'm')
}
