package app

import (
	"nestedterm/internal/keyboard"
	"nestedterm/internal/workspace"
)

// EventType defines application event categories.
type EventType int

const (
	EventKey EventType = iota
	EventResize
	EventShutdown
	EventPrefixTimeout
	EventPaneExit
	EventMouse
)

// AppEvent represents an event dispatched to the main application loop.
type AppEvent struct {
	Type      EventType
	KeyEvents []keyboard.DecodeEvent
	Mouse     keyboard.MouseEvent
	Cols      int
	Rows      int
	PaneID    workspace.PaneID // EventPaneExit: the pane whose process ended
}
