// The context menu example demonstrates right-click menus and
// action handling.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Status string
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Status: "Right-click anywhere"},
		Title:  "Context Menu Example",
		Width:  500,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	return gui.ContextMenu(w, gui.ContextMenuCfg{
		ID:     "ctx",
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Items: []gui.MenuItemCfg{
			gui.MenuSubtitle("Actions"),
			{ID: "cut", Text: "Cut"},
			{ID: "copy", Text: "Copy"},
			{ID: "paste", Text: "Paste"},
			gui.MenuSeparator(),
			gui.MenuSubmenu("more", "More", []gui.MenuItemCfg{
				{ID: "selectall", Text: "Select All"},
				{ID: "find", Text: "Find"},
			}),
			gui.MenuSeparator(),
			{ID: "delete", Text: "Delete"},
		},
		Action: func(id string, ctx gui.EventCtx) {
			app := gui.State[App](ctx.Window)
			// Mirror the last menu action in the main view.
			app.Status = "Selected: " + id
			ctx.Consume()
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Right-click anywhere for a context menu",
				TextStyle: gui.CurrentTheme().B1,
			}),
			gui.Text(gui.TextCfg{
				Text: app.Status,
			}),
		},
	})
}
