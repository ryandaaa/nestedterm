package workspace

import (
	"fmt"
	"sync"

	"nestedterm/internal/layout"
	"nestedterm/internal/pty"
	"nestedterm/internal/terminal"
)

// CreateInitialPane creates the first pane in the workspace using the given directory.
func (w *Workspace) CreateInitialPane(cwd string) (*Pane, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	id := PaneID("pane-1")
	rects, err := layout.ComputeLayout(w.TotalRect, []string{string(id)}, w.MinCols, w.MinRows)
	if err != nil {
		return nil, fmt.Errorf("initial layout failed: %w", err)
	}

	paneRect := rects[string(id)]
	inner := paneRect.Inner(1)

	sess, err := pty.SpawnSession(string(id), w.DefaultShell, cwd, inner.W, inner.H)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn initial shell: %w", err)
	}

	emu := terminal.NewAdapter(inner.W, inner.H, 5000)

	pane := &Pane{
		ID:           id,
		Index:        w.nextIndex,
		Title:        shellBaseName(w.DefaultShell),
		InitialCWD:   cwd,
		LastKnownCWD: cwd,
		Status:       PaneRunning,
		Session:      sess,
		Terminal:     emu,
		Rect:         paneRect,
		InnerRect:    inner,
	}
	w.nextIndex++

	w.Panes[id] = pane
	w.Order = append(w.Order, id)
	w.ActivePaneID = id

	w.startIO(pane)

	return pane, nil
}

// startIO starts the pane's input writer and PTY output pump. When the child
// process ends, OnPaneExit is invoked so the owner can close the pane.
func (w *Workspace) startIO(pane *Pane) {
	pane.startWriter()
	cwd := &cwdRefresher{pane: pane, redraw: w.OnRedraw}

	go pty.RunSessionIO(pane.Session, pane.Terminal, cwd.output, func(code int) {
		pane.mu.Lock()
		pane.Status = PaneExited
		pane.ExitCode = &code
		pane.mu.Unlock()

		cwd.stop()
		if w.OnPaneExit != nil {
			w.OnPaneExit(pane.ID)
		}
	})
}

// CreatePane spawns a new pane inheritng the active pane's current working directory,
// recomputing and applying the tiled layout.
func (w *Workspace) CreatePane() (*Pane, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.Order) >= w.MaxPanes {
		return nil, ErrMaxPanesReached
	}

	// Resolve target working directory from active pane
	cwd := ""
	if active, ok := w.Panes[w.ActivePaneID]; ok {
		cwd = active.GetCWD()
	}

	newID := PaneID(fmt.Sprintf("pane-%d", w.nextIndex))

	// Provisional layout check
	var candidateIDs []string
	for _, id := range w.Order {
		candidateIDs = append(candidateIDs, string(id))
	}
	candidateIDs = append(candidateIDs, string(newID))

	rects, err := layout.ComputeLayout(w.TotalRect, candidateIDs, w.MinCols, w.MinRows)
	if err != nil {
		return nil, fmt.Errorf("layout calculation rejected: %w", err)
	}

	newPaneRect := rects[string(newID)]
	inner := newPaneRect.Inner(1)

	// Spawn PTY session
	sess, err := pty.SpawnSession(string(newID), w.DefaultShell, cwd, inner.W, inner.H)
	if err != nil {
		return nil, fmt.Errorf("failed to spawn shell for new pane: %w", err)
	}

	emu := terminal.NewAdapter(inner.W, inner.H, 5000)

	pane := &Pane{
		ID:           newID,
		Index:        w.nextIndex,
		Title:        shellBaseName(w.DefaultShell),
		InitialCWD:   cwd,
		LastKnownCWD: cwd,
		Status:       PaneRunning,
		Session:      sess,
		Terminal:     emu,
		Rect:         newPaneRect,
		InnerRect:    inner,
	}
	w.nextIndex++

	w.Panes[newID] = pane
	w.Order = append(w.Order, newID)
	w.ActivePaneID = newID

	// Apply layout resize to all panes
	for _, id := range w.Order {
		if p, ok := w.Panes[id]; ok {
			_ = p.Resize(rects[string(id)])
		}
	}

	w.startIO(pane)

	if w.OnRedraw != nil {
		w.OnRedraw()
	}

	return pane, nil
}

