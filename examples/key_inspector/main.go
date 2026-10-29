// This example shows every key event the window receives: the key a press
// reports, the key it typed, the modifiers, the character, and the
// shortcut to bind it with. Use it to check a keyboard layout, or to
// report what go-gui sees on yours (each line is also printed to stdout).
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// maxLines caps the event history.
const maxLines = 40

// keyboardMods are the Modifiers bits a keyboard sets; the rest are mouse
// buttons.
const keyboardMods = gui.ModShift | gui.ModCtrl | gui.ModAlt | gui.ModAltGr | gui.ModSuper

// App holds the last key down, the last character and the history.
type App struct {
	Last     gui.Event
	HasLast  bool
	LastChar string
	Lines    []string // newest first
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:   &App{},
		Title:   "Key Inspector",
		Width:   760,
		Height:  640,
		OnEvent: onEvent,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
	})
	if *screenshot != "" {
		replaySample(w)
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	log.Print(sessionLine())
	backend.Run(w)
}

// replaySample feeds the events of Shift+7 and AltGr+8 on a German layout,
// for the screenshot.
func replaySample(w *gui.Window) {
	for _, e := range []gui.Event{
		{Type: gui.EventKeyDown, KeyCode: gui.Key7, Modifiers: gui.ModShift, TypedKey: gui.KeySlash},
		{Type: gui.EventChar, CharCode: '/', IMEText: "/", Modifiers: gui.ModShift},
		{Type: gui.EventKeyUp, KeyCode: gui.Key7, Modifiers: gui.ModShift, TypedKey: gui.KeySlash},
		{Type: gui.EventKeyDown, KeyCode: gui.Key8, Modifiers: gui.ModAltGr, TypedKey: gui.KeyLeftBracket},
		{Type: gui.EventChar, CharCode: '[', IMEText: "[", Modifiers: gui.ModAltGr},
	} {
		onEvent(&e, w)
	}
}

// onEvent records key and character events. The window has nothing
// focusable, so every key but F12 (the inspector) arrives here unhandled.
func onEvent(e *gui.Event, w *gui.Window) {
	switch e.Type {
	case gui.EventKeyDown, gui.EventKeyUp, gui.EventChar:
	default:
		return
	}
	app := gui.State[App](w)
	line := eventLine(e)
	log.Print(line)
	if e.Type == gui.EventKeyDown {
		log.Print("      bind as: " + bindAs(*e)) // the title line
	}
	app.Lines = append([]string{line}, app.Lines...)
	if len(app.Lines) > maxLines {
		app.Lines = app.Lines[:maxLines]
	}
	switch e.Type {
	case gui.EventKeyDown:
		app.Last, app.HasLast, app.LastChar = *e, true, ""
	case gui.EventChar:
		app.LastChar = charText(e)
	}
}

// eventLine is one history line: a key down or up, or a typed character.
func eventLine(e *gui.Event) string {
	switch e.Type {
	case gui.EventKeyDown, gui.EventKeyUp:
		kind := "down"
		if e.Type == gui.EventKeyUp {
			kind = "up  "
		}
		s := fmt.Sprintf("%s  KeyCode=%d %s  mods=%s  typed=%d %s  typedMods=%s",
			kind, e.KeyCode, keyName(e.KeyCode), modNames(e.Modifiers),
			e.TypedKey, keyName(e.TypedKey), modNames(e.TypedModifiers))
		if e.KeyRepeat {
			s += "  repeat"
		}
		return s
	case gui.EventChar:
		return fmt.Sprintf("char  %s  mods=%s", charText(e), modNames(e.Modifiers))
	}
	return ""
}

// modifierKeyNames names the keys Shortcut labels leave out: nobody binds
// a shortcut to a lone modifier, but the inspector shows them.
var modifierKeyNames = map[gui.KeyCode]string{
	gui.KeyLeftShift: "LeftShift", gui.KeyRightShift: "RightShift",
	gui.KeyLeftControl: "LeftCtrl", gui.KeyRightControl: "RightCtrl",
	gui.KeyLeftAlt: "LeftAlt", gui.KeyRightAlt: "RightAlt",
	gui.KeyLeftSuper: "LeftSuper", gui.KeyRightSuper: "RightSuper",
	gui.KeyCapsLock: "CapsLock",
}

