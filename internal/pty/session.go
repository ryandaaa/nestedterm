package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// PTYSession represents an active pseudo-terminal session running a child process.
type PTYSession struct {
	ID         string
	PTY        *os.File
	Cmd        *exec.Cmd
	PID        int
	SessionID  int
	InitialCWD string

	mu        sync.Mutex
	lastCWD   string
	closed    bool
	exitCode  *int
	exited    chan struct{}
	waitErr   error
	closeOnce sync.Once
}

// Write writes bytes directly to the PTY master (input to child process).
func (s *PTYSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	closed := s.closed
	f := s.PTY
	s.mu.Unlock()

	if closed || f == nil {
		return 0, errors.New("pty session is closed")
	}
	return f.Write(p)
}

// Resize propagates the terminal window size to the PTY master.
func (s *PTYSession) Resize(cols, rows int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed || s.PTY == nil {
		return nil
	}

	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}

	ws := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}
	return pty.Setsize(s.PTY, ws)
}

// GetCWD returns the current working directory, dynamically refreshing via /proc.
func (s *PTYSession) GetCWD() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := ResolveCWD(s.PID, s.lastCWD)
	s.lastCWD = current
	return current
}

// IsExited reports whether the child process has terminated.
func (s *PTYSession) IsExited() bool {
	select {
	case <-s.exited:
		return true
	default:
		return false
	}
}

// ExitCode returns the exit code if the process has exited, or nil.
func (s *PTYSession) ExitCode() *int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode
}

// Close gracefully terminates the session processes and closes PTY resources.
func (s *PTYSession) Close(gracePeriod time.Duration) error {
	var err error
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()

		// Terminate process session
		if s.SessionID > 0 {
			_ = TerminateSession(s.SessionID, gracePeriod)
		} else if s.PID > 0 {
			_ = syscall.Kill(s.PID, syscall.SIGTERM)
		}

		// Close PTY master
		if s.PTY != nil {
			err = s.PTY.Close()
		}
	})
	return err
}

// Wait blocks until the child process terminates and returns its exit code.
func (s *PTYSession) Wait() (*int, error) {
	<-s.exited
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode, s.waitErr
}

func (s *PTYSession) recordExit(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.waitErr = err
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	s.exitCode = &code
	close(s.exited)
}

func (s *PTYSession) String() string {
	return fmt.Sprintf("PTYSession[id=%s, pid=%d, sid=%d, cwd=%s]", s.ID, s.PID, s.SessionID, s.lastCWD)
}
