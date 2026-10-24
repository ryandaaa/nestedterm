package workspace

// PaneStatus represents the current lifecycle status of a pane.
type PaneStatus int

const (
	PaneStarting PaneStatus = iota
	PaneRunning
	PaneExited
	PaneFailed
	PaneClosing
)

func (s PaneStatus) String() string {
	switch s {
	case PaneStarting:
		return "starting"
	case PaneRunning:
		return "running"
	case PaneExited:
		return "exited"
	case PaneFailed:
		return "failed"
	case PaneClosing:
		return "closing"
	default:
		return "unknown"
	}
}
