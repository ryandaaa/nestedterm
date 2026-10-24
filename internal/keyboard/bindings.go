package keyboard

import (
	"fmt"
)

// Action represents an action triggered by a key sequence.
type Action int

const (
	ActionNone Action = iota
	ActionNewPane
	ActionClosePane
	ActionQuit
	ActionFocusLeft
	ActionFocusDown
	ActionFocusUp
	ActionFocusRight
	ActionScrollback
	ActionHelp
	ActionSendPrefix
)

func (a Action) String() string {
	switch a {
	case ActionNewPane:
		return "NewPane"
	case ActionClosePane:
		return "ClosePane"
	case ActionQuit:
		return "Quit"
	case ActionFocusLeft:
		return "FocusLeft"
	case ActionFocusDown:
		return "FocusDown"
	case ActionFocusUp:
		return "FocusUp"
	case ActionFocusRight:
		return "FocusRight"
	case ActionScrollback:
		return "Scrollback"
	case ActionHelp:
		return "Help"
	case ActionSendPrefix:
		return "SendPrefix"
	default:
		return "None"
	}
}

// KeyBindingMap maps a shortcut character to an Action.
type KeyBindingMap map[byte]Action

// DefaultKeyBindings returns the standard action mappings after prefix key is pressed.
func DefaultKeyBindings() KeyBindingMap {
	return KeyBindingMap{
		'n': ActionNewPane,
		'x': ActionClosePane,
		'q': ActionQuit,
		'h': ActionFocusLeft,
		'j': ActionFocusDown,
		'k': ActionFocusUp,
		'l': ActionFocusRight,
		'[': ActionScrollback,
		'?': ActionHelp,
	}
}

// BuildHelpLines generates the formatted help overlay text using the configured prefix.
func BuildHelpLines(prefix byte) []string {
	p := PrefixDisplayName(prefix)
	return []string{
		fmt.Sprintf(" %-16s : Create a new pane", p+" n"),
		fmt.Sprintf(" %-16s : Close the active pane", p+" x"),
		fmt.Sprintf(" %-16s : Focus pane to the left", p+" h"),
		fmt.Sprintf(" %-16s : Focus pane below", p+" j"),
		fmt.Sprintf(" %-16s : Focus pane above", p+" k"),
		fmt.Sprintf(" %-16s : Focus pane to the right", p+" l"),
		fmt.Sprintf(" %-16s : Enter scrollback mode", p+" ["),
		fmt.Sprintf(" %-16s : Display this shortcut help", p+" ?"),
		fmt.Sprintf(" %-16s : Exit NestedTerm", p+" q"),
		fmt.Sprintf(" %-16s : Send literal prefix to shell", p+" "+p),
		"",
		" Scrollback mode: Up/Down or k/j to scroll, q to exit",
	}
}
