package main

import "github.com/go-gui-org/go-gui/gui"

func demoBreadcrumb(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Breadcrumb(gui.BreadcrumbCfg{
				ID: "breadcrumb-demo",
				Items: []gui.BreadcrumbItemCfg{
					gui.NewBreadcrumbItem("home", "Home", nil),
					gui.NewBreadcrumbItem("docs", "Docs", nil),
					gui.NewBreadcrumbItem("api", "API Reference", nil),
					gui.NewBreadcrumbItem("widgets", "Widgets", nil),
				},
				Selected: app.BCSelected,
				OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).BCSelected = id
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + app.BCSelected,
				TextStyle: t.N3,
			}),
		},
	})
}

func demoTabControl(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	t := gui.CurrentTheme()
	return gui.TabControl(gui.TabControlCfg{
		ID:        "tab-demo",
		Focusable: true,
		Items: []gui.TabItemCfg{
			gui.NewTabItem("tab1", "Overview", []gui.View{
				gui.Column(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Padding: gui.SomeP(12, 12, 12, 12),
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: "Overview tab content.", TextStyle: t.N3}),
					},
				}),
			}),
			gui.NewTabItem("tab2", "Details", []gui.View{
				gui.Column(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Padding: gui.SomeP(12, 12, 12, 12),
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: "Details tab content.", TextStyle: t.N3}),
					},
				}),
			}),
			gui.NewTabItem("tab3", "Settings", []gui.View{
				gui.Column(gui.ContainerCfg{
					Sizing:  gui.FillFit,
					Padding: gui.SomeP(12, 12, 12, 12),
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: "Settings tab content.", TextStyle: t.N3}),
					},
				}),
			}),
		},
		Selected: app.TabSelected,
		OnSelect: func(id string, _ *gui.Event, w *gui.Window) {
			gui.State[ShowcaseApp](w).TabSelected = id
		},
	})
}

func demoMenus(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)

	return gui.Menubar(w, gui.MenubarCfg{
		ID: "menubar-demo",
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
							Text:        app.MenuSearchText,
							ID:          "showcase_menu_search",
							Focusable:   true,
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
								gui.State[ShowcaseApp](w).MenuSearchText = s
							},
						}),
					},
					gui.MenuSeparator(),
					gui.MenuItemText("help-me", "Help"),
				},
			},
		},
		Action: func(id string, _ *gui.Event, w *gui.Window) {
			w.Toast(gui.ToastCfg{Title: "Menu", Body: "Action: " + id})
		},
	})
}

func demoCommandPalette(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)

	const paletteID = "cmd-palette"

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      "btn-palette",
				Padding: gui.SomeP(8, 16, 8, 16),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Open Command Palette", TextStyle: t.N3}),
				},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					gui.CommandPaletteToggle(paletteID, w)
					e.IsHandled = true
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Last action: " + app.PaletteAction,
				TextStyle: t.N3,
			}),
			gui.CommandPalette(gui.CommandPaletteCfg{
				ID:          paletteID,
				Placeholder: "Type a command...",
				Items: []gui.CommandPaletteItem{
					{ID: "new-file", Label: "New File", Icon: gui.IconPlus, Group: "File"},
					{ID: "open-file", Label: "Open File", Icon: gui.IconBook, Group: "File"},
					{ID: "save", Label: "Save", Icon: gui.IconDownload, Group: "File"},
					{ID: "toggle-theme", Label: "Toggle Theme", Icon: gui.IconMoon, Group: "View"},
					{ID: "search", Label: "Search", Icon: gui.IconSearch, Group: "Edit"},
				},
				OnAction: func(id string, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).PaletteAction = id
				},
			}),
		},
	})
}
