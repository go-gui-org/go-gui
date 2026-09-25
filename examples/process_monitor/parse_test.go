//go:build darwin || linux

package main

import (
	"testing"
	"time"
)

func TestParsePSLineLinuxEtimes(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	line := "  123    1  2.5  1024   4 S someuser 3600 /usr/bin/foo --bar"
	p, ok := parsePSLine(line, true, 8, now)
	if !ok {
		t.Fatal("parsePSLine rejected a well-formed Linux line")
	}
	if p.PID != 123 || p.PPID != 1 {
		t.Fatalf("identity = (%d, %d), want (123, 1)", p.PID, p.PPID)
	}
	if p.CPUPercent != 2.5 {
		t.Fatalf("CPUPercent = %v, want 2.5", p.CPUPercent)
	}
	if p.RSSBytes != 1024*1024 {
		t.Fatalf("RSSBytes = %d, want %d", p.RSSBytes, 1024*1024)
	}
	if p.Threads != 4 {
		t.Fatalf("Threads = %d, want 4", p.Threads)
	}
	if p.User != "someuser" || p.State != "S" {
		t.Fatalf("user/state = %q/%q, want someuser/S", p.User, p.State)
	}
	wantStart := now.Add(-3600 * time.Second)
	if !p.StartTime.Equal(wantStart) {
		t.Fatalf("StartTime = %v, want %v (now - etimes)", p.StartTime, wantStart)
	}
	if p.Cmdline != "/usr/bin/foo --bar" {
		t.Fatalf("Cmdline = %q, want the full trailing args", p.Cmdline)
	}
	if p.Name != "foo" {
		t.Fatalf("Name = %q, want foo", p.Name)
	}
}

func TestParsePSLineDarwinLstart(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	line := "  123    1  2.5  1024 S someuser Sun Sep 13 20:46:57 2026 /usr/bin/foo --bar"
	p, ok := parsePSLine(line, false, 6, now)
	if !ok {
		t.Fatal("parsePSLine rejected a well-formed macOS line")
	}
	if p.PID != 123 || p.PPID != 1 || p.User != "someuser" {
		t.Fatalf("identity = (%d, %d, %q), want (123, 1, someuser)", p.PID, p.PPID, p.User)
	}
	// macOS ps has no thread column; Threads stays unknown, not a failure.
	if p.Threads != 0 || p.MetricsUnknown {
		t.Fatalf("Threads = %d, MetricsUnknown = %v, want 0/false", p.Threads, p.MetricsUnknown)
	}
	wantStart := time.Date(2026, time.September, 13, 20, 46, 57, 0, time.UTC)
	if !p.StartTime.Equal(wantStart) {
		t.Fatalf("StartTime = %v, want %v", p.StartTime, wantStart)
	}
	if p.Cmdline != "/usr/bin/foo --bar" {
		t.Fatalf("Cmdline = %q, want the full trailing args", p.Cmdline)
	}
}

func TestParsePSLineDarwinLstartSingleDigitDay(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	// Real ps pads single-digit days ("Sep  5"); Fields collapses it.
	line := "  123    1  0.0  512 S me Sun Sep  5 10:00:00 2026 /bin/sleep 60"
	p, ok := parsePSLine(line, false, 6, now)
	if !ok {
		t.Fatal("parsePSLine rejected a single-digit-day macOS line")
	}
	wantStart := time.Date(2026, time.September, 5, 10, 0, 0, 0, time.UTC)
	if !p.StartTime.Equal(wantStart) {
		t.Fatalf("StartTime = %v, want %v", p.StartTime, wantStart)
	}
	if p.Cmdline != "/bin/sleep 60" {
		t.Fatalf("Cmdline = %q, want /bin/sleep 60", p.Cmdline)
	}
}

func TestParsePSLineLinuxBadEtimesDegradesToPIDOnly(t *testing.T) {
	t.Parallel()
	now := time.Unix(1_700_000_000, 0)
	line := "  123    1  2.5  1024   4 S someuser ??? /usr/bin/foo"
	p, ok := parsePSLine(line, true, 8, now)
	if !ok {
		t.Fatal("a bad etimes column must not drop the row")
	}
	if !p.StartTime.IsZero() {
		t.Fatalf("StartTime = %v, want zero (PID-only fallback)", p.StartTime)
	}
	if p.PID != 123 || p.Cmdline != "/usr/bin/foo" {
		t.Fatalf("row = (%d, %q), want (123, /usr/bin/foo)", p.PID, p.Cmdline)
	}
}
