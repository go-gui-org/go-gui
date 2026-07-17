//go:build linux

package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// systemMemory reads total and used RAM from /proc/meminfo. Used is derived as
// MemTotal - MemAvailable, matching what tools like free(1) report. On any
// error it returns (0, 0) and the header simply omits the memory bar.
func systemMemory() (total, used uint64) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer func() { _ = f.Close() }()

	var memTotal, memAvail uint64
	var haveTotal, haveAvail bool
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "MemTotal:"):
			memTotal, haveTotal = meminfoBytes(line)
		case strings.HasPrefix(line, "MemAvailable:"):
			memAvail, haveAvail = meminfoBytes(line)
		}
		if haveTotal && haveAvail {
			break
		}
	}
	if !haveTotal || !haveAvail || memAvail > memTotal {
		return 0, 0
	}
	return memTotal, memTotal - memAvail
}

// meminfoBytes parses a "Key:   12345 kB" line into bytes.
func meminfoBytes(line string) (uint64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0, false
	}
	kib, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, false
	}
	return kib * 1024, true
}
