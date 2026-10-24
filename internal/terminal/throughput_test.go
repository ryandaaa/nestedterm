package terminal

import (
	"os"
	"testing"
)

// BenchmarkEmulatorWrite measures raw parse+apply throughput on colored,
// scrolling output (the `cat bigfile` case). Set NESTEDTERM_FLOOD_FILE to a
// sample file; it is skipped otherwise.
func BenchmarkEmulatorWrite(b *testing.B) {
	path := os.Getenv("NESTEDTERM_FLOOD_FILE")
	if path == "" {
		b.Skip("NESTEDTERM_FLOOD_FILE not set")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		b.Fatal(err)
	}
	if len(data) > 8<<20 {
		data = data[:8<<20]
	}

	emu := NewAdapter(198, 48, 5000)
	defer emu.Close()
	b.SetBytes(int64(len(data)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for off := 0; off < len(data); off += 8192 {
			end := off + 8192
			if end > len(data) {
				end = len(data)
			}
			_, _ = emu.Write(data[off:end])
			emu.TakeResponses()
		}
	}
}
