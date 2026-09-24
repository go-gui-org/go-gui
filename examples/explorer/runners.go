package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Runner tracks child example processes.
type Runner struct {
	mu   sync.Mutex
	cmds map[string]*exec.Cmd
}

// NewRunner creates a Runner.
func NewRunner() *Runner {
	return &Runner{cmds: make(map[string]*exec.Cmd)}
}

// IsRunning reports whether name is currently running.
func (r *Runner) IsRunning(name string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	cmd, ok := r.cmds[name]
	if !ok || cmd == nil || cmd.Process == nil {
		return false
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		delete(r.cmds, name)
		return false
	}
	return true
}

// validExampleName rejects anything that is not a plain example
// directory basename: separators, parent refs, and flag-like names
// must never reach exec.Command.
func validExampleName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return false
	}
	if strings.HasPrefix(name, "-") {
		return false
	}
	return true
}

// repoRootDir walks up from the working directory to the module root
// so child examples run with the repo as their working directory.
func repoRootDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for range 5 {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// Start launches "go run ./examples/<name>" if not already running.
// Returns error if already running or if name is not runnable.
func (r *Runner) Start(name string, runnable bool) error {
	if !runnable {
		return fmt.Errorf("example %q is not runnable with go run", name)
	}
	if !validExampleName(name) {
		return fmt.Errorf("invalid example name %q", name)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if cmd, ok := r.cmds[name]; ok && cmd.Process != nil {
		if cmd.ProcessState == nil || !cmd.ProcessState.Exited() {
			return fmt.Errorf("already running (pid %d)", cmd.Process.Pid)
		}
		delete(r.cmds, name)
	}
	cmd := exec.Command("go", "run", "./examples/"+name) // #nosec G204 -- name is validated basename
	if root := repoRootDir(); root != "" {
		cmd.Dir = root
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %q: %w", name, err)
	}
	r.cmds[name] = cmd
	// Reap in background to avoid zombies; drop the entry so a
	// later Start does not see a stale pid. Caller polls IsRunning.
	go func(c *exec.Cmd, n string) {
		_ = c.Wait()
		r.mu.Lock()
		if cur, ok := r.cmds[n]; ok && cur == c {
			delete(r.cmds, n)
		}
		r.mu.Unlock()
	}(cmd, name)
	return nil
}

// Stop sends Interrupt, then Kill if needed.
func (r *Runner) Stop(name string) error {
	r.mu.Lock()
	cmd, ok := r.cmds[name]
	r.mu.Unlock()
	if !ok || cmd == nil || cmd.Process == nil {
		return fmt.Errorf("not running: %q", name)
	}
	if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
		r.mu.Lock()
		delete(r.cmds, name)
		r.mu.Unlock()
		return fmt.Errorf("not running: %q", name)
	}
	// Try interrupt, fall back to kill.
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		_ = cmd.Process.Kill()
	}
	// Wait briefly not to block UI; reaper goroutine will collect.
	return nil
}

// KillAll terminates all tracked processes.
func (r *Runner) KillAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cmd := range r.cmds {
		if cmd != nil && cmd.Process != nil && (cmd.ProcessState == nil || !cmd.ProcessState.Exited()) {
			_ = cmd.Process.Kill()
		}
	}
}
