package workspace

import (
	"errors"
	"path/filepath"
	"sync"
	"time"

	"nestedterm/internal/config"
	"nestedterm/internal/layout"
)

var (
	ErrMaxPanesReached = errors.New("maximum pane limit reached")
	ErrPaneNotFound    = errors.New("pane not found")
)

// Workspace manages all active panes, layout calculation, and focus state.
type Workspace struct {
	mu            sync.RWMutex
	Panes         map[PaneID]*Pane
	Order         []PaneID
	ActivePaneID  PaneID
	TotalRect     layout.Rect
	MaxPanes      int
	MinCols       int
	MinRows       int
	DefaultShell  string
	GraceInterval time.Duration
	OnRedraw      func()
	// OnPaneExit, if set, is called from the pane's I/O goroutine once its
	// child process has ended. Set it before creating panes.
	OnPaneExit func(PaneID)
	nextIndex  int
}

// NewWorkspace creates a new Workspace instance.
func NewWorkspace(
	total layout.Rect,
	maxPanes int,
	shell string,
	grace time.Duration,
	onRedraw func(),
) *Workspace {
	if maxPanes <= 0 {
		maxPanes = config.DefaultMaxPanes
	}
	if grace <= 0 {
		grace = config.DefaultGraceInterval
	}

	return &Workspace{
		Panes:         make(map[PaneID]*Pane),
		Order:         make([]PaneID, 0),
		TotalRect:     total,
		MaxPanes:      maxPanes,
		MinCols:       config.DefaultMinCols,
		MinRows:       config.DefaultMinRows,
		DefaultShell:  shell,
		GraceInterval: grace,
		OnRedraw:      onRedraw,
		nextIndex:     1,
	}
}

// PaneCount returns the current number of panes.
func (w *Workspace) PaneCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.Order)
}

// GetActivePane returns the currently focused pane.
func (w *Workspace) GetActivePane() *Pane {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Panes[w.ActivePaneID]
}

// GetPane returns the pane with the specified ID.
func (w *Workspace) GetPane(id PaneID) *Pane {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.Panes[id]
}

// AllPanes returns a slice of all panes in display order.
func (w *Workspace) AllPanes() []*Pane {
	w.mu.RLock()
	defer w.mu.RUnlock()

	result := make([]*Pane, 0, len(w.Order))
	for _, id := range w.Order {
		if p, ok := w.Panes[id]; ok {
			result = append(result, p)
		}
	}
	return result
}

// SetActivePane updates the active pane ID.
func (w *Workspace) SetActivePane(id PaneID) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, exists := w.Panes[id]; exists {
		w.ActivePaneID = id
	}
}

func shellBaseName(path string) string {
	b := filepath.Base(path)
	if b == "" || b == "." {
		return "shell"
	}
	return b
}
