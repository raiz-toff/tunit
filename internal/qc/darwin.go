//go:build darwin

package qc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

func spProfiler(dataType string) (string, error) {
	out, err := exec.Command("system_profiler", dataType, "-json").Output()
	return string(out), err
}

func populateIdentity(r *types.QCResult) {
	r.OS, _ = run("sw_vers", "-productVersion")
	r.OS = "macOS " + strings.TrimSpace(r.OS)
	r.Serial, _ = ioregKey("IOPlatformSerialNumber")
	r.Model, _ = ioregKey("model")
	r.BIOSVer, _ = run("system_profiler", "SPHardwareDataType")
	r.CPU, _ = run("sysctl", "-n", "machdep.cpu.brand_string")
	r.CPU = strings.TrimSpace(r.CPU)

	memBytes, _ := run("sysctl", "-n", "hw.memsize")
	var memB int64
	fmt.Sscanf(strings.TrimSpace(memBytes), "%d", &memB)
	if memB > 0 {
		r.RAM = fmt.Sprintf("%.0f GB", float64(memB)/1024/1024/1024)
	}
}

func runAutomatedChecks(r *types.QCResult) []types.CheckResult {
	var results []types.CheckResult
	results = append(results, check(2, "Serial / DMI", checkSerial))
	results = append(results, check(3, "Battery & charging", checkBattery))
	results = append(results, check(5, "Device drivers", checkDrivers))
	results = append(results, check(6, "Disk & SMART health", checkDisk))
	results = append(results, check(7, "Display connected", checkDisplay))
	results = append(results, check(8, "FileVault encryption", checkEncryption))
	results = append(results, check(10, "Network adapters", checkNetwork))
	results = append(results, check(12, "Power / sleep states", checkPower))
	results = append(results, check(13, "Audio devices", checkAudio))
	results = append(results, check(17, "OS version & SIP", checkOSUpdates))
	return results
}

func checkSerial() (types.Status, string, string) {
	serial, err := ioregKey("IOPlatformSerialNumber")
	if err != nil || strings.TrimSpace(serial) == "" {
		return types.StatusFail, "Serial not found", ""
	}
	return types.StatusPass, strings.TrimSpace(serial), ""
}

func checkBattery() (types.Status, string, string) {
	out, err := exec.Command("system_profiler", "SPPowerDataType").Output()
	if err != nil {
		return types.StatusSkip, "Could not read power data", ""
	}
	text := string(out)
	detail := ""
	if strings.Contains(text, "Cycle Count") {
		for _, line := range strings.Split(text, "\n") {
			if strings.Contains(line, "Cycle Count") || strings.Contains(line, "Condition") || strings.Contains(line, "Maximum Capacity") {
				detail += strings.TrimSpace(line) + " | "
			}
		}
		detail = strings.TrimRight(detail, " | ")
	}
	if strings.Contains(text, "Service Battery") || strings.Contains(text, "Replace Now") {
		return types.StatusWarn, detail, "Battery service recommended"
	}
	return types.StatusPass, detail, ""
}

func checkDrivers() (types.Status, string, string) {
	// On macOS, missing kexts/dext surface as "No driver loaded" in system_profiler
	out, _ := exec.Command("system_profiler", "SPUSBDataType").Output()
	if strings.Contains(string(out), "No driver loaded") {
		return types.StatusWarn, "One or more USB devices have no driver loaded", ""
	}
	return types.StatusPass, "No driver errors detected", ""
}

func checkDisk() (types.Status, string, string) {
	out, err := exec.Command("diskutil", "info", "/").Output()
	if err != nil {
		return types.StatusSkip, "diskutil failed", ""
	}
	detail := ""
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Media Name") || strings.Contains(line, "Disk Size") {
			detail += strings.TrimSpace(line) + " "
		}
	}

	// Try smartctl
	smartOut, err := exec.Command("smartctl", "-H", "/dev/disk0").Output()
	if err == nil {
		for _, line := range strings.Split(string(smartOut), "\n") {
			if strings.Contains(line, "SMART overall-health") {
				if strings.Contains(line, "PASSED") {
					detail += "| SMART: PASSED"
				} else {
					return types.StatusFail, detail, "SMART health check FAILED"
				}
			}
		}
	}
	return types.StatusPass, strings.TrimSpace(detail), ""
}

func checkDisplay() (types.Status, string, string) {
	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return types.StatusSkip, "Could not query displays", ""
	}
	count := strings.Count(string(out), "Resolution:")
	if count == 0 {
		return types.StatusWarn, "No display info found", ""
	}
	return types.StatusPass, fmt.Sprintf("%d display(s) detected", count), ""
}

func checkEncryption() (types.Status, string, string) {
	out, err := exec.Command("fdesetup", "status").Output()
	if err != nil {
		return types.StatusSkip, "fdesetup not available", ""
	}
	text := strings.TrimSpace(string(out))
	if strings.Contains(text, "FileVault is On") {
		return types.StatusWarn, "FileVault enabled", "Verify user has recovery key"
	}
	return types.StatusPass, "FileVault Off", ""
}

