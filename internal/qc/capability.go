package qc

import (
	"fmt"
	"os/exec"

	"github.com/tunit/tunit/internal/types"
)

// capability describes whether an interactive hardware check can actually run
// on this machine, and if not, what the technician would install to enable it.
type capability struct {
	ok   bool
	tool string // the external tool that will be used (when ok)
	hint string // what to install when not ok
}

// capabilities is the result of probing the machine before interactive checks.
// Each platform file implements probeCapabilities().
type capabilities struct {
	audioPlay   capability // playing the stereo test tone
	audioRecord capability // recording the microphone sample
	camera      capability // webcam preview
}

// lookTools returns the first of names found on PATH, and whether any matched.
func lookTools(names ...string) (string, bool) {
	for _, n := range names {
		if _, err := exec.LookPath(n); err == nil {
			return n, true
		}
	}
	return "", false
}

// printPreflight shows a short summary so the tester knows up front which
// interactive checks will run and which will be skipped for lack of a tool.
func printPreflight(c capabilities) {
	fmt.Print("\n\033[1m  Preflight — interactive check availability\033[0m\n\n")
	line := func(label string, cap capability) {
		if cap.ok {
			fmt.Printf("    \033[32m✓\033[0m %-22s (%s)\n", label, cap.tool)
		} else {
			fmt.Printf("    \033[33m—\033[0m %-22s will skip: %s\n", label, cap.hint)
		}
	}
	line("Speaker test", c.audioPlay)
	line("Microphone test", c.audioRecord)
	line("Webcam preview", c.camera)
	fmt.Println()
}

// skipResult builds a SKIP check result with a clear reason, used when a
// capability is missing so the report shows WHY a check did not run.
func skipResult(step int, name, hint string) types.CheckResult {
	return types.CheckResult{
		Step:   step,
		Name:   name,
		Status: types.StatusSkip,
		Detail: "Skipped — " + hint,
	}
}
