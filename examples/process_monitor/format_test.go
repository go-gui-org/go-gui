package main

import (
	"math"
	"testing"
	"time"
)

func TestFormatCPUNegativeAndZero(t *testing.T) {
	t.Parallel()
	if got := formatCPU(-1); got != "--" {
		t.Fatalf("formatCPU(-1) = %q, want \"--\"", got)
	}
	if got := formatCPU(0); got != "0.0" {
		t.Fatalf("formatCPU(0) = %q, want \"0.0\"", got)
	}
}

func TestFormatCPUNaN(t *testing.T) {
	t.Parallel()
	if got := formatCPU(math.NaN()); got != "--" {
		t.Fatalf("formatCPU(NaN) = %q, want \"--\"", got)
	}
	if got := formatCPUPercent(math.NaN()); got != "--" {
		t.Fatalf("formatCPUPercent(NaN) = %q, want \"--\"", got)
	}
}

func TestFormatCPUPercent(t *testing.T) {
	t.Parallel()
	if got := formatCPUPercent(-1); got != "--" {
		t.Fatalf("formatCPUPercent(-1) = %q, want \"--\"", got)
	}
	if got := formatCPUPercent(42.5); got != "42.5%" {
		t.Fatalf("formatCPUPercent(42.5) = %q, want \"42.5%%\"", got)
	}
}

func TestFormatBytes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    uint64
		want string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1024 * 1024, "1.0M"},
		{1024 * 1024 * 1024, "1.0G"},
		{1024 * 1024 * 1024 * 1024, "1.0T"},
	}
	for _, c := range cases {
		if got := formatBytes(c.v); got != c.want {
			t.Fatalf("formatBytes(%d) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		v    int
		want string
	}{
		{0, "0"},
		{999, "999"},
		{1000, "1,000"},
		{1000000, "1,000,000"},
		{1234567890, "1,234,567,890"},
	}
	for _, c := range cases {
		if got := formatCount(c.v); got != c.want {
			t.Fatalf("formatCount(%d) = %q, want %q", c.v, got, c.want)
		}
	}
}

func TestFormatTimeZero(t *testing.T) {
	t.Parallel()
	if got := formatTime(time.Time{}); got != "unknown" {
		t.Fatalf("formatTime(zero) = %q, want \"unknown\"", got)
	}
}

func TestFormatTimeRecent(t *testing.T) {
	t.Parallel()
	now := time.Now().Add(-2 * time.Second)
	got := formatTime(now)
	if got == "unknown" {
		t.Fatal("recent time should not render as unknown")
	}
	// The format is HH:MM:SS for recent times.
	if len(got) != 8 {
		t.Fatalf("expected clock format (8 chars), got %q", got)
	}
}

func TestFormatTimeOld(t *testing.T) {
	t.Parallel()
	old := time.Now().Add(-48 * time.Hour)
	got := formatTime(old)
	if got == "unknown" {
		t.Fatal("old time should not render as unknown")
	}
}

func TestTruncate(t *testing.T) {
	t.Parallel()
	cases := []struct {
		s    string
		n    int
		want string
	}{
		{"", 0, ""},
		{"hello", -1, ""},
		{"hello", 0, ""},
		{"h", 1, "h"},
		{"hello", 1, "…"},
		{"hello", 5, "hello"},
		{"hello", 6, "hello"},
		{"hello world", 7, "hello …"},
	}
	for _, c := range cases {
		if got := truncate(c.s, c.n); got != c.want {
			t.Fatalf("truncate(%q, %d) = %q, want %q", c.s, c.n, got, c.want)
		}
	}
}

func TestProcInfoCPUText(t *testing.T) {
	t.Parallel()
	p := ProcInfo{CPUPercent: -1}
	if got := p.CPUText(); got != "--" {
		t.Fatalf("CPUText(unknown) = %q, want \"--\"", got)
	}
	p.CPUPercent = 12.3
	if got := p.CPUText(); got != "12.3" {
		t.Fatalf("CPUText(12.3) = %q, want \"12.3\"", got)
	}
}

func TestProcInfoRSSText(t *testing.T) {
	t.Parallel()
	p := ProcInfo{MetricsUnknown: true}
	if got := p.RSSText(); got != "--" {
		t.Fatalf("RSSText(unknown) = %q, want \"--\"", got)
	}
	p = ProcInfo{RSSBytes: 10 << 20}
	if got := p.RSSText(); got != "10.0M" {
		t.Fatalf("RSSText(10MiB) = %q, want \"10.0M\"", got)
	}
}

func TestProcInfoMemText(t *testing.T) {
	t.Parallel()
	p := ProcInfo{MetricsUnknown: true}
	if got := p.MemText(); got != "--" {
		t.Fatalf("MemText(unknown) = %q, want \"--\"", got)
	}
	p = ProcInfo{MemPercent: 3.75}
	if got := p.MemText(); got != "3.75%" {
		t.Fatalf("MemText(3.75) = %q, want \"3.75%%\"", got)
	}
}

func TestProcInfoThreadsText(t *testing.T) {
	t.Parallel()
	p := ProcInfo{MetricsUnknown: true}
	if got := p.ThreadsText(); got != "--" {
		t.Fatalf("ThreadsText(unknown flags) = %q, want \"--\"", got)
	}
	p = ProcInfo{MetricsUnknown: false, Threads: 0}
	if got := p.ThreadsText(); got != "--" {
		t.Fatalf("ThreadsText(0 threads) = %q, want \"--\"", got)
	}
	p = ProcInfo{Threads: 4}
	if got := p.ThreadsText(); got != "4" {
		t.Fatalf("ThreadsText(4) = %q, want \"4\"", got)
	}
}
