//go:build windows

package main

import (
	"encoding/csv"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// collectProcesses parses `tasklist` CSV output on Windows. This lists every
// process with image name, PID, memory usage, status, and user, keeping the
// example dependency-free. Live per-interval CPU% is not available from a
// single tasklist call, so CPU is reported as unknown (renders "--"); a
// two-sample CPU-time delta would be the enhancement.
func collectProcesses() ([]ProcInfo, error) {
	// /v = verbose (adds Status and User Name columns), /fo csv, /nh = no header.
	// #nosec G204 — all arguments are constants.
	cmd := exec.Command("tasklist", "/fo", "csv", "/nh", "/v")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	r := csv.NewReader(strings.NewReader(string(out)))
	r.FieldsPerRecord = -1 // rows vary; be lenient
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	// Verbose column order: Image Name, PID, Session Name, Session#,
	// Mem Usage, Status, User Name, CPU Time, Window Title.
	procs := make([]ProcInfo, 0, len(records))
	for _, rec := range records {
		if len(rec) < 7 {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(rec[1]))
		if err != nil {
			continue
		}
		p := ProcInfo{
			PID:            pid,
			Name:           imageBase(rec[0]),
			Cmdline:        rec[0],
			User:           rec[6],
			State:          shortStatus(rec[5]),
			RSSBytes:       parseMemUsage(rec[4]),
			CPUPercent:     CPUPercentUnknown,
			MetricsUnknown: true, // no live CPU% / thread count from tasklist
		}
		procs = append(procs, p)
	}
	return procs, nil
}

// parseMemUsage turns a "12,345 K" mem-usage string into bytes.
func parseMemUsage(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, " K")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "") // non-breaking space thousands sep
	kib, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return kib * 1024
}

func imageBase(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	if base == "" {
		return "?"
	}
	return base
}

func shortStatus(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "?"
	}
	return s[:1]
}
