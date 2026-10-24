package app

import (
	"io"
	"os"

	"nestedterm/internal/keyboard"
)

// StartInputReader runs a goroutine reading raw stdin bytes, decoding them,
// and sending key events to eventCh.
func StartInputReader(
	decoder *keyboard.Decoder,
	eventCh chan<- AppEvent,
	stopCh <-chan struct{},
) {
	go func() {
		buf := make([]byte, 512)
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			n, err := os.Stdin.Read(buf)
			if n > 0 {
				events := decoder.Feed(buf[:n])
				if len(events) > 0 {
					select {
					case eventCh <- AppEvent{Type: EventKey, KeyEvents: events}:
					case <-stopCh:
						return
					}
				}
			}

			if err != nil {
				if err != io.EOF {
					select {
					case eventCh <- AppEvent{Type: EventShutdown}:
					default:
					}
				}
				return
			}
		}
	}()
}
