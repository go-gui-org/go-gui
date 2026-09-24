// This example demonstrates multi-window support: two windows with independent state and cross-window messaging (advanced: multi-window).
// Multi_window demonstrates multi-window support: two windows
// with independent state, cross-window communication, and
// runtime window creation.
package main

import (
	"fmt"
	"strings"

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

// maxInspectorLogLines caps the inspector event log: the demo appends
// one line per broadcast click, so an unattended run would otherwise
// grow it without bound.
const maxInspectorLogLines = 200

// trimLogLines keeps the last n lines of a newline-terminated log.
func trimLogLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
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
			w.SetView(mainView)
		},
	})

	w2 := gui.NewWindow(gui.WindowCfg{
		State:  &InspectorState{Log: "Ready.\n"},
		Title:  "Inspector",
		Width:  300,
		Height: 200,
		OnInit: func(w *gui.Window) {
			w.SetView(inspectorView)
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
		// Structural wrapper: an unset border still reserves height.
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Main Window",
				TextStyle: gui.CurrentTheme().TextStyleDisplay,
			}),
			gui.Button(gui.ButtonCfg{
				ID: "mw_broadcast_clicks",
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf(
							"Clicked %d times", app.Clicks),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					src := ctx.Window
					srcState := gui.State[MainState](src)
					srcState.Clicks++
					// Snapshot for the queued closure below:
					// it runs later, on another window, where
					// the event-scoped ctx is long gone.
					clicks := srcState.Clicks
					// Broadcast to inspector.
					if a := src.App(); a != nil {
						a.Broadcast(func(other *gui.Window) {
							if other == src {
								return
							}
							other.QueueCommand(
								func(o *gui.Window) {
									s := gui.State[InspectorState](o)
									s.Log += fmt.Sprintf(
										"Click #%d\n", clicks)
									s.Log = trimLogLines(s.Log,
										maxInspectorLogLines)
									o.InvalidateLayout()
								})
						})
					}
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID: "mw_open_child",
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
								w.SetView(inspectorView)
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
				TextStyle: gui.CurrentTheme().TextStyleTitle,
			}),
			gui.Text(gui.TextCfg{
				Text: state.Log,
			}),
		},
	})
}
