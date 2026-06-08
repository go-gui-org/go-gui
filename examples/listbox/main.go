// The listbox example demonstrates a virtualized list with a large
// option set and simple selection state.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// Virtualized List Box Scrolling
// Demonstrates list box virtualization with 10,000 items.

type App struct {
	Items       []gui.ListBoxOption
	SelectedIDs []string
}

func main() {
	const size = 10_000
	items := make([]gui.ListBoxOption, 0, size)
	// Pre-build the option slice so the view only has to render it.
	for i := 1; i <= size; i++ {
		id := fmt.Sprintf("%05d", i)
		items = append(items, gui.NewListBoxOption(id, id+" text list item", id))
	}

	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Items: items},
		Title:  "ListBox - Virtualized",
		Width:  240,
		Height: 420,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	selected := "none"
	if len(app.SelectedIDs) > 0 {
		selected = app.SelectedIDs[0]
	}

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		HAlign:  gui.HAlignCenter,
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some(theme.SpacingSmall),
		Padding: gui.SomeP(8, 8, 8, 8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "10,000-item virtualized list box",
				TextStyle: theme.B4,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected id: " + selected,
				TextStyle: theme.N5,
			}),
			gui.ListBox(gui.ListBoxCfg{
				ID:          "virtual-listbox-10k",
				IDScroll:    1,
				Height:      float32(wh) - 70,
				Sizing:      gui.FillFixed,
				SelectedIDs: app.SelectedIDs,
				Data:        app.Items,
				OnSelect: func(ids []string, e *gui.Event, w *gui.Window) {
					gui.State[App](w).SelectedIDs = ids
					e.IsHandled = true
				},
			}),
		},
	})
}
