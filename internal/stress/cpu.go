package stress

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"

	"github.com/tunit/tunit/internal/types"
)

type Config struct {
	Duration  time.Duration
	WarnTempC float64
	FailTempC float64
}

func RunTier1(cfg Config) *types.StressResult {
	res := &types.StressResult{Tier: 1, Duration: cfg.Duration}

	// Measure idle temp before we start
	res.IdleTempC = ReadCPUTemp()
	fmt.Printf("  Idle temperature: %.1f°C\n", res.IdleTempC)
	fmt.Printf("  Starting %s CPU stress across %d cores...\n", cfg.Duration.Round(time.Second), runtime.NumCPU())

	ncpu := runtime.NumCPU()
	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Spin up one goroutine per logical CPU
	for i := 0; i < ncpu; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			matrixBurn(stop)
		}()
	}

	// Sample thermals during the run
	var peakTemp float64
	deadline := time.Now().Add(cfg.Duration)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()

	baseFreq := readBaseFreqMHz()
	throttled := false

	for range tick.C {
		if time.Now().After(deadline) {
			close(stop)
			wg.Wait()
			break
		}
		t := ReadCPUTemp()
		if t > peakTemp {
			peakTemp = t
		}
		cur := readCurFreqMHz()
		if baseFreq > 0 && cur > 0 && cur < int64(float64(baseFreq)*0.9) {
			throttled = true
		}
		col := "\033[32m"
		if t > cfg.WarnTempC {
			col = "\033[33m"
		}
		if t > cfg.FailTempC {
			col = "\033[31m"
		}
		fmt.Printf("\r  Temp: %s%.1f°C\033[0m  Peak: %.1f°C%s",
			col, t, peakTemp, throttleFlag(throttled))
	}

	res.PeakTempC = peakTemp
	res.Throttled = throttled
	fmt.Println("\n  Stress complete. Measuring cooldown (15s)...")

	time.Sleep(15 * time.Second)
	coolTemp := ReadCPUTemp()
	res.CooldownDC = peakTemp - coolTemp
	fmt.Printf("  Cooled to %.1f°C (dropped %.1f°C)\n", coolTemp, res.CooldownDC)

	if cfg.FailTempC > 0 && peakTemp >= cfg.FailTempC {
		res.Detail = fmt.Sprintf("CRITICAL: peak %.1f°C exceeded fail threshold %.1f°C", peakTemp, cfg.FailTempC)
	} else if cfg.WarnTempC > 0 && peakTemp >= cfg.WarnTempC {
		res.Detail = fmt.Sprintf("WARNING: peak %.1f°C exceeded warn threshold %.1f°C", peakTemp, cfg.WarnTempC)
	}

	return res
}

// matrixBurn does tight floating point work to generate real CPU heat.
// A 64×64 matrix multiply per iteration, running until stopped.
func matrixBurn(stop <-chan struct{}) {
	const n = 64
	a := make([]float64, n*n)
	b := make([]float64, n*n)
	c := make([]float64, n*n)
	for i := range a {
		a[i] = math.Sqrt(float64(i + 1))
		b[i] = math.Sin(float64(i + 1))
	}
	for {
		select {
		case <-stop:
			return
		default:
		}
		for i := 0; i < n; i++ {
			for j := 0; j < n; j++ {
				sum := 0.0
				for k := 0; k < n; k++ {
					sum += a[i*n+k] * b[k*n+j]
				}
				c[i*n+j] = sum
			}
		}
		// Use c to prevent compiler from eliminating the work
		_ = c[0]
	}
}

func throttleFlag(throttled bool) string {
	if throttled {
		return "  \033[31m⚠ THROTTLING\033[0m"
	}
	return ""
}
