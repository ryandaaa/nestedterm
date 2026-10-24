package keyboard

// DecoderMode represents the parser mode.
type DecoderMode int

const (
	ModeNormal DecoderMode = iota
	ModePrefixPending
	ModeScrollback
	ModeModalHelp
	ModeModalQuitConfirm
)

// DecodeEvent represents the outcome of decoding an input sequence.
type DecodeEvent struct {
	Action   Action
	RawBytes []byte
}

// Decoder parses the outer terminal byte stream.
type Decoder struct {
	PrefixByte byte
	Bindings   KeyBindingMap
	Mode       DecoderMode
}

// NewDecoder creates a new keyboard Decoder.
func NewDecoder(prefixByte byte) *Decoder {
	return &Decoder{
		PrefixByte: prefixByte,
		Bindings:   DefaultKeyBindings(),
		Mode:       ModeNormal,
	}
}

// Feed consumes incoming bytes and yields decoded events.
func (d *Decoder) Feed(data []byte) []DecodeEvent {
	var events []DecodeEvent

	i := 0
	for i < len(data) {
		switch d.Mode {
		case ModeNormal:
			b := data[i]
			if b == d.PrefixByte {
				d.Mode = ModePrefixPending
				events = append(events, DecodeEvent{
					Action: ActionPrefixPending,
				})
				i++
			} else if b == 0x1b && isAltShortcut(data[i:]) {
				// Direct Alt+key shortcut without prefix
				act, consumed := parseAltShortcut(data[i:])
				events = append(events, DecodeEvent{Action: act})
				i += consumed
			} else {
				// Accumulate ordinary bytes until next prefix, Alt shortcut, or end of data
				start := i
				for i < len(data) && data[i] != d.PrefixByte {
					if data[i] == 0x1b && isAltShortcut(data[i:]) {
						break
					}
					i++
				}
				if i > start {
					events = append(events, DecodeEvent{
						Action:   ActionNone,
						RawBytes: data[start:i],
					})
				}
			}

		case ModePrefixPending:
			// Check for ANSI arrow escape sequences (\033[A, \033[B, \033[C, \033[D)
			if data[i] == 0x1b && i+2 < len(data) && data[i+1] == '[' {
				d.Mode = ModeNormal
				switch data[i+2] {
				case 'A':
					events = append(events, DecodeEvent{Action: ActionFocusUp})
					i += 3
					continue
				case 'B':
					events = append(events, DecodeEvent{Action: ActionFocusDown})
					i += 3
					continue
				case 'C':
					events = append(events, DecodeEvent{Action: ActionFocusRight})
					i += 3
					continue
				case 'D':
					events = append(events, DecodeEvent{Action: ActionFocusLeft})
					i += 3
					continue
				}
			}

			b := data[i]
			i++
			if b == d.PrefixByte {
				// Literal prefix sent twice
				d.Mode = ModeNormal
				events = append(events, DecodeEvent{
					Action:   ActionSendPrefix,
					RawBytes: []byte{d.PrefixByte},
				})
			} else if b == 0x1b { // Escape cancels prefix mode cleanly
				d.Mode = ModeNormal
			} else if act, ok := d.Bindings[b]; ok {
				d.Mode = ModeNormal
				events = append(events, DecodeEvent{
					Action: act,
				})
			} else {
				// Unknown key sequence after prefix: forward literal prefix + unhandled key
				d.Mode = ModeNormal
				events = append(events, DecodeEvent{
					Action:   ActionNone,
					RawBytes: []byte{d.PrefixByte, b},
				})
			}

		case ModeScrollback:
			b := data[i]
			i++
			events = append(events, DecodeEvent{
				Action:   ActionNone,
				RawBytes: []byte{b},
			})

		case ModeModalHelp:
			b := data[i]
			i++
			events = append(events, DecodeEvent{
				Action:   ActionHelp, // any key closes help
				RawBytes: []byte{b},
			})

		case ModeModalQuitConfirm:
			b := data[i]
			i++
			events = append(events, DecodeEvent{
				Action:   ActionQuit,
				RawBytes: []byte{b},
			})
		}
	}

	return events
}

// ResetMode resets decoder back to normal input routing mode.
func (d *Decoder) ResetMode() {
	d.Mode = ModeNormal
}

func parseAltShortcut(data []byte) (Action, int) {
	if len(data) < 2 || data[0] != 0x1b {
		return ActionNone, 0
	}

	// Alt+Arrow sequences (\033[1;3A, \033\033[A)
	if len(data) >= 6 && string(data[:5]) == "\033[1;3" {
		switch data[5] {
		case 'A':
			return ActionFocusUp, 6
		case 'B':
			return ActionFocusDown, 6
		case 'C':
			return ActionFocusRight, 6
		case 'D':
			return ActionFocusLeft, 6
		}
	}
	if len(data) >= 4 && data[1] == 0x1b && data[2] == '[' {
		switch data[3] {
		case 'A':
			return ActionFocusUp, 4
		case 'B':
			return ActionFocusDown, 4
		case 'C':
			return ActionFocusRight, 4
		case 'D':
			return ActionFocusLeft, 4
		}
	}

	// Alt + key
	switch data[1] {
	case 'n', 'N':
		return ActionNewPane, 2
	case 'x', 'X':
		return ActionClosePane, 2
	case 'h', 'H':
		return ActionFocusLeft, 2
	case 'j', 'J':
		return ActionFocusDown, 2
	case 'k', 'K':
		return ActionFocusUp, 2
	case 'l', 'L':
		return ActionFocusRight, 2
	case 'q', 'Q':
		return ActionQuit, 2
	}
	return ActionNone, 0
}

func isAltShortcut(data []byte) bool {
	act, _ := parseAltShortcut(data)
	return act != ActionNone
}
