//go:build race && windows && !js

package gl

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = true
