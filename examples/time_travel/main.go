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
	"fmt"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
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
	gui.SetTheme(gui.ThemeLightNoPadding)

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

	backend.RunApp(app, main)
}

func mainView(w *gui.Window) gui.View {
	s := gui.State[appState](w)
	return gui.Column(gui.ContainerCfg{
		Padding: gui.Some(gui.PadAll(16)),
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
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Increment"}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							st := gui.State[appState](w)
							st.Count++
							st.Log = append(st.Log,
								fmt.Sprintf("inc → %d", st.Count))
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						Content: []gui.View{
							gui.Text(gui.TextCfg{Text: "Reset"}),
						},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							st := gui.State[appState](w)
							st.Count = 0
							st.Log = append(st.Log, "reset")
							e.IsHandled = true
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
