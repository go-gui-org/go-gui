//go:build ios

// This example demonstrates go-gui on iOS (advanced: iOS).
// Command ios_demo is an iOS demo app for go-gui.
// Compiled as a c-archive and linked into a native Xcode project.
package main

import (
	"fmt"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ios"
)

type App struct {
	Clicks   int
	Name     string
	Password string
	Amount   string
	// Fullscreen is what the app asked for. IsFullscreen can differ:
	// a windowed iPad grants it only once the window fills the
	// screen.
	Fullscreen bool
	// Set by onEvent from EventVisibilityChanged.
	hiddenAt time.Time
	returns  int
	away     string
}

// iosWindow holds the window Init created; a repeat Init call reuses
// it so the first window never leaks.
var iosWindow *gui.Window

// Init creates the window and hands it to the backend. Called once
// from init so the linked archive still initializes on load; the
// host can also call it directly.
func Init() {
	if iosWindow != nil {
		return
	}
	w := gui.NewWindow(gui.WindowCfg{
		State: &App{},
		OnInit: func(w *gui.Window) {
			w.OnEvent = onEvent
			w.SetView(view)
		},
	})
	ios.SetWindow(w)
	iosWindow = w
}

// onEvent shows that the app learns when it leaves the screen (issue
// #1170): it counts the returns and how long the last absence was. A
// player would stop decoding and pause its audio on the way out.
func onEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventVisibilityChanged {
		return
	}
	app := gui.State[App](w)
	if w.IsOccluded() {
		app.hiddenAt = time.Now()
		return
	}
	app.returns++
	app.away = fmt.Sprintf("Left the screen %d times, last for %s",
		app.returns, time.Since(app.hiddenAt).Round(100*time.Millisecond))
}

func init() {
	Init()
}

// awayText is the readout onEvent fills, or a hint before the first
// return.
func awayText(app *App) string {
	if app.away == "" {
		return "Leave the app and come back."
	}
	return app.away
}

func view(w *gui.Window) gui.View {
	app := gui.State[App](w)
	// The framework moves no layout for the notch, the home bar or the
	// soft keyboard. This demo opts in: it pads by the safe area and by
	// the keyboard height. The safe area's bottom already gives way to
	// the keyboard, so the two add up without counting the home bar
	// twice.
	safe := w.SafeAreaPadding()

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Padding: gui.NewPadding(16+safe.Top, 16+safe.Right,
			16+safe.Bottom+w.SoftKeyboardInset(), 16+safe.Left),
		Content: []gui.View{
			// Theme intentionally unpinned: the demo follows the host
			// default (web_demo pins dark for its look).
			gui.Text(gui.TextCfg{
				Text:      "Go-Gui on iOS",
				TextStyle: gui.CurrentTheme().TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text: "Tap the button to increment.",
			}),
			gui.Text(gui.TextCfg{
				Text:      awayText(app),
				TextStyle: gui.CurrentTheme().TextStyleSecondary,
			}),
			gui.Button(gui.ButtonCfg{
				ID: gui.ScopeID("ios-demo", "click"),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks",
							app.Clicks),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Clicks++
					ctx.Consume()
				},
			}),
			fullscreenButton(w),
			// Each field asks for its own soft keyboard (issue #806).
			gui.Input(gui.InputCfg{
				ID:          gui.ScopeID("ios-demo", "name"),
				A11YCfg:     gui.A11YCfg{A11YLabel: "Name"},
				Placeholder: "Name",
				Text:        app.Name,
				Width:       260,
				Sizing:      gui.FixedFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Name = s
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          gui.ScopeID("ios-demo", "password"),
				A11YCfg:     gui.A11YCfg{A11YLabel: "Password"},
				Placeholder: "Password",
				Text:        app.Password,
				IsPassword:  true,
				Width:       260,
				Sizing:      gui.FixedFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Password = s
				},
			}),
			gui.Input(gui.InputCfg{
				ID:          gui.ScopeID("ios-demo", "amount"),
				A11YCfg:     gui.A11YCfg{A11YLabel: "Amount"},
				Placeholder: "Amount",
				Text:        app.Amount,
				Keyboard:    gui.KeyboardDecimal,
				Width:       260,
				Sizing:      gui.FixedFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Amount = s
				},
			}),
		},
	})
}

// fullscreenButton hides the status bar and lets the home indicator
// fade (issue #1171). It toggles the app's own request, not
// IsFullscreen: on a windowed iPad IsFullscreen stays false, and a
// toggle of it would ask again on every tap and never take the request
// back.
func fullscreenButton(w *gui.Window) gui.View {
	label := "Fullscreen"
	if gui.State[App](w).Fullscreen {
		label = "Leave fullscreen"
	}
	return gui.Button(gui.ButtonCfg{
		ID:      gui.ScopeID("ios-demo", "fullscreen"),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
		OnClick: func(ctx gui.EventCtx) {
			app := gui.State[App](ctx.Window)
			app.Fullscreen = !app.Fullscreen
			ctx.Window.SetFullscreen(app.Fullscreen)
			ctx.Consume()
		},
	})
}

func main() {}
