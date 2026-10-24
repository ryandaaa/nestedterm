package layout

import (
	"fmt"
	"testing"
)

func TestRectangleHelpers(t *testing.T) {
	r := Rect{X: 10, Y: 5, W: 40, H: 20}
	if !r.Contains(10, 5) || !r.Contains(49, 24) || r.Contains(50, 25) {
		t.Fatalf("Contains failed for rectangle %s", r)
	}
	cx, cy := r.Center()
	if cx != 30 || cy != 15 {
		t.Fatalf("expected center (30,15), got (%d,%d)", cx, cy)
	}
	inner := r.Inner(1)
	if inner.X != 11 || inner.Y != 6 || inner.W != 38 || inner.H != 18 {
		t.Fatalf("expected inner [11,6 38x18], got %s", inner)
	}
}

func verifyLayoutIntegrity(t *testing.T, total Rect, paneIDs []string, result map[string]Rect, minCols, minRows int) {
	t.Helper()
	if len(result) != len(paneIDs) {
		t.Fatalf("expected %d panes in result, got %d", len(paneIDs), len(result))
	}

	for _, id := range paneIDs {
		r, exists := result[id]
		if !exists {
			t.Fatalf("pane %s missing from result", id)
		}
		// Must be within total bounds
		if r.X < total.X || r.Y < total.Y || r.X+r.W > total.X+total.W || r.Y+r.H > total.Y+total.H {
			t.Fatalf("pane %s outer rect %s exceeds total rect %s", id, r, total)
		}

		inner := r.Inner(1)
		if inner.W < minCols || inner.H < minRows {
			t.Fatalf("pane %s inner rect %s smaller than min (%dx%d)", id, inner, minCols, minRows)
		}
	}

	// Verify that inner rectangles do NOT overlap
	for i := 0; i < len(paneIDs); i++ {
		r1 := result[paneIDs[i]].Inner(1)
		for j := i + 1; j < len(paneIDs); j++ {
			r2 := result[paneIDs[j]].Inner(1)
			// Check intersection
			overlapX := r1.X < r2.X+r2.W && r1.X+r1.W > r2.X
			overlapY := r1.Y < r2.Y+r2.H && r1.Y+r1.H > r2.Y
			if overlapX && overlapY {
				t.Fatalf("panes %s inner %s and %s inner %s overlap!", paneIDs[i], r1, paneIDs[j], r2)
			}
		}
	}
}

func TestTilerLayouts(t *testing.T) {
	total := Rect{X: 0, Y: 0, W: 120, H: 40}
	minCols, minRows := 10, 3

	// Test 1 to 8 panes
	for n := 1; n <= 8; n++ {
		t.Run(fmt.Sprintf("%d_panes", n), func(t *testing.T) {
			var ids []string
			for i := 1; i <= n; i++ {
				ids = append(ids, fmt.Sprintf("p%d", i))
			}
			result, err := ComputeLayout(total, ids, minCols, minRows)
			if err != nil {
				t.Fatalf("ComputeLayout failed for %d panes: %v", n, err)
			}
			verifyLayoutIntegrity(t, total, ids, result, minCols, minRows)
		})
	}
}

func TestTilerScreenTooSmall(t *testing.T) {
	total := Rect{X: 0, Y: 0, W: 20, H: 8}
	minCols, minRows := 15, 5
	ids := []string{"p1", "p2"}

	_, err := ComputeLayout(total, ids, minCols, minRows)
	if err == nil {
		t.Fatal("expected ErrScreenTooSmall, got nil")
	}
}

func TestTilerDirectionalFocus(t *testing.T) {
	// 4 panes in 2x2 grid
	total := Rect{X: 0, Y: 0, W: 100, H: 40}
	ids := []string{"p1", "p2", "p3", "p4"}
	result, err := ComputeLayout(total, ids, 10, 3)
	if err != nil {
		t.Fatalf("ComputeLayout failed: %v", err)
	}

	// In 2x2:
	// p1 is top-left, p2 is bottom-left, p3 is top-right, p4 is bottom-right (or vice-versa depending on partitioning)
	// Let's test navigation from whatever p1's position is
	p1 := "p1"
	r1 := result[p1]
	c1x, c1y := r1.Center()

	// Find which pane is to the right
	rightID := FindNearestInDirection(result, p1, DirectionRight)
	if rightID != p1 {
		rRight := result[rightID]
		rx, _ := rRight.Center()
		if rx <= c1x {
			t.Fatalf("expected right pane to have center X > %d, got %d", c1x, rx)
		}
	}

	// Find which pane is below
	downID := FindNearestInDirection(result, p1, DirectionDown)
	if downID != p1 {
		rDown := result[downID]
		_, dy := rDown.Center()
		if dy <= c1y {
			t.Fatalf("expected down pane to have center Y > %d, got %d", c1y, dy)
		}
	}
}
