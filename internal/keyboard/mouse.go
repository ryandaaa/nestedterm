package keyboard

import "bytes"

// MouseEvent is a decoded SGR (mode 1006) mouse report. X and Y are 0-indexed
// screen cells.
type MouseEvent struct {
	Button  int // 0 left, 1 middle, 2 right; 64/65 wheel up/down
	X, Y    int
	Release bool
	Motion  bool // pointer moved while a button was held
}

const (
	mouseWheelUp   = 64
	mouseWheelDown = 65
)

// IsWheelUp reports whether the event is a scroll-wheel step away from the user.
func (m MouseEvent) IsWheelUp() bool { return m.Button == mouseWheelUp }

// IsWheelDown reports whether the event is a scroll-wheel step toward the user.
func (m MouseEvent) IsWheelDown() bool { return m.Button == mouseWheelDown }

var mousePrefix = []byte("\x1b[<")

// MouseFilter splits SGR mouse reports out of the raw input stream so they are
// never forwarded to a pane as keystrokes. A report cut off at the end of one
// read is held back until the rest arrives.
type MouseFilter struct {
	pending []byte
}

// Feed returns data with all complete mouse reports removed, plus the reports.
func (f *MouseFilter) Feed(data []byte) (rest []byte, events []MouseEvent) {
	if len(f.pending) > 0 {
		data = append(f.pending, data...)
		f.pending = nil
	}

	for len(data) > 0 {
		i := bytes.Index(data, mousePrefix)
		if i < 0 {
			rest = append(rest, data...)
			break
		}
		rest = append(rest, data[:i]...)
		data = data[i:]

		ev, n, ok := parseSGRMouse(data)
		if ok {
			events = append(events, ev)
			data = data[n:]
			continue
		}
		if n == 0 && len(data) < 32 {
			// Incomplete report: wait for the remaining bytes.
			f.pending = append([]byte(nil), data...)
			break
		}
		// Malformed: let the prefix through and keep scanning.
		rest = append(rest, data[:len(mousePrefix)]...)
		data = data[len(mousePrefix):]
	}
	return rest, events
}

// parseSGRMouse parses "\x1b[<b;x;y(M|m)" at the start of data. It returns
// n == 0 when data ends before the report does.
func parseSGRMouse(data []byte) (ev MouseEvent, n int, ok bool) {
	var nums [3]int
	idx := 0
	i := len(mousePrefix)
	digits := 0
	for ; i < len(data); i++ {
		c := data[i]
		switch {
		case c >= '0' && c <= '9':
			nums[idx] = nums[idx]*10 + int(c-'0')
			digits++
		case c == ';' && idx < 2 && digits > 0:
			idx++
			digits = 0
		case (c == 'M' || c == 'm') && idx == 2 && digits > 0:
			b := nums[0]
			ev = MouseEvent{
				Button:  b & 0x43, // keep bits 0-1 and the wheel bit
				X:       nums[1] - 1,
				Y:       nums[2] - 1,
				Release: c == 'm',
				Motion:  b&32 != 0,
			}
			return ev, i + 1, true
		default:
			return MouseEvent{}, -1, false
		}
	}
	return MouseEvent{}, 0, false
}
