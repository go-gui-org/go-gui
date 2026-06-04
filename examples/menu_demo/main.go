// The menu demo shows a menu bar, nested items, and simple stateful
// actions.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	idFocusMenu   uint32 = 1
	idFocusButton uint32 = 2
	idFocusSearch uint32 = 3
)

type MenuApp struct {
	SearchText string
	SelectedID string
	Clicks     int
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &MenuApp{},
		Title:  "Menu Demo",
		Width:  600,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Padding: gui.NoPadding,
		Sizing:  gui.FixedFixed,
		Spacing: gui.Some[float32](0),
		Content: []gui.View{
			menu(w),
			body(w),
		},
	})
}

func menu(w *gui.Window) gui.View {
	app := gui.State[MenuApp](w)

	return gui.Menubar(w, gui.MenubarCfg{
		Float:       true,
		FloatAnchor: gui.FloatTopCenter,
		FloatTieOff: gui.FloatTopCenter,
		IDFocus:     idFocusMenu,
		Action: func(id string, _ *gui.Event, w *gui.Window) {
			gui.State[MenuApp](w).SelectedID = id
		},
		Items: []gui.MenuItemCfg{
			{
				ID:   "file",
				Text: "File",
				Submenu: []gui.MenuItemCfg{
					gui.MenuSubmenu("new", "New", []gui.MenuItemCfg{
						gui.MenuItemText("here", "Here"),
						gui.MenuItemText("there", "There"),
					}),
					gui.MenuSubmenu("open", "Open", []gui.MenuItemCfg{
						gui.MenuItemText("no-where", "No Where"),
						gui.MenuItemText("some-where", "Some Where"),
						gui.MenuSubmenu("keep-going", "Keep Going", []gui.MenuItemCfg{
							gui.MenuItemText("you-are-done", "OK, you're done"),
						}),
					}),
					gui.MenuSeparator(),
					gui.MenuItemText("exit", "Exit"),
				},
			},
			{
				ID:   "edit",
				Text: "Edit",
				Submenu: []gui.MenuItemCfg{
					gui.MenuItemText("cut", "Cut"),
					gui.MenuItemText("copy", "Copy"),
					gui.MenuItemText("paste", "Paste"),
					gui.MenuSeparator(),
					gui.MenuItemText("emoji", "Emoji & Symbols"),
					gui.MenuItemText("too-long", "Long menu text item to test line wrapping in menu"),
				},
			},
			{
				ID:   "view",
				Text: "View",
				Submenu: []gui.MenuItemCfg{
					gui.MenuItemText("zoom-in", "Zoom In"),
					gui.MenuItemText("zoom-out", "Zoom Out"),
					gui.MenuItemText("zoom-reset", "Reset Zoom"),
					gui.MenuSeparator(),
					gui.MenuItemText("project-panel", "Project Panel"),
					gui.MenuItemText("outline-panel", "Outline Panel"),
					gui.MenuItemText("terminal-panel", "Terminal Panel"),
					gui.MenuSeparator(),
					gui.MenuItemText("full-screen", "Enter Full Screen"),
				},
			},
			{
				ID:   "go",
				Text: "Go",
				Submenu: []gui.MenuItemCfg{
					gui.MenuItemText("go-back", "Back"),
					gui.MenuItemText("go-forward", "Forward"),
					gui.MenuSeparator(),
					gui.MenuItemText("go-definition", "Go to Definition"),
					gui.MenuItemText("go-declaration", "Go to Declaration"),
					gui.MenuItemText("go-to-moon-alice", "Go to the Moon Alice"),
				},
			},
			{
				ID:   "window",
				Text: "Window",
				Submenu: []gui.MenuItemCfg{
					gui.MenuItemText("window-fill", "Fill"),
					gui.MenuItemText("window-center", "Center"),
					gui.MenuSeparator(),
					gui.MenuSubmenu("window-move", "Move & Resize", []gui.MenuItemCfg{
						gui.MenuSubtitle("Halves"),
						gui.MenuItemText("half-left", "Left"),
						gui.MenuItemText("half-top", "Top"),
						gui.MenuItemText("half-right", "Right"),
						gui.MenuItemText("half-bottom", "Bottom"),
						gui.MenuSeparator(),
						gui.MenuSubtitle("Quarters"),
						gui.MenuItemText("quarter-top-left", "Top Left"),
						gui.MenuItemText("quarter-top-right", "Top Right"),
						gui.MenuItemText("quarter-bottom-left", "Bottom Left"),
						gui.MenuItemText("quarter-bottom-right", "Bottom Right"),
					}),
					gui.MenuItemText("window-full-screen-tile", "Full Screen Tile"),
				},
			},
			{
				ID:   "help",
				Text: "Help",
				Submenu: []gui.MenuItemCfg{
					{
						ID:      "search",
						Padding: gui.NoPadding,
						CustomView: gui.Input(gui.InputCfg{
							Text:        app.SearchText,
							IDFocus:     idFocusSearch,
							Width:       100,
							MinWidth:    100,
							MaxWidth:    100,
							Sizing:      gui.FixedFill,
							Placeholder: "Search",
							Padding: gui.SomeP(2,
								gui.CurrentTheme().InputStyle.Padding.Right,
								2,
								gui.CurrentTheme().InputStyle.Padding.Left),
							Radius:           gui.Some[float32](0),
							SizeBorder:       gui.Some[float32](0),
							TextStyle:        gui.CurrentTheme().MenubarStyle.TextStyle,
							PlaceholderStyle: gui.CurrentTheme().MenubarStyle.TextStyle,
							OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
								gui.State[MenuApp](w).SearchText = s
							},
						}),
					},
					gui.MenuSeparator(),
					gui.MenuItemText("help-me", "Help"),
				},
			},
			{
				ID: "theme",
				CustomView: gui.ThemePicker(gui.ThemePickerCfg{
					ID: "theme-picker",
				}),
			},
		},
	})
}

func body(w *gui.Window) gui.View {
	app := gui.State[MenuApp](w)
	theme := gui.CurrentTheme()

	var selectedText, searchText string
	if app.SelectedID != "" {
		selectedText = fmt.Sprintf("Menu %q selected", app.SelectedID)
		searchText = fmt.Sprintf("Search text: %q", app.SearchText)
	}

	return gui.Column(gui.ContainerCfg{
		HAlign:  gui.HAlignCenter,
		Padding: gui.NoPadding,
		Sizing:  gui.FillFill,
		Content: []gui.View{
			gui.Rectangle(gui.RectangleCfg{
				Height: 40,
				Color:  gui.ColorTransparent,
				Sizing: gui.FillFixed,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Welcome to GUI",
				TextStyle: theme.B1,
			}),
			gui.Button(gui.ButtonCfg{
				IDFocus: idFocusButton,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: fmt.Sprintf("%d Clicks", app.Clicks),
					}),
				},
				OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
					gui.State[MenuApp](w).Clicks++
				},
			}),
			gui.Text(gui.TextCfg{Text: ""}),
			gui.Text(gui.TextCfg{
				Text:      selectedText,
				TextStyle: theme.M3,
			}),
			gui.Text(gui.TextCfg{
				Text:      searchText,
				TextStyle: theme.M3,
			}),
		},
	})
}
