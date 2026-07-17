package main

import (
	"testing"
	"time"
)

func TestSortColumnIndex(t *testing.T) {
	t.Parallel()
	cases := []struct {
		key  string
		want int
	}{
		{"pid", colPID},
		{"cpu", colCPU},
		{"mem", colRSS},
		{"rss", colRSS},
		{"user", colUser},
		{"state", colState},
		{"threads", colThreads},
		{"thr", colThreads},
		{"name", colName},
		{"unknown", colCPU}, // default
		{"", colCPU},        // default
	}
	for _, c := range cases {
		if got := sortColumnIndex(c.key); got != c.want {
			t.Fatalf("sortColumnIndex(%q) = %d, want %d", c.key, got, c.want)
		}
	}
}

func TestSnapInterval(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d    time.Duration
		want time.Duration
	}{
		{0, time.Second},                 // zero snaps to 1s
		{-time.Millisecond, time.Second}, // negative snaps to 1s
		{400 * time.Millisecond, 500 * time.Millisecond},
		{700 * time.Millisecond, 500 * time.Millisecond}, // closer to 500ms than 1s
		{1200 * time.Millisecond, time.Second},
		{1500 * time.Millisecond, time.Second}, // equidistant to 1s and 2s, first wins
		{3 * time.Second, 2 * time.Second},
		{3800 * time.Millisecond, 5 * time.Second},
		{10 * time.Second, 5 * time.Second},
	}
	for _, c := range cases {
		if got := snapInterval(c.d); got != c.want {
			t.Fatalf("snapInterval(%v) = %v, want %v", c.d, got, c.want)
		}
	}
}

func TestIntervalLabelRoundtrip(t *testing.T) {
	t.Parallel()
	for _, d := range []time.Duration{
		500 * time.Millisecond, time.Second, 2 * time.Second, 5 * time.Second,
	} {
		label := intervalLabel(d)
		back := intervalFromLabel(label)
		if back != d {
			t.Fatalf("interval roundtrip: %v -> %q -> %v", d, label, back)
		}
	}
}

func TestAbsDuration(t *testing.T) {
	t.Parallel()
	if got := absDuration(5 * time.Second); got != 5*time.Second {
		t.Fatalf("absDuration(5s) = %v", got)
	}
	if got := absDuration(-3 * time.Second); got != 3*time.Second {
		t.Fatalf("absDuration(-3s) = %v", got)
	}
	if got := absDuration(0); got != 0 {
		t.Fatalf("absDuration(0) = %v", got)
	}
}
