package report

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
)

func statusColor(s types.Status) string {
	switch s {
	case types.StatusPass:
		return colorGreen
	case types.StatusWarn:
		return colorYellow
	case types.StatusFail:
		return colorRed
	case types.StatusSkip:
		return colorDim
	default:
		return colorCyan
	}
}

func PrintBanner() {
	width := 62
	line := strings.Repeat("═", width)
	fmt.Printf("\n%s%s%s\n", colorBold, line, colorReset)
	title := "SYSTEM DIAGNOSTIC TOOL  v1.0"
	pad := (width - len(title)) / 2
	fmt.Printf("%s%s%s%s%s\n", colorBold, colorCyan, strings.Repeat(" ", pad), title, colorReset)
	fmt.Printf("%s%s%s\n\n", colorBold, line, colorReset)
}

func PrintQCReport(r *types.QCResult) {
	width := 62
	line := strings.Repeat("═", width)
	divider := strings.Repeat("─", width)

	fmt.Printf("\n%s%s%s\n", colorBold, line, colorReset)
	fmt.Printf("%s  SYSTEM DIAGNOSTIC REPORT%s\n", colorBold, colorReset)
	fmt.Printf("%s%s%s\n", colorBold, line, colorReset)

	if r.Model != "" {
		fmt.Printf(" HOST:  %-32s S/N: %s\n", trunc(r.Model, 32), r.Serial)
	}
	if r.CPU != "" {
		fmt.Printf(" CPU:   %-32s RAM: %s\n", trunc(r.CPU, 32), r.RAM)
	}
	fmt.Printf(" OS:    %s\n", r.OS)
	if r.BIOSVer != "" {
		fmt.Printf(" BIOS:  %s\n", r.BIOSVer)
	}
	if r.WarrantyExpiry != "" {
		fmt.Printf(" WARRANTY: %s via %s\n", r.WarrantyExpiry, r.WarrantyVendor)
	}
	fmt.Printf("%s%s%s\n", colorBold, divider, colorReset)

	for _, c := range r.Checks {
		col := statusColor(c.Status)
		label := fmt.Sprintf("[%s]", c.Status.String())
		detail := c.Detail
		if c.Warning != "" {
			detail = c.Detail + "  ⚠ " + c.Warning
		}
		fmt.Printf(" %s%-6s%s %-26s : %s\n", col, label, colorReset, trunc(c.Name, 26), detail)
	}

	fmt.Printf("%s%s%s\n", colorBold, divider, colorReset)

	verdictColor := statusColor(r.Verdict)
	fmt.Printf("%s OVERALL VERDICT: %s%s%s%s\n", colorBold, verdictColor, r.Verdict.String(), colorReset, colorReset)
	fmt.Printf(" Report saved: QC_REPORT_%s_%s.txt\n", r.Serial, r.Timestamp.Format("20060102_150405"))
	fmt.Printf("%s%s%s\n\n", colorBold, line, colorReset)
}

func PrintStressResult(res *types.StressResult) {
	divider := strings.Repeat("─", 62)
	fmt.Printf("\n%s STRESS TEST — TIER %d%s\n", colorBold, res.Tier, colorReset)
	fmt.Printf("%s\n", divider)
	fmt.Printf(" Duration  : %s\n", res.Duration.Round(time.Second))
	fmt.Printf(" Idle temp : %.1f°C\n", res.IdleTempC)
	fmt.Printf(" Peak temp : %.1f°C\n", res.PeakTempC)
	if res.Throttled {
		fmt.Printf(" Throttled : %sYES — clock dropped during load%s\n", colorRed, colorReset)
	} else {
		fmt.Printf(" Throttled : %sNo%s\n", colorGreen, colorReset)
	}
	if res.CooldownDC > 0 {
		fmt.Printf(" Cooldown  : -%.1f°C in 15s\n", res.CooldownDC)
	}
	if res.Detail != "" {
		fmt.Printf(" Note      : %s\n", res.Detail)
	}
	fmt.Printf("%s\n\n", divider)
}

