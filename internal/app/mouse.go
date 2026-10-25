package app

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"nestedterm/internal/keyboard"
	"nestedterm/internal/workspace"
)

// wheelStep is how many lines one wheel notch scrolls.
const wheelStep = 3

// handleMouse implements per-pane text selection and wheel scrolling. Because
// the app owns the mouse, the host terminal no longer selects across panes.
func (a *App) handleMouse(m keyboard.MouseEvent) {
	if a.overlays.ShowHelp || a.overlays.ShowQuitConfirm {
		return
	}

	switch {
	case m.IsWheelUp() || m.IsWheelDown():
		a.clearSelections()
		if p := a.paneAt(m.X, m.Y); p != nil {
			a.workspace.SetActivePane(p.ID)
			if m.IsWheelUp() {
				p.ScrollUp(wheelStep)
			} else {
				p.ScrollDown(wheelStep)
			}
		}
		a.syncScrollMode()

	case m.Button != 0:
		// Middle/right buttons are not used.

	case m.Release:
		p := a.workspace.GetPane(a.dragPane)
		a.dragPane = ""
		if p == nil {
			return
		}
		if text := p.SelectedText(); text != "" {
			copyToClipboard(text)
			a.overlays.BannerMessage = fmt.Sprintf("Copied %d chars", len([]rune(text)))
		} else {
			p.ClearSelection()
		}

	case m.Motion:
		if p := a.workspace.GetPane(a.dragPane); p != nil {
			p.ExtendSelection(m.X-p.InnerRect.X, m.Y-p.InnerRect.Y)
		}

	default: // left button press
		a.clearSelections()
		a.overlays.BannerMessage = ""
		p := a.paneAt(m.X, m.Y)
		if p == nil {
			return
		}
		a.workspace.SetActivePane(p.ID)
		a.syncScrollMode()
		if p.InnerRect.Contains(m.X, m.Y) {
			p.BeginSelection(m.X-p.InnerRect.X, m.Y-p.InnerRect.Y)
			a.dragPane = p.ID
		}
	}
}

func (a *App) paneAt(x, y int) *workspace.Pane {
	for _, p := range a.workspace.AllPanes() {
		if p.Rect.Contains(x, y) {
			return p
		}
	}
	return nil
}

func (a *App) clearSelections() {
	a.dragPane = ""
	for _, p := range a.workspace.AllPanes() {
		p.ClearSelection()
	}
}

// syncScrollMode keeps the key decoder in step with the active pane's
// scrollback state, which the wheel can change without a key press.
func (a *App) syncScrollMode() {
	p := a.workspace.GetActivePane()
	switch {
	case p != nil && p.InScrollback && a.decoder.Mode == keyboard.ModeNormal:
		a.decoder.Mode = keyboard.ModeScrollback
	case (p == nil || !p.InScrollback) && a.decoder.Mode == keyboard.ModeScrollback:
		a.decoder.ResetMode()
	}
}

// copyToClipboard sets the system clipboard through OSC 52 (works over SSH and
// in most modern terminals) and, when available, a local clipboard tool for
// terminals that ignore OSC 52.
func copyToClipboard(text string) {
	_, _ = fmt.Fprintf(os.Stdout, "\033]52;c;%s\a", base64.StdEncoding.EncodeToString([]byte(text)))

	for _, argv := range [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
	} {
		path, err := exec.LookPath(argv[0])
		if err != nil {
			continue
		}
		cmd := exec.Command(path, argv[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if cmd.Start() == nil {
			go func() { _ = cmd.Wait() }()
			return
		}
	}
}
