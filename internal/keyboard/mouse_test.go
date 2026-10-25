package keyboard

import "testing"

func TestMouseFilterStripsReports(t *testing.T) {
	var f MouseFilter
	rest, evs := f.Feed([]byte("ab\x1b[<0;11;6Mcd\x1b[<0;12;6m"))
	if string(rest) != "abcd" {
		t.Fatalf("rest = %q", rest)
	}
	if len(evs) != 2 || evs[0].X != 10 || evs[0].Y != 5 || evs[0].Release || !evs[1].Release {
		t.Fatalf("events = %+v", evs)
	}
}

func TestMouseFilterSplitReport(t *testing.T) {
	var f MouseFilter
	rest, evs := f.Feed([]byte("x\x1b[<32;4"))
	if string(rest) != "x" || len(evs) != 0 {
		t.Fatalf("first chunk: rest=%q evs=%v", rest, evs)
	}
	rest, evs = f.Feed([]byte(";9Mz"))
	if string(rest) != "z" || len(evs) != 1 || !evs[0].Motion || evs[0].Button != 0 {
		t.Fatalf("second chunk: rest=%q evs=%+v", rest, evs)
	}
}

func TestMouseFilterWheelAndPlainKeys(t *testing.T) {
	var f MouseFilter
	rest, evs := f.Feed([]byte("\x1b[A\x1b[<64;1;1M"))
	if string(rest) != "\x1b[A" || len(evs) != 1 || !evs[0].IsWheelUp() {
		t.Fatalf("rest=%q evs=%+v", rest, evs)
	}
}
