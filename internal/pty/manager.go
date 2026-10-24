package pty

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// SpawnSession starts a new interactive shell in a newly allocated PTY.
func SpawnSession(id string, shellPath string, cwd string, cols, rows int) (*PTYSession, error) {
	if cols < 1 {
		cols = 80
	}
	if rows < 1 {
		rows = 24
	}

	if cwd == "" {
		current, err := os.Getwd()
		if err == nil {
			cwd = current
		}
	}

	// Prepare interactive shell command
	cmd := exec.Command(shellPath, "-i")
	cmd.Dir = cwd

	// Inherit environment, overriding terminal-specific variables
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}

	envMap["TERM"] = "xterm-256color"
	envMap["COLORTERM"] = "truecolor"
	envMap["PWD"] = cwd

	var envList []string
	for k, v := range envMap {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	// Set initial window size
	ws := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}

	ptym, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return nil, fmt.Errorf("failed to start pty: %w", err)
	}

	pid := cmd.Process.Pid
	sid, err := unix.Getsid(pid)
	if err != nil || sid <= 0 {
		sid = pid
	}

	sess := &PTYSession{
		ID:         id,
		PTY:        ptym,
		Cmd:        cmd,
		PID:        pid,
		SessionID:  sid,
		InitialCWD: cwd,
		lastCWD:    cwd,
		exited:     make(chan struct{}),
	}

	// Wait for process in background to record exit code
	go func() {
		waitErr := cmd.Wait()
		sess.recordExit(waitErr)
	}()

	return sess, nil
}
