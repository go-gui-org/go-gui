//go:build race && darwin && !ios

package sysbeep

// raceEnabled reports whether the test binary was built with -race. An
// allocation budget measured without it does not hold with it: under
// -race, a cue sometimes counts one allocation from the race runtime.
const raceEnabled = true
