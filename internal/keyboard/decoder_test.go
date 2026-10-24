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
	if len(events) != 1 || events[0].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane, got %v", events)
	}

	// Ctrl+Space followed by 'x' (Close pane)
	events = dec.Feed([]byte{0x00, 'x'})
	if len(events) != 1 || events[0].Action != ActionClosePane {
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
		if len(events) != 1 || events[0].Action != expectedAction {
			t.Fatalf("expected %v for %c, got %v", expectedAction, b, events)
		}
	}
}

func TestDecoderDoublePrefix(t *testing.T) {
	dec := NewDecoder(0x00)

	// Ctrl+Space sent twice should send literal 0x00
	events := dec.Feed([]byte{0x00, 0x00})
	if len(events) != 1 || events[0].Action != ActionSendPrefix {
		t.Fatalf("expected ActionSendPrefix, got %v", events)
	}
	if !bytes.Equal(events[0].RawBytes, []byte{0x00}) {
		t.Fatalf("expected literal prefix byte, got %v", events[0].RawBytes)
	}
}

func TestDecoderUnknownKeyAfterPrefix(t *testing.T) {
	dec := NewDecoder(0x00)

	// Ctrl+Space followed by 'z' (unbound)
	events := dec.Feed([]byte{0x00, 'z'})
	if len(events) != 1 || events[0].Action != ActionNone {
		t.Fatalf("expected ActionNone for unbound shortcut, got %v", events)
	}
	if !bytes.Equal(events[0].RawBytes, []byte{0x00, 'z'}) {
		t.Fatalf("expected literal prefix + 'z', got %v", events[0].RawBytes)
	}
}

func TestDecoderSplitReads(t *testing.T) {
	dec := NewDecoder(0x00)

	// Chunk 1: prefix byte only
	events1 := dec.Feed([]byte{0x00})
	if len(events1) != 0 {
		t.Fatalf("expected 0 events on prefix byte, got %d", len(events1))
	}
	if dec.Mode != ModePrefixPending {
		t.Fatal("expected decoder to be in ModePrefixPending")
	}

	// Chunk 2: action byte 'n'
	events2 := dec.Feed([]byte{'n'})
	if len(events2) != 1 || events2[0].Action != ActionNewPane {
		t.Fatalf("expected ActionNewPane on split read, got %v", events2)
	}
	if dec.Mode != ModeNormal {
		t.Fatal("expected decoder to return to ModeNormal")
	}
}
