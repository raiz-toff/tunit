package stress

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/tunit/tunit/internal/cleanup"
	"github.com/tunit/tunit/internal/types"
)

const (
	furmarkURL = "https://geeks3d.com/dl/get/695" // FurMark portable
	gpuTestURL = "https://www.geeks3d.com/dl/get/557"
)

func RunTier2(cfg Config) *types.StressResult {
	res := &types.StressResult{Tier: 2, Duration: cfg.Duration}

	// Prefer the session temp dir so an interrupted download is still cleaned
	// up; fall back to a tracked OS-temp dir when running without a session.
	var tmpDir string
	if s := cleanup.Current(); s != nil && s.Dir() != "" {
		tmpDir = filepath.Join(s.Dir(), "stress")
	} else {
		tmpDir = filepath.Join(os.TempDir(), "tunit_stress")
		defer os.RemoveAll(tmpDir)
	}
	if err := os.MkdirAll(tmpDir, 0o700); err != nil {
		res.Detail = "Could not create temp dir: " + err.Error()
		return res
	}

	fmt.Println("  Downloading GPU stress tool...")

	var toolPath string
	var err error

	switch runtime.GOOS {
	case "windows":
		toolPath = filepath.Join(tmpDir, "furmark.exe")
		err = downloadFile(toolPath, furmarkURL)
	case "linux":
		toolPath = filepath.Join(tmpDir, "furmark")
		err = downloadFile(toolPath, furmarkURL)
		if err == nil {
			os.Chmod(toolPath, 0o755)
		}
	default:
		res.Detail = "Tier 2 GPU stress not supported on this platform"
		return res
	}

	if err != nil {
		fmt.Printf("  Download failed (%v) — falling back to Tier 1\n", err)
		return RunTier1(cfg)
	}

	res.IdleTempC = ReadCPUTemp()
	res.TempAvailable = res.IdleTempC > 0
	if res.TempAvailable {
		fmt.Printf("  Idle GPU temp (CPU proxy): %.1f°C\n", res.IdleTempC)
	} else {
		res.SensorHint = tempHint()
		fmt.Printf("  \033[33m—\033[0m Temperature sensor unavailable; GPU load will run without thermal data.\n")
	}
	fmt.Printf("  Running GPU stress for %s...\n", cfg.Duration.Round(time.Second))

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command(toolPath,
			"/nogui",
			fmt.Sprintf("/width=1280"),
			"/height=720",
			fmt.Sprintf("/duration=%d", int(cfg.Duration.Milliseconds())),
			"/log_temp",
		)
	case "linux":
		cmd = exec.Command(toolPath,
			"--test=fur",
			"--width=1280",
			"--height=720",
			fmt.Sprintf("--duration=%d", int(cfg.Duration.Seconds())),
		)
	}

	cmd.Dir = tmpDir
	if err := cmd.Start(); err != nil {
		fmt.Printf("  Failed to start GPU stress tool: %v — falling back to Tier 1\n", err)
		return RunTier1(cfg)
	}

	// Poll thermals while GPU burns
	var peakTemp float64
	tick := time.NewTicker(500 * time.Millisecond)
	deadline := time.Now().Add(cfg.Duration)
	for time.Now().Before(deadline) {
		<-tick.C
		t := ReadCPUTemp()
		if t > peakTemp {
			peakTemp = t
		}
		fmt.Printf("\r  Temp: %.1f°C  Peak: %.1f°C", t, peakTemp)
	}
	tick.Stop()

	cmd.Process.Kill()
	cmd.Wait()

	res.PeakTempC = peakTemp
	fmt.Println("\n  Stress complete. Measuring cooldown (15s)...")
	time.Sleep(15 * time.Second)
	coolTemp := ReadCPUTemp()
	res.CooldownDC = peakTemp - coolTemp
	fmt.Printf("  Cooled to %.1f°C (dropped %.1f°C)\n", coolTemp, res.CooldownDC)

	if cfg.FailTempC > 0 && peakTemp >= cfg.FailTempC {
		res.Detail = fmt.Sprintf("CRITICAL: peak %.1f°C exceeded %.1f°C", peakTemp, cfg.FailTempC)
	}
	return res
}

func downloadFile(dest, url string) error {
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
