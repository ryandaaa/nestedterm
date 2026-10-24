package app

import (
	"os"
	"testing"
	"time"

	"nestedterm/internal/config"
	"nestedterm/internal/keyboard"
	"nestedterm/internal/layout"
	"nestedterm/internal/render"
	"nestedterm/internal/workspace"
)

func TestAppKeyHandling(t *testing.T) {
	cfg := config.NewDefaultConfig()
	app := NewApp(cfg)

	total := layout.Rect{X: 0, Y: 0, W: 80, H: 24}
	app.workspace = workspace.NewWorkspace(total, 4, "/bin/bash", 100*time.Millisecond, nil)
	p1, err := app.workspace.CreateInitialPane(os.TempDir())
	if err != nil {
		t.Fatalf("CreateInitialPane failed: %v", err)
	}
	defer app.workspace.CloseAll()

	app.renderer = render.NewRenderer(80, 24, "single")

	// 1. Test ActionHelp opens help overlay
	exit := app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionHelp})
	if exit {
		t.Fatal("unexpected exit on ActionHelp")
	}
	if !app.overlays.ShowHelp {
		t.Fatal("expected ShowHelp to be true")
	}

	// Any subsequent key dismisses help
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionNone, RawBytes: []byte("a")})
	if exit {
		t.Fatal("unexpected exit on dismissing help")
	}
	if app.overlays.ShowHelp {
		t.Fatal("expected ShowHelp to be false after keypress")
	}

	// 2. Test ActionNewPane creates pane 2
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionNewPane})
	if exit {
		t.Fatal("unexpected exit on ActionNewPane")
	}
	if app.workspace.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", app.workspace.PaneCount())
	}

	// 3. Test ActionQuit with live panes prompts quit confirm modal
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionQuit})
	if exit {
		t.Fatal("ActionQuit should not exit immediately when live panes exist")
	}
	if !app.overlays.ShowQuitConfirm {
		t.Fatal("expected ShowQuitConfirm to be true")
	}

	// Cancel quit with 'n'
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionNone, RawBytes: []byte("n")})
	if exit {
		t.Fatal("unexpected exit when cancelling quit")
	}
	if app.overlays.ShowQuitConfirm {
		t.Fatal("expected ShowQuitConfirm to be false after 'n'")
	}

	// 4. Test Scrollback toggle
	active := app.workspace.GetActivePane()
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionScrollback})
	if exit {
		t.Fatal("unexpected exit on ActionScrollback")
	}
	if !active.InScrollback {
		t.Fatal("expected active pane InScrollback == true")
	}

	// Exit scrollback with 'q'
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionNone, RawBytes: []byte("q")})
	if exit {
		t.Fatal("unexpected exit on 'q' in scrollback")
	}
	if active.InScrollback {
		t.Fatal("expected active pane InScrollback == false after 'q'")
	}

	// 5. Test ActionClosePane on active pane
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionClosePane})
	if exit {
		t.Fatal("closing 1 of 2 panes should not exit")
	}
	if app.workspace.PaneCount() != 1 {
		t.Fatalf("expected 1 pane remaining, got %d", app.workspace.PaneCount())
	}
	if app.workspace.ActivePaneID != p1.ID {
		t.Fatalf("expected active pane %s, got %s", p1.ID, app.workspace.ActivePaneID)
	}

	// Closing last pane should exit
	exit = app.handleKeyEvent(keyboard.DecodeEvent{Action: keyboard.ActionClosePane})
	if !exit {
		t.Fatal("closing last pane should return exit == true")
	}
}

func TestSendDoesNotBlockAfterStop(t *testing.T) {
	a := NewApp(config.NewDefaultConfig())
	for i := 0; i < cap(a.eventCh); i++ {
		a.eventCh <- AppEvent{Type: EventKey}
	}
	close(a.stopCh)

	done := make(chan struct{})
	go func() {
		a.send(AppEvent{Type: EventShutdown}) // channel is full; must return via stopCh
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("send blocked on a full eventCh after stopCh was closed")
	}
}

func TestRedrawSignalsCoalesce(t *testing.T) {
	a := NewApp(config.NewDefaultConfig())
	for i := 0; i < 1000; i++ {
		select {
		case a.redrawCh <- struct{}{}:
		default:
		}
	}
	if got := len(a.redrawCh); got != 1 {
		t.Fatalf("expected 1 pending redraw signal, got %d", got)
	}
}
