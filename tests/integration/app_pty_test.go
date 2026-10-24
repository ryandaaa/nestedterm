//go:build linux

package integration

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// These tests drive the real nestedterm binary inside a PTY, the way a user's
// terminal would.

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
)

func nestedtermBinary(t *testing.T) string {
	t.Helper()
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "nestedterm-bin")
		if err != nil {
			buildErr = err
			return
		}
		binPath = filepath.Join(dir, "nestedterm")
		out, err := exec.Command("go", "build", "-o", binPath, "../../cmd/nestedterm").CombinedOutput()
		if err != nil {
			buildErr = &buildError{err: err, out: string(out)}
		}
	})
	if buildErr != nil {
		t.Fatalf("building nestedterm: %v", buildErr)
	}
	return binPath
}

type buildError struct {
	err error
	out string
}

func (e *buildError) Error() string { return e.err.Error() + ": " + e.out }

type session struct {
	t    *testing.T
	cmd  *exec.Cmd
	ptmx *os.File

	mu   sync.Mutex
	out  bytes.Buffer
	done chan struct{} // closed when the process exits
	werr error
}

func startApp(t *testing.T, env ...string) *session {
	t.Helper()
	cmd := exec.Command(nestedtermBinary(t))
	cmd.Env = append([]string{"TERM=xterm-256color", "SHELL=/bin/sh", "HOME=" + t.TempDir()}, env...)
	cmd.Dir = t.TempDir()
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 30, Cols: 100})
	if err != nil {
		t.Fatal(err)
	}
	s := &session{t: t, cmd: cmd, ptmx: ptmx, done: make(chan struct{})}

	go func() {
		buf := make([]byte, 64<<10)
		for {
			n, err := ptmx.Read(buf)
			if n > 0 {
				s.mu.Lock()
				s.out.Write(buf[:n])
				s.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	go func() {
		s.werr = cmd.Wait()
		close(s.done)
	}()

	t.Cleanup(func() {
		select {
		case <-s.done:
		default:
			_ = cmd.Process.Kill()
			<-s.done
		}
		_ = ptmx.Close()
	})
	return s
}

func (s *session) send(b string) {
	s.t.Helper()
	if _, err := s.ptmx.WriteString(b); err != nil {
		s.t.Fatal(err)
	}
}

func (s *session) output() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.out.Bytes()...)
}

func (s *session) waitFor(sub string, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		if bytes.Contains(s.output(), []byte(sub)) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func (s *session) exited(d time.Duration) bool {
	select {
	case <-s.done:
		return true
	case <-time.After(d):
		return false
	}
}

func (s *session) waitReady() {
	s.t.Helper()
	if !s.waitFor("\x1b[", 5*time.Second) {
		s.t.Fatal("no initial paint")
	}
	time.Sleep(300 * time.Millisecond) // let the shell print its prompt
}

func TestAppExitInLastPaneClosesApp(t *testing.T) {
	s := startApp(t)
	s.waitReady()
	s.send("exit\n")
	if !s.exited(5 * time.Second) {
		t.Fatal("app kept running after the only shell exited")
	}
	if s.werr != nil {
		t.Fatalf("app exited with error: %v", s.werr)
	}
}

func TestAppExitInOnePaneKeepsOthers(t *testing.T) {
	s := startApp(t)
	s.waitReady()

	s.send("\x1bn") // Alt+n: new pane
	time.Sleep(400 * time.Millisecond)
	s.send("exit\n")
	if s.exited(1 * time.Second) {
		t.Fatal("app exited although one pane was still alive")
	}

	s.send("echo still_$((6*7))\n")
	if !s.waitFor("still_42", 3*time.Second) {
		t.Fatal("remaining pane is not responsive")
	}
	s.send("exit\n")
	if !s.exited(5 * time.Second) {
		t.Fatal("app kept running after the last shell exited")
	}
}

func TestAppIdleWritesNothing(t *testing.T) {
	s := startApp(t)
	s.waitReady()
	before := len(s.output())
	time.Sleep(700 * time.Millisecond)
	if n := len(s.output()) - before; n != 0 {
		t.Fatalf("idle app wrote %d bytes", n)
	}
}

func TestAppKeystrokeLatency(t *testing.T) {
	s := startApp(t)
	s.waitReady()

	var lat []time.Duration
	for _, ch := range "abcdefgh" {
		time.Sleep(120 * time.Millisecond)
		before := len(s.output())
		start := time.Now()
		s.send(string(ch))
		for len(s.output()) == before && time.Since(start) < time.Second {
			time.Sleep(200 * time.Microsecond)
		}
		lat = append(lat, time.Since(start))
	}
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	// A 25ms frame timer would put the median near 12ms+; the generous bound
	// only catches regressions such as an always-on ticker delaying echoes.
	if med := lat[len(lat)/2]; med > 20*time.Millisecond {
		t.Fatalf("median keystroke-to-output latency %v (all: %v)", med, lat)
	}
}

func TestAppPassesPaletteColorsThrough(t *testing.T) {
	s := startApp(t, "NESTEDTERM_TRUECOLOR=0")
	s.waitReady()
	s.send(`printf '\033[31mRED\033[0m \033[1;92mBRIGHT\033[0m \033[38;2;10;200;120mRGB\033[0m\n'` + "\n")
	if !s.waitFor("\x1b[31mRED", 3*time.Second) {
		t.Fatalf("basic red was not passed through as SGR 31:\n%q", s.output())
	}
	if !bytes.Contains(s.output(), []byte("\x1b[92mBRIGHT")) {
		t.Fatal("bright green was not passed through as SGR 92")
	}
	if bytes.Contains(s.output(), []byte("\x1b[38;2;10;200;120m")) {
		t.Fatal("truecolor was emitted although NESTEDTERM_TRUECOLOR=0")
	}
}

func TestAppSIGTERMShutsDownCleanly(t *testing.T) {
	s := startApp(t)
	s.waitReady()
	_ = s.cmd.Process.Signal(syscall.SIGTERM)
	if !s.exited(5 * time.Second) {
		t.Fatal("app did not exit on SIGTERM")
	}
}
