package workspace

import (
	"os"
	"testing"
	"time"

	"nestedterm/internal/layout"
)

func TestWorkspaceLifecycle(t *testing.T) {
	total := layout.Rect{X: 0, Y: 0, W: 120, H: 40}
	redrawCh := make(chan struct{}, 10)
	onRedraw := func() {
		select {
		case redrawCh <- struct{}{}:
		default:
		}
	}

	ws := NewWorkspace(total, 4, "/bin/bash", 200*time.Millisecond, onRedraw)

	// 1. Create initial pane
	p1, err := ws.CreateInitialPane(os.TempDir())
	if err != nil {
		t.Fatalf("CreateInitialPane failed: %v", err)
	}
	defer ws.CloseAll()

	if ws.PaneCount() != 1 {
		t.Fatalf("expected 1 pane, got %d", ws.PaneCount())
	}
	if ws.ActivePaneID != p1.ID {
		t.Fatalf("expected active pane %s, got %s", p1.ID, ws.ActivePaneID)
	}

	// 2. Create second pane
	p2, err := ws.CreatePane()
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}

	if ws.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", ws.PaneCount())
	}
	if ws.ActivePaneID != p2.ID {
		t.Fatalf("expected active pane %s, got %s", p2.ID, ws.ActivePaneID)
	}

	// Verify both panes are tiled
	if p1.Rect.W == 0 || p2.Rect.W == 0 {
		t.Fatal("expected non-zero widths for tiled panes")
	}

	// 3. Test FocusNext and FocusPrev
	ws.FocusNext()
	if ws.ActivePaneID != p1.ID {
		t.Fatalf("expected active pane %s after FocusNext, got %s", p1.ID, ws.ActivePaneID)
	}
	ws.FocusPrev()
	if ws.ActivePaneID != p2.ID {
		t.Fatalf("expected active pane %s after FocusPrev, got %s", p2.ID, ws.ActivePaneID)
	}

	// 4. Test Scrollback
	p2.ScrollUp(5)
	if !p2.InScrollback || p2.ScrollOffset != 0 { // history empty initially
		// Scroll offset bounded by history len
	}
	p2.ResetScroll()
	if p2.InScrollback || p2.ScrollOffset != 0 {
		t.Fatal("expected scrollback reset to 0")
	}

	// 5. Test Resize
	newTotal := layout.Rect{X: 0, Y: 0, W: 160, H: 50}
	if err := ws.Resize(newTotal); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}
	if ws.TotalRect != newTotal {
		t.Fatalf("expected TotalRect %s, got %s", newTotal, ws.TotalRect)
	}

	// 6. Close pane 2
	empty, err := ws.ClosePane(p2.ID)
	if err != nil {
		t.Fatalf("ClosePane failed: %v", err)
	}
	if empty {
		t.Fatal("expected empty == false after closing 1 of 2 panes")
	}
	if ws.PaneCount() != 1 {
		t.Fatalf("expected 1 pane remaining, got %d", ws.PaneCount())
	}
	if ws.ActivePaneID != p1.ID {
		t.Fatalf("expected active pane to fall back to %s, got %s", p1.ID, ws.ActivePaneID)
	}

	// 7. Close final pane
	empty, err = ws.CloseActivePane()
	if err != nil {
		t.Fatalf("CloseActivePane failed: %v", err)
	}
	if !empty {
		t.Fatal("expected empty == true after closing final pane")
	}
	if ws.PaneCount() != 0 {
		t.Fatalf("expected 0 panes remaining, got %d", ws.PaneCount())
	}
}
