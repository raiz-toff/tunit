package types

import "time"

type Status int

const (
	StatusPass Status = iota
	StatusWarn
	StatusFail
	StatusSkip
	StatusInfo
)

func (s Status) String() string {
	switch s {
	case StatusPass:
		return "PASS"
	case StatusWarn:
		return "WARN"
	case StatusFail:
		return "FAIL"
	case StatusSkip:
		return "SKIP"
	default:
		return "INFO"
	}
}

type CheckResult struct {
	Step    int
	Name    string
	Status  Status
	Detail  string
	Warning string
}

type QCResult struct {
	Timestamp time.Time
	OS        string
	Hostname  string
	Serial    string
	Model     string
	CPU       string
	RAM       string
	BIOSVer   string
	Checks    []CheckResult
	Verdict   Status
	// warranty lookup (optional, may be empty)
	WarrantyExpiry string
	WarrantyVendor string
}

type StressResult struct {
	Tier       int
	Duration   time.Duration
	PeakTempC  float64
	IdleTempC  float64
	Throttled  bool
	CooldownDC float64 // degrees dropped in 15s after stress
	Detail     string
}

type LoadResult struct {
	URL        string
	Method     string
	Duration   time.Duration
	TotalReqs  int64
	Errors     int64
	LatP50     time.Duration
	LatP95     time.Duration
	LatP99     time.Duration
	LatP999    time.Duration
	LatMax     time.Duration
	Throughput float64 // req/s
}

type NetResult struct {
	PingHost    string
	PingMin     float64
	PingAvg     float64
	PingMax     float64
	PingLossPct float64
	DNSLatencyMs float64
	DownloadMbps float64
	TCPChecks   []TCPCheck
}

type TCPCheck struct {
	Host    string
	Port    int
	Open    bool
	Latency time.Duration
}
