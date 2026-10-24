//go:build !js && !android && !ios

package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui/audio"
)

// TestAmbienceTrackDecodes guards the embedded track. A file the
// decoder rejects only logs at startup, so a bad conversion would ship
// a silent orrery without failing anything else.
func TestAmbienceTrackDecodes(t *testing.T) {
	m, err := audio.LoadMusicBytes(ambienceFLAC)
	if err != nil {
		t.Fatalf("embedded ambience does not decode: %v", err)
	}
	m.Free()
}
