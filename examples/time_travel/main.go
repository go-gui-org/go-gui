// This example demonstrates time-travel debugging: a counter app with a state scrubber window (advanced: time travel).
// The time_travel example demonstrates time-travel debugging.
// A small counter app opts into DebugTimeTravel; the framework
// auto-spawns a scrubber window with a slider, step buttons,
// and keyboard shortcuts (arrows, home/end, space, esc) so the
// user can rewind and replay state.
//
// State is captured after every event; scrolling back through
// the timeline restores the counter and input to their prior
// values while the app window freezes input.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type appState struct {
	Log   []string
	Count int
}

// Snapshot deep-copies the state into a fresh instance so the
// time-travel ring holds an independent value per entry.
func (s *appState) Snapshot() any {
	return &appState{
		Count: s.Count,
		Log:   slices.Clone(s.Log),
	}
}

// Restore overwrites the receiver from a prior Snapshot.
func (s *appState) Restore(v any) {
	src := v.(*appState)
	s.Count = src.Count
	s.Log = slices.Clone(src.Log)
}

// Size approximates heap cost so byte-cap eviction behaves
// reasonably with a growing Log.
func (s *appState) Size() int {
	total := 32
	for _, line := range s.Log {
		total += len(line) + 16
	}
	return total
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeLight.WithPadding(false))

	app := gui.NewApp()
	app.ExitMode = gui.ExitOnMainClose

	main := gui.NewWindow(gui.WindowCfg{
		State:           &appState{},
		Title:           "Counter",
		Width:           320,
		Height:          220,
		DebugTimeTravel: true,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(main, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.RunApp(app, main)
}

func mainView(w *gui.Window) gui.View {
	s := gui.State[appState](w)
	return gui.Column(gui.ContainerCfg{
		Padding: gui.PadAll(16),

		Spacing: gui.SomeF(12),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Count: %d", s.Count),
				TextStyle: gui.TextStyle{
					Size: 28,
				},
			}),
			gui.Row(gui.ContainerCfg{
				Spacing: gui.SomeF(8),
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID: "time_travel_increment",
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Increment"}),
						},
						OnClick: func(ctx gui.EventCtx) {
							st := gui.State[appState](ctx.Window)
							st.Count++
							st.Log = append(st.Log,
								fmt.Sprintf("inc → %d", st.Count))
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID: "time_travel_reset",
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Reset"}),
						},
						OnClick: func(ctx gui.EventCtx) {
							st := gui.State[appState](ctx.Window)
							st.Count = 0
							st.Log = append(st.Log, "reset")
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Events: %d", len(s.Log)),
			}),
		},
	})
}
