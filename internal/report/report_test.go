package report

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tunit/tunit/internal/types"
)

func TestTrunc(t *testing.T) {
	tests := []struct {
		input string
		max   int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hell…"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "ab…"},
		// multi-byte: 3 CJK chars, trunc to 2 should give 1 char + ellipsis
		{"日本語", 2, "日…"},
		// emoji
		{"🔥🔥🔥", 2, "🔥…"},
	}
	for _, tt := range tests {
		got := trunc(tt.input, tt.max)
		if got != tt.want {
			t.Errorf("trunc(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
		}
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"ABC123", "ABC123"},
		{"../../../etc/passwd", "______etc_passwd"},
		{"SERIAL/NUM", "SERIAL_NUM"},
		{"back\\slash", "back_slash"},
		{"normal-serial", "normal-serial"},
		{"has spaces", "has_spaces"},
	}
	for _, tt := range tests {
		got := sanitizeFilename(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSaveTextSlip(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	ts := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	r := &types.QCResult{
		Timestamp: ts,
		Serial:    "ABC123",
		Model:     "TestModel",
		OS:        "Linux",
		Verdict:   types.StatusPass,
		Checks: []types.CheckResult{
			{Step: 1, Name: "Test check", Status: types.StatusPass, Detail: "All good"},
		},
	}

	fname, err := SaveTextSlip(r)
	if err != nil {
		t.Fatalf("SaveTextSlip: %v", err)
	}
	if fname != "QC_REPORT_ABC123_20260115_103000.txt" {
		t.Errorf("filename = %q, want QC_REPORT_ABC123_20260115_103000.txt", fname)
	}

	data, err := os.ReadFile(filepath.Join(dir, fname))
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "VERDICT") {
		t.Error("report missing VERDICT line")
	}
	if !strings.Contains(content, "PASS") {
		t.Error("report missing PASS status")
	}
	if !strings.Contains(content, "ABC123") {
		t.Error("report missing serial number")
	}
}

func TestSaveTextSlipEmptySerial(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	ts := time.Date(2026, 3, 20, 14, 0, 0, 0, time.UTC)
	r := &types.QCResult{
		Timestamp: ts,
		Serial:    "",
		Verdict:   types.StatusPass,
	}

	fname, err := SaveTextSlip(r)
	if err != nil {
		t.Fatalf("SaveTextSlip: %v", err)
	}
	if fname != "QC_REPORT_20260320_140000.txt" {
		t.Errorf("filename = %q, want QC_REPORT_20260320_140000.txt", fname)
	}
}

func TestSaveTextSlipPathInjection(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	r := &types.QCResult{
		Timestamp: time.Now(),
		Serial:    "../../etc/evil",
		Verdict:   types.StatusPass,
	}
	fname, err := SaveTextSlip(r)
	if err != nil {
		t.Fatalf("SaveTextSlip: %v", err)
	}
	if strings.Contains(fname, "/") || strings.Contains(fname, "..") {
		t.Errorf("filename contains path chars: %q", fname)
	}
}

func TestSaveLoadJSON(t *testing.T) {
	dir := t.TempDir()
	outFile := filepath.Join(dir, "results.json")

	r := &types.LoadResult{
		URL:       "https://example.com",
		Method:    "GET",
		TotalReqs: 1000,
		Errors:    5,
		Duration:  30 * time.Second,
	}

	err := SaveLoadJSON(r, outFile)
	if err != nil {
		t.Fatalf("SaveLoadJSON: %v", err)
	}

	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "https://example.com") {
		t.Error("JSON missing URL")
	}
	if !strings.Contains(content, "1000") {
		t.Error("JSON missing TotalReqs")
	}
}
