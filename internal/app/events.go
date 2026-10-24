package app

import (
	"nestedterm/internal/keyboard"
)

// EventType defines application event categories.
type EventType int

const (
	EventRedraw EventType = iota
	EventKey
	EventResize
	EventShutdown
	EventPrefixTimeout
)

// AppEvent represents an event dispatched to the main application loop.
type AppEvent struct {
	Type      EventType
	KeyEvents []keyboard.DecodeEvent
	Cols      int
	Rows      int
}
