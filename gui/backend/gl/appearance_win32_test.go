//go:build windows && !js

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// resetWinAppearance clears the shared state and stubs the registry
// query with the given reading for the rest of the test.
func resetWinAppearance(t *testing.T, a gui.Appearance, ok bool) {
	t.Helper()
	winAppearanceMu.Lock()
	winAppearanceCBs = make(map[*nativePlatform]func(gui.Appearance))
	winAppearanceKnown = false
	winAppearanceMu.Unlock()
	old := winAppearanceQuery
	winAppearanceQuery = func() (gui.Appearance, bool) { return a, ok }
	t.Cleanup(func() { winAppearanceQuery = old })
}

func TestNoteSystemAppearanceFansOutOnFlip(t *testing.T) {
	// No setting at subscribe time: nothing is seeded, so the
	// first reading counts as a flip.
	resetWinAppearance(t, gui.AppearanceLight, false)

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

// Regression: the first WM_SETTINGCHANGE after subscribe fanned out
// even when the setting had not changed, because nothing seeded the
// last-known value. Any setting change broadcasts the message.
func TestSubscribeSeedsLastAppearance(t *testing.T) {
	resetWinAppearance(t, gui.AppearanceDark, true)

	n := &nativePlatform{}
	var got []gui.Appearance
	n.SetSystemAppearanceCallback(func(a gui.Appearance) { got = append(got, a) })
	defer n.SetSystemAppearanceCallback(nil)

	systemAppearanceChanged() // unrelated setting: still dark
	if len(got) != 0 {
		t.Fatalf("callback fired with no flip: %v", got)
	}

	winAppearanceQuery = func() (gui.Appearance, bool) { return gui.AppearanceLight, true }
	systemAppearanceChanged()
	if len(got) != 1 || got[0] != gui.AppearanceLight {
		t.Errorf("callbacks: got %v, want [light]", got)
	}
}
