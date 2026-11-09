package report

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_")
	return r.Replace(s)
}

// reportFilename builds the QC report's base filename from the serial and
// timestamp, falling back to a timestamp-only name when the serial is empty.
func reportFilename(r *types.QCResult) string {
	serial := sanitizeFilename(r.Serial)
	if serial == "" || strings.EqualFold(serial, "Unknown") {
		return fmt.Sprintf("QC_REPORT_%s.txt", r.Timestamp.Format("20060102_150405"))
	}
	return fmt.Sprintf("QC_REPORT_%s_%s.txt", serial, r.Timestamp.Format("20060102_150405"))
}

// SaveTextSlip writes the report to the current directory and returns its
// filename. Retained for the CI/automation path and existing callers.
func SaveTextSlip(r *types.QCResult) (string, error) {
	return SaveTextSlipTo(r, ".")
}

// SaveTextSlipTo writes the report into dir and returns the full path. The
// bench/interactive path passes the session temp dir so the file is removed
// when tunit exits, leaving no report on the serviced machine.
func SaveTextSlipTo(r *types.QCResult, dir string) (string, error) {
	name := reportFilename(r)
	path := name
	if dir != "" && dir != "." {
		path = filepath.Join(dir, name)
	}

	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	w := func(format string, a ...any) {
		fmt.Fprintf(f, format+"\n", a...)
	}

	sep := strings.Repeat("=", 42)
	divider := strings.Repeat("-", 42)

	w(sep)
	w("        SYSTEM DIAGNOSTIC REPORT")
	if r.Serial != "" {
		w("S/N: %-20s DATE: %s", r.Serial, r.Timestamp.Format("2006-01-02"))
	}
	if r.Model != "" {
		w("MODEL: %s", r.Model)
	}
	if r.WarrantyExpiry != "" {
		w("WARRANTY: %s (%s)", r.WarrantyExpiry, r.WarrantyVendor)
	}
	w(divider)

	for _, c := range r.Checks {
		label := fmt.Sprintf("[%s]", c.Status.String())
		detail := c.Detail
		if c.Warning != "" {
			detail += " !" + c.Warning
		}
		w("%-6s %-22s : %s", label, trunc(c.Name, 22), detail)
	}

	w(divider)
	w("VERDICT  : %s", r.Verdict.String())
	w("TESTED   : %s", r.Timestamp.Format(time.RFC1123))
	w("")
	w("QC TESTER: _______________________________")
	w(sep)

	return path, nil
}
