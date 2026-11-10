// Package cleanup centralizes every temporary file and terminal-state change
// tunit makes during a run, and guarantees they are undone — on normal exit
// AND on Ctrl+C / termination.
//
// The goal is a bench-service guarantee: a unit is handed back to its owner
// with nothing tunit created left behind, and with the terminal in a usable
// state, even if the technician interrupts a run partway through.
//
// Scope is deliberately narrow. This package only ever removes files that
// tunit itself created (tracked explicitly via TempPath/Track) and only ever
// restores terminal state that tunit itself changed. It never touches system
// logs, shell history, timestamps, or any file it did not create.
package cleanup

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

// Session owns the artifacts of a single tunit run.
type Session struct {
	mu       sync.Mutex
	dir      string   // session temp dir; all transient files live here
	extra    []string // tracked files created outside dir (e.g. a report in CWD)
	restores []func() // terminal-state restorers, run LIFO before file removal
	cleaned  bool
	sigCh    chan os.Signal
}

// pkg-level current session, so signal handling and helpers have one target.
var (
	current *Session
	curMu   sync.Mutex
)

// New creates a session with a private temp directory and installs a handler
// that cleans up on SIGINT/SIGTERM. Call Close (usually via defer) on normal
// exit. If the temp dir cannot be created, transient files fall back to the
// OS temp dir and are still tracked individually.
func New() *Session {
	s := &Session{}
	if dir, err := os.MkdirTemp("", "tunit-"); err == nil {
		s.dir = dir
	}

	s.sigCh = make(chan os.Signal, 1)
	signal.Notify(s.sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-s.sigCh
		s.Close()
		// Re-raise with default disposition so the exit code reflects the
		// interrupt rather than a clean 0.
		signal.Stop(s.sigCh)
		if p, err := os.FindProcess(os.Getpid()); err == nil {
			_ = p.Signal(syscall.SIGINT)
		}
		os.Exit(130) // 128 + SIGINT, in case the re-raise is swallowed
	}()

	curMu.Lock()
	current = s
	curMu.Unlock()
	return s
}

// Dir returns the session temp directory (may be "" if it could not be made).
func (s *Session) Dir() string { return s.dir }

// TempPath returns a path for a transient file with the given base name,
// inside the session dir when available. The file is removed on Close.
func (s *Session) TempPath(name string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dir != "" {
		return filepath.Join(s.dir, name)
	}
	// Fallback: OS temp dir, but track the exact path so it is still removed.
	p := filepath.Join(os.TempDir(), "tunit-"+name)
	s.extra = append(s.extra, p)
	return p
}

// Track registers a file tunit created outside the session dir (for example a
// report written to the working directory) so it is removed on Close. Pass
// only paths tunit itself wrote.
func (s *Session) Track(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extra = append(s.extra, path)
}

// AddRestore registers a terminal-state restorer (show cursor, leave alt
// screen, restore raw mode). Restorers run in reverse order on Close and on
// interrupt, before any files are removed, so the terminal is usable first.
func (s *Session) AddRestore(fn func()) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restores = append(s.restores, fn)
}

// Close runs all restorers, then removes the session dir and every tracked
// file. It is safe to call multiple times and from multiple goroutines.
func (s *Session) Close() {
	s.mu.Lock()
	if s.cleaned {
		s.mu.Unlock()
		return
	}
	s.cleaned = true
	restores := s.restores
	extra := s.extra
	dir := s.dir
	s.mu.Unlock()

	// Terminal first, so the operator sees a working screen immediately.
	for i := len(restores) - 1; i >= 0; i-- {
		runSafe(restores[i])
	}
	for _, p := range extra {
		_ = os.Remove(p)
	}
	if dir != "" {
		_ = os.RemoveAll(dir)
	}
}

// runSafe runs a restorer without letting a panic abort the rest of cleanup.
func runSafe(fn func()) {
	defer func() { _ = recover() }()
	fn()
}

// Current returns the active session, or nil if New has not been called. It
// lets leaf code (hardware actions, stress) reach the session without
// threading it through every signature.
func Current() *Session {
	curMu.Lock()
	defer curMu.Unlock()
	return current
}

// TempPathFor is a nil-safe convenience: it uses the current session when one
// exists, otherwise falls back to the OS temp dir (untracked). Leaf code that
// may run in tests without a session uses this.
func TempPathFor(name string) string {
	if s := Current(); s != nil {
		return s.TempPath(name)
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("tunit-%d-%s", os.Getpid(), name))
}
