//go:build darwin

package stress

import (
	"os/exec"
	"strconv"
	"strings"
)

func ReadCPUTemp() float64 {
	// powermetrics requires root but gives accurate SMC readings
	out, err := exec.Command("powermetrics", "--samplers", "smc", "-n", "1", "-i", "100").Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(out), "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "cpu die") || strings.Contains(lower, "cpu package") {
			fields := strings.Fields(line)
			for _, f := range fields {
				f = strings.TrimSuffix(f, "C")
				f = strings.TrimSuffix(f, "°C")
				n, err := strconv.ParseFloat(f, 64)
				if err == nil && n > 20 && n < 120 {
					return n
				}
			}
		}
	}
	return 0
}

func readBaseFreqMHz() int64 {
	out, err := exec.Command("sysctl", "-n", "hw.cpufrequency_max").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n / 1_000_000 // Hz → MHz
}

func readCurFreqMHz() int64 {
	out, err := exec.Command("sysctl", "-n", "hw.cpufrequency").Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n / 1_000_000
}

// tempHint tells the technician why no CPU temperature was readable on macOS.
func tempHint() string {
	return "powermetrics needs root for SMC readings — run: sudo tunit stress"
}
