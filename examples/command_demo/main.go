// Command demo shows a menu bar with shortcut hints and
// buttons wired to registered commands. Demonstrates global
// vs non-global hotkeys, CanExecute auto-disable, and
// CommandPalette integration.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	idFocusMenu     uint32 = 1
	idFocusPalette  uint32 = 2
	idScrollPalette uint32 = 3
)

type App struct {
	Log     string
	Counter int
	Saved   bool
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Log: "Ready."},
		Title:  "Command Demo",
		Width:  700,
		Height: 500,
		OnInit: func(w *gui.Window) {
			registerCommands(w)
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func registerCommands(w *gui.Window) {
	_ = w.RegisterCommands(
		gui.Command{
			ID:       "file.new",
			Label:    "New",
			Group:    "File",
			Shortcut: gui.Shortcut{Key: gui.KeyN, Modifiers: gui.ModSuper},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				app := gui.State[App](w)
				app.Counter = 0
				app.Saved = false
				app.Log = "New document created."
			},
		},
		gui.Command{
			ID:       "file.save",
			Label:    "Save",
			Group:    "File",
			Shortcut: gui.Shortcut{Key: gui.KeyS, Modifiers: gui.ModSuper},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				app := gui.State[App](w)
				app.Saved = true
				app.Log = fmt.Sprintf("Saved (counter=%d).", app.Counter)
			},
		},
		gui.Command{
			ID:       "edit.undo",
			Label:    "Undo",
			Group:    "Edit",
			Shortcut: gui.Shortcut{Key: gui.KeyZ, Modifiers: gui.ModSuper},
			Execute: func(_ *gui.Event, w *gui.Window) {
				app := gui.State[App](w)
				if app.Counter > 0 {
					app.Counter--
					app.Log = fmt.Sprintf("Undo. Counter=%d", app.Counter)
				}
			},
			CanExecute: func(w *gui.Window) bool {
				return gui.State[App](w).Counter > 0
			},
		},
		gui.Command{
			ID:       "edit.increment",
			Label:    "Increment",
			Group:    "Edit",
			Shortcut: gui.Shortcut{Key: gui.KeyEqual, Modifiers: gui.ModSuper},
			Execute: func(_ *gui.Event, w *gui.Window) {
				app := gui.State[App](w)
				app.Counter++
				app.Saved = false
				app.Log = fmt.Sprintf("Incremented. Counter=%d", app.Counter)
			},
		},
		gui.Command{
			ID:       "edit.decrement",
			Label:    "Decrement",
			Group:    "Edit",
			Shortcut: gui.Shortcut{Key: gui.KeyMinus, Modifiers: gui.ModSuper},
			Execute: func(_ *gui.Event, w *gui.Window) {
				app := gui.State[App](w)
				if app.Counter > 0 {
					app.Counter--
					app.Saved = false
					app.Log = fmt.Sprintf("Decremented. Counter=%d", app.Counter)
				}
			},
			CanExecute: func(w *gui.Window) bool {
				return gui.State[App](w).Counter > 0
			},
		},
		gui.Command{
			ID:       "view.palette",
			Label:    "Command Palette",
			Group:    "View",
			Shortcut: gui.Shortcut{Key: gui.KeyP, Modifiers: gui.ModSuperShift},
			Global:   true,
			Execute: func(_ *gui.Event, w *gui.Window) {
				gui.CommandPaletteToggle(
					"palette", idFocusPalette, idScrollPalette, w)
			},
		},
	)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Padding: gui.NoPadding,
		Sizing:  gui.FixedFixed,
		Spacing: gui.SomeF(0),
		Content: []gui.View{
			menuBar(w),
			body(w, app, theme),
			statusBar(app, theme),
			gui.CommandPalette(gui.CommandPaletteCfg{
				ID:        "palette",
				IDFocus:   idFocusPalette,
				IDScroll:  idScrollPalette,
				Items:     w.CommandPaletteItems(),
				OnAction:  paletteAction,
				OnDismiss: func(_ *gui.Window) {},
			}),
		},
	})
}

func menuBar(w *gui.Window) gui.View {
	return gui.Menubar(w, gui.MenubarCfg{
		Float:       true,
		FloatAnchor: gui.FloatTopCenter,
		FloatTieOff: gui.FloatTopCenter,
		IDFocus:     idFocusMenu,
		Items: []gui.MenuItemCfg{
			gui.MenuSubmenu("file", "File", []gui.MenuItemCfg{
				{ID: "file.new", CommandID: "file.new"},
				{ID: "file.save", CommandID: "file.save"},
				gui.MenuSeparator(),
				gui.MenuItemText("exit", "Exit"),
			}),
			gui.MenuSubmenu("edit", "Edit", []gui.MenuItemCfg{
				{ID: "edit.undo", CommandID: "edit.undo"},
				gui.MenuSeparator(),
				{ID: "edit.increment", CommandID: "edit.increment"},
				{ID: "edit.decrement", CommandID: "edit.decrement"},
			}),
			gui.MenuSubmenu("view", "View", []gui.MenuItemCfg{
				{ID: "view.palette", CommandID: "view.palette"},
			}),
		},
	})
}

func body(w *gui.Window, app *App, theme gui.Theme) gui.View {
	savedText := "unsaved"
	if app.Saved {
		savedText = "saved"
	}

	return gui.Column(gui.ContainerCfg{
		HAlign:  gui.HAlignCenter,
		Padding: gui.NoPadding,
		Sizing:  gui.FillFill,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Height: 45,
				Color:  gui.ColorTransparent,
				Sizing: gui.FillFixed,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Command Demo",
				TextStyle: theme.B1,
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf(
					"Counter: %d  (%s)", app.Counter, savedText),
				TextStyle: theme.M1,
			}),
			gui.Text(gui.TextCfg{Text: ""}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				SizeBorder: gui.NoBorder,
				Padding:    gui.NoPadding,
				Content: []gui.View{
					gui.CommandButton(w, "edit.increment",
						gui.ButtonCfg{IDFocus: 10}),
					gui.CommandButton(w, "edit.decrement",
						gui.ButtonCfg{IDFocus: 11}),
					gui.CommandButton(w, "edit.undo",
						gui.ButtonCfg{IDFocus: 12}),
				},
			}),
			gui.Text(gui.TextCfg{Text: ""}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				SizeBorder: gui.NoBorder,
				Padding:    gui.NoPadding,
				Content: []gui.View{
					gui.CommandButton(w, "file.new",
						gui.ButtonCfg{IDFocus: 13}),
					gui.CommandButton(w, "file.save",
						gui.ButtonCfg{IDFocus: 14}),
				},
			}),
		},
	})
}

func statusBar(app *App, theme gui.Theme) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		SizeBorder: gui.NoBorder,
		Padding:    gui.SomeP(4, 8, 4, 8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      app.Log,
				TextStyle: theme.M3,
			}),
		},
	})
}

func paletteAction(id string, e *gui.Event, w *gui.Window) {
	cmd, ok := w.CommandByID(id)
	if ok && cmd.Execute != nil {
		cmd.Execute(e, w)
	}
}
