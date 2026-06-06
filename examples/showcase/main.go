// Package main implements a faithful showcase port for the Go-Gui framework.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	scrollCatalog uint32 = iota + 1
	scrollDetail
	scrollPlaceholderA
	scrollPlaceholderB
	focusSearch
	focusMenu
	focusMenuSearch
	focusDocToggle
	focusTabDemo
	focusDrawCanvas
)

const catalogWidth float32 = 300

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	gui.SetMarkdownExternalAPIsEnabled(true)

	app := gui.NewApp()
	app.ExitMode = gui.ExitOnMainClose

	w := gui.NewWindow(gui.WindowCfg{
		State:  newShowcaseApp(),
		Title:  "Gui Showcase " + gui.Version,
		Width:  950,
		Height: 700,
		OnInit: func(w *gui.Window) {
			loadEmbeddedLocales()
			sa := gui.State[ShowcaseApp](w)
			syncThemeGenFromCfg(sa, gui.CurrentTheme().Cfg)
			_ = w.RegisterCommands(
				gui.Command{
					ID: "sc.greet", Label: "Greet", Icon: gui.IconBell,
					Shortcut: gui.Shortcut{Key: gui.KeyF5},
					Execute: func(_ *gui.Event, w *gui.Window) {
						w.Toast(gui.ToastCfg{Title: "Command", Body: "Hello from CommandButton!"})
					},
				},
				gui.Command{
					ID: "sc.count", Label: "Count", Icon: gui.IconPlus,
					Shortcut: gui.Shortcut{Key: gui.KeyF5, Modifiers: gui.ModShift},
					Execute: func(_ *gui.Event, w *gui.Window) {
						gui.State[ShowcaseApp](w).CmdButtonCount++
					},
				},
				gui.Command{
					ID: "sc.disabled", Label: "Delete", Icon: gui.IconTrash,
					CanExecute: func(_ *gui.Window) bool { return false },
				},
			)
			w.UpdateView(mainView)
		},
	})
	backend.RunApp(app, w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	return gui.Row(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Spacing: gui.NoSpacing,
		Content: []gui.View{catalogPanel(w), detailPanel(w)},
	})
}
