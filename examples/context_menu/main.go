// This example demonstrates right-click context menus and their action handling.
// The context menu example demonstrates right-click menus and
// action handling.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct {
	Status string
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Status: "Right-click anywhere"},
		Title:  "Context Menu Example",
		Width:  500,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
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
