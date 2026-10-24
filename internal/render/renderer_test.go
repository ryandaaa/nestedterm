package render

import (
	"bytes"
	"image/color"
	"strings"
	"testing"
	"time"

	"nestedterm/internal/layout"
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
