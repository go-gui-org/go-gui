//go:build !darwin && !linux

package main

// systemMemory has no cheap portable source on other platforms, so it reports
// zero. The header treats (0, 0) as "unknown" and omits the memory bar.
func systemMemory() (total, used uint64) {
	return 0, 0
}
