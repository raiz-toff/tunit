package load

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPercentile(t *testing.T) {
	sorted := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	p50 := percentile(sorted, 50)
	if p50 < 40*time.Millisecond || p50 > 60*time.Millisecond {
		t.Errorf("p50 = %v, want ~50ms", p50)
	}

	p99 := percentile(sorted, 99)
	if p99 < 80*time.Millisecond {
		t.Errorf("p99 = %v, want high end", p99)
	}
}

func TestPercentileEmpty(t *testing.T) {
	got := percentile(nil, 50)
	if got != 0 {
		t.Errorf("percentile(nil, 50) = %v, want 0", got)
	}
}

func TestRepeatStr(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"=", 3, "==="},
		{"ab", 2, "abab"},
		{"x", 0, ""},
		{"x", -1, ""},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := repeatStr(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("repeatStr(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestStringReader(t *testing.T) {
	r := stringReader("hello world")
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", data, "hello world")
	}
}

func TestStringReaderEmpty(t *testing.T) {
	r := stringReader("")
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("got %d bytes, want 0", len(data))
	}
}

func TestRunAgainstServer(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	res := Run(Config{
		URL:      srv.URL,
		Method:   "GET",
		RPS:      10,
		Duration: 2 * time.Second,
	})

	if res.TotalReqs == 0 {
		t.Error("TotalReqs should be > 0")
	}
	if res.Errors != 0 {
		t.Errorf("Errors = %d, want 0", res.Errors)
	}
	if res.Throughput <= 0 {
		t.Error("Throughput should be > 0")
	}
	if res.LatP50 <= 0 {
		t.Error("LatP50 should be > 0")
	}
}

func TestRunServer500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	res := Run(Config{
		URL:      srv.URL,
		Method:   "GET",
		RPS:      5,
		Duration: 1 * time.Second,
	})

	if res.Errors == 0 {
		t.Error("should have counted 500s as errors")
	}
}
