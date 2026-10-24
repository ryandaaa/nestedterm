package terminal

import (
	"sync"
	"testing"
)

func TestTerminalAdapterBasics(t *testing.T) {
	emu := NewAdapter(80, 24, 100)
	defer emu.Close()

	if emu.Width() != 80 || emu.Height() != 24 {
		t.Fatalf("expected 80x24, got %dx%d", emu.Width(), emu.Height())
	}

	// Write simple text
	_, err := emu.Write([]byte("Hello, NestedTerm!"))
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	c0 := emu.CellAt(0, 0)
	if c0.Content != "H" {
		t.Fatalf("expected 'H', got %q", c0.Content)
	}
	c6 := emu.CellAt(6, 0)
	if c6.Content != " " {
		t.Fatalf("expected ' ', got %q", c6.Content)
	}
	c7 := emu.CellAt(7, 0)
	if c7.Content != "N" {
		t.Fatalf("expected 'N', got %q", c7.Content)
	}

	// Cursor position check
	cx, cy, visible := emu.CursorPosition()
	if cx != 18 || cy != 0 || !visible {
		t.Fatalf("unexpected cursor position: (%d,%d) visible=%v", cx, cy, visible)
	}
}

func TestTerminalAdapterStyles(t *testing.T) {
	emu := NewAdapter(80, 24, 100)
	defer emu.Close()

	// Write bold and underline text: ESC[1;4m BOLD_UNDERLINE ESC[0m
	_, _ = emu.Write([]byte("\033[1;4mBOLD_UNDERLINE\033[0m"))

	c := emu.CellAt(0, 0)
	if c.Content != "B" {
		t.Fatalf("expected 'B', got %q", c.Content)
	}
	if c.Attrs&AttrBold == 0 {
		t.Fatal("expected AttrBold to be set")
	}
	if c.Attrs&AttrUnderline == 0 {
		t.Fatal("expected AttrUnderline to be set")
	}
}

func TestTerminalAdapterResizeAndScrollback(t *testing.T) {
	emu := NewAdapter(40, 5, 50)
	defer emu.Close()

	// Write 10 lines of text to trigger scrollback
	for i := 0; i < 10; i++ {
		_, _ = emu.Write([]byte("Line\n"))
	}

	if emu.ScrollbackLen() == 0 {
		t.Fatal("expected scrollback lines to be populated")
	}

	sc := emu.ScrollbackCellAt(0, 0)
	if sc == nil {
		t.Fatal("expected valid scrollback cell")
	}

	// Test resize
	emu.Resize(60, 10)
	if emu.Width() != 60 || emu.Height() != 10 {
		t.Fatalf("expected 60x10, got %dx%d", emu.Width(), emu.Height())
	}
}

func TestTerminalAdapterQueries(t *testing.T) {
	emu := NewAdapter(80, 24, 100)
	defer emu.Close()

	// Send Cursor Position Report query: ESC[6n
	_, _ = emu.Write([]byte("\033[6n"))

	responses := emu.TakeResponses()
	if len(responses) == 0 {
		t.Fatal("expected query response to ESC[6n, got empty")
	}
	t.Logf("Query response: %q", string(responses))

	// Draining again should return empty
	empty := emu.TakeResponses()
	if len(empty) != 0 {
		t.Fatalf("expected empty responses after drain, got %q", string(empty))
	}
}

func TestTerminalAdapterConcurrency(t *testing.T) {
	emu := NewAdapter(80, 24, 100)
	defer emu.Close()

	var wg sync.WaitGroup
	// Writer goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_, _ = emu.Write([]byte("Thread test write\r\n"))
				_ = emu.TakeResponses()
			}
		}(i)
	}

	// Reader goroutines
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				_ = emu.CellAt(0, 0)
				_, _, _ = emu.CursorPosition()
				_ = emu.ScrollbackLen()
			}
		}()
	}

	wg.Wait()
}
