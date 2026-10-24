package keyboard

import (
	"fmt"
)

// Action represents an action triggered by a key sequence.
type Action int

const (
	ActionNone Action = iota
	ActionPrefixPending
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
	case ActionPrefixPending:
		return "PrefixPending"
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
// It includes lowercase, uppercase, control-key, and common multiplexer aliases.
func DefaultKeyBindings() KeyBindingMap {
	return KeyBindingMap{
		// New pane: n, N, Ctrl+N (0x0e), c, C
		'n':  ActionNewPane,
		'N':  ActionNewPane,
		0x0e: ActionNewPane,
		'c':  ActionNewPane,
		'C':  ActionNewPane,

		// Close pane: x, X, Ctrl+X (0x18)
		'x':  ActionClosePane,
		'X':  ActionClosePane,
		0x18: ActionClosePane,

		// Quit: q, Q, Ctrl+Q (0x11)
		'q':  ActionQuit,
		'Q':  ActionQuit,
		0x11: ActionQuit,

		// Directional navigation
		'h':  ActionFocusLeft,
		'H':  ActionFocusLeft,
		0x08: ActionFocusLeft, // Ctrl+H / Backspace
		'j':  ActionFocusDown,
		'J':  ActionFocusDown,
		0x0a: ActionFocusDown, // Ctrl+J / LF
		'k':  ActionFocusUp,
		'K':  ActionFocusUp,
		0x0b: ActionFocusUp, // Ctrl+K
		'l':  ActionFocusRight,
		'L':  ActionFocusRight,
		0x0c: ActionFocusRight, // Ctrl+L

		// Scrollback
		'[': ActionScrollback,

		// Help
		'?': ActionHelp,
		'/': ActionHelp,
	}
}

// BuildHelpLines generates the formatted help overlay text using the configured prefix.
func BuildHelpLines(prefix byte) []string {
	p := PrefixDisplayName(prefix)
	return []string{
		fmt.Sprintf(" %-16s : Create a new pane (or %s c)", p+" n", p),
		fmt.Sprintf(" %-16s : Close the active pane", p+" x"),
		fmt.Sprintf(" %-16s : Focus pane to the left (or Left Arrow)", p+" h"),
		fmt.Sprintf(" %-16s : Focus pane below (or Down Arrow)", p+" j"),
		fmt.Sprintf(" %-16s : Focus pane above (or Up Arrow)", p+" k"),
		fmt.Sprintf(" %-16s : Focus pane to the right (or Right Arrow)", p+" l"),
		fmt.Sprintf(" %-16s : Enter scrollback mode", p+" ["),
		fmt.Sprintf(" %-16s : Display this shortcut help", p+" ?"),
		fmt.Sprintf(" %-16s : Exit NestedTerm", p+" q"),
		fmt.Sprintf(" %-16s : Send literal prefix to shell", p+" "+p),
		"",
		" Scrollback mode: Up/Down or k/j to scroll, q to exit",
	}
}
