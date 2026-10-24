package layout

import (
	"errors"
	"fmt"
	"math"
)

// Direction represents focus movement direction.
type Direction int

const (
	DirectionLeft Direction = iota
	DirectionRight
	DirectionUp
	DirectionDown
)

var (
	ErrScreenTooSmall = errors.New("terminal area is too small to accommodate layout")
	ErrNoPanes        = errors.New("cannot compute layout for 0 panes")
)

// ComputeLayout calculates outer rectangles for all paneIDs within the usable screen rect.
// minInnerCols and minInnerRows define minimum terminal area needed per pane.
func ComputeLayout(total Rect, paneIDs []string, minInnerCols, minInnerRows int) (map[string]Rect, error) {
	n := len(paneIDs)
	if n == 0 {
		return nil, ErrNoPanes
	}

	result := make(map[string]Rect, n)
	if err := tileRecursive(total, paneIDs, minInnerCols, minInnerRows, result); err != nil {
		return nil, err
	}

	// Verify all panes satisfy constraints
	for _, id := range paneIDs {
		r := result[id]
		inner := r.Inner(1)
		if inner.W < minInnerCols || inner.H < minInnerRows {
			return nil, fmt.Errorf("%w: pane %s inner size %dx%d below minimum %dx%d",
				ErrScreenTooSmall, id, inner.W, inner.H, minInnerCols, minInnerRows)
		}
	}

	return result, nil
}

func tileRecursive(r Rect, paneIDs []string, minCols, minRows int, out map[string]Rect) error {
	n := len(paneIDs)
	if n == 0 {
		return nil
	}
	if n == 1 {
		out[paneIDs[0]] = r
		return nil
	}

	// Minimum outer dimension required for a split
	minOuterW := (minCols + 2) * 2
	minOuterH := (minRows + 2) * 2

	// Decide split orientation:
	// A terminal cell has ~1:2 aspect ratio (twice as tall as wide).
	// Normalized width-to-height ratio:
	normAspect := float64(r.W) / float64(r.H*2)

	// Can we split vertically (side-by-side)?
	canSplitV := r.W >= minOuterW
	// Can we split horizontally (top / bottom)?
	canSplitH := r.H >= minOuterH

	if !canSplitV && !canSplitH {
		return fmt.Errorf("%w: rectangle %s cannot be subdivided for %d panes", ErrScreenTooSmall, r, n)
	}

	splitV := false
	if canSplitV && canSplitH {
		splitV = normAspect >= 1.0
	} else if canSplitV {
		splitV = true
	} else {
		splitV = false
	}

	var n1 int
	if n == 3 && splitV {
		// Prefer 1 left pane, 2 stacked right panes on wide screens
		n1 = 1
	} else {
		n1 = n / 2
	}

	leftPanes := paneIDs[:n1]
	rightPanes := paneIDs[n1:]

	if splitV {
		// Split vertically (columns: left | right)
		w1 := int(float64(r.W) * float64(n1) / float64(n))
		minReq1 := (minCols + 2)
		minReq2 := (minCols + 2)
		if w1 < minReq1 {
			w1 = minReq1
		}
		if r.W-w1 < minReq2 {
			w1 = r.W - minReq2
		}
		if w1 < minReq1 || r.W-w1 < minReq2 {
			return fmt.Errorf("%w: insufficient width %d to split %d panes", ErrScreenTooSmall, r.W, n)
		}

		r1 := Rect{X: r.X, Y: r.Y, W: w1 + 1, H: r.H}
		r2 := Rect{X: r.X + w1, Y: r.Y, W: r.W - w1, H: r.H}

		if err := tileRecursive(r1, leftPanes, minCols, minRows, out); err != nil {
			return err
		}
		return tileRecursive(r2, rightPanes, minCols, minRows, out)
	}

	// Split horizontally (rows: top / bottom)
	h1 := int(float64(r.H) * float64(n1) / float64(n))
	minReq1 := (minRows + 2)
	minReq2 := (minRows + 2)
	if h1 < minReq1 {
		h1 = minReq1
	}
	if r.H-h1 < minReq2 {
		h1 = r.H - minReq2
	}
	if h1 < minReq1 || r.H-h1 < minReq2 {
		return fmt.Errorf("%w: insufficient height %d to split %d panes", ErrScreenTooSmall, r.H, n)
	}

	r1 := Rect{X: r.X, Y: r.Y, W: r.W, H: h1 + 1}
	r2 := Rect{X: r.X, Y: r.Y + h1, W: r.W, H: r.H - h1}

	if err := tileRecursive(r1, leftPanes, minCols, minRows, out); err != nil {
		return err
	}
	return tileRecursive(r2, rightPanes, minCols, minRows, out)
}

// FindNearestInDirection returns the pane ID of the nearest pane in the specified direction.
// If no pane exists in that direction, it returns currentActiveID.
func FindNearestInDirection(panes map[string]Rect, currentActiveID string, dir Direction) string {
	activeRect, exists := panes[currentActiveID]
	if !exists || len(panes) <= 1 {
		return currentActiveID
	}

	ax, ay := activeRect.Center()
	bestID := currentActiveID
	bestScore := math.MaxFloat64

	for id, r := range panes {
		if id == currentActiveID {
			continue
		}
		cx, cy := r.Center()
		dx := float64(cx - ax)
		dy := float64(cy - ay)

		valid := false
		var score float64

		switch dir {
		case DirectionLeft:
			// Target center is strictly to the left (cx < ax) or bounding box is to the left
			if r.X+r.W <= activeRect.X || (cx < ax && math.Abs(dy) < float64(activeRect.H)) {
				valid = true
				distX := math.Abs(dx)
				distY := math.Abs(dy)
				score = distX + (distY * 2.5) // penalize vertical displacement
			}
		case DirectionRight:
			// Target is strictly to the right
			if r.X >= activeRect.X+activeRect.W || (cx > ax && math.Abs(dy) < float64(activeRect.H)) {
				valid = true
				distX := math.Abs(dx)
				distY := math.Abs(dy)
				score = distX + (distY * 2.5)
			}
		case DirectionUp:
			// Target is strictly above
			if r.Y+r.H <= activeRect.Y || (cy < ay && math.Abs(dx) < float64(activeRect.W)) {
				valid = true
				distX := math.Abs(dx)
				distY := math.Abs(dy)
				score = distY + (distX * 2.5) // penalize horizontal displacement
			}
		case DirectionDown:
			// Target is strictly below
			if r.Y >= activeRect.Y+activeRect.H || (cy > ay && math.Abs(dx) < float64(activeRect.W)) {
				valid = true
				distX := math.Abs(dx)
				distY := math.Abs(dy)
				score = distY + (distX * 2.5)
			}
		}

		if valid && score < bestScore {
			bestScore = score
			bestID = id
		}
	}

	return bestID
}
