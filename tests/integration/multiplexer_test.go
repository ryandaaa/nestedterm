package integration

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nestedterm/internal/layout"
	"nestedterm/internal/pty"
	"nestedterm/internal/workspace"
)

func TestIntegrationMultiPaneMultiplexing(t *testing.T) {
	total := layout.Rect{X: 0, Y: 0, W: 120, H: 40}
	dirA := t.TempDir()
	dirB := t.TempDir()

	var redrawCount int64
	ws := workspace.NewWorkspace(total, 6, "/bin/bash", 300*time.Millisecond, func() {
		atomic.AddInt64(&redrawCount, 1)
	})

	// 1. Initial pane in dirA
	p1, err := ws.CreateInitialPane(dirA)
	if err != nil {
		t.Fatalf("CreateInitialPane failed: %v", err)
	}
	defer ws.CloseAll()

	time.Sleep(300 * time.Millisecond)

	// Verify p1 CWD
	p1CWD := p1.GetCWD()
	if !strings.HasSuffix(p1CWD, dirA) && p1CWD != dirA {
		t.Fatalf("expected p1 CWD %s, got %s", dirA, p1CWD)
	}

	// 2. Spawn second pane (p2)
	p2, err := ws.CreatePane()
	if err != nil {
		t.Fatalf("CreatePane failed: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	if ws.PaneCount() != 2 {
		t.Fatalf("expected 2 panes, got %d", ws.PaneCount())
	}
	if ws.ActivePaneID != p2.ID {
		t.Fatalf("expected active pane %s, got %s", p2.ID, ws.ActivePaneID)
	}

	// Change dir in p2
	_, err = p2.Write([]byte("cd " + dirB + "\n"))
	if err != nil {
		t.Fatalf("write to p2 failed: %v", err)
	}

	var p2CWD string
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p2CWD = p2.GetCWD()
		if strings.HasSuffix(p2CWD, dirB) || p2CWD == dirB {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !strings.HasSuffix(p2CWD, dirB) && p2CWD != dirB {
		t.Fatalf("expected p2 CWD to change to %s, got %s", dirB, p2CWD)
	}
	// Verify p1 CWD did NOT change
	if p1.GetCWD() != p1CWD {
		t.Fatalf("p1 CWD should remain %s, got %s", p1CWD, p1.GetCWD())
	}

	// 3. Test independent input and simultaneous output
	_, _ = p1.Write([]byte("echo P1_EXCLUSIVE_OUTPUT\n"))
	_, _ = p2.Write([]byte("echo P2_EXCLUSIVE_OUTPUT\n"))

	var p1Screen, p2Screen string
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		p1Screen = dumpScreen(p1)
		p2Screen = dumpScreen(p2)
		if strings.Contains(p1Screen, "P1_EXCLUSIVE_OUTPUT") && strings.Contains(p2Screen, "P2_EXCLUSIVE_OUTPUT") {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !strings.Contains(p1Screen, "P1_EXCLUSIVE_OUTPUT") {
		t.Fatalf("expected p1 to contain P1_EXCLUSIVE_OUTPUT, got:\n%s", p1Screen)
	}
	if strings.Contains(p1Screen, "P2_EXCLUSIVE_OUTPUT") {
		t.Fatalf("p1 terminal should NOT contain P2_EXCLUSIVE_OUTPUT")
	}

	// Verify p2 terminal has P2_EXCLUSIVE_OUTPUT
	if !strings.Contains(p2Screen, "P2_EXCLUSIVE_OUTPUT") {
		t.Fatalf("expected p2 to contain P2_EXCLUSIVE_OUTPUT, got:\n%s", p2Screen)
	}
	if strings.Contains(p2Screen, "P1_EXCLUSIVE_OUTPUT") {
		t.Fatalf("p2 terminal should NOT contain P1_EXCLUSIVE_OUTPUT")
	}

	// 4. Test 3rd and 4th panes simultaneous visibility
	p3, err := ws.CreatePane()
	if err != nil {
		t.Fatalf("CreatePane p3 failed: %v", err)
	}
	p4, err := ws.CreatePane()
	if err != nil {
		t.Fatalf("CreatePane p4 failed: %v", err)
	}

	if ws.PaneCount() != 4 {
		t.Fatalf("expected 4 panes, got %d", ws.PaneCount())
	}

	// All 4 panes must have non-zero inner dimensions
	panes := []*workspace.Pane{p1, p2, p3, p4}
	for i, p := range panes {
		if p.InnerRect.W < 10 || p.InnerRect.H < 3 {
			t.Fatalf("pane %d inner dimensions %s too small", i+1, p.InnerRect)
		}
	}

	// 5. Directional focus movement
	ws.FocusDirection(layout.DirectionLeft)
	ws.FocusDirection(layout.DirectionUp)
	ws.FocusDirection(layout.DirectionRight)
	ws.FocusDirection(layout.DirectionDown)

	// 6. Close p3 and verify remaining panes re-balance without killing processes
	p1PID := p1.Session.PID
	p2PID := p2.Session.PID
	p4PID := p4.Session.PID

	empty, err := ws.ClosePane(p3.ID)
	if err != nil || empty {
		t.Fatalf("ClosePane p3 failed: empty=%v, err=%v", empty, err)
	}

	if ws.PaneCount() != 3 {
		t.Fatalf("expected 3 panes remaining, got %d", ws.PaneCount())
	}

	// Verify process identities preserved
	if p1.Session.PID != p1PID || p2.Session.PID != p2PID || p4.Session.PID != p4PID {
		t.Fatal("process PIDs changed after layout rebalance")
	}

	// 7. Verify process containment / cleanup
	sid1 := p1.Session.SessionID
	ws.CloseAll()

	time.Sleep(300 * time.Millisecond)

	remaining, _ := pty.FindSessionPIDs(sid1)
	if len(remaining) > 0 {
		t.Fatalf("processes in session %d still running after CloseAll: %v", sid1, remaining)
	}
}

func dumpScreen(p *workspace.Pane) string {
	var sb strings.Builder
	for y := 0; y < p.Terminal.Height(); y++ {
		var row strings.Builder
		for x := 0; x < p.Terminal.Width(); x++ {
			c := p.Terminal.CellAt(x, y)
			if c != nil && c.Content != "" {
				row.WriteString(c.Content)
			} else {
				row.WriteByte(' ')
			}
		}
		trimmed := strings.TrimRight(row.String(), " ")
		if trimmed != "" {
			sb.WriteString(trimmed)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
