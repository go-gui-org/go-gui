// This example demonstrates an app-drawn PIN pad that types into a focused Input
// while the OS soft keyboard stays down (issue #770).
//
// Two things make it work, and neither is special keypad code:
//
//   - The field sets Keyboard: gui.KeyboardNone. It keeps focus, caret and
//     input method, but a touch platform does not open its own keyboard over
//     the pad.
//   - Each key is a FocusDisabled button. A press that a non-focusable widget
//     consumes leaves focus where it is, so the field stays focused.
//
// A key sends the same events a physical key sends (EventChar for a digit,
// EventKeyDown for Backspace), so masking, undo, the caret and OnTextChanged
// all behave as they do for typing.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// pinLength is how many digits the PIN takes.
const pinLength = 6

// App holds the typed PIN and the last submitted one.
type App struct {
	PIN       string
	Submitted string
}

// fieldID is the PIN field's effective ID. It has no ID-bearing
// ancestor, so the leaf is the effective ID.
var fieldID = gui.ScopeID("pin", "field")

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	w := gui.SimpleWindow("PIN Pad", 320, 480, &App{}, func(w *gui.Window) {
		w.SetView(mainView)
		// Focus the field at start: the pad types into whatever
		// field holds focus.
		w.SetFocus(fieldID)
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	status := "Enter your PIN"
	if app.Submitted != "" {
		status = fmt.Sprintf("Submitted %d digits", len(app.Submitted))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(12),
		// Keep the pad above an OS keyboard if one is ever shown (a
		// physical keyboard attached, another field focused). The
		// framework reports the inset; the app decides what to move.
		Padding: gui.NewPadding(16, 16, 16+w.SoftKeyboardInset(), 16),
		Content: []gui.View{
			gui.Label(status, theme.TextStyleSecondary),
			gui.Input(gui.InputCfg{
				ID:         fieldID,
				A11YCfg:    gui.A11YCfg{A11YLabel: "PIN"},
				Text:       app.PIN,
				IsPassword: true,
				// The OS keyboard stays down; the pad below types.
				Keyboard: gui.KeyboardNone,
				Width:    3*64 + 2*8, // the pad width: three keys, two gaps
				Sizing:   gui.FixedFit,
				PreTextChange: func(_, proposed string) (string, bool) {
					return proposed, validPIN(proposed)
				},
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).PIN = s
				},
				OnEnter: submit,
			}),
			keypad(),
		},
	})
}

// validPIN reports whether s is digits only and no longer than a PIN.
func validPIN(s string) bool {
	if len(s) > pinLength {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// submit records the PIN and clears the field.
func submit(ctx gui.EventCtx) {
	app := gui.State[App](ctx.Window)
	app.Submitted = app.PIN
	app.PIN = ""
	ctx.Consume()
}

// keypad lays the keys out in four rows of three.
func keypad() gui.View {
	rows := [][]string{
		{"1", "2", "3"},
		{"4", "5", "6"},
		{"7", "8", "9"},
		{"⌫", "0", "OK"},
	}
	views := make([]gui.View, 0, len(rows))
	for _, row := range rows {
		keys := make([]gui.View, 0, len(row))
		for _, label := range row {
			keys = append(keys, key(label))
		}
		views = append(views, gui.Row(gui.ContainerCfg{
			Spacing:    gui.SomeF(8),
			Padding:    gui.NoPadding,
			SizeBorder: gui.NoBorder,
			Content:    keys,
		}))
	}
	return gui.Column(gui.ContainerCfg{
		ID:         "pin:pad",
		Spacing:    gui.SomeF(8),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content:    views,
	})
}

// key builds one keypad button. It is FocusDisabled, so pressing it
// leaves the PIN field focused.
func key(label string) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:            gui.ScopeID("key", keyName(label)),
		FocusDisabled: true,
		A11YCfg:       gui.A11YCfg{A11YLabel: keyA11YLabel(label)},
		Width:         64,
		Height:        48,
		Sizing:        gui.FixedFixed,
		Content:       []gui.View{gui.Text(gui.TextCfg{Text: label})},
		OnClick: func(ctx gui.EventCtx) {
			ctx.Consume()
			// Send the key after this press has been dispatched.
			// QueueCommand runs at the start of the next frame, on
			// the main thread with no lock held, like a backend
			// event.
			ctx.Window.QueueCommand(func(w *gui.Window) {
				w.EventFn(keyEvent(label))
			})
		},
	})
}

// keyEvent returns the event a physical key with this label sends.
func keyEvent(label string) *gui.Event {
	switch label {
	case "⌫":
		return &gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyBackspace}
	case "OK":
		return &gui.Event{Type: gui.EventKeyDown, KeyCode: gui.KeyEnter}
	}
	return &gui.Event{Type: gui.EventChar, CharCode: uint32(label[0])}
}

// keyName returns an ID-safe part for a key label.
func keyName(label string) string {
	switch label {
	case "⌫":
		return "backspace"
	case "OK":
		return "ok"
	}
	return label
}

// keyA11YLabel returns the spoken name of a key.
func keyA11YLabel(label string) string {
	switch label {
	case "⌫":
		return "Delete"
	case "OK":
		return "Submit"
	}
	return label
}
