package qc

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type usbDevice struct {
	name string
	when time.Time
}

func renderUSBDisplay(devices []usbDevice, elapsed time.Duration, timeout time.Duration) {
	var sb strings.Builder
	sb.WriteString("\033[H") // cursor home

	remaining := timeout - elapsed
	if remaining < 0 {
		remaining = 0
	}

	sb.WriteString("\033[1m  USB PORT TEST\033[0m\n\n")
	sb.WriteString(fmt.Sprintf("  Plug a USB device into each port, one at a time.\n"))
	sb.WriteString(fmt.Sprintf("  The tool detects insertions automatically.\n\n"))

	sb.WriteString(fmt.Sprintf("  %-40s  %s\n", "Device", "Detected at"))
	sb.WriteString(fmt.Sprintf("  %s  %s\n", strings.Repeat("─", 40), strings.Repeat("─", 12)))

	if len(devices) == 0 {
		sb.WriteString("\033[2m  Waiting for USB devices...\033[0m\n")
	}
	for _, d := range devices {
		sb.WriteString(fmt.Sprintf("  \033[32m✓\033[0m %-38s  %s\n",
			truncUSB(d.name, 38),
			d.when.Format("15:04:05")))
	}

	// Pad to clear old lines
	for i := len(devices); i < 10; i++ {
		sb.WriteString(strings.Repeat(" ", 60) + "\n")
	}

	sb.WriteString(fmt.Sprintf("\n  \033[1m%d\033[0m device(s) detected  │  Time remaining: \033[33m%ds\033[0m\n",
		len(devices), int(remaining.Seconds())))
	sb.WriteString("\n  Press ENTER when done.")

	os.Stdout.WriteString(sb.String())
}

func truncUSB(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
