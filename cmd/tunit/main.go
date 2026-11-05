package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tunit/tunit/internal/load"
	"github.com/tunit/tunit/internal/lookup"
	"github.com/tunit/tunit/internal/menu"
	netpkg "github.com/tunit/tunit/internal/net"
	"github.com/tunit/tunit/internal/qc"
	"github.com/tunit/tunit/internal/report"
	"github.com/tunit/tunit/internal/stress"
	"github.com/tunit/tunit/internal/types"
)

func main() {
	root := &cobra.Command{
		Use:   "tunit",
		Short: "Universal system diagnostic, stress, and load testing tool",
		Long:  "Run with no arguments to launch the interactive guided menu.",
		// No args → interactive menu
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInteractiveMenu()
		},
	}

	root.AddCommand(cmdQC(), cmdStress(), cmdLoad(), cmdNet(), cmdReport())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ── interactive menu (default entry point) ─────────────────────

func runInteractiveMenu() error {
	for {
		choice := menu.AskMain()
		switch choice {
		case menu.ChoiceFullQC:
			repair := menu.AskRepairType()
			result := runQC(repair, false, false)
			saveAndPrint(result)
		case menu.ChoiceStress:
			tier := menu.AskStressTier()
			runStressInteractive(tier)
		case menu.ChoiceNetwork:
			res := netpkg.Run(netpkg.Config{})
			report.PrintNetResult(res)
		case menu.ChoiceLoadTest:
			url, method, rps, dur := menu.AskLoadTestParams()
			d, err := time.ParseDuration(dur)
			if err != nil {
				d = 30 * time.Second
			}
			res := load.Run(load.Config{URL: url, Method: method, RPS: rps, Duration: d})
			report.PrintLoadResult(res)
		case menu.ChoiceLastReport:
			fmt.Println("\n  Feature coming soon — reports saved as QC_REPORT_*.txt in current directory.\n")
		}

		fmt.Print("\n  Press ENTER to return to main menu, or Ctrl+C to exit...")
		fmt.Scanln()
	}
}

func runQC(repair menu.RepairType, autoOnly, doLookup bool) *types.QCResult {
	r := qc.Run(repair, autoOnly)
	if doLookup && r.Serial != "" {
		fmt.Print("  Looking up warranty info...")
		info := lookup.TryAll(r.Serial)
		if info != nil {
			r.WarrantyExpiry = info.Expiry
			r.WarrantyVendor = info.Vendor
			fmt.Printf(" found (%s)\n", info.Vendor)
		} else {
			fmt.Println(" not found (offline or unknown vendor)")
		}
	}
	return r
}

func saveAndPrint(r *types.QCResult) {
	report.PrintQCReport(r)
	if fname, err := report.SaveTextSlip(r); err == nil {
		fmt.Printf("  Saved: %s\n", fname)
	}
}

func runStressInteractive(tier int) {
	cfg := stress.Config{WarnTempC: 85, FailTempC: 95}
	switch tier {
	case 1:
		cfg.Duration = 20 * time.Second
		res := stress.RunTier1(cfg)
		report.PrintStressResult(res)
	case 2:
		cfg.Duration = 45 * time.Second
		res := stress.RunTier1(cfg)
		report.PrintStressResult(res)
	case 3:
		cfg.Duration = 120 * time.Second
		res := stress.RunTier1(cfg)
		report.PrintStressResult(res)
	case 4:
		cfg.Duration = 60 * time.Second
		res := stress.RunTier2(cfg)
		report.PrintStressResult(res)
	}
}

// ── subcommands (power-user / CI flags) ────────────────────────

