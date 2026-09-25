//go:build darwin || linux

package main

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// collectProcesses shells out to `ps` and parses one line per process. This
// keeps the example dependency-free while still working on macOS and Linux.
//
// The column list is chosen so every field except the last is a single
// whitespace-free token; the trailing `args` column (the full command line)
// absorbs the rest of the line. Linux additionally exposes `nlwp` (thread
// count); macOS `ps` does not, so threads are reported as unknown there.
// lstartTokens is the token count of `ps lstart` output ("Sun Sep 13
// 20:46:57 2026"). Fields collapses the padding of single-digit days
// ("Sep  5" -> "Sep","5"), so the count is stable at five.
const lstartTokens = 5

func collectProcesses() ([]ProcInfo, error) {
	linux := runtime.GOOS == "linux"
	now := time.Now()

	// Field order must match the parsing below. Trailing "=" suppresses the
	// header line for every column. The start-time column sits just before
	// args: Linux reports elapsed seconds (one numeric token), macOS a
	// start timestamp (five tokens, see lstartTokens). It feeds the store's
	// (PID, StartTime) identity, which guards against PID reuse.
	format := "pid=,ppid=,pcpu=,rss=,state=,user=,lstart=,args="
	fixed := 6 // number of leading fixed tokens before args
	if linux {
		format = "pid=,ppid=,pcpu=,rss=,nlwp=,state=,user=,etimes=,args="
		fixed = 8
	}

	// #nosec G204 — format is a constant, not user input.
	cmd := exec.Command("ps", "-axo", format)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var procs []ProcInfo
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\n")
		if strings.TrimSpace(line) == "" {
			continue
		}
		p, ok := parsePSLine(line, linux, fixed, now)
		if ok {
			procs = append(procs, p)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return procs, nil
}

// parsePSLine splits one `ps` line into its fixed columns plus the free-form
// command line. Unparseable numeric fields fall back to MetricsUnknown so the
// row still lists but shows "--" for the missing metric. now anchors the
// Linux etimes column (elapsed seconds) to an absolute start time.
func parsePSLine(line string, linux bool, fixed int, now time.Time) (ProcInfo, bool) {
	// Fields() collapses runs of spaces; the trailing args column can contain
	// spaces, so grab the fixed columns first and keep the remainder intact.
	fields := strings.Fields(line)
	if len(fields) < fixed {
		return ProcInfo{}, false
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return ProcInfo{}, false
	}
	p := ProcInfo{PID: pid}
	p.PPID, _ = strconv.Atoi(fields[1])

	if cpu, err := strconv.ParseFloat(fields[2], 64); err == nil {
		p.CPUPercent = cpu
	} else {
		p.CPUPercent = CPUPercentUnknown
		p.MetricsUnknown = true
	}

	// RSS is reported in KiB.
	if rssKiB, err := strconv.ParseUint(fields[3], 10, 64); err == nil {
		p.RSSBytes = rssKiB * 1024
	} else {
		p.MetricsUnknown = true
	}

	idx := 4
	if linux {
		if n, err := strconv.Atoi(fields[idx]); err == nil {
			p.Threads = n
		} else {
			p.MetricsUnknown = true
		}
		idx++
	}
	// macOS ps has no thread column; Threads stays 0 and ThreadsText renders
	// "--". That is not a metrics failure, so MetricsUnknown is left unset.

	p.State = normalizeState(fields[idx])
	idx++
	p.User = fields[idx]
	idx++

	// Start time feeds the store's (PID, StartTime) identity and guards
	// against PID reuse. Either column may fail on odd rows; a zero
	// StartTime degrades that row to PID-only, never to a wrong time.
	if linux {
		if elapsed, err := strconv.ParseUint(fields[idx], 10, 64); err == nil {
			p.StartTime = now.Add(-time.Duration(elapsed) * time.Second)
		}
		idx++
	} else {
		if len(fields) < idx+lstartTokens {
			return ProcInfo{}, false
		}
		p.StartTime = parseLstart(strings.Join(fields[idx:idx+lstartTokens], " "))
		idx += lstartTokens
	}

	// The command line is everything from the first args token to end of line.
	// Recover it from the original string to preserve internal spacing.
	p.Cmdline = commandTail(line, fields[:idx])
	p.Name = processName(p.Cmdline)
	return p, true
}

// parseLstart parses `ps lstart` output ("Sun Sep 13 20:46:57 2026").
// Fields collapses the space padding of single-digit days, so the unpadded
// layout comes first; the padded one stays as a fallback.
func parseLstart(s string) time.Time {
	for _, layout := range []string{"Mon Jan 2 15:04:05 2006", "Mon Jan _2 15:04:05 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// commandTail returns the portion of line following the already-consumed
// leading tokens, trimmed of leading whitespace.
func commandTail(line string, consumed []string) string {
	rest := line
	for _, tok := range consumed {
		i := strings.Index(rest, tok)
		if i < 0 {
			return strings.TrimSpace(rest)
		}
		rest = rest[i+len(tok):]
	}
	return strings.TrimSpace(rest)
}

// processName derives a short display name from a command line: the base name
// of the executable (first argument).
func processName(cmdline string) string {
	if cmdline == "" {
		return "?"
	}
	first := cmdline
	if before, _, ok := strings.Cut(cmdline, " "); ok {
		first = before
	}
	base := filepath.Base(first)
	if base == "" || base == "." || base == "/" {
		return cmdline
	}
	return base
}

// normalizeState keeps the primary process-state letter and drops trailing
// scheduler flags (e.g. "Ss" -> "S", "R+" -> "R") so the STATE column stays
// narrow and comparable across platforms.
func normalizeState(s string) string {
	if s == "" {
		return "?"
	}
	return s[:1]
}
