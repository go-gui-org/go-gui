//go:build ios

// This example demonstrates go-gui on iOS (advanced: iOS).
// Command ios_demo is an iOS demo app for go-gui.
// Compiled as a c-archive and linked into a native Xcode project.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/ios"
)

type App struct {
	Clicks int
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
			w.SetView(view)
		},
	})
	ios.SetWindow(w)
	iosWindow = w
}

func init() {
	Init()
}

func view(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
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
				},
			}),
		},
	})
}

func main() {}
