package qc

import (
	"fmt"
	"time"

	"github.com/tunit/tunit/internal/menu"
	"github.com/tunit/tunit/internal/report"
	"github.com/tunit/tunit/internal/types"
)

// Run executes all QC checks and returns the result.
// autoOnly skips interactive human-confirmation checks.
func Run(repair menu.RepairType, autoOnly bool) *types.QCResult {
	r := &types.QCResult{
		Timestamp: time.Now(),
	}

	fmt.Println("\n  Running automated checks...\n")

	// Gather system identity first (serial, model, OS, CPU, RAM, BIOS)
	populateIdentity(r)

	checks := runAutomatedChecks(r)
	r.Checks = append(r.Checks, checks...)

	if !autoOnly {
		human := runInteractiveChecks(repair)
		r.Checks = append(r.Checks, human...)
	}

	r.Verdict = computeVerdict(r.Checks)
	return r
}

func computeVerdict(checks []types.CheckResult) types.Status {
	verdict := types.StatusPass
	for _, c := range checks {
		if c.Status == types.StatusFail {
			return types.StatusFail
		}
		if c.Status == types.StatusWarn && verdict == types.StatusPass {
			verdict = types.StatusWarn
		}
	}
	return verdict
}

func check(step int, name string, fn func() (types.Status, string, string)) types.CheckResult {
	fmt.Printf("  Checking %-32s", name+"...")
	status, detail, warning := fn()
	col := ""
	switch status {
	case types.StatusPass:
		col = "\033[32m"
	case types.StatusWarn:
		col = "\033[33m"
	case types.StatusFail:
		col = "\033[31m"
	default:
		col = "\033[2m"
	}
	fmt.Printf("%s[%s]\033[0m\n", col, status.String())
	return types.CheckResult{
		Step:    step,
		Name:    name,
		Status:  status,
		Detail:  detail,
		Warning: warning,
	}
}

func runInteractiveChecks(repair menu.RepairType) []types.CheckResult {
	var results []types.CheckResult

	fmt.Println("\n\033[1m  Human verification checks starting...\033[0m")
	fmt.Println("  Answer Y (yes), N (no), or S (skip) for each.\n")

	total := 6
	step := 1

	// Audio
	report.PrintStepHeader(step, total, "Audio Check")
	fmt.Println("\n  ♪  Playing test tone through speakers...")
	playTestTone()
	status := report.AskYNS(
		"Did you hear audio from BOTH speakers clearly?",
		"Both speakers worked fine",
		"Silent or crackling detected",
	)
	results = append(results, types.CheckResult{Step: 13, Name: "Audio (speakers)", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// Microphone
	report.PrintStepHeader(step, total, "Microphone Check")
	fmt.Println("\n  ⚡  Recording 3 seconds... speak or make noise now...")
	playMicTest()
	status = report.AskYNS(
		"Did you hear your voice clearly in the playback?",
		"Microphone captured audio clearly",
		"No audio or very distorted",
	)
	results = append(results, types.CheckResult{Step: 13, Name: "Microphone", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// Webcam
	report.PrintStepHeader(step, total, "Webcam Check")
	fmt.Println("\n  ⚡  Opening camera preview for 3 seconds...")
	openCameraPreview()
	status = report.AskYNS(
		"Did you see a live image from the camera? (No black screen?)",
		"Camera showed live image",
		"Black screen or no camera found",
	)
	results = append(results, types.CheckResult{Step: 16, Name: "Webcam", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// Screen dead pixel
	report.PrintStepHeader(step, total, "Display / Dead Pixel Check")
	fmt.Println("\n  ⚡  Displaying test screens: white → black → red → green → blue")
	fmt.Println("  Press ENTER to advance through each screen.")
	runScreenTest()
	status = report.AskYNS(
		"Was the screen solid with no dead pixels, stuck pixels, or bleed?",
		"Screen looks clean",
		"Dead pixel, stuck pixel, or bleed detected",
	)
	results = append(results, types.CheckResult{Step: 7, Name: "Display dead pixel", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// Keyboard
	report.PrintStepHeader(step, total, "Keyboard Check")
	fmt.Println("\n  Type each letter, number, and symbol shown.")
	fmt.Println("  Press Escape or Enter when finished.\n")
	runKeyboardTest()
	status = report.AskYNS(
		"Did all keys register correctly with no stuck or missing keys?",
		"All keys functional",
		"One or more keys missing or stuck",
	)
	results = append(results, types.CheckResult{Step: 9, Name: "Keyboard", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// USB ports
	report.PrintStepHeader(step, total, "USB Port Check")
	fmt.Println("\n  Plug a USB device into each port one at a time.")
	fmt.Println("  The tool will detect insertions automatically.\n")
	runUSBWatcher()
	status = report.AskYNS(
		"Did all USB ports detect the device when inserted?",
		"All USB ports detected device",
		"One or more USB ports did not respond",
	)
	results = append(results, types.CheckResult{Step: 15, Name: "USB ports", Status: status, Detail: humanDetail(status, "Human verified")})

	// Repair-specific checks
	repairChecks := repairDisturbedChecks(repair)
	results = append(results, repairChecks...)

	return results
}

func repairDisturbedChecks(repair menu.RepairType) []types.CheckResult {
	if repair == menu.RepairNone {
		return nil
	}

	fmt.Printf("\n\033[1m  Disturbed component checks for: %s repair\033[0m\n\n", repair.String())

	var results []types.CheckResult
	total := 1

	switch repair {
	case menu.RepairScreen:
		report.PrintStepHeader(1, total, "Wi-Fi Signal Check (antennas may be disturbed)")
		status := report.AskYNS(
			"Is Wi-Fi connecting and showing good signal strength?",
			"Wi-Fi signal OK",
			"Weak or no Wi-Fi (antenna may be disconnected)",
		)
		results = append(results, types.CheckResult{Name: "Wi-Fi antennas (post-LCD)", Status: status, Detail: humanDetail(status, "Post-screen repair check")})

	case menu.RepairMotherboard:
		report.PrintStepHeader(1, total, "Windows Activation (may need re-sync after board swap)")
		status := report.AskYNS(
			"Is Windows showing as Activated in Settings → System → Activation?",
			"Activated",
			"Not activated or showing error",
		)
		results = append(results, types.CheckResult{Name: "Win activation (post-BLR)", Status: status, Detail: humanDetail(status, "Post-motherboard check")})

	case menu.RepairBattery:
		report.PrintStepHeader(1, total, "Charging Circuit (reconnect battery)")
		status := report.AskYNS(
			"Is the battery showing as Charging when AC is connected?",
			"Charging correctly",
			"Not charging or showing error",
		)
		results = append(results, types.CheckResult{Name: "Charging circuit (post-battery)", Status: status, Detail: humanDetail(status, "Post-battery check")})
	}

	return results
}

func humanDetail(s types.Status, base string) string {
	switch s {
	case types.StatusPass:
		return base + " — PASS"
	case types.StatusFail:
		return base + " — FAIL (tester reported)"
	default:
		return "Skipped"
	}
}
