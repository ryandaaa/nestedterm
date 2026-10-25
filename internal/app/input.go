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
		var mouse keyboard.MouseFilter
		for {
			select {
			case <-stopCh:
				return
			default:
			}

			n, err := os.Stdin.Read(buf)
			if n > 0 {
				data, mouseEvents := mouse.Feed(buf[:n])
				for _, m := range mouseEvents {
					select {
					case eventCh <- AppEvent{Type: EventMouse, Mouse: m}:
					case <-stopCh:
						return
					}
				}
				events := decoder.Feed(data)
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
