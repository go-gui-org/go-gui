//go:build !race && darwin && !ios

package sysbeep

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = false
