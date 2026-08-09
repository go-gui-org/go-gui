// This example demonstrates a virtualized list with a large option set and simple selection state.
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
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	selected := "none"
	if len(app.SelectedIDs) > 0 {
		selected = app.SelectedIDs[0]
	}

	return gui.Column(gui.ContainerCfg{
		HAlign:  gui.HAlignCenter,
		Sizing:  gui.FillFill,
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
				ID:         "virtual-listbox-10k",
				Scrollable: true,
				// Fill takes whatever the two labels above leave, so no
				// "window height minus 70" guess to keep in sync.
				Sizing:      gui.FillFill,
				SelectedIDs: app.SelectedIDs,
				Data:        app.Items,
				OnSelect: func(ids []string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).SelectedIDs = ids
					ctx.Consume()
				},
			}),
		},
	})
}