func PrintLoadResult(res *types.LoadResult) {
	divider := strings.Repeat("─", 62)
	fmt.Printf("\n%s HTTP LOAD TEST%s\n", colorBold, colorReset)
	fmt.Printf("%s\n", divider)
	fmt.Printf(" Target    : %s %s\n", res.Method, res.URL)
	fmt.Printf(" Requests  [total, rate, throughput]  %d, %.2f, %.2f req/s\n",
		res.TotalReqs, float64(res.TotalReqs)/res.Duration.Seconds(), res.Throughput)
	fmt.Printf(" Duration  %s\n", res.Duration.Round(time.Millisecond))
	fmt.Printf(" Latencies [p50, p95, p99, p99.9, max]\n")
	fmt.Printf("           %s  %s  %s  %s  %s\n",
		res.LatP50, res.LatP95, res.LatP99, res.LatP999, res.LatMax)
	successPct := 0.0
	if res.TotalReqs > 0 {
		successPct = float64(res.TotalReqs-res.Errors) / float64(res.TotalReqs) * 100
	}
	col := colorGreen
	if successPct < 99 {
		col = colorYellow
	}
	if successPct < 95 {
		col = colorRed
	}
	fmt.Printf(" Success   %s%.2f%%%s\n", col, successPct, colorReset)
	fmt.Printf(" Errors    %d\n", res.Errors)
	fmt.Printf("%s\n\n", divider)
}

func PrintNetResult(res *types.NetResult) {
	divider := strings.Repeat("─", 62)
	fmt.Printf("\n%s NETWORK TEST%s\n", colorBold, colorReset)
	fmt.Printf("%s\n", divider)

	lossCol := colorGreen
	if res.PingLossPct > 0 {
		lossCol = colorYellow
	}
	if res.PingLossPct > 5 {
		lossCol = colorRed
	}
	fmt.Printf(" Ping      %-16s %s%.1f%% loss%s | avg %.0fms (min %.0f max %.0f jitter %.0f)\n",
		res.PingHost, lossCol, res.PingLossPct, colorReset,
		res.PingAvg, res.PingMin, res.PingMax, res.PingMax-res.PingMin)

	fmt.Printf(" DNS       avg %.0fms\n", res.DNSLatencyMs)

	for _, tc := range res.TCPChecks {
		col := colorGreen
		state := "open"
		if !tc.Open {
			col = colorRed
			state = "closed/filtered"
		}
		fmt.Printf(" TCP       %-20s %s%-16s%s %s\n",
			fmt.Sprintf("%s:%d", tc.Host, tc.Port), col, state, colorReset, tc.Latency.Round(time.Millisecond))
	}

	if res.DownloadMbps > 0 {
		fmt.Printf(" Bandwidth %.1f Mbps down (Cloudflare CDN)\n", res.DownloadMbps)
	}
	fmt.Printf("%s\n\n", divider)
}

// Step prints a section header for interactive check steps.
func PrintStepHeader(step, total int, name string) {
	divider := strings.Repeat("━", 50)
	fmt.Printf("\n%s%s%s\n", colorBold, divider, colorReset)
	fmt.Printf("%s  STEP %d OF %d — %s%s\n", colorBold, step, total, strings.ToUpper(name), colorReset)
	fmt.Printf("%s%s%s\n", colorBold, divider, colorReset)
}

// AskYNS prompts the tester for Y / N / S and returns the choice.
// question is the full question text.
// passLabel / failLabel describe the outcome of Y and N.
func AskYNS(question, passLabel, failLabel string) types.Status {
	fmt.Printf("\n  %s\n\n", question)
	fmt.Printf("    %sY%s → %s        %s[PASS]%s\n", colorBold, colorReset, passLabel, colorGreen, colorReset)
	fmt.Printf("    %sN%s → %s   %s[FAIL]%s\n", colorBold, colorReset, failLabel, colorRed, colorReset)
	fmt.Printf("    %sS%s → Skip                    %s[SKIP]%s\n", colorBold, colorReset, colorDim, colorReset)
	fmt.Printf("\n  Enter Y / N / S: ")

	var input string
	for {
		fmt.Fscan(os.Stdin, &input)
		switch strings.ToUpper(strings.TrimSpace(input)) {
		case "Y", "YES":
			return types.StatusPass
		case "N", "NO":
			return types.StatusFail
		case "S", "SKIP", "":
			return types.StatusSkip
		default:
			fmt.Printf("  Please enter Y, N, or S: ")
		}
	}
}

func trunc(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}
