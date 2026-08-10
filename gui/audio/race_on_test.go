//go:build race && !js && !android && !ios

package audio

// raceEnabled reports whether the test binary was built with -race.
const raceEnabled = true
