//go:build !race && linux && !android && (amd64 || arm64)

package wl

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = false
