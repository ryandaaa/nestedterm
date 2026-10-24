package pty

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// FindSessionPIDs searches /proc for all running processes whose session ID matches sessionID.
func FindSessionPIDs(sessionID int) ([]int, error) {
	if sessionID <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc: %w", err)
	}

	var pids []int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue // not a PID directory
		}

		statBytes, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
		if err != nil {
			continue // process might have exited
		}

		statStr := string(statBytes)
		rParen := strings.LastIndex(statStr, ")")
		if rParen == -1 || rParen+2 >= len(statStr) {
			continue
		}

		fields := strings.Fields(statStr[rParen+2:])
		// In /proc/<pid>/stat, after ') ':
		// field 0: state
		// field 1: ppid
		// field 2: pgrp
		// field 3: session
		if len(fields) > 3 {
			sid, err := strconv.Atoi(fields[3])
			if err == nil && sid == sessionID {
				pids = append(pids, pid)
			}
		}
	}

	return pids, nil
}

// TerminateSession terminates all processes belonging to the managed session ID.
// It sends SIGTERM to the session group, waits up to gracePeriod, and forcefully
// sends SIGKILL to any remaining processes in the session.
func TerminateSession(sessionID int, gracePeriod time.Duration) error {
	if sessionID <= 0 {
		return nil
	}

	if gracePeriod <= 0 {
		gracePeriod = 750 * time.Millisecond
	}

	// Step 1: Send SIGTERM to the process group (-sessionID) and discovered PIDs
	_ = syscall.Kill(-sessionID, syscall.SIGTERM)
	pids, _ := FindSessionPIDs(sessionID)
	for _, pid := range pids {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}

	// Step 2: Bounded wait loop
	deadline := time.Now().Add(gracePeriod)
	for time.Now().Before(deadline) {
		remaining, _ := FindSessionPIDs(sessionID)
		if len(remaining) == 0 {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	// Step 3: Send SIGKILL to remaining processes
	remaining, _ := FindSessionPIDs(sessionID)
	_ = syscall.Kill(-sessionID, syscall.SIGKILL)
	for _, pid := range remaining {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}

	// Give a brief moment for SIGKILL cleanup
	time.Sleep(50 * time.Millisecond)
	return nil
}
