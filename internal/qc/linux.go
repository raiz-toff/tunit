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

	"github.com/tunit/tunit/internal/cleanup"
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

// checkDrivers parses `lspci -k` device blocks and flags the real problem
// case: a device that has a kernel module available but no driver currently
// bound ("Kernel modules:" present, "Kernel driver in use:" absent). Devices
// with no module at all (host bridges, etc.) are expected and ignored, which
// keeps false positives down.
func checkDrivers() (types.Status, string, string) {
	out, err := exec.Command("lspci", "-k").Output()
	if err != nil {
		return types.StatusSkip, "lspci not available", ""
	}

	var unbound []string
	var name string
	var hasDriver, hasModules bool

	flush := func() {
		if name != "" && hasModules && !hasDriver {
			unbound = append(unbound, name)
		}
	}

	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			// New device block begins; finalize the previous one.
			flush()
			name = line
			hasDriver, hasModules = false, false
			continue
		}
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "Kernel driver in use:"):
			hasDriver = true
		case strings.HasPrefix(trimmed, "Kernel modules:"):
			hasModules = true
		}
	}
	flush()

	if len(unbound) == 0 {
		return types.StatusPass, "All devices have drivers bound", ""
	}
	// Shorten device descriptions for the warning detail.
	short := make([]string, 0, len(unbound))
	for _, d := range unbound {
		if i := strings.Index(d, ": "); i >= 0 && i+2 < len(d) {
			d = d[i+2:]
		}
		short = append(short, d)
	}
	return types.StatusWarn,
		fmt.Sprintf("%d device(s) with no driver loaded", len(unbound)),
		strings.Join(short, "; ")
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

func probeCapabilities() capabilities {
	var c capabilities
	if t, ok := lookTools("aplay", "paplay", "speaker-test"); ok {
		c.audioPlay = capability{ok: true, tool: t}
	} else {
		c.audioPlay = capability{hint: "install alsa-utils (provides aplay)"}
	}
	if t, ok := lookTools("arecord", "ffmpeg"); ok {
		c.audioRecord = capability{ok: true, tool: t}
	} else {
		c.audioRecord = capability{hint: "install alsa-utils (provides arecord) or ffmpeg"}
	}
	if t, ok := lookTools("ffmpeg", "ffplay"); ok {
		c.camera = capability{ok: true, tool: t}
	} else {
		c.camera = capability{hint: "install ffmpeg (for terminal or window preview)"}
	}
	return c
}

func playTestTone() {
	tmp := cleanup.TempPathFor("stereo_test.wav")
	if err := generateTestWAV(tmp); err == nil {
		exec.Command("aplay", tmp).Run()
		os.Remove(tmp)
		return
	}
	// Fallback: speaker-test channel sweep
	cmd := exec.Command("speaker-test", "-t", "wav", "-c", "2", "-l", "1")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return
	}
	time.Sleep(5 * time.Second)
	cmd.Process.Kill()
	cmd.Wait()
}

func playMicTest() {
	tmp := cleanup.TempPathFor("mic_test.wav")
	rec := exec.Command("arecord", "-d", "3", "-f", "cd", "-q", tmp)
	rec.Run()
	exec.Command("aplay", "-q", tmp).Run()
	os.Remove(tmp)
}

func openCameraPreview() {
	err := terminalCameraPreview(
		[]string{"-f", "v4l2", "-video_size", "320x240", "-i", "/dev/video0"},
		5*time.Second,
	)
	if err != nil {
		// Fallback to ffplay window
		cmd := exec.Command("ffplay", "-f", "v4l2", "-i", "/dev/video0",
			"-vf", "scale=640:480", "-t", "5", "-autoexit")
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Run()
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

	fmt.Print("\033[?25l") // hide cursor
	defer fmt.Print("\033[?25h\033[0m")

	timer := time.NewTimer(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	defer timer.Stop()

	seen := map[string]bool{}
	var devices []usbDevice

	// Snapshot current devices so we only show NEW ones
	dirs, _ := filepath.Glob("/sys/bus/usb/devices/*/product")
	for _, d := range dirs {
		prod := strings.TrimSpace(readSysFile(d))
		if prod != "" {
			seen[prod] = true
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
			dirs, _ := filepath.Glob("/sys/bus/usb/devices/*/product")
			for _, d := range dirs {
				prod := strings.TrimSpace(readSysFile(d))
				if prod != "" && !seen[prod] {
					seen[prod] = true
					devices = append(devices, usbDevice{name: prod, when: time.Now()})
				}
			}
			renderUSBDisplay(devices, time.Since(start), timeout)
		}
	}
}
