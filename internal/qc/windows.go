//go:build windows

package qc

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/cleanup"
	"github.com/tunit/tunit/internal/types"
)

// psQuery runs a PowerShell one-liner and returns stdout.
func psQuery(script string) (string, error) {
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive",
		"-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// psJSON runs a PowerShell snippet that emits JSON and unmarshals into v.
func psJSON(script string, v any) error {
	out, err := psQuery(script + " | ConvertTo-Json -Compress")
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(out), v)
}

// psInt runs a PowerShell snippet and parses its output as an integer,
// returning 0 when the query fails or the value is empty/non-numeric.
func psInt(script string) int {
	out, err := psQuery(script)
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		return 0
	}
	return n
}

func populateIdentity(r *types.QCResult) {
	type sysInfo struct {
		Serial  string `json:"Serial"`
		Model   string `json:"Model"`
		BIOSVer string `json:"BIOSVer"`
		CPU     string `json:"CPU"`
		RAMGB   string `json:"RAMGB"`
		OS      string `json:"OS"`
	}
	script := `[PSCustomObject]@{
		Serial  = (Get-WmiObject Win32_BIOS).SerialNumber
		Model   = (Get-WmiObject Win32_ComputerSystem).Model
		BIOSVer = (Get-WmiObject Win32_BIOS).SMBIOSBIOSVersion
		CPU     = (Get-WmiObject Win32_Processor).Name
		RAMGB   = [string]([math]::Round((Get-WmiObject Win32_ComputerSystem).TotalPhysicalMemory/1GB,1)) + " GB"
		OS      = (Get-WmiObject Win32_OperatingSystem).Caption + " " + (Get-WmiObject Win32_OperatingSystem).Version
	}`
	var info sysInfo
	if err := psJSON(script, &info); err == nil {
		r.Serial = info.Serial
		r.Model = info.Model
		r.BIOSVer = info.BIOSVer
		r.CPU = info.CPU
		r.RAM = info.RAMGB
		r.OS = info.OS
	}
}

func runAutomatedChecks(r *types.QCResult) []types.CheckResult {
	var results []types.CheckResult
	results = append(results, check(1, "Windows activation", checkActivation))
	results = append(results, check(2, "Serial / DMI", checkSerial))
	results = append(results, check(3, "Battery & charging", checkBattery))
	results = append(results, check(4, "Account / OOBE audit", checkAccounts))
	results = append(results, check(5, "Device drivers", checkDrivers))
	results = append(results, check(6, "Disk & SMART health", checkDisk))
	results = append(results, check(7, "Display connected", checkDisplay))
	results = append(results, check(8, "BitLocker encryption", checkBitLocker))
	results = append(results, check(10, "Network adapters", checkNetwork))
	results = append(results, check(12, "Sleep / power states", checkPower))
	results = append(results, check(13, "Audio devices", checkAudio))
	results = append(results, check(17, "OS version & updates", checkOSUpdates))
	return results
}

func checkActivation() (types.Status, string, string) {
	out, err := psQuery(`(Get-WmiObject SoftwareLicensingProduct -Filter "PartialProductKey IS NOT NULL AND LicenseFamily IS NOT NULL").LicenseStatus`)
	if err != nil {
		return types.StatusSkip, "Could not query activation", ""
	}
	// 1 = Licensed
	if strings.TrimSpace(out) == "1" {
		return types.StatusPass, "Digitally activated", ""
	}
	return types.StatusFail, "Not activated (status: "+out+")", "Windows is not activated"
}

func checkSerial() (types.Status, string, string) {
	serial, _ := psQuery(`(Get-WmiObject Win32_BIOS).SerialNumber`)
	bad := []string{"", "Default string", "To Be Filled By O.E.M.", "System Serial Number", "None", "0"}
	for _, b := range bad {
		if strings.EqualFold(strings.TrimSpace(serial), b) {
			return types.StatusFail, "Serial is placeholder or missing", ""
		}
	}
	return types.StatusPass, strings.TrimSpace(serial), ""
}

