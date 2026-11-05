//go:build linux

package qc

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

func populateIdentity(r *types.QCResult) {
	r.OS = readOSRelease()
	r.Serial = readSysFile("/sys/class/dmi/id/product_serial")
	r.Model = readSysFile("/sys/class/dmi/id/product_name")
	r.BIOSVer = readSysFile("/sys/class/dmi/id/bios_version")
	r.CPU = readCPUInfo()
	r.RAM = readRAMInfo()
}

func runAutomatedChecks(r *types.QCResult) []types.CheckResult {
	var results []types.CheckResult

	results = append(results, check(2, "Serial / DMI", checkSerial))
	results = append(results, check(3, "Battery & charging", checkBattery))
	results = append(results, check(5, "Device drivers", checkDrivers))
	results = append(results, check(6, "Disk & SMART health", checkDisk))
	results = append(results, check(7, "Display connected", checkDisplay))
	results = append(results, check(8, "Disk encryption", checkEncryption))
	results = append(results, check(10, "Network adapters", checkNetwork))
	results = append(results, check(12, "Power / sleep states", checkPower))
	results = append(results, check(13, "Audio devices", checkAudio))
	results = append(results, check(14, "Touchpad / input", checkTouchpad))
	results = append(results, check(17, "OS version & updates", checkOSUpdates))

	return results
}

// ── individual checks ──────────────────────────────────────────

func checkSerial() (types.Status, string, string) {
	serial := readSysFile("/sys/class/dmi/id/product_serial")
	bad := []string{"", "Default string", "To Be Filled By O.E.M.", "System Serial Number", "None"}
	for _, b := range bad {
		if strings.EqualFold(serial, b) {
			return types.StatusFail, "Serial is placeholder or missing", ""
		}
	}
	return types.StatusPass, serial, ""
}

func checkBattery() (types.Status, string, string) {
	dirs, _ := filepath.Glob("/sys/class/power_supply/BAT*")
	if len(dirs) == 0 {
		return types.StatusSkip, "No battery detected (desktop?)", ""
	}
	dir := dirs[0]
	status := strings.TrimSpace(readSysFile(dir + "/status"))
	capacity := strings.TrimSpace(readSysFile(dir + "/capacity"))
	energyFull := parseSysInt(dir + "/energy_full")
	energyDesign := parseSysInt(dir + "/energy_full_design")

	health := 0
	if energyDesign > 0 {
		health = energyFull * 100 / energyDesign
	}
	detail := fmt.Sprintf("%s%% capacity | Status: %s", capacity, status)
	if health > 0 {
		detail += fmt.Sprintf(" | Health: %d%%", health)
	}
	if health > 0 && health < 50 {
		return types.StatusWarn, detail, "Battery health below 50% — consider replacement"
	}
	return types.StatusPass, detail, ""
}

func checkDrivers() (types.Status, string, string) {
	out, err := exec.Command("lspci", "-v").Output()
	if err != nil {
		return types.StatusSkip, "lspci not available", ""
	}
	lines := strings.Split(string(out), "\n")
	missing := 0
	for _, l := range lines {
		if strings.Contains(l, "Kernel driver in use") == false &&
			strings.Contains(l, "Subsystem:") &&
			!strings.Contains(l, "Kernel modules:") {
			// not conclusive without more parsing — just count devices with no kernel driver
		}
		if strings.Contains(strings.ToLower(l), "unknown") {
			missing++
		}
	}
	if missing > 0 {
		return types.StatusWarn, fmt.Sprintf("%d device(s) may lack drivers", missing), ""
	}
	return types.StatusPass, "No driver errors detected", ""
}

func checkDisk() (types.Status, string, string) {
	// Try nvme first, then sda
	var model, health string

	nvmeDirs, _ := filepath.Glob("/sys/block/nvme*/device/model")
	sdDirs, _ := filepath.Glob("/sys/block/sd*/device/model")

	switch {
	case len(nvmeDirs) > 0:
		model = strings.TrimSpace(readSysFile(nvmeDirs[0]))
	case len(sdDirs) > 0:
		model = strings.TrimSpace(readSysFile(sdDirs[0]))
	default:
		model = "Unknown"
	}

	// Try smartctl for health
	out, err := exec.Command("smartctl", "-H", "/dev/nvme0").Output()
	if err != nil {
		out, err = exec.Command("smartctl", "-H", "/dev/sda").Output()
	}
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(line, "SMART overall-health") {
				if strings.Contains(line, "PASSED") {
					health = "SMART: PASSED"
				} else {
					return types.StatusFail, model+" — SMART: FAILED", "Drive may be failing"
				}
			}
		}
	}

	detail := model
	if health != "" {
		detail += " | " + health
	}
	return types.StatusPass, detail, ""
}

func checkDisplay() (types.Status, string, string) {
	dirs, _ := filepath.Glob("/sys/class/drm/card*/status")
	connected := 0
	for _, f := range dirs {
		if strings.TrimSpace(readSysFile(f)) == "connected" {
			connected++
		}
	}
	if connected == 0 {
		return types.StatusWarn, "No display detected via DRM", "May be normal on some systems"
	}
	return types.StatusPass, fmt.Sprintf("%d display(s) connected", connected), ""
}

func checkEncryption() (types.Status, string, string) {
	out, err := exec.Command("lsblk", "-o", "TYPE,NAME,MOUNTPOINT").Output()
	if err != nil {
		return types.StatusSkip, "lsblk not available", ""
	}
	if strings.Contains(string(out), "crypt") {
		return types.StatusPass, "LUKS encryption detected", ""
	}
	return types.StatusInfo, "No encryption detected", ""
}

