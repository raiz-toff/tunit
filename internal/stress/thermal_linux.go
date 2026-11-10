//go:build linux

package stress

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func ReadCPUTemp() float64 {
	// Try hwmon first (more accurate on modern systems)
	hwmon, _ := filepath.Glob("/sys/class/hwmon/hwmon*/temp*_input")
	for _, f := range hwmon {
		name := strings.Replace(f, "temp1_input", "", 1)
		label, _ := os.ReadFile(name + "label")
		labelStr := strings.ToLower(strings.TrimSpace(string(label)))
		if strings.Contains(labelStr, "package") || strings.Contains(labelStr, "tdie") || strings.Contains(labelStr, "cpu") {
			val := readMilliC(f)
			if val > 0 {
				return val
			}
		}
	}
	// Fallback: thermal_zone (usually zone 0 is CPU on most laptops)
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	for _, z := range zones {
		typeF := strings.Replace(z, "/temp", "/type", 1)
		ztype, _ := os.ReadFile(typeF)
		if strings.Contains(strings.ToLower(string(ztype)), "x86_pkg") ||
			strings.Contains(strings.ToLower(string(ztype)), "cpu") {
			val := readMilliC(z)
			if val > 0 {
				return val
			}
		}
	}
	// Last resort: first thermal zone
	if len(zones) > 0 {
		return readMilliC(zones[0])
	}
	return 0
}

func readBaseFreqMHz() int64 {
	b, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/base_frequency")
	if err != nil {
		b, err = os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/cpuinfo_max_freq")
	}
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n / 1000 // kHz → MHz
}

func readCurFreqMHz() int64 {
	b, err := os.ReadFile("/sys/devices/system/cpu/cpu0/cpufreq/scaling_cur_freq")
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n / 1000
}

func readMilliC(path string) float64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil || n <= 0 {
		return 0
	}
	return float64(n) / 1000.0
}

// tempHint tells the technician why no CPU temperature was readable on Linux
// and what usually fixes it.
func tempHint() string {
	return "No hwmon/thermal_zone sensor found — load the coretemp/k10temp module or install lm-sensors."
}
