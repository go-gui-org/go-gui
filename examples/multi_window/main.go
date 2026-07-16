// Multi_window demonstrates multi-window support: two windows
// with independent state, cross-window communication, and
// runtime window creation.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type MainState struct {
	Clicks int
}

type InspectorState struct {
	Log string
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	app := gui.NewApp()
	app.ExitMode = gui.ExitOnMainClose

	w1 := gui.NewWindow(gui.WindowCfg{
		State:  &MainState{},
		Title:  "Main Window",
		Width:  400,
		Height: 300,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	w2 := gui.NewWindow(gui.WindowCfg{
		State:  &InspectorState{Log: "Ready.\n"},
		Title:  "Inspector",
		Width:  300,
		Height: 200,
		OnInit: func(w *gui.Window) {
			w.UpdateView(inspectorView)
		},
	})

	backend.RunApp(app, w1, w2)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[MainState](w)

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Main Window",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				ID:        "mw_open_child",
				Focusable: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf(
							"Clicked %d times", app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event,
					w *gui.Window) {
					gui.State[MainState](w).Clicks++
					// Broadcast to inspector.
					if a := w.App(); a != nil {
						a.Broadcast(func(other *gui.Window) {
							if other == w {
								return
							}
							other.QueueCommand(
								func(o *gui.Window) {
									s := gui.State[InspectorState](o)
									s.Log += fmt.Sprintf(
										"Click #%d\n",
										gui.State[MainState](w).Clicks)
									o.UpdateWindow()
								})
						})
					}
					e.IsHandled = true
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:        "mw_close_child",
				Focusable: true,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "Open New Window",
					}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event,
					w *gui.Window) {
					if a := w.App(); a != nil {
						a.OpenWindow(gui.WindowCfg{
							State: &InspectorState{
								Log: "New window opened.\n",
							},
							Title:  "Dynamic Window",
							Width:  250,
							Height: 150,
							OnInit: func(w *gui.Window) {
								w.UpdateView(inspectorView)
							},
						})
					}
					e.IsHandled = true
				},
			}),
		},
	})
}

func inspectorView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	state := gui.State[InspectorState](w)

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.Some(gui.PadAll(8)),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Event Log",
				TextStyle: gui.CurrentTheme().B2,
			}),
			gui.Text(gui.TextCfg{
				Text: state.Log,
			}),
		},
	})
}