func checkNetwork() (types.Status, string, string) {
	out, err := exec.Command("networksetup", "-listallhardwareports").Output()
	if err != nil {
		return types.StatusSkip, "networksetup not available", ""
	}
	if strings.Contains(string(out), "Wi-Fi") {
		return types.StatusPass, "Wi-Fi adapter present", ""
	}
	return types.StatusWarn, "No Wi-Fi adapter found", ""
}

func checkPower() (types.Status, string, string) {
	out, err := exec.Command("pmset", "-g").Output()
	if err != nil {
		return types.StatusSkip, "pmset not available", ""
	}
	if strings.Contains(string(out), "sleep") {
		return types.StatusPass, "Sleep configured", ""
	}
	return types.StatusInfo, "pmset output retrieved", ""
}

func checkAudio() (types.Status, string, string) {
	out, err := exec.Command("system_profiler", "SPAudioDataType").Output()
	if err != nil {
		return types.StatusSkip, "Could not query audio", ""
	}
	if strings.Contains(string(out), "Built-in") || strings.Contains(string(out), "Speaker") {
		return types.StatusPass, "Built-in audio detected", ""
	}
	return types.StatusWarn, "No built-in audio found", ""
}

func checkOSUpdates() (types.Status, string, string) {
	ver, _ := run("sw_vers", "-productVersion")
	ver = "macOS " + strings.TrimSpace(ver)

	// SIP status
	sip, err := exec.Command("csrutil", "status").Output()
	sipNote := ""
	if err == nil {
		if strings.Contains(string(sip), "enabled") {
			sipNote = "SIP: enabled"
		} else {
			sipNote = "SIP: DISABLED"
		}
	}

	pending, _ := run("softwareupdate", "-l")
	if strings.Contains(pending, "No new software available") || strings.TrimSpace(pending) == "" {
		return types.StatusPass, ver+" | "+sipNote, ""
	}
	return types.StatusWarn, ver+" | "+sipNote, "Software updates available"
}

// ── helpers ────────────────────────────────────────────────────

func run(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func ioregKey(key string) (string, error) {
	out, err := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, `"`+key+`"`) {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.Trim(strings.TrimSpace(parts[1]), `"<>`), nil
			}
		}
	}
	return "", fmt.Errorf("key not found: %s", key)
}

// ── interactive hardware actions (macOS) ───────────────────────

func playTestTone() {
	exec.Command("afplay", "/System/Library/Sounds/Ping.aiff").Run()
	time.Sleep(500 * time.Millisecond)
	exec.Command("afplay", "/System/Library/Sounds/Pop.aiff").Run()
}

func playMicTest() {
	tmp := "/tmp/tunit_mic_test.aiff"
	exec.Command("sox", "-d", "-t", "aiff", tmp, "trim", "0", "3").Run()
	exec.Command("afplay", tmp).Run()
	os.Remove(tmp)
}

func openCameraPreview() {
	exec.Command("open", "-a", "Photo Booth").Run()
	time.Sleep(3 * time.Second)
	exec.Command("osascript", "-e", `tell application "Photo Booth" to quit`).Run()
}

func runScreenTest() {
	colors := []string{"white", "black", "red", "green", "blue"}
	for _, c := range colors {
		script := fmt.Sprintf(`tell application "System Events" to set background color of first desktop to {%s}`, colorToRGB(c))
		exec.Command("osascript", "-e", script).Run()
		fmt.Printf("  Screen: %s — press ENTER...", c)
		fmt.Scanln()
	}
}

func colorToRGB(name string) string {
	switch name {
	case "white":
		return "65535, 65535, 65535"
	case "black":
		return "0, 0, 0"
	case "red":
		return "65535, 0, 0"
	case "green":
		return "0, 65535, 0"
	case "blue":
		return "0, 0, 65535"
	}
	return "32768, 32768, 32768"
}

func runKeyboardTest() {
	fmt.Println("  Type freely. Press Enter twice when done.")
	reader := bufio.NewReader(os.Stdin)
	emptyCount := 0
	for emptyCount < 2 {
		line, _ := reader.ReadString('\n')
		if strings.TrimSpace(line) == "" {
			emptyCount++
		} else {
			emptyCount = 0
			fmt.Printf("  Got: %q\n", strings.TrimSpace(line))
		}
	}
}

func runUSBWatcher() {
	fmt.Println("  Watching for USB insertions for 15 seconds...")
	fmt.Println("  Insert devices now. Press ENTER to stop early.")

	done := make(chan struct{})
	go func() {
		fmt.Scanln()
		close(done)
	}()

	timer := time.NewTimer(15 * time.Second)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer timer.Stop()

	seen := map[string]bool{}
	for {
		select {
		case <-done:
			return
		case <-timer.C:
			return
		case <-ticker.C:
			out, _ := exec.Command("system_profiler", "SPUSBDataType").Output()
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "Product ID:") {
					dev := strings.TrimSpace(line)
					if !seen[dev] {
						seen[dev] = true
						fmt.Printf("  ✓ USB detected: %s\n", dev)
					}
				}
			}
		}
	}
}
