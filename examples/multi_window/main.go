// This example demonstrates multi-window support: two windows with independent state and cross-window messaging (advanced: multi-window).
// Multi_window demonstrates multi-window support: two windows
// with independent state, cross-window communication, and
// runtime window creation.
package main

import (
	"fmt"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type MainState struct {
	Clicks int
}

type InspectorState struct {
	Log string
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
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

	if *screenshot != "" {
		if err := soft.RenderToPNG(w1, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.RunApp(app, w1, w2)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[MainState](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SomeF(8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Main Window",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "mw_open_child",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf(
							"Clicked %d times", app.Clicks),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					gui.State[MainState](ctx.Window).Clicks++
					// Broadcast to inspector.
					if a := ctx.Window.App(); a != nil {
						a.Broadcast(func(other *gui.Window) {
							if other == ctx.Window {
								return
							}
							other.QueueCommand(
								func(o *gui.Window) {
									s := gui.State[InspectorState](o)
									s.Log += fmt.Sprintf(
										"Click #%d\n",
										gui.State[MainState](ctx.Window).Clicks)
									o.UpdateWindow()
								})
						})
					}
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID: "mw_close_child",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: "Open New Window",
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					if a := ctx.Window.App(); a != nil {
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
				},
			}),
		},
	})
}

func inspectorView(w *gui.Window) gui.View {
	state := gui.State[InspectorState](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(8),

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
