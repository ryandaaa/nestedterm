package layout

import (
	"fmt"
	"math"
)

// Rect represents a 2D integer rectangle in screen coordinates (0-indexed).
type Rect struct {
	X int // 0-based column
	Y int // 0-based row
	W int // width (columns)
	H int // height (rows)
}

// Inner returns the inner rectangle, inset by the given border margin.
func (r Rect) Inner(border int) Rect {
	innerW := r.W - (border * 2)
	innerH := r.H - (border * 2)
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	return Rect{
		X: r.X + border,
		Y: r.Y + border,
		W: innerW,
		H: innerH,
	}
}

// Contains reports whether the point (x, y) is inside the rectangle.
func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Center returns the center coordinates (col, row) of the rectangle.
func (r Rect) Center() (int, int) {
	return r.X + (r.W / 2), r.Y + (r.H / 2)
}

// Area returns the total number of cells in the rectangle.
func (r Rect) Area() int {
	if r.W <= 0 || r.H <= 0 {
		return 0
	}
	return r.W * r.H
}

// IsEmpty reports whether the rectangle has zero or negative area.
func (r Rect) IsEmpty() bool {
	return r.W <= 0 || r.H <= 0
}

func (r Rect) String() string {
	return fmt.Sprintf("[%d,%d %dx%d]", r.X, r.Y, r.W, r.H)
}

// Distance returns Euclidean distance between centers of two rectangles.
func Distance(a, b Rect) float64 {
	ax, ay := a.Center()
	bx, by := b.Center()
	dx := float64(ax - bx)
	dy := float64(ay - by)
	return math.Hypot(dx, dy)
}
