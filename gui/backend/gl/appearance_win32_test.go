//go:build windows && !js

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestNoteSystemAppearanceFansOutOnFlip(t *testing.T) {
	winAppearanceMu.Lock()
	winAppearanceCBs = make(map[*nativePlatform]func(gui.Appearance))
	winAppearanceKnown = false
	winAppearanceMu.Unlock()

	n := &nativePlatform{}
	var got []gui.Appearance
	n.SetSystemAppearanceCallback(func(a gui.Appearance) { got = append(got, a) })

	noteSystemAppearance(gui.AppearanceDark, true)
	noteSystemAppearance(gui.AppearanceDark, true) // no flip: silent
	noteSystemAppearance(gui.AppearanceLight, false)
	noteSystemAppearance(gui.AppearanceLight, true)

	if len(got) != 2 || got[0] != gui.AppearanceDark || got[1] != gui.AppearanceLight {
		t.Errorf("callbacks: got %v, want [dark light]", got)
	}

	n.SetSystemAppearanceCallback(nil)
	noteSystemAppearance(gui.AppearanceDark, true)
	if len(got) != 2 {
		t.Errorf("callback fired after unregister: %v", got)
	}
}
