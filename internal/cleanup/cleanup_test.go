package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTempPathRemovedOnClose(t *testing.T) {
	s := New()
	p := s.TempPath("scratch.wav")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if filepath.Dir(p) != s.Dir() {
		t.Errorf("temp path %q not inside session dir %q", p, s.Dir())
	}
	s.Close()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("temp file still present after Close: %v", err)
	}
	if _, err := os.Stat(s.Dir()); !os.IsNotExist(err) {
		t.Errorf("session dir still present after Close")
	}
}

func TestTrackedFileRemoved(t *testing.T) {
	s := New()
	defer s.Close()
	dir := t.TempDir()
	report := filepath.Join(dir, "QC_REPORT_test.txt")
	if err := os.WriteFile(report, []byte("report"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	s.Track(report)
	s.Close()
	if _, err := os.Stat(report); !os.IsNotExist(err) {
		t.Errorf("tracked report still present after Close")
	}
}

func TestRestoresRunBeforeRemovalAndLIFO(t *testing.T) {
	s := New()
	var order []int
	s.AddRestore(func() { order = append(order, 1) })
	s.AddRestore(func() { order = append(order, 2) })
	s.Close()
	if len(order) != 2 || order[0] != 2 || order[1] != 1 {
		t.Errorf("restores ran in wrong order: %v (want [2 1])", order)
	}
}

func TestCloseIdempotent(t *testing.T) {
	s := New()
	s.Close()
	s.Close() // must not panic or double-run
}

func TestRestorePanicDoesNotAbortCleanup(t *testing.T) {
	s := New()
	p := s.TempPath("x")
	_ = os.WriteFile(p, []byte("x"), 0o600)
	s.AddRestore(func() { panic("boom") })
	s.Close()
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Errorf("file not removed after a panicking restorer")
	}
}
