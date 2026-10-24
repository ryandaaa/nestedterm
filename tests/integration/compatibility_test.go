package integration

import (
	"strings"
	"testing"
	"time"

	"nestedterm/internal/pty"
	"nestedterm/internal/terminal"
)

func runInteractiveTest(t *testing.T, cmd string, inputs []string, check func(screen string) bool) {
	t.Helper()

	sess, err := pty.SpawnSession("compat-test", "/bin/bash", t.TempDir(), 80, 24)
	if err != nil {
		t.Fatalf("SpawnSession failed: %v", err)
	}
	defer sess.Close(200 * time.Millisecond)

	emu := terminal.NewAdapter(80, 24, 500)
	defer emu.Close()

	go pty.RunSessionIO(sess, emu, nil, nil)

	// Wait for prompt
	time.Sleep(300 * time.Millisecond)

	// Send command
	_, err = sess.Write([]byte(cmd + "\n"))
	if err != nil {
		t.Fatalf("Write cmd failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	for _, input := range inputs {
		time.Sleep(200 * time.Millisecond)
		_, _ = sess.Write([]byte(input))
	}

	// Poll screen for expected condition
	deadline := time.Now().Add(4 * time.Second)
	matched := false
	var finalScreen string

	for time.Now().Before(deadline) {
		var sb strings.Builder
		for y := 0; y < emu.Height(); y++ {
			var row strings.Builder
			for x := 0; x < emu.Width(); x++ {
				c := emu.CellAt(x, y)
				if c != nil && c.Content != "" {
					row.WriteString(c.Content)
				} else {
					row.WriteByte(' ')
				}
			}
			trimmed := strings.TrimRight(row.String(), " ")
			if trimmed != "" {
				sb.WriteString(trimmed)
				sb.WriteString("\n")
			}
		}
		finalScreen = sb.String()
		if check(finalScreen) {
			matched = true
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !matched {
		t.Fatalf("Compatibility test for %q failed. Screen output:\n%s", cmd, finalScreen)
	}
}

func TestCompatInteractiveBash(t *testing.T) {
	runInteractiveTest(t, "echo COMPAT_BASH_OK", nil, func(screen string) bool {
		return strings.Contains(screen, "COMPAT_BASH_OK")
	})
}

func TestCompatGitStatus(t *testing.T) {
	runInteractiveTest(t, "git status", nil, func(screen string) bool {
		return strings.Contains(screen, "branch") || strings.Contains(screen, "Untracked") || strings.Contains(screen, "nothing to commit") || strings.Contains(screen, "fatal")
	})
}

func TestCompatLess(t *testing.T) {
	// Start less on a generated file, then exit with 'q'
	runInteractiveTest(t, "seq 1 100 | less", []string{"q"}, func(screen string) bool {
		return strings.Contains(screen, "1") && strings.Contains(screen, "2")
	})
}

func TestCompatNano(t *testing.T) {
	// Start nano editor, verify header/editor shows, then exit with Ctrl+X (\x18)
	runInteractiveTest(t, "nano", []string{"\x18"}, func(screen string) bool {
		return strings.Contains(screen, "GNU nano") || strings.Contains(screen, "Help") || strings.Contains(screen, "Exit")
	})
}

func TestCompatSSH(t *testing.T) {
	runInteractiveTest(t, "ssh -V", nil, func(screen string) bool {
		return strings.Contains(screen, "OpenSSH") || strings.Contains(screen, "ssh")
	})
}