func checkBattery() (types.Status, string, string) {
	// Charge % and status come from Win32_Battery. True capacity (needed for a
	// health %) lives in the root\wmi battery classes — Win32_Battery itself
	// almost always reports those fields as null, which is why the old single
	// query never produced a health number.
	var bat struct {
		Status          int
		EstimatedCharge int
	}
	basic := `Get-CimInstance Win32_Battery | Select-Object -First 1 @{N='Status';E={[int]$_.BatteryStatus}},@{N='EstimatedCharge';E={[int]$_.EstimatedChargeRemaining}}`
	if err := psJSON(basic, &bat); err != nil {
		return types.StatusSkip, "No battery detected (desktop?)", ""
	}

	full := psInt(`(Get-CimInstance -Namespace root\wmi -ClassName BatteryFullChargedCapacity -ErrorAction SilentlyContinue | Select-Object -First 1).FullChargedCapacity`)
	design := psInt(`(Get-CimInstance -Namespace root\wmi -ClassName BatteryStaticData -ErrorAction SilentlyContinue | Select-Object -First 1).DesignedCapacity`)
	health := 0
	if design > 0 && full > 0 {
		health = full * 100 / design
	}

	statusStr := "Unknown"
	switch bat.Status {
	case 1:
		statusStr = "Discharging"
	case 2:
		statusStr = "AC (not charging)"
	case 3:
		statusStr = "Fully charged"
	case 6, 7, 8, 9:
		statusStr = "Charging"
	}

	detail := fmt.Sprintf("%d%% charge | %s", bat.EstimatedCharge, statusStr)
	if health > 0 {
		detail += fmt.Sprintf(" | Health: %d%%", health)
	}

	if health > 0 && health < 50 {
		return types.StatusWarn, detail, "Battery health below 50%"
	}
	return types.StatusPass, detail, ""
}

func checkAccounts() (types.Status, string, string) {
	out, _ := psQuery(`Get-WmiObject Win32_UserAccount -Filter "LocalAccount=True" | Where-Object { $_.Name -notin @('Administrator','Guest','DefaultAccount','WDAGUtilityAccount') } | Select-Object -ExpandProperty Name`)
	if strings.TrimSpace(out) == "" {
		return types.StatusPass, "No personal user accounts found", ""
	}
	accounts := strings.Join(strings.Fields(out), ", ")
	return types.StatusWarn, "Local accounts: "+accounts, "Verify no personal data remains"
}

func checkDrivers() (types.Status, string, string) {
	type pnp struct {
		Name              string
		ConfigManagerErrorCode int
	}
	var devices []pnp
	script := `Get-WmiObject Win32_PnPEntity | Where-Object { $_.ConfigManagerErrorCode -ne 0 } | Select-Object Name,ConfigManagerErrorCode`
	if err := psJSON(script, &devices); err != nil || len(devices) == 0 {
		return types.StatusPass, "No driver errors detected", ""
	}
	names := make([]string, 0, len(devices))
	for _, d := range devices {
		names = append(names, fmt.Sprintf("%s (Code %d)", d.Name, d.ConfigManagerErrorCode))
	}
	return types.StatusFail, fmt.Sprintf("%d driver error(s)", len(devices)), strings.Join(names, "; ")
}

func checkDisk() (types.Status, string, string) {
	type diskInfo struct {
		FriendlyName    string
		HealthStatus    string
		OperationalStatus string
		Size            int64
	}
	var disks []diskInfo
	script := `Get-PhysicalDisk | Select-Object FriendlyName,HealthStatus,OperationalStatus,Size`
	if err := psJSON(script, &disks); err != nil || len(disks) == 0 {
		return types.StatusSkip, "Could not query disks", ""
	}
	for _, d := range disks {
		if !strings.EqualFold(d.HealthStatus, "Healthy") {
			return types.StatusFail, d.FriendlyName+" — "+d.HealthStatus, "Drive health issue detected"
		}
	}
	names := make([]string, 0, len(disks))
	for _, d := range disks {
		gb := d.Size / 1024 / 1024 / 1024
		names = append(names, fmt.Sprintf("%s (%dGB, %s)", d.FriendlyName, gb, d.HealthStatus))
	}
	return types.StatusPass, strings.Join(names, " | "), ""
}

func checkDisplay() (types.Status, string, string) {
	out, err := psQuery(`Get-WmiObject WmiMonitorID | Measure-Object | Select-Object -ExpandProperty Count`)
	if err != nil {
		return types.StatusSkip, "Could not query displays", ""
	}
	if strings.TrimSpace(out) == "0" {
		return types.StatusWarn, "No external displays detected", ""
	}
	return types.StatusPass, strings.TrimSpace(out)+" display(s) detected", ""
}

func checkBitLocker() (types.Status, string, string) {
	out, err := psQuery(`(Get-BitLockerVolume -MountPoint C:).VolumeStatus`)
	if err != nil {
		return types.StatusSkip, "BitLocker query failed (may need admin)", ""
	}
	vol := strings.TrimSpace(out)
	switch vol {
	case "FullyEncrypted":
		return types.StatusWarn, "Drive fully encrypted", "Verify BitLocker key is accessible"
	case "FullyDecrypted":
		return types.StatusPass, "Not encrypted", ""
	default:
		return types.StatusInfo, "Status: "+vol, ""
	}
}

func checkNetwork() (types.Status, string, string) {
	out, err := psQuery(`Get-NetAdapter | Where-Object Status -eq 'Up' | Select-Object -ExpandProperty Name`)
	if err != nil {
		return types.StatusSkip, "Could not query adapters", ""
	}
	names := strings.TrimSpace(out)
	if names == "" {
		return types.StatusWarn, "No network adapters UP", ""
	}
	return types.StatusPass, strings.Join(strings.Fields(names), ", "), ""
}

