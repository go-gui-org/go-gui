package main

import "github.com/go-gui-org/go-gui/gui"

func dockInitialLayout() *gui.DockNode {
	return gui.DockSplit("root", gui.DockSplitHorizontal, 0.2,
		gui.DockPanelGroup("left", []string{"explorer"}, "explorer"),
		gui.DockSplit("right", gui.DockSplitVertical, 0.75,
			gui.DockPanelGroup("top", []string{"editor", "preview"}, "editor"),
			gui.DockPanelGroup("bottom", []string{"console", "problems"}, "console"),
		),
	)
}

func demoDockLayout(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	if app.DockRoot == nil {
		app.DockRoot = dockInitialLayout()
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFixed,
		Height:     500,
		Spacing:    gui.SomeF(8),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			dockToolbar(app),
			gui.DockLayout(gui.DockLayoutCfg{
				ID:     "showcase-dock",
				Root:   app.DockRoot,
				Panels: dockPanels(),
				OnLayoutChange: func(root *gui.DockNode, w *gui.Window) {
					gui.State[ShowcaseApp](w).DockRoot = root
				},
				OnPanelSelect: func(groupID, panelID string, w *gui.Window) {
					a := gui.State[ShowcaseApp](w)
					a.DockRoot = gui.DockTreeSelectPanel(a.DockRoot, groupID, panelID)
				},
				OnPanelClose: func(panelID string, w *gui.Window) {
					a := gui.State[ShowcaseApp](w)
					a.DockRoot = gui.DockTreeRemovePanel(a.DockRoot, panelID)
				},
			}),
		},
	})
}

func dockToolbar(_ *ShowcaseApp) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NoPadding,
		Spacing:    gui.SomeF(8),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset"})},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).DockRoot = dockInitialLayout()
					e.IsHandled = true
				},
			}),
			gui.Button(gui.ButtonCfg{
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Add Properties"})},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					a := gui.State[ShowcaseApp](w)
					if _, ok := gui.DockTreeFindGroupByPanel(a.DockRoot, "properties"); ok {
						e.IsHandled = true
						return
					}
					a.DockRoot = gui.DockTreeAddTab(a.DockRoot, "left", "properties")
					e.IsHandled = true
				},
			}),
		},
	})
}

func dockPanels() []gui.DockPanelDef {
	return []gui.DockPanelDef{
		{ID: "explorer", Label: "Explorer", Content: dockPanelContent("Explorer", "src/\n  main.go\n  app.go\n  util.go")},
		{ID: "editor", Label: "Editor", Closable: true, Content: dockPanelContent("Editor", "func main() {\n    fmt.Println(\"Hello\")\n}")},
		{ID: "preview", Label: "Preview", Closable: true, Content: dockPanelContent("Preview", "Live preview area")},
		{ID: "console", Label: "Console", Closable: true, Content: dockPanelContent("Console", "$ go build ./...\nok")},
		{ID: "problems", Label: "Problems", Closable: true, Content: dockPanelContent("Problems", "No problems detected.")},
		{ID: "properties", Label: "Properties", Closable: true, Content: dockPanelContent("Properties", "Name: app.go\nSize: 1.2 KB")},
	}
}

func dockPanelContent(title, body string) []gui.View {
	t := gui.CurrentTheme()
	return []gui.View{
		gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Padding: gui.SomeP(8, 12, 8, 12),
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: title, TextStyle: t.B2}),
				gui.Text(gui.TextCfg{Text: body}),
			},
		}),
	}
}
