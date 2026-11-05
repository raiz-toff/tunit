package net

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/tunit/tunit/internal/types"
)

var dnsProbes = []string{
	"google.com", "github.com", "cloudflare.com", "amazon.com", "microsoft.com",
}

type Config struct {
	PingHost   string
	TCPTargets []types.TCPCheck // pre-populated with host/port, Open/Latency filled in
	NoBandwidth bool
}

func Run(cfg Config) *types.NetResult {
	res := &types.NetResult{PingHost: cfg.PingHost}
	if res.PingHost == "" {
		res.PingHost = "8.8.8.8"
	}

	fmt.Printf("  Ping %s...", res.PingHost)
	pingResult := runPing(res.PingHost, 10)
	res.PingMin = pingResult.min
	res.PingAvg = pingResult.avg
	res.PingMax = pingResult.max
	res.PingLossPct = pingResult.loss
	fmt.Printf(" avg %.0fms\n", res.PingAvg)

	fmt.Print("  DNS latency...")
	res.DNSLatencyMs = measureDNS()
	fmt.Printf(" avg %.0fms\n", res.DNSLatencyMs)

	if len(cfg.TCPTargets) == 0 {
		cfg.TCPTargets = []types.TCPCheck{
			{Host: "8.8.8.8", Port: 53},
			{Host: "1.1.1.1", Port: 53},
			{Host: "google.com", Port: 443},
		}
	}
	for i, tc := range cfg.TCPTargets {
		fmt.Printf("  TCP %s:%d...", tc.Host, tc.Port)
		open, lat := tcpProbe(tc.Host, tc.Port)
		cfg.TCPTargets[i].Open = open
		cfg.TCPTargets[i].Latency = lat
		state := "open"
		if !open {
			state = "closed"
		}
		fmt.Printf(" %s (%s)\n", state, lat.Round(time.Millisecond))
	}
	res.TCPChecks = cfg.TCPTargets

	if !cfg.NoBandwidth {
		fmt.Print("  Bandwidth (Cloudflare CDN, 25MB)...")
		mbps := measureBandwidth()
		res.DownloadMbps = mbps
		fmt.Printf(" %.1f Mbps\n", mbps)
	}

	return res
}

type pingStats struct {
	min, avg, max, loss float64
}

func runPing(host string, count int) pingStats {
	// Try system ping binary — works unprivileged everywhere
	var args []string
	switch runtime.GOOS {
	case "windows":
		args = []string{"-n", fmt.Sprintf("%d", count), host}
	default:
		args = []string{"-c", fmt.Sprintf("%d", count), "-q", host}
	}
	out, err := exec.Command("ping", args...).Output()
	if err != nil {
		return pingStats{loss: 100}
	}
	return parsePingOutput(string(out))
}

func parsePingOutput(out string) pingStats {
	var stats pingStats
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		lower := strings.ToLower(line)
		// Linux/macOS: "rtt min/avg/max/mdev = 4.123/6.456/9.789/1.234 ms"
		if strings.Contains(lower, "min/avg/max") {
			parts := strings.Split(line, "=")
			if len(parts) >= 2 {
				nums := strings.Split(strings.TrimSpace(parts[1]), "/")
				if len(nums) >= 3 {
					fmt.Sscanf(nums[0], "%f", &stats.min)
					fmt.Sscanf(nums[1], "%f", &stats.avg)
					fmt.Sscanf(nums[2], "%f", &stats.max)
				}
			}
		}
		// Linux: "2 packets transmitted, 2 received, 0% packet loss"
		if strings.Contains(lower, "packet loss") {
			var loss float64
			if _, err := fmt.Sscanf(line, "%*d packets transmitted, %*d received, %f%% packet loss", &loss); err == nil {
				stats.loss = loss
			}
		}
		// Windows: "Minimum = 4ms, Maximum = 9ms, Average = 6ms"
		if strings.Contains(lower, "minimum =") {
			fmt.Sscanf(line, "    Minimum = %fms, Maximum = %fms, Average = %fms",
				&stats.min, &stats.max, &stats.avg)
		}
	}
	return stats
}

func measureDNS() float64 {
	var total time.Duration
	count := 0
	for _, host := range dnsProbes {
		start := time.Now()
		_, err := net.LookupHost(host)
		if err == nil {
			total += time.Since(start)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return float64(total.Milliseconds()) / float64(count)
}

func tcpProbe(host string, port int) (bool, time.Duration) {
	addr := fmt.Sprintf("%s:%d", host, port)
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	lat := time.Since(start)
	if err != nil {
		return false, lat
	}
	conn.Close()
	return true, lat
}

func measureBandwidth() float64 {
	url := "https://speed.cloudflare.com/__down?bytes=25000000"
	start := time.Now()
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0
	}
	elapsed := time.Since(start).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(n) * 8 / elapsed / 1_000_000 // bits/s → Mbps
}
