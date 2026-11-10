package qc

import (
	"fmt"
	"os"
	"strings"
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

	fmt.Print("\n  Running automated checks...\n\n")

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

// readChoice reads a single R/Y/N/S answer. On EOF or read error (piped or no
// stdin) it returns "S" so an unattended run skips the human check rather than
// recording a false PASS with nobody at the keyboard.
func readChoice() string {
	var input string
	if _, err := fmt.Fscan(os.Stdin, &input); err != nil {
		return "S"
	}
	return strings.ToUpper(strings.TrimSpace(input))
}

func runInteractiveChecks(repair menu.RepairType) []types.CheckResult {
	var results []types.CheckResult

	fmt.Println("\n\033[1m  Human verification checks starting...\033[0m")
	fmt.Print("  Answer Y (yes), N (no), or S (skip) for each.\n\n")

	// Probe what this machine can actually do before prompting the tester, so
	// a missing tool becomes a clear SKIP instead of a silent no-op.
	caps := probeCapabilities()
	printPreflight(caps)

	total := 6
	step := 1

	// Audio — stereo channel test with replay
	report.PrintStepHeader(step, total, "Audio — Stereo Speaker Test")
	var status types.Status
	if !caps.audioPlay.ok {
		fmt.Printf("\n  \033[33m—\033[0m Speaker test unavailable: %s\n", caps.audioPlay.hint)
		results = append(results, skipResult(13, "Audio (speakers)", caps.audioPlay.hint))
	} else {
		fmt.Println("\n  ♪  Stereo channel test: LEFT → RIGHT → BOTH")
		fmt.Print("     Listen for sound from each speaker separately.\n\n")
		for {
			playTestTone()
			fmt.Print("\n  [R] Replay  [Y] Both speakers OK  [N] Problem  [S] Skip: ")
			switch readChoice() {
			case "R":
				fmt.Print("\n  ♪  Replaying stereo test...\n\n")
				continue
			case "N":
				status = types.StatusFail
			case "S":
				status = types.StatusSkip
			default:
				status = types.StatusPass
			}
			break
		}
		results = append(results, types.CheckResult{Step: 13, Name: "Audio (speakers)", Status: status, Detail: humanDetail(status, "Human verified")})
	}
	step++

	// Microphone — record and playback with visual feedback
	report.PrintStepHeader(step, total, "Microphone Check")
	if !caps.audioRecord.ok {
		fmt.Printf("\n  \033[33m—\033[0m Microphone test unavailable: %s\n", caps.audioRecord.hint)
		results = append(results, skipResult(13, "Microphone", caps.audioRecord.hint))
	} else {
		fmt.Println()
		for {
			fmt.Println("  🎤  Recording 3 seconds — speak or clap now!")
			fmt.Println()
			// Animated countdown
			for i := 3; i > 0; i-- {
				bar := strings.Repeat("█", (4-i)*8) + strings.Repeat("░", i*8)
				fmt.Printf("  ⏺  Recording... %d  [%s]\r", i, bar)
				time.Sleep(1 * time.Second)
			}
			fmt.Printf("  ✓  Recording complete.%s\n\n", strings.Repeat(" ", 30))

			fmt.Print("  🔊  Playing back your recording...\n\n")
			playMicTest()

			fmt.Print("\n  [R] Record again  [Y] Heard clearly  [N] Problem  [S] Skip: ")
			switch readChoice() {
			case "R":
				fmt.Println()
				continue
			case "N":
				status = types.StatusFail
			case "S":
				status = types.StatusSkip
			default:
				status = types.StatusPass
			}
			break
		}
		results = append(results, types.CheckResult{Step: 13, Name: "Microphone", Status: status, Detail: humanDetail(status, "Human verified")})
	}
	step++

	// Webcam — terminal camera preview with replay
	report.PrintStepHeader(step, total, "Webcam Check")
	if !caps.camera.ok {
		fmt.Printf("\n  \033[33m—\033[0m Webcam test unavailable: %s\n", caps.camera.hint)
		results = append(results, skipResult(16, "Webcam", caps.camera.hint))
	} else {
		for {
			fmt.Print("\n  📷  Showing live camera feed in terminal (5 seconds)...\n\n")
			openCameraPreview()
			fmt.Print("\n  [R] Show again  [Y] Camera works  [N] Problem  [S] Skip: ")
			switch readChoice() {
			case "R":
				continue
			case "N":
				status = types.StatusFail
			case "S":
				status = types.StatusSkip
			default:
				status = types.StatusPass
			}
			break
		}
		results = append(results, types.CheckResult{Step: 16, Name: "Webcam", Status: status, Detail: humanDetail(status, "Human verified")})
	}
	step++

	// Screen dead pixel — fullscreen truecolor test
	report.PrintStepHeader(step, total, "Display — Dead Pixel Check")
	fmt.Print("\n  Maximize your terminal window for best results.\n")
	fmt.Print("  5 solid colors will fill the screen. Look for dead or stuck pixels.\n")
	fmt.Print("  Press ENTER to start, then ENTER to advance each color.\n\n")
	fmt.Print("  Ready? Press ENTER...")
	fmt.Scanln()
	runScreenTest()
	status = report.AskYNS(
		"Was the screen clean — no dead pixels, stuck pixels, or bleed?",
		"Screen looks clean",
		"Dead pixel, stuck pixel, or bleed detected",
	)
	results = append(results, types.CheckResult{Step: 7, Name: "Display dead pixel", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// Keyboard — visual keyboard test
	report.PrintStepHeader(step, total, "Keyboard Test")
	fmt.Print("\n  A visual keyboard will appear. Press every key to test it.\n")
	fmt.Print("  Keys turn \033[32mgreen\033[0m as you press them.\n")
	fmt.Print("  Press ESC three times to finish.\n\n")
	fmt.Print("  Ready? Press ENTER...")
	fmt.Scanln()
	tested := runKeyboardTest()
	_ = tested
	status = report.AskYNS(
		"Did all keys register correctly?",
		"All keys functional",
		"One or more keys missing or stuck",
	)
	results = append(results, types.CheckResult{Step: 9, Name: "Keyboard", Status: status, Detail: humanDetail(status, "Human verified")})
	step++

	// USB ports — live detection display
	report.PrintStepHeader(step, total, "USB Port Test")
	fmt.Print("\n  Plug a USB device into each port, one at a time.\n")
	fmt.Print("  New devices are detected and listed automatically.\n")
	fmt.Print("  Press ENTER to start...\n")
	fmt.Scanln()
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
