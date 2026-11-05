//go:build windows

package qc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	type batInfo struct {
		Status        int
		EstimatedCharge int
		FullChargeCapacity int
		DesignCapacity int
	}
	var bat batInfo
	script := `Get-WmiObject Win32_Battery | Select-Object BatteryStatus,EstimatedChargeRemaining,FullChargeCapacity,DesignCapacity | Select-Object -First 1 @{N='Status';E={$_.BatteryStatus}},@{N='EstimatedCharge';E={$_.EstimatedChargeRemaining}},@{N='FullChargeCapacity';E={$_.FullChargeCapacity}},@{N='DesignCapacity';E={$_.DesignCapacity}}`
	if err := psJSON(script, &bat); err != nil {
		return types.StatusSkip, "No battery detected (desktop?)", ""
	}

	health := 0
	if bat.DesignCapacity > 0 {
		health = bat.FullChargeCapacity * 100 / bat.DesignCapacity
	}
	statusStr := "Unknown"
	charging := false
	switch bat.Status {
	case 1:
		statusStr = "Discharging"
	case 2:
		statusStr = "AC (not charging)"
	case 3:
		statusStr = "Fully charged"
		charging = true
	case 6, 7, 8, 9:
		statusStr = "Charging"
		charging = true
	}

	detail := fmt.Sprintf("%d%% charge | Health: %d%% | %s", bat.EstimatedCharge, health, statusStr)
	_ = charging

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

func playTestTone() {
	psQuery(`[console]::beep(1000,500); [console]::beep(1500,500)`)
}

func playMicTest() {
	// Record via PowerShell / Windows Media API — simplified prompt
	fmt.Println("  (Microphone recording requires the full Windows build)")
	time.Sleep(3 * time.Second)
}

func openCameraPreview() {
	exec.Command("explorer.exe", "microsoft.windows.camera:").Run()
	time.Sleep(3 * time.Second)
}

func runScreenTest() {
	colors := []string{"WHITE", "BLACK", "RED", "GREEN", "BLUE"}
	for _, c := range colors {
		script := fmt.Sprintf(`Add-Type -AssemblyName System.Windows.Forms; $f=New-Object System.Windows.Forms.Form; $f.WindowState='Maximized'; $f.BackColor='%s'; $f.Show(); Start-Sleep 2; $f.Close()`, c)
		exec.Command("powershell.exe", "-NoProfile", "-Command", script).Run()
	}
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
			out, err := psQuery(`Get-WmiObject Win32_PnPEntity | Where-Object { $_.PNPClass -eq 'USB' } | Select-Object -ExpandProperty Name`)
			if err == nil {
				for _, d := range strings.Split(out, "\n") {
					d = strings.TrimSpace(d)
					if d != "" && !seen[d] {
						seen[d] = true
						fmt.Printf("  ✓ USB detected: %s\n", d)
					}
				}
			}
		}
	}
}
