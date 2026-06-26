// Package process owns the daemonless launch model and all OS-level process
// control — the heart of the design's "no supervisor" decision (§3.1, §7).
//
// Launch starts a command in its OWN session/process-group and returns
// immediately without waiting. Because the child is detached (Setsid), it
// reparents to launchd when qwok exits and keeps running — qwok itself never
// stays resident. The child PID equals its process-group ID, so we can later
// signal the whole tree (portless + the dev server it spawns) by sending to the
// negative PID.
//
// Status is derived, never stored: IsAlive asks the kernel. The PID file written
// here is only a *hint* — every reader re-checks liveness. Consumed by
// internal/app. Read alongside internal/portless (the other half of status).
package process

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// Launch starts argv detached in dir, with env merged over the current
// environment, writing the child's stdout+stderr to logPath. It returns the
// child PID (== process-group leader) and does NOT wait — the process outlives
// qwok by design.
func Launch(dir string, argv []string, env map[string]string, logPath string) (int, error) {
	if len(argv) == 0 {
		return 0, fmt.Errorf("empty command")
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		return 0, err
	}
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return 0, err
	}
	defer logf.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.Stdin = nil
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	// Setsid: new session + process group, detached from our controlling
	// terminal, so the child survives qwok's exit and can be signalled as a group.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	// Release so the Go runtime stops tracking the detached child.
	_ = cmd.Process.Release()
	return pid, nil
}

// IsAlive reports whether pid is a live process. A "permission denied" from
// signal 0 still means the process exists, so we treat it as alive.
func IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return err == syscall.EPERM
}

// SignalGroup sends sig to the entire process group led by pid (note the
// negative target). This reaches both portless and the dev server it spawned.
func SignalGroup(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	return syscall.Kill(-pid, sig)
}

// WritePID records the hint PID for an app.
func WritePID(path string, pid int) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strconv.Itoa(pid)), 0o644)
}

// ReadPID returns the hint PID, or 0 if there is no hint.
func ReadPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	return pid
}

// RemovePID clears the hint (best-effort).
func RemovePID(path string) { _ = os.Remove(path) }