// keyName is a key's display name, quoted, or "none".
func keyName(k gui.KeyCode) string {
	if k == gui.KeyInvalid {
		return "none"
	}
	if name, ok := modifierKeyNames[k]; ok {
		return fmt.Sprintf("%q", name)
	}
	return fmt.Sprintf("%q", gui.Shortcut{Key: k}.String())
}

// modNames lists the keyboard modifiers in m, or "none".
func modNames(m gui.Modifier) string {
	var names []string
	for _, mod := range []struct {
		bit  gui.Modifier
		name string
	}{
		{gui.ModCtrl, "Ctrl"}, {gui.ModAlt, "Alt"}, {gui.ModAltGr, "AltGr"},
		{gui.ModShift, "Shift"}, {gui.ModSuper, "Super"},
	} {
		if m.Has(mod.bit) {
			names = append(names, mod.name)
		}
	}
	if len(names) == 0 {
		return "none"
	}
	return strings.Join(names, "+")
}

// charText is a typed character with its code point.
func charText(e *gui.Event) string {
	text := e.IMEText
	if text == "" {
		text = string(rune(e.CharCode))
	}
	return fmt.Sprintf("%q U+%04X", text, e.CharCode)
}

// bindAs is how to bind the last key down as a Shortcut: by the key it
// reports, and by the key it typed when that differs.
func bindAs(e gui.Event) string {
	if e.KeyCode == gui.KeyInvalid && e.TypedKey == gui.KeyInvalid {
		return "no KeyCode: this key cannot be bound"
	}
	if name, ok := modifierKeyNames[e.KeyCode]; ok {
		return name + ": a modifier, held with another key"
	}
	exact := gui.Shortcut{Key: e.KeyCode, Modifiers: e.Modifiers & keyboardMods}
	typed := gui.Shortcut{Key: e.TypedKey, Modifiers: e.TypedModifiers & keyboardMods}
	switch {
	case e.KeyCode == gui.KeyInvalid:
		return typed.String()
	case e.TypedKey == gui.KeyInvalid || typed == exact:
		return exact.String()
	}
	return exact.String() + "   or   " + typed.String()
}

// sessionLine says which display the window uses on Linux, from the same
// variables the backend reads.
func sessionLine() string {
	if runtime.GOOS != "linux" {
		return "platform: " + runtime.GOOS
	}
	wl, x := os.Getenv("WAYLAND_DISPLAY"), os.Getenv("DISPLAY")
	switch {
	case wl != "" && os.Getenv("GOGUI_WAYLAND") != "0":
		return fmt.Sprintf("display: Wayland (WAYLAND_DISPLAY=%s); GOGUI_WAYLAND=0 runs it on X11", wl)
	case x != "":
		return fmt.Sprintf("display: X11 (DISPLAY=%s)", x)
	}
	return "display: unknown (neither WAYLAND_DISPLAY nor DISPLAY is set)"
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	th := gui.CurrentTheme()

	last := []gui.View{
		gui.Text(gui.TextCfg{Text: "Press any key or key combination. F12 opens the inspector.",
			TextStyle: th.TextStyleSecondary}),
		gui.Text(gui.TextCfg{Text: sessionLine(), TextStyle: th.TextStyleSecondary}),
	}
	if app.HasLast {
		e := app.Last
		last = append(last,
			gui.Text(gui.TextCfg{Text: bindAs(e), TextStyle: th.TextStyleTitle}),
			gui.Text(gui.TextCfg{TextStyle: th.TextStyleCode, Text: fmt.Sprintf(
				"key        %s (KeyCode %d), modifiers %s",
				keyName(e.KeyCode), e.KeyCode, modNames(e.Modifiers))}),
			gui.Text(gui.TextCfg{TextStyle: th.TextStyleCode, Text: fmt.Sprintf(
				"typed key  %s (TypedKey %d), modifiers left %s",
				keyName(e.TypedKey), e.TypedKey, modNames(e.TypedModifiers))}),
			gui.Text(gui.TextCfg{TextStyle: th.TextStyleCode, Text: "character  " +
				orNone(app.LastChar)}),
		)
	}

	history := make([]gui.View, 0, len(app.Lines))
	for _, l := range app.Lines {
		history = append(history, gui.Text(gui.TextCfg{Text: l, TextStyle: th.TextStyleCodeSmall}))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(16),
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SpacingSmall,
				Content: last,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Spacing: gui.SpacingSmall,
				Clip:    true,
				Content: history,
			}),
		},
	})
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
