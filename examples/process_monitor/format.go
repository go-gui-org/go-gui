package main

import (
	"fmt"
	"strconv"
	"time"
)

// Metric cell text: "--" when the OS refused the reading, never a fake 0.

// CPUText renders CPU% with one decimal, or "--" when unknown.
func (p *ProcInfo) CPUText() string { return formatCPU(p.CPUPercent) }

// RSSText renders resident memory, or "--" when unknown.
func (p *ProcInfo) RSSText() string {
	if p.MetricsUnknown {
		return "--"
	}
	return formatBytes(p.RSSBytes)
}

// MemText renders memory share as a percentage, or "--" when unknown.
func (p *ProcInfo) MemText() string {
	if p.MetricsUnknown {
		return "--"
	}
	return fmt.Sprintf("%.2f%%", p.MemPercent)
}

// ThreadsText renders the thread count, or "--" when unknown.
func (p *ProcInfo) ThreadsText() string {
	if p.MetricsUnknown || p.Threads <= 0 {
		return "--"
	}
	return strconv.Itoa(p.Threads)
}

// formatCPU renders a CPU percentage, with "--" for unreadable values.
func formatCPU(v float64) string {
	if v < 0 {
		return "--"
	}
	return fmt.Sprintf("%.1f", v)
}

// formatCPUPercent is formatCPU with a trailing percent sign.
func formatCPUPercent(v float64) string {
	if v < 0 {
		return "--"
	}
	return fmt.Sprintf("%.1f%%", v)
}

// formatBytes renders a byte count with a binary unit suffix.
func formatBytes(v uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case v >= tb:
		return fmt.Sprintf("%.1fT", float64(v)/tb)
	case v >= gb:
		return fmt.Sprintf("%.1fG", float64(v)/gb)
	case v >= mb:
		return fmt.Sprintf("%.1fM", float64(v)/mb)
	case v >= kb:
		return fmt.Sprintf("%.1fK", float64(v)/kb)
	default:
		return fmt.Sprintf("%dB", v)
	}
}

// formatCount renders an integer with thousands separators.
func formatCount(v int) string {
	s := strconv.Itoa(v)
	if len(s) <= 3 {
		return s
	}
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, c)
	}
	return string(out)
}

// formatTime renders a timestamp as a clock (today) or short date.
func formatTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	if time.Since(t) < 24*time.Hour {
		return t.Format("15:04:05")
	}
	return t.Format("Jan 2 15:04")
}

// truncate shortens s to at most n runes, appending an ellipsis.
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(r[:n-1]) + "…"
}