func checkNetwork() (types.Status, string, string) {
	out, err := exec.Command("ip", "link", "show").Output()
	if err != nil {
		return types.StatusSkip, "ip command not available", ""
	}
	lines := strings.Split(string(out), "\n")
	up := 0
	for _, l := range lines {
		if strings.Contains(l, "state UP") {
			up++
		}
	}
	if up == 0 {
		return types.StatusWarn, "No network interfaces UP", ""
	}
	return types.StatusPass, fmt.Sprintf("%d interface(s) UP", up), ""
}

func checkPower() (types.Status, string, string) {
	states := strings.TrimSpace(readSysFile("/sys/power/state"))
	if states == "" {
		return types.StatusSkip, "Could not read power states", ""
	}
	return types.StatusPass, "Available: "+states, ""
}

func checkAudio() (types.Status, string, string) {
	cards := strings.TrimSpace(readSysFile("/proc/asound/cards"))
	if cards == "" || strings.Contains(cards, "no soundcards") {
		return types.StatusFail, "No audio devices detected", ""
	}
	lines := strings.Split(cards, "\n")
	count := 0
	for _, l := range lines {
		if strings.Contains(l, "[") {
			count++
		}
	}
	return types.StatusPass, fmt.Sprintf("%d audio device(s) found", count), ""
}

func checkTouchpad() (types.Status, string, string) {
	out, err := exec.Command("libinput", "list-devices").Output()
	if err != nil {
		// fallback: scan /dev/input
		inputs, _ := filepath.Glob("/dev/input/event*")
		if len(inputs) > 0 {
			return types.StatusPass, fmt.Sprintf("%d input device(s) found", len(inputs)), ""
		}
		return types.StatusSkip, "libinput not available", ""
	}
	if strings.Contains(strings.ToLower(string(out)), "touchpad") {
		return types.StatusPass, "Precision touchpad detected", ""
	}
	return types.StatusWarn, "No touchpad found in libinput", ""
}

func checkOSUpdates() (types.Status, string, string) {
	osRelease := readOSRelease()
	// Check for pending updates non-interactively (exit code 100 = updates available on apt)
	out, err := exec.Command("apt-get", "-s", "upgrade").Output()
	if err == nil {
		pending := 0
		for _, l := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(l, "Inst ") {
				pending++
			}
		}
		if pending > 0 {
			return types.StatusWarn, osRelease, fmt.Sprintf("%d pending package updates", pending)
		}
	}
	return types.StatusPass, osRelease, ""
}

// ── helpers ────────────────────────────────────────────────────

func readSysFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func parseSysInt(path string) int {
	s := readSysFile(path)
	n, _ := strconv.Atoi(s)
	return n
}

func readOSRelease() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "Linux"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var name, version string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			name = strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
		}
		if strings.HasPrefix(line, "VERSION_ID=") {
			version = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), `"`)
		}
	}
	if name != "" {
		return name
	}
	return "Linux " + version
}

func readCPUInfo() string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "model name") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func readRAMInfo() string {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				kb, _ := strconv.ParseInt(parts[1], 10, 64)
				return fmt.Sprintf("%.1f GB", float64(kb)/1024/1024)
			}
		}
	}
	return ""
}

// ── interactive hardware actions (Linux) ───────────────────────

func playTestTone() {
	cmd := exec.Command("speaker-test", "-t", "sine", "-f", "1000", "-l", "1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return
	}
	time.Sleep(1 * time.Second)
	cmd.Process.Kill()
	cmd.Wait()
}

func playMicTest() {
	// Record 3s then play back
	tmp := "/tmp/tunit_mic_test.wav"
	rec := exec.Command("arecord", "-d", "3", "-f", "cd", tmp)
	rec.Stdout = nil
	rec.Stderr = nil
	rec.Run()
	play := exec.Command("aplay", tmp)
	play.Stdout = nil
	play.Stderr = nil
	play.Run()
	os.Remove(tmp)
}

func openCameraPreview() {
	// Try ffplay for 3 seconds, suppress output
	cmd := exec.Command("ffplay", "-f", "v4l2", "-i", "/dev/video0", "-vf", "scale=640:480", "-t", "3", "-autoexit")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Run()
}

func runScreenTest() {
	// Use clear + printf to flash full color screens in the terminal
	colors := []struct {
		name string
		ansi string
	}{
		{"WHITE", "\033[47m"},
		{"BLACK", "\033[40m"},
		{"RED", "\033[41m"},
		{"GREEN", "\033[42m"},
		{"BLUE", "\033[44m"},
	}
	for _, c := range colors {
		fmt.Printf("\033[2J\033[H%s%s                                        DEAD PIXEL TEST — %s\n\n  Press ENTER to continue...\033[0m", c.ansi, strings.Repeat(" ", 0), c.name)
		fmt.Scanln()
	}
	fmt.Print("\033[2J\033[H")
}

func runKeyboardTest() {
	fmt.Println("  Type freely. Press Enter twice when done.")
	fmt.Println("  ─────────────────────────────────────────")
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
	ticker := time.NewTicker(500 * time.Millisecond)
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
			dirs, _ := filepath.Glob("/sys/bus/usb/devices/*/product")
			for _, d := range dirs {
				prod := strings.TrimSpace(readSysFile(d))
				if prod != "" && !seen[prod] {
					seen[prod] = true
					fmt.Printf("  ✓ USB detected: %s\n", prod)
				}
			}
		}
	}
}
