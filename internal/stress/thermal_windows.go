//go:build windows

package stress

import (
	"os/exec"
	"strconv"
	"strings"
)

func ReadCPUTemp() float64 {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-Command", `(Get-WmiObject MSAcpi_ThermalZoneTemperature -Namespace root/wmi).CurrentTemperature`).Output()
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		n, err := strconv.ParseFloat(strings.TrimSpace(line), 64)
		if err == nil && n > 0 {
			// WMI returns tenths of Kelvin
			return (n/10.0) - 273.15
		}
	}
	return 0
}

func readBaseFreqMHz() int64 {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-Command", `(Get-WmiObject Win32_Processor).MaxClockSpeed`).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}

func readCurFreqMHz() int64 {
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-Command", `(Get-WmiObject Win32_Processor).CurrentClockSpeed`).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	return n
}
