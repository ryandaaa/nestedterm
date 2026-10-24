package render

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"
	"testing"
	"time"

	"nestedterm/internal/layout"
	"nestedterm/internal/terminal"
	"nestedterm/internal/workspace"
)

func TestCanvasBasics(t *testing.T) {
	c := NewCanvas(20, 10)
	if c.Width != 20 || c.Height != 10 {
		t.Fatalf("expected 20x10, got %dx%d", c.Width, c.Height)
	}

	c.SetCell(5, 5, RenderCell{Content: "X", Width: 1})
	cell := c.GetCell(5, 5)
	if cell.Content != "X" {
		t.Fatalf("expected 'X', got %q", cell.Content)
	}

	// Test Fill
	c.Fill(layout.Rect{X: 1, Y: 1, W: 3, H: 2}, RenderCell{Content: "#", Width: 1})
	if c.GetCell(1, 1).Content != "#" || c.GetCell(3, 2).Content != "#" {
		t.Fatal("Fill did not set cells correctly")
	}

	// Test DrawString
	c.DrawString(2, 4, "TEST", nil, nil, AttrBold)
	if c.GetCell(2, 4).Content != "T" || c.GetCell(5, 4).Content != "T" {
		t.Fatal("DrawString failed")
	}
	if c.GetCell(2, 4).Attrs&AttrBold == 0 {
		t.Fatal("expected AttrBold on DrawString cell")
	}
}

func TestSGRDiff(t *testing.T) {
	c1 := RenderCell{Content: "A", Width: 1}
	c2 := RenderCell{Content: "B", Width: 1, Attrs: AttrBold}
	diff := SGRDiff(c1, c2)
	if !strings.Contains(diff, SeqBold) {
		t.Fatalf("expected diff to contain SeqBold, got %q", diff)
	}

	c3 := RenderCell{Content: "C", Width: 1, Fg: color.RGBA{R: 255, G: 0, B: 0, A: 255}}
	diffColor := SGRDiff(c1, c3)
	if !strings.Contains(diffColor, "\033[38;2;255;0;0m") {
		t.Fatalf("expected truecolor sequence in diff, got %q", diffColor)
	}

	// Reset diff
	diffReset := SGRDiff(c2, c1)
	if !strings.Contains(diffReset, SeqReset) {
		t.Fatalf("expected reset sequence in diff, got %q", diffReset)
	}
}

func TestSGRStrikethrough(t *testing.T) {
	plain := RenderCell{Content: "A", Width: 1}
	struck := RenderCell{Content: "A", Width: 1, Attrs: AttrStrikethrough}
	if got := SGRDiff(plain, struck); got != SeqStrike {
		t.Fatalf("got %q, want %q", got, SeqStrike)
	}
	if got := SGRDiff(struck, plain); !strings.HasPrefix(got, SeqReset) {
		t.Fatalf("turning strikethrough off must reset, got %q", got)
	}
}

func TestRendererWorkspace(t *testing.T) {
	total := layout.Rect{X: 0, Y: 0, W: 80, H: 24}
	ws := workspace.NewWorkspace(total, 4, "/bin/bash", 100*time.Millisecond, nil)
	p, err := ws.CreateInitialPane(t.TempDir())
	if err != nil {
		t.Fatalf("CreateInitialPane failed: %v", err)
	}
	defer ws.CloseAll()

	// Write text into pane terminal
	_, _ = p.Terminal.Write([]byte("Hello from Pane 1\r\n"))

	renderer := NewRenderer(80, 24, "single")
	var buf bytes.Buffer
	err = renderer.Render(ws, OverlayState{}, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Hello from Pane 1") {
		t.Fatalf("expected output to contain pane text, got: %q", out)
	}
	if !strings.Contains(out, "┌") || !strings.Contains(out, "┐") {
		t.Fatal("expected output to contain borders")
	}

	// Test with Help Overlay
	buf.Reset()
	helpState := OverlayState{
		ShowHelp: true,
		HelpText: []string{"Ctrl+Space n : New pane", "Ctrl+Space q : Quit"},
	}
	err = renderer.Render(ws, helpState, &buf)
	if err != nil {
		t.Fatalf("Render with help failed: %v", err)
	}
	if !strings.Contains(buf.String(), "NestedTerm Keybindings") {
		t.Fatal("expected help overlay title in output")
	}

	// Test with Quit Confirm Overlay
	buf.Reset()
	quitState := OverlayState{
		ShowQuitConfirm: true,
	}
	err = renderer.Render(ws, quitState, &buf)
	if err != nil {
		t.Fatalf("Render with quit confirm failed: %v", err)
	}
	if !strings.Contains(buf.String(), "Confirm Quit") {
		t.Fatal("expected quit confirm overlay in output")
	}
}

type benchMode int

const (
	benchSteady benchMode = iota // nothing changes between frames
	benchTyping                  // a few cells change per frame
	benchScroll                  // output scrolls the active pane each frame
)

