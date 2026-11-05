package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

func sanitizeFilename(s string) string {
	r := strings.NewReplacer("/", "_", "\\", "_", "..", "_", " ", "_")
	return r.Replace(s)
}

func SaveTextSlip(r *types.QCResult) (string, error) {
	serial := sanitizeFilename(r.Serial)
	filename := fmt.Sprintf("QC_REPORT_%s_%s.txt", serial, r.Timestamp.Format("20060102_150405"))
	if serial == "" || strings.EqualFold(serial, "Unknown") {
		filename = fmt.Sprintf("QC_REPORT_%s.txt", r.Timestamp.Format("20060102_150405"))
	}

	f, err := os.Create(filename)
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

	return filename, nil
}
