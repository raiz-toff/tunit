//go:build darwin

package qc

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/cleanup"
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

func probeCapabilities() capabilities {
	var c capabilities
	// afplay ships with every macOS install.
	c.audioPlay = capability{ok: true, tool: "afplay"}
	if t, ok := lookTools("sox", "ffmpeg"); ok {
		c.audioRecord = capability{ok: true, tool: t}
	} else {
		c.audioRecord = capability{hint: "brew install sox (or ffmpeg) for mic capture"}
	}
	// ffmpeg gives the terminal preview; Photo Booth is the GUI fallback and
	// is present on stock macOS, so the webcam check can always run.
	if t, ok := lookTools("ffmpeg"); ok {
		c.camera = capability{ok: true, tool: t}
	} else {
		c.camera = capability{ok: true, tool: "Photo Booth (GUI fallback)"}
	}
	return c
}

func playTestTone() {
	tmp := cleanup.TempPathFor("stereo_test.wav")
	if err := generateTestWAV(tmp); err == nil {
		exec.Command("afplay", tmp).Run()
		os.Remove(tmp)
		return
	}
	exec.Command("afplay", "/System/Library/Sounds/Ping.aiff").Run()
}

func playMicTest() {
	tmp := cleanup.TempPathFor("mic_test.wav")
	// Use sox if available, otherwise ffmpeg
	if _, err := exec.LookPath("sox"); err == nil {
		exec.Command("sox", "-d", "-r", "44100", "-c", "1", tmp, "trim", "0", "3").Run()
	} else {
		exec.Command("ffmpeg", "-y", "-f", "avfoundation", "-i", ":0", "-t", "3", tmp).Run()
	}
	exec.Command("afplay", tmp).Run()
	os.Remove(tmp)
}

func openCameraPreview() {
	err := terminalCameraPreview(
		[]string{"-f", "avfoundation", "-framerate", "15", "-video_size", "320x240", "-i", "0"},
		5*time.Second,
	)
	if err != nil {
		exec.Command("open", "-a", "Photo Booth").Run()
		time.Sleep(5 * time.Second)
		exec.Command("osascript", "-e", `tell application "Photo Booth" to quit`).Run()
	}
}

func runScreenTest() {
	fullScreenColorTest()
}

func runKeyboardTest() int {
	return runVisualKeyboardTest()
}

func runUSBWatcher() {
	timeout := 30 * time.Second
	start := time.Now()

	done := make(chan struct{})
	go func() {
		fmt.Scanln()
		close(done)
	}()

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h\033[0m")

	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	defer timer.Stop()

	seen := map[string]bool{}
	var devices []usbDevice

	// Snapshot existing devices
	out, _ := exec.Command("system_profiler", "SPUSBDataType").Output()
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Product ID:") {
			seen[strings.TrimSpace(line)] = true
		}
	}

	renderUSBDisplay(devices, 0, timeout)

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
						devices = append(devices, usbDevice{name: dev, when: time.Now()})
					}
				}
			}
			renderUSBDisplay(devices, time.Since(start), timeout)
		}
	}
}
