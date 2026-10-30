//go:build race && linux && !js && !android && (amd64 || arm64)

package gl

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = true