func checkPower() (types.Status, string, string) {
	out, err := psQuery(`powercfg /a`)
	if err != nil {
		return types.StatusSkip, "powercfg not available", ""
	}
	if strings.Contains(out, "Standby") || strings.Contains(out, "Sleep") {
		return types.StatusPass, "Sleep states available", ""
	}
	return types.StatusWarn, "No sleep states detected", ""
}

func checkAudio() (types.Status, string, string) {
	out, err := psQuery(`Get-WmiObject Win32_SoundDevice | Select-Object -ExpandProperty Name`)
	if err != nil || strings.TrimSpace(out) == "" {
		return types.StatusFail, "No audio devices found", ""
	}
	devices := strings.Fields(strings.TrimSpace(out))
	return types.StatusPass, fmt.Sprintf("%d audio device(s)", len(devices)), ""
}

func checkOSUpdates() (types.Status, string, string) {
	osName, _ := psQuery(`(Get-WmiObject Win32_OperatingSystem).Caption + " Build " + (Get-WmiObject Win32_OperatingSystem).BuildNumber`)
	pendingReboot, _ := psQuery(`Test-Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending"`)
	if strings.TrimSpace(pendingReboot) == "True" {
		return types.StatusWarn, strings.TrimSpace(osName), "Pending reboot detected (Windows Update)"
	}
	return types.StatusPass, strings.TrimSpace(osName), ""
}

// ── interactive hardware actions (Windows) ─────────────────────

func probeCapabilities() capabilities {
	var c capabilities
	// System.Media.SoundPlayer / console beep are always available via .NET.
	c.audioPlay = capability{ok: true, tool: "SoundPlayer"}
	if t, ok := lookTools("ffmpeg"); ok {
		c.audioRecord = capability{ok: true, tool: t}
	} else {
		c.audioRecord = capability{hint: "install ffmpeg for microphone capture"}
	}
	// ffmpeg gives the terminal preview; the Windows Camera app is the GUI
	// fallback, so the webcam check can run even without ffmpeg.
	if t, ok := lookTools("ffmpeg"); ok {
		c.camera = capability{ok: true, tool: t}
	} else {
		c.camera = capability{ok: true, tool: "Camera app (GUI fallback)"}
	}
	return c
}

func playTestTone() {
	tmp := cleanup.TempPathFor("stereo_test.wav")
	if err := generateTestWAV(tmp); err == nil {
		psQuery(fmt.Sprintf(`(New-Object System.Media.SoundPlayer '%s').PlaySync()`, tmp))
		os.Remove(tmp)
		return
	}
	psQuery(`[console]::beep(1000,500); [console]::beep(1500,500)`)
}

func playMicTest() {
	tmp := cleanup.TempPathFor("mic_test.wav")
	// Use ffmpeg for recording — most reliable cross-platform
	rec := exec.Command("ffmpeg", "-y", "-f", "dshow", "-i", "audio=Microphone", "-t", "3", "-ar", "44100", "-ac", "1", tmp)
	rec.Stdout = nil
	rec.Stderr = nil
	if err := rec.Run(); err != nil {
		// Fallback: use PowerShell with Windows built-in recorder
		psQuery(fmt.Sprintf(`
			Add-Type -AssemblyName System.Speech
			$r = New-Object System.Speech.Recognition.SpeechRecognitionEngine
			$r.SetInputToDefaultAudioDevice()
			Start-Sleep 3
		`))
		fmt.Println("  (Mic recording limited — verify manually)")
		return
	}
	psQuery(fmt.Sprintf(`(New-Object System.Media.SoundPlayer '%s').PlaySync()`, tmp))
	os.Remove(tmp)
}

func openCameraPreview() {
	err := terminalCameraPreview(
		[]string{"-f", "dshow", "-video_size", "320x240", "-i", "video=Integrated Webcam"},
		5*time.Second,
	)
	if err != nil {
		exec.Command("explorer.exe", "microsoft.windows.camera:").Run()
		time.Sleep(5 * time.Second)
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
	out, _ := psQuery(`Get-WmiObject Win32_PnPEntity | Where-Object { $_.PNPClass -eq 'USB' } | Select-Object -ExpandProperty Name`)
	for _, d := range strings.Split(out, "\n") {
		d = strings.TrimSpace(d)
		if d != "" {
			seen[d] = true
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
			out, err := psQuery(`Get-WmiObject Win32_PnPEntity | Where-Object { $_.PNPClass -eq 'USB' } | Select-Object -ExpandProperty Name`)
			if err == nil {
				for _, d := range strings.Split(out, "\n") {
					d = strings.TrimSpace(d)
					if d != "" && !seen[d] {
						seen[d] = true
						devices = append(devices, usbDevice{name: d, when: time.Now()})
					}
				}
			}
			renderUSBDisplay(devices, time.Since(start), timeout)
		}
	}
}
