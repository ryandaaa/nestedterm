package workspace

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"nestedterm/internal/layout"
	"nestedterm/internal/pty"
)

func newTestWorkspace(t *testing.T) *Workspace {
	t.Helper()
	return NewWorkspace(layout.Rect{W: 100, H: 30}, 4, "/bin/sh", 200*time.Millisecond, nil)
}

func TestPaneExitNotifiesOwner(t *testing.T) {
	ws := newTestWorkspace(t)
	exited := make(chan PaneID, 1)
	ws.OnPaneExit = func(id PaneID) { exited <- id }

	p, err := ws.CreateInitialPane(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseAll()

	if _, err := p.Write([]byte("exit\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case id := <-exited:
		if id != p.ID {
			t.Fatalf("got exit for %s, want %s", id, p.ID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnPaneExit was not called after the shell exited")
	}

	// The owner reacts by closing the pane; the last pane leaves the workspace empty.
	empty, err := ws.ClosePane(p.ID)
	if err != nil || !empty {
		t.Fatalf("ClosePane = (%v, %v), want (true, nil)", empty, err)
	}
}

func TestPaneWriteDoesNotBlockWhenBacklogged(t *testing.T) {
	// No writer goroutine drains the queue, like a child that stopped reading.
	p := &Pane{ID: "p", Session: &pty.PTYSession{}, inputCh: make(chan []byte, 2)}

	for i := 0; i < 2; i++ {
		if _, err := p.Write([]byte("x")); err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
	}
	done := make(chan error, 1)
	go func() {
		_, err := p.Write([]byte("x"))
		done <- err
	}()
	select {
	case err := <-done:
		if !errors.Is(err, ErrInputBacklog) {
			t.Fatalf("got %v, want ErrInputBacklog", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Write blocked on a full input queue")
	}
}

func TestPaneTitleFollowsCd(t *testing.T) {
	ws := newTestWorkspace(t)
	start := t.TempDir()
	p, err := ws.CreateInitialPane(start)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseAll()

	target, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Write([]byte("cd " + target + "\n")); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if p.cachedCWD() == target {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("pane cwd stayed %q, want %q", p.cachedCWD(), target)
}

func TestCloseActivePaneIsRaceFree(t *testing.T) {
	ws := newTestWorkspace(t)
	if _, err := ws.CreateInitialPane(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	defer ws.CloseAll()
	for i := 0; i < 2; i++ {
		if _, err := ws.CreatePane(); err != nil {
			t.Fatal(err)
		}
	}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
				ws.FocusNext() // writes ActivePaneID under the lock
			}
		}
	}()
	time.Sleep(20 * time.Millisecond) // let the writer run many unsynchronized-with-us iterations

	if _, err := ws.CloseActivePane(); err != nil {
		t.Fatal(err)
	}
	close(stop)
	<-done
}

func TestPaneSelectedTextStaysInPane(t *testing.T) {
	ws := NewWorkspace(layout.Rect{W: 80, H: 24}, 4, "/bin/sh", 100*time.Millisecond, nil)
	p, err := ws.CreateInitialPane(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseAll()

	_, _ = p.Terminal.Write([]byte("\x1b[2J\x1b[Hhello world\r\nsecond line"))
	p.BeginSelection(6, 0)
	p.ExtendSelection(5, 1)
	if got := p.SelectedText(); got != "world\nsecond" {
		t.Fatalf("SelectedText = %q", got)
	}
	p.ClearSelection()
	if p.HasSelection() || p.SelectedText() != "" {
		t.Fatal("selection should be cleared")
	}
}
