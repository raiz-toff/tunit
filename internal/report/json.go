package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/tunit/tunit/internal/types"
)

func SaveJSON(r *types.QCResult) (string, error) {
	serial := sanitizeFilename(r.Serial)
	filename := fmt.Sprintf("QC_REPORT_%s_%s.json", serial, r.Timestamp.Format("20060102_150405"))
	if serial == "" || strings.EqualFold(serial, "Unknown") {
		filename = fmt.Sprintf("QC_REPORT_%s.json", r.Timestamp.Format("20060102_150405"))
	}

	f, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return "", err
	}

	return filename, nil
}

func SaveLoadJSON(r *types.LoadResult, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
