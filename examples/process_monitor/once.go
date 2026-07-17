package main

import (
	"fmt"
	"os"
	"sort"
)

// runOnce prints a single sorted process report to stdout and exits — a quick
// headless check that needs no window. It relies on the collector's snapshot
// CPU% (ps pcpu on Unix), so no second sample is required.
func runOnce(limit int, sortBy string) {
	snap, err := Collect()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sample:", err)
		os.Exit(1)
	}

	procs := append([]ProcInfo(nil), snap.Processes...)
	sortProcInfos(procs, sortBy)

	if limit > len(procs) {
		limit = len(procs)
	}
	if limit < 0 {
		limit = 0
	}

	fmt.Printf("Top %d processes by %s\n", limit, sortBy)
	if snap.TotalMemoryBytes > 0 {
		fmt.Printf("Memory: %s used / %s total\n",
			formatBytes(snap.UsedMemoryBytes), formatBytes(snap.TotalMemoryBytes))
	}
	fmt.Println()
	fmt.Printf("%-7s %7s %9s %7s %-12s %-5s %5s %s\n",
		"PID", "CPU%", "RSS", "MEM%", "USER", "STATE", "THR", "NAME")
	for _, p := range procs[:limit] {
		fmt.Printf("%-7d %7s %9s %7s %-12.12s %-5s %5s %s\n",
			p.PID, p.CPUText(), p.RSSText(), p.MemText(),
			p.User, p.State, p.ThreadsText(), truncate(nameOr(p), 80))
	}
}

// nameOr falls back to the command line when the name is empty.
func nameOr(p ProcInfo) string {
	if p.Name != "" {
		return p.Name
	}
	return p.Cmdline
}

// sortProcInfos sorts snapshot rows for -once. High-to-low for the numeric
// metrics, alphabetical for the text columns, with a PID tiebreak.
func sortProcInfos(procs []ProcInfo, sortBy string) {
	switch sortColumnIndex(sortBy) {
	case colPID:
		sort.SliceStable(procs, func(i, j int) bool { return procs[i].PID < procs[j].PID })
	case colRSS:
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].RSSBytes != procs[j].RSSBytes {
				return procs[i].RSSBytes > procs[j].RSSBytes
			}
			return procs[i].PID < procs[j].PID
		})
	case colUser:
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].User != procs[j].User {
				return procs[i].User < procs[j].User
			}
			return procs[i].PID < procs[j].PID
		})
	case colState:
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].State != procs[j].State {
				return procs[i].State < procs[j].State
			}
			return procs[i].PID < procs[j].PID
		})
	case colThreads:
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].Threads != procs[j].Threads {
				return procs[i].Threads > procs[j].Threads
			}
			return procs[i].PID < procs[j].PID
		})
	case colName:
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].Name != procs[j].Name {
				return procs[i].Name < procs[j].Name
			}
			return procs[i].PID < procs[j].PID
		})
	default: // colCPU
		sort.SliceStable(procs, func(i, j int) bool {
			if procs[i].CPUPercent != procs[j].CPUPercent {
				return procs[i].CPUPercent > procs[j].CPUPercent
			}
			return procs[i].PID < procs[j].PID
		})
	}
}