func benchmarkRender(b *testing.B, cols, rows, panes int, mode benchMode) {
	total := layout.Rect{X: 0, Y: 0, W: cols, H: rows}
	ws := workspace.NewWorkspace(total, 4, "/bin/sh", 100*time.Millisecond, nil)
	if _, err := ws.CreateInitialPane(b.TempDir()); err != nil {
		b.Fatalf("CreateInitialPane failed: %v", err)
	}
	defer ws.CloseAll()
	for i := 1; i < panes; i++ {
		if _, err := ws.CreatePane(); err != nil {
			b.Fatalf("CreatePane failed: %v", err)
		}
	}

	var line strings.Builder
	for i := 0; i < rows*2; i++ {
		line.WriteString("\033[32mgreen \033[0mplain \033[1;31mbold red\033[0m the quick brown fox jumps over the lazy dog 0123456789\r\n")
	}
	for _, p := range ws.AllPanes() {
		_, _ = p.Terminal.Write([]byte(line.String()))
	}

	renderer := NewRenderer(cols, rows, "single")
	var buf bytes.Buffer
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		switch mode {
		case benchTyping:
			_, _ = ws.GetActivePane().Terminal.Write([]byte(fmt.Sprintf("\033[1;1H%08d", i)))
		case benchScroll:
			_, _ = ws.GetActivePane().Terminal.Write([]byte(fmt.Sprintf("unique line %d\r\n", i)))
		}
		if err := renderer.Render(ws, OverlayState{}, &buf); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(buf.Len()), "bytes/frame")
}

func BenchmarkRender80x24Steady(b *testing.B)    { benchmarkRender(b, 80, 24, 1, benchSteady) }
func BenchmarkRender200x50x4Steady(b *testing.B) { benchmarkRender(b, 200, 50, 4, benchSteady) }
func BenchmarkRender200x50x4Typing(b *testing.B) { benchmarkRender(b, 200, 50, 4, benchTyping) }
func BenchmarkRender200x50x4Scroll(b *testing.B) { benchmarkRender(b, 200, 50, 4, benchScroll) }

// replay feeds frames into a fresh emulator and returns it.
func replay(t *testing.T, cols, rows int, frames ...[]byte) *terminal.VTAdapter {
	t.Helper()
	emu := terminal.NewAdapter(cols, rows, 100)
	for _, f := range frames {
		if _, err := emu.Write(f); err != nil {
			t.Fatal(err)
		}
	}
	return emu
}

func assertSameScreen(t *testing.T, cols, rows int, got, want *terminal.VTAdapter, label string) {
	t.Helper()
	for y := 0; y < rows; y++ {
		for x := 0; x < cols; x++ {
			g, w := got.CellAt(x, y), want.CellAt(x, y)
			if g.Content != w.Content || g.Width != w.Width || g.Attrs != w.Attrs ||
				!colorsEqual(g.Fg, w.Fg) || !colorsEqual(g.Bg, w.Bg) {
				t.Fatalf("%s: cell (%d,%d) differs: diff-rendered %+v, full-rendered %+v", label, x, y, *g, *w)
			}
		}
	}
}

func TestRenderDiffMatchesFullRepaint(t *testing.T) {
	const cols, rows = 100, 30
	ws := workspace.NewWorkspace(layout.Rect{W: cols, H: rows}, 4, "/bin/sh", 100*time.Millisecond, nil)
	if _, err := ws.CreateInitialPane(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer ws.CloseAll()
	if _, err := ws.CreatePane(); err != nil {
		t.Fatal(err)
	}

	renderer := NewRenderer(cols, rows, "single")
	var frames [][]byte
	snap := func(o OverlayState) {
		var b bytes.Buffer
		if err := renderer.Render(ws, o, &b); err != nil {
			t.Fatal(err)
		}
		frames = append(frames, append([]byte(nil), b.Bytes()...))
	}

	panes := ws.AllPanes()
	snap(OverlayState{})
	writes := []string{
		"hello\r\n",
		"\033[1;31mred bold\033[0m and \033[44mblue bg\033[0m\r\n",
		"wide: 漢字 mixed\r\n",
		"\033[3;5Hoverwrite",
		strings.Repeat("scroll line\r\n", 40),
		"\033[2J\033[Hcleared",
	}
	for i, w := range writes {
		_, _ = panes[i%len(panes)].Terminal.Write([]byte(w))
		snap(OverlayState{})
		if i == 2 {
			snap(OverlayState{ShowHelp: true, HelpText: []string{"one", "two"}})
			snap(OverlayState{BannerMessage: "[ctrl+space]"})
			snap(OverlayState{})
		}
	}

	// A no-op frame must emit nothing.
	var idle bytes.Buffer
	if err := renderer.Render(ws, OverlayState{}, &idle); err != nil {
		t.Fatal(err)
	}
	if idle.Len() != 0 {
		t.Fatalf("unchanged frame wrote %d bytes: %q", idle.Len(), idle.String())
	}

	// Replaying every diff frame must equal a from-scratch full repaint.
	var full bytes.Buffer
	if err := NewRenderer(cols, rows, "single").Render(ws, OverlayState{}, &full); err != nil {
		t.Fatal(err)
	}
	assertSameScreen(t, cols, rows, replay(t, cols, rows, frames...), replay(t, cols, rows, full.Bytes()), "diff vs full")

	// Invalidate must produce a full repaint on its own.
	renderer.Invalidate()
	var again bytes.Buffer
	if err := renderer.Render(ws, OverlayState{}, &again); err != nil {
		t.Fatal(err)
	}
	if again.Len() < cols*rows/2 {
		t.Fatalf("Invalidate did not trigger a full repaint (%d bytes)", again.Len())
	}
	assertSameScreen(t, cols, rows, replay(t, cols, rows, again.Bytes()), replay(t, cols, rows, full.Bytes()), "invalidate vs full")
}
