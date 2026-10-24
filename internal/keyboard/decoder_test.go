package keyboard

import (
	"bytes"
	"testing"
)

func TestDecoderNormalPassThrough(t *testing.T) {
	dec := NewDecoder(0x00) // Ctrl+Space

	events := dec.Feed([]byte("echo 'hello world'\n"))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != ActionNone {
		t.Fatalf("expected ActionNone, got %v", events[0].Action)
	}
	if !bytes.Equal(events[0].RawBytes, []byte("echo 'hello world'\n")) {
		t.Fatalf("unexpected raw bytes: %q", string(events[0].RawBytes))
	}
}

func TestDecoderPrefixShortcuts(t *testing.T) {
	dec := NewDecoder(0x00)

	// Ctrl+Space followed by 'n' (New pane)
	events := dec.Feed([]byte{0x00, 'n'})
	if len(events) != 2 || events[0].Action != ActionPrefixPending || events[1].Action != ActionNewPane {
		t.Fatalf("expected [ActionPrefixPending, ActionNewPane], got %v", events)
	}

	// Ctrl+Space followed by 'N' (uppercase)
	events = dec.Feed([]byte{0x00, 'N'})
	if len(events) != 2 || events[1].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane for 'N', got %v", events)
	}

	// Ctrl+Space followed by Ctrl+N (0x0e)
	events = dec.Feed([]byte{0x00, 0x0e})
	if len(events) != 2 || events[1].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane for Ctrl+N, got %v", events)
	}

	// Ctrl+Space followed by 'c' (tmux alias)
	events = dec.Feed([]byte{0x00, 'c'})
	if len(events) != 2 || events[1].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane for 'c', got %v", events)
	}

	// Ctrl+Space followed by 'x' (Close pane)
	events = dec.Feed([]byte{0x00, 'x'})
	if len(events) != 2 || events[1].Action != ActionClosePane {
		t.Fatalf("expected ActionClosePane, got %v", events)
	}

	// Directional focus: h, j, k, l
	expectedDirs := map[byte]Action{
		'h': ActionFocusLeft,
		'j': ActionFocusDown,
		'k': ActionFocusUp,
		'l': ActionFocusRight,
	}
	for b, expectedAction := range expectedDirs {
		events = dec.Feed([]byte{0x00, b})
		if len(events) != 2 || events[1].Action != expectedAction {
			t.Fatalf("expected %v for %c, got %v", expectedAction, b, events)
		}
	}

	// Arrow keys after prefix: \033[A, \033[B, \033[C, \033[D
	events = dec.Feed([]byte{0x00, 0x1b, '[', 'A'})
	if len(events) != 2 || events[1].Action != ActionFocusUp {
		t.Fatalf("expected ActionFocusUp for Up Arrow, got %v", events)
	}
	events = dec.Feed([]byte{0x00, 0x1b, '[', 'D'})
	if len(events) != 2 || events[1].Action != ActionFocusLeft {
		t.Fatalf("expected ActionFocusLeft for Left Arrow, got %v", events)
	}
}

func TestDecoderDoublePrefix(t *testing.T) {
	dec := NewDecoder(0x00)

	// Ctrl+Space sent twice should send literal 0x00
	events := dec.Feed([]byte{0x00, 0x00})
	if len(events) != 2 || events[1].Action != ActionSendPrefix {
		t.Fatalf("expected ActionSendPrefix, got %v", events)
	}
	if !bytes.Equal(events[1].RawBytes, []byte{0x00}) {
		t.Fatalf("expected literal prefix byte, got %v", events[1].RawBytes)
	}
}

func TestDecoderUnknownKeyAfterPrefix(t *testing.T) {
	dec := NewDecoder(0x00)

	// Ctrl+Space followed by 'z' (unbound)
	events := dec.Feed([]byte{0x00, 'z'})
	if len(events) != 2 || events[1].Action != ActionNone {
		t.Fatalf("expected ActionNone for unbound shortcut, got %v", events)
	}
	if !bytes.Equal(events[1].RawBytes, []byte{0x00, 'z'}) {
		t.Fatalf("expected literal prefix + 'z', got %v", events[1].RawBytes)
	}
}

func TestDecoderSplitReads(t *testing.T) {
	dec := NewDecoder(0x00)

	// Chunk 1: prefix byte only -> emits ActionPrefixPending
	events1 := dec.Feed([]byte{0x00})
	if len(events1) != 1 || events1[0].Action != ActionPrefixPending {
		t.Fatalf("expected ActionPrefixPending on prefix byte, got %v", events1)
	}
	if dec.Mode != ModePrefixPending {
		t.Fatal("expected decoder to be in ModePrefixPending")
	}

	// Chunk 2: action byte 'n' -> emits ActionNewPane
	events2 := dec.Feed([]byte{'n'})
	if len(events2) != 1 || events2[0].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane on split read, got %v", events2)
	}
	if dec.Mode != ModeNormal {
		t.Fatal("expected decoder to return to ModeNormal")
	}
}

func TestDecoderAltDirectShortcuts(t *testing.T) {
	dec := NewDecoder(0x00)

	// Alt+n directly creates new pane without prefix
	events := dec.Feed([]byte{0x1b, 'n'})
	if len(events) != 1 || events[0].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane for Alt+n, got %v", events)
	}

	// Alt+x closes pane
	events = dec.Feed([]byte{0x1b, 'x'})
	if len(events) != 1 || events[0].Action != ActionClosePane {
		t.Fatalf("expected ActionClosePane for Alt+x, got %v", events)
	}

	// Alt+Left arrow (\033[1;3D)
	events = dec.Feed([]byte("\033[1;3D"))
	if len(events) != 1 || events[0].Action != ActionFocusLeft {
		t.Fatalf("expected ActionFocusLeft for Alt+Left, got %v", events)
	}

	// Non-shortcut escape sequence (e.g. Esc alone or unknown escape)
	events = dec.Feed([]byte("\033[200~"))
	if len(events) != 1 || events[0].Action != ActionNone {
		t.Fatalf("expected non-shortcut escape to pass through as ActionNone, got %v", events)
	}
}
