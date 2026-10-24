package pty

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"nestedterm/internal/terminal"
)

func TestPTYSpawnAndIO(t *testing.T) {
	tmpDir := t.TempDir()
	sess, err := SpawnSession("test-1", "/bin/bash", tmpDir, 80, 24)
	if err != nil {
		t.Fatalf("SpawnSession failed: %v", err)
	}
	defer sess.Close(200 * time.Millisecond)

	if sess.PID <= 0 {
		t.Fatalf("invalid PID: %d", sess.PID)
	}
	if sess.SessionID <= 0 {
		t.Fatalf("invalid SessionID: %d", sess.SessionID)
	}

	// Verify CWD
	cwd := sess.GetCWD()
	if !strings.HasSuffix(cwd, tmpDir) && cwd != tmpDir {
		t.Fatalf("expected CWD %s, got %s", tmpDir, cwd)
	}

	emu := terminal.NewAdapter(80, 24, 100)
	defer emu.Close()

	var redrawCount int
	var mu sync.Mutex
	onRedraw := func() {
		mu.Lock()
		redrawCount++
		mu.Unlock()
	}

	exitChan := make(chan int, 1)
	onExit := func(code int) {
		exitChan <- code
	}

	go RunSessionIO(sess, emu, onRedraw, onExit)

	// Give bash time to start
	time.Sleep(500 * time.Millisecond)
	_, err = sess.Write([]byte("echo PTY_TEST_OUTPUT_123\n"))
	if err != nil {
		t.Fatalf("sess.Write failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := redrawCount
	mu.Unlock()

	if count == 0 {
		t.Fatal("expected onRedraw to be triggered")
	}

	// Test Resize
	if err := sess.Resize(100, 30); err != nil {
		t.Fatalf("Resize failed: %v", err)
	}

	// Test natural exit
	_, err = sess.Write([]byte("exit 0\n"))
	if err != nil {
		t.Fatalf("sess.Write exit failed: %v", err)
	}

	select {
	case code := <-exitChan:
		if code != 0 {
			t.Fatalf("expected exit code 0, got %d", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for shell to exit, emu string: %q", emu.CellAt(0, 0).Content)
	}

	if !sess.IsExited() {
		t.Fatal("expected sess.IsExited() == true")
	}
}

func TestCleanupSession(t *testing.T) {
	sess, err := SpawnSession("test-cleanup", "/bin/bash", os.TempDir(), 80, 24)
	if err != nil {
		t.Fatalf("SpawnSession failed: %v", err)
	}

	sid := sess.SessionID
	// Close session with grace period
	if err := sess.Close(300 * time.Millisecond); err != nil {
		t.Fatalf("sess.Close failed: %v", err)
	}

	// Verify all processes in session are terminated
	pids, err := FindSessionPIDs(sid)
	if err != nil {
		t.Fatalf("FindSessionPIDs failed: %v", err)
	}
	if len(pids) != 0 {
		t.Fatalf("expected 0 remaining processes in session, found: %v", pids)
	}
}
