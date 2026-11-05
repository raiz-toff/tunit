package stress

import (
	"testing"
	"time"
)

func TestMatrixBurnStops(t *testing.T) {
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		matrixBurn(stop)
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	close(stop)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("matrixBurn did not stop within 2s after close(stop)")
	}
}

func TestThrottleFlag(t *testing.T) {
	if got := throttleFlag(false); got != "" {
		t.Errorf("throttleFlag(false) = %q, want empty", got)
	}
	if got := throttleFlag(true); got == "" {
		t.Error("throttleFlag(true) should not be empty")
	}
}

func TestRunTier1Short(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	cfg := Config{
		Duration:  2 * time.Second,
		WarnTempC: 200,
		FailTempC: 200,
	}
	res := RunTier1(cfg)
	if res.Tier != 1 {
		t.Errorf("Tier = %d, want 1", res.Tier)
	}
	if res.Duration != 2*time.Second {
		t.Errorf("Duration = %v, want 2s", res.Duration)
	}
}
