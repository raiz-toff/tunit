package load

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tunit/tunit/internal/types"
)

type Config struct {
	URL      string
	Method   string
	Body     string
	Headers  map[string]string
	RPS      int
	Duration time.Duration
}

func Run(cfg Config) *types.LoadResult {
	res := &types.LoadResult{
		URL:    cfg.URL,
		Method: cfg.Method,
	}

	var totalReqs, errors int64
	var mu sync.Mutex
	var latencies []time.Duration

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConnsPerHost: cfg.RPS * 2,
		},
	}

	// Token-bucket: one tick per 1/RPS interval
	interval := time.Second / time.Duration(cfg.RPS)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	deadline := time.Now().Add(cfg.Duration)
	var wg sync.WaitGroup

	// Live progress display
	go func() {
		bar := time.NewTicker(500 * time.Millisecond)
		defer bar.Stop()
		start := time.Now()
		for time.Now().Before(deadline.Add(500 * time.Millisecond)) {
			<-bar.C
			elapsed := time.Since(start)
			pct := float64(elapsed) / float64(cfg.Duration) * 100
			if pct > 100 {
				pct = 100
			}
			filled := int(pct / 5)
			bar_str := fmt.Sprintf("[%s%s]", repeatStr("=", filled), repeatStr(" ", 20-filled))
			rps := float64(atomic.LoadInt64(&totalReqs)) / elapsed.Seconds()
			errs := atomic.LoadInt64(&errors)
			fmt.Printf("\r  %s %.0f%%  %.0f req/s  errors: %d   ",
				bar_str, pct, rps, errs)
		}
	}()

	for time.Now().Before(deadline) {
		<-ticker.C
		wg.Add(1)
		go func() {
			defer wg.Done()
			lat, err := doRequest(client, cfg)
			atomic.AddInt64(&totalReqs, 1)
			if err != nil {
				atomic.AddInt64(&errors, 1)
				return
			}
			mu.Lock()
			latencies = append(latencies, lat)
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println()

	res.TotalReqs = atomic.LoadInt64(&totalReqs)
	res.Errors = atomic.LoadInt64(&errors)
	res.Duration = cfg.Duration
	res.Throughput = float64(res.TotalReqs) / cfg.Duration.Seconds()

	if len(latencies) > 0 {
		sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
		res.LatP50 = percentile(latencies, 50)
		res.LatP95 = percentile(latencies, 95)
		res.LatP99 = percentile(latencies, 99)
		res.LatP999 = percentile(latencies, 99.9)
		res.LatMax = latencies[len(latencies)-1]
	}

	return res
}

func doRequest(client *http.Client, cfg Config) (time.Duration, error) {
	var body io.Reader
	if cfg.Body != "" {
		body = stringReader(cfg.Body)
	}
	req, err := http.NewRequest(cfg.Method, cfg.URL, body)
	if err != nil {
		return 0, err
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	lat := time.Since(start)
	if err != nil {
		return lat, err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 500 {
		return lat, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return lat, nil
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p / 100)
	return sorted[idx]
}

func repeatStr(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, len(s)*n)
	for i := 0; i < n; i++ {
		copy(out[i*len(s):], s)
	}
	return string(out)
}

type stringReaderT struct {
	s   string
	pos int
}

func stringReader(s string) io.Reader { return &stringReaderT{s: s} }

func (r *stringReaderT) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.s) {
		return 0, io.EOF
	}
	n = copy(p, r.s[r.pos:])
	r.pos += n
	return n, nil
}
