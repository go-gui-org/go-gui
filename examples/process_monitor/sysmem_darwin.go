//go:build darwin

package main

import (
	"os/exec"
	"strconv"
	"strings"
)

// systemMemory reports total and used RAM on macOS. Total comes from
// `sysctl hw.memsize`; used is total minus an approximation of available
// memory built from `vm_stat` page counts. On any failure it returns (0, 0)
// and the header omits the memory bar rather than showing a fake value.
func systemMemory() (total, used uint64) {
	total = sysctlUint("hw.memsize")
	if total == 0 {
		return 0, 0
	}
	avail := availableMemory()
	if avail == 0 || avail > total {
		return 0, 0
	}
	return total, total - avail
}

// sysctlUint runs `sysctl -n <name>` and parses the result as a uint64.
func sysctlUint(name string) uint64 {
	// #nosec G204 — name is a constant.
	out, err := exec.Command("sysctl", "-n", name).Output()
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// availableMemory sums the reclaimable page classes reported by vm_stat
// (free + inactive + speculative + purgeable) and multiplies by the page size.
// This is an approximation adequate for the header's usage bar.
func availableMemory() uint64 {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}
	pageSize := uint64(4096)
	var freePages uint64
	for line := range strings.SplitSeq(string(out), "\n") {
		if i := strings.Index(line, "page size of"); i >= 0 {
			// "...(page size of 16384 bytes)"
			if n := firstUint(line[i:]); n > 0 {
				pageSize = n
			}
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Pages free", "Pages inactive", "Pages speculative",
			"Pages purgeable":
			freePages += parseVMStatPages(val)
		}
	}
	return freePages * pageSize
}

// parseVMStatPages parses a vm_stat count field like "  123456." into a uint.
func parseVMStatPages(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".")
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

// firstUint extracts the first run of digits in s as a uint64.
func firstUint(s string) uint64 {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' {
			start = i
			break
		}
	}
	if start < 0 {
		return 0
	}
	end := start
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	v, _ := strconv.ParseUint(s[start:end], 10, 64)
	return v
}
