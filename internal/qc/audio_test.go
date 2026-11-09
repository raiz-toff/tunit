package qc

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateTestWAV(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "test.wav")
	if err := generateTestWAV(tmp); err != nil {
		t.Fatalf("generateTestWAV: %v", err)
	}

	info, err := os.Stat(tmp)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	// 6.6s of stereo 16-bit 44100Hz ≈ ~1.16MB
	if info.Size() < 500_000 {
		t.Errorf("WAV too small: %d bytes", info.Size())
	}

	// Verify RIFF header
	f, _ := os.Open(tmp)
	defer f.Close()
	header := make([]byte, 12)
	f.Read(header)
	if string(header[:4]) != "RIFF" {
		t.Errorf("missing RIFF header, got %q", header[:4])
	}
	if string(header[8:12]) != "WAVE" {
		t.Errorf("missing WAVE marker, got %q", header[8:12])
	}
}
