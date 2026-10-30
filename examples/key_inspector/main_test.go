package main

import (
	"runtime"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Width:  760,
		Height: 640,
	})
	_ = mainView(w).GenerateLayout(w)
	// With a key recorded, as after a press.
	onEvent(&gui.Event{Type: gui.EventKeyDown, KeyCode: gui.Key7, Modifiers: gui.ModShift,
		TypedKey: gui.KeySlash}, w)
	onEvent(&gui.Event{Type: gui.EventChar, CharCode: '/', IMEText: "/"}, w)
	_ = mainView(w).GenerateLayout(w)
	if got := len(gui.State[App](w).Lines); got != 2 {
		t.Errorf("history has %d lines, want 2", got)
	}
}

// The lines a German Shift+7 and AltGr+8 produce (#1188).
func TestEventLineAndBindAs(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("shortcut labels use macOS glyphs there")
	}
	shift7 := gui.Event{Type: gui.EventKeyDown, KeyCode: gui.Key7, Modifiers: gui.ModShift,
		TypedKey: gui.KeySlash, TypedModifiers: gui.ModNone}
	if got := eventLine(&shift7); !strings.Contains(got, `KeyCode=55 "7"  mods=Shift  typed=47 "/"  typedMods=none`) {
		t.Errorf("eventLine = %q", got)
	}
	if got, want := bindAs(shift7), "Shift+7   or   /"; got != want {
		t.Errorf("bindAs = %q, want %q", got, want)
	}
	altGr8 := gui.Event{Type: gui.EventKeyDown, KeyCode: gui.Key8, Modifiers: gui.ModAltGr,
		TypedKey: gui.KeyLeftBracket}
	if got, want := bindAs(altGr8), "AltGr+8   or   ["; got != want {
		t.Errorf("bindAs = %q, want %q", got, want)
	}
	if got, want := bindAs(gui.Event{}), "no KeyCode: this key cannot be bound"; got != want {
		t.Errorf("bindAs = %q, want %q", got, want)
	}
	shift := gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyLeftShift, TypedKey: gui.KeyLeftShift}
	if got, want := bindAs(shift), "LeftShift: a modifier, held with another key"; got != want {
		t.Errorf("bindAs = %q, want %q", got, want)
	}
	if got := keyName(gui.KeyRightAlt); got != `"RightAlt"` {
		t.Errorf("keyName(KeyRightAlt) = %s", got)
	}
	char := gui.Event{Type: gui.EventChar, CharCode: 'ä', IMEText: "ä"}
	if got := eventLine(&char); got != `char  "ä" U+00E4  mods=none` {
		t.Errorf("eventLine = %q", got)
	}
}