func cmdQC() *cobra.Command {
	var autoOnly, doLookup bool
	var repairStr string
	cmd := &cobra.Command{
		Use:   "qc",
		Short: "Run the full hardware QC checklist",
		RunE: func(cmd *cobra.Command, args []string) error {
			repair := menu.RepairNone
			switch repairStr {
			case "screen", "lcd":
				repair = menu.RepairScreen
			case "motherboard", "blr":
				repair = menu.RepairMotherboard
			case "battery":
				repair = menu.RepairBattery
			case "storage", "ssd", "hdd":
				repair = menu.RepairStorage
			case "keyboard":
				repair = menu.RepairKeyboard
			}
			r := runQC(repair, autoOnly, doLookup)
			saveAndPrint(r)
			if r.Verdict == types.StatusFail {
				os.Exit(2)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&autoOnly, "auto", false, "Skip interactive human checks")
	cmd.Flags().BoolVar(&doLookup, "lookup", false, "Look up warranty via manufacturer API")
	cmd.Flags().StringVar(&repairStr, "repaired", "", "Repair type: screen|motherboard|battery|storage|keyboard")
	return cmd
}

func cmdStress() *cobra.Command {
	var tier int
	var durationStr string
	var warnTemp, failTemp float64
	cmd := &cobra.Command{
		Use:   "stress",
		Short: "Run CPU/thermal stress test",
		RunE: func(cmd *cobra.Command, args []string) error {
			dur, err := time.ParseDuration(durationStr)
			if err != nil {
				return fmt.Errorf("invalid duration: %s", durationStr)
			}
			cfg := stress.Config{Duration: dur, WarnTempC: warnTemp, FailTempC: failTemp}
			var res *types.StressResult
			if tier == 2 {
				res = stress.RunTier2(cfg)
			} else {
				res = stress.RunTier1(cfg)
			}
			report.PrintStressResult(res)
			return nil
		},
	}
	cmd.Flags().IntVar(&tier, "tier", 1, "Stress tier: 1 (native Go) or 2 (FurMark/Prime95)")
	cmd.Flags().StringVar(&durationStr, "duration", "45s", "Stress duration (e.g. 20s, 2m)")
	cmd.Flags().Float64Var(&warnTemp, "warn-temp", 85, "Warn if CPU temp exceeds this °C")
	cmd.Flags().Float64Var(&failTemp, "fail-temp", 95, "Fail if CPU temp exceeds this °C")
	return cmd
}

func cmdLoad() *cobra.Command {
	var url, method, body, jsonFile string
	var headers []string
	var rps int
	var durationStr string
	cmd := &cobra.Command{
		Use:   "load",
		Short: "HTTP load / stress test against an endpoint",
		RunE: func(cmd *cobra.Command, args []string) error {
			if url == "" {
				return fmt.Errorf("--url is required")
			}
			dur, err := time.ParseDuration(durationStr)
			if err != nil {
				return fmt.Errorf("invalid duration: %s", durationStr)
			}
			headerMap := make(map[string]string)
			for _, h := range headers {
				parts := splitHeader(h)
				if len(parts) == 2 {
					headerMap[parts[0]] = parts[1]
				}
			}
			res := load.Run(load.Config{
				URL:      url,
				Method:   method,
				Body:     body,
				Headers:  headerMap,
				RPS:      rps,
				Duration: dur,
			})
			report.PrintLoadResult(res)
			if jsonFile != "" {
				if err := report.SaveLoadJSON(res, jsonFile); err != nil {
					fmt.Fprintf(os.Stderr, "JSON save failed: %v\n", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "Target URL (required)")
	cmd.Flags().StringVar(&method, "method", "GET", "HTTP method")
	cmd.Flags().StringVar(&body, "body", "", "Request body")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "Header: Key: Value (repeatable)")
	cmd.Flags().IntVar(&rps, "rps", 50, "Requests per second")
	cmd.Flags().StringVar(&durationStr, "duration", "30s", "Test duration")
	cmd.Flags().StringVar(&jsonFile, "json", "", "Save results to JSON file")
	return cmd
}

func cmdNet() *cobra.Command {
	var host string
	var noBandwidth bool
	cmd := &cobra.Command{
		Use:   "net",
		Short: "Run network tests: ping, DNS, TCP, traceroute, bandwidth",
		RunE: func(cmd *cobra.Command, args []string) error {
			res := netpkg.Run(netpkg.Config{
				PingHost:    host,
				NoBandwidth: noBandwidth,
			})
			report.PrintNetResult(res)
			return nil
		},
	}
	cmd.Flags().StringVar(&host, "host", "8.8.8.8", "Host to ping and trace")
	cmd.Flags().BoolVar(&noBandwidth, "no-bandwidth", false, "Skip bandwidth test")
	return cmd
}

func cmdReport() *cobra.Command {
	return &cobra.Command{
		Use:   "report",
		Short: "List saved QC reports in current directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			entries, err := os.ReadDir(".")
			if err != nil {
				return err
			}
			found := false
			for _, e := range entries {
				if !e.IsDir() && len(e.Name()) > 10 &&
					e.Name()[:10] == "QC_REPORT_" {
					info, _ := e.Info()
					fmt.Printf("  %s  (%s)\n", e.Name(), info.ModTime().Format("2006-01-02 15:04"))
					found = true
				}
			}
			if !found {
				fmt.Println("  No QC reports found in current directory.")
			}
			return nil
		},
	}
}

func splitHeader(h string) []string {
	for i, c := range h {
		if c == ':' {
			return []string{h[:i], h[i+2:]}
		}
	}
	return nil
}