// ClosePane closes the specified pane and rebalances the layout.
// If no panes remain, empty returns true.
func (w *Workspace) ClosePane(id PaneID) (empty bool, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	pane, ok := w.Panes[id]
	if !ok {
		return len(w.Order) == 0, ErrPaneNotFound
	}

	// Remove from order and map
	var newOrder []PaneID
	for _, pID := range w.Order {
		if pID != id {
			newOrder = append(newOrder, pID)
		}
	}
	w.Order = newOrder
	delete(w.Panes, id)

	// Close pane resources asynchronously to avoid blocking layout update
	go func(p *Pane) {
		_ = p.Close(w.GraceInterval)
	}(pane)

	if len(w.Order) == 0 {
		return true, nil
	}

	// Update active pane if closed pane was active
	if w.ActivePaneID == id {
		w.ActivePaneID = w.Order[len(w.Order)-1]
	}

	// Recompute layout for remaining panes
	var ids []string
	for _, pID := range w.Order {
		ids = append(ids, string(pID))
	}

	rects, err := layout.ComputeLayout(w.TotalRect, ids, w.MinCols, w.MinRows)
	if err == nil {
		for _, pID := range w.Order {
			if p, ok := w.Panes[pID]; ok {
				_ = p.Resize(rects[string(pID)])
			}
		}
	}

	if w.OnRedraw != nil {
		w.OnRedraw()
	}

	return false, nil
}

// CloseActivePane closes the currently active pane.
func (w *Workspace) CloseActivePane() (empty bool, err error) {
	activeID := w.ActivePaneID
	if activeID == "" {
		return true, nil
	}
	return w.ClosePane(activeID)
}

// FocusDirection moves focus to the nearest pane in the specified direction.
func (w *Workspace) FocusDirection(dir layout.Direction) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.Order) <= 1 {
		return
	}

	rectMap := make(map[string]layout.Rect, len(w.Panes))
	for id, p := range w.Panes {
		rectMap[string(id)] = p.Rect
	}

	nearest := layout.FindNearestInDirection(rectMap, string(w.ActivePaneID), dir)
	if nearest != string(w.ActivePaneID) {
		w.ActivePaneID = PaneID(nearest)
		if w.OnRedraw != nil {
			w.OnRedraw()
		}
	}
}

// FocusNext cycles focus to the next pane.
func (w *Workspace) FocusNext() {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(w.Order)
	if n <= 1 {
		return
	}

	for i, id := range w.Order {
		if id == w.ActivePaneID {
			w.ActivePaneID = w.Order[(i+1)%n]
			break
		}
	}
	if w.OnRedraw != nil {
		w.OnRedraw()
	}
}

// FocusPrev cycles focus to the previous pane.
func (w *Workspace) FocusPrev() {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(w.Order)
	if n <= 1 {
		return
	}

	for i, id := range w.Order {
		if id == w.ActivePaneID {
			w.ActivePaneID = w.Order[(i-1+n)%n]
			break
		}
	}
	if w.OnRedraw != nil {
		w.OnRedraw()
	}
}

// Resize updates the workspace bounding rectangle and recomputes all pane geometry.
func (w *Workspace) Resize(total layout.Rect) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.TotalRect = total
	if len(w.Order) == 0 {
		return nil
	}

	var ids []string
	for _, id := range w.Order {
		ids = append(ids, string(id))
	}

	rects, err := layout.ComputeLayout(total, ids, w.MinCols, w.MinRows)
	if err != nil {
		return err
	}

	for _, id := range w.Order {
		if p, ok := w.Panes[id]; ok {
			_ = p.Resize(rects[string(id)])
		}
	}

	if w.OnRedraw != nil {
		w.OnRedraw()
	}
	return nil
}

// CloseAll closes all active panes in the workspace.
func (w *Workspace) CloseAll() {
	w.mu.Lock()
	panes := make([]*Pane, 0, len(w.Panes))
	for _, p := range w.Panes {
		panes = append(panes, p)
	}
	w.Panes = make(map[PaneID]*Pane)
	w.Order = nil
	w.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range panes {
		wg.Add(1)
		go func(target *Pane) {
			defer wg.Done()
			_ = target.Close(w.GraceInterval)
		}(p)
	}
	wg.Wait()
}
