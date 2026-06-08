// Dock_layout demonstrates the DockLayout widget: IDE-style
// docking panels with splits, tabs, drag-and-drop rearrangement,
// and closable panels.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Root *gui.DockNode
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Root: initialLayout()},
		Title:  "dock_layout",
		Width:  1000,
		Height: 700,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func initialLayout() *gui.DockNode {
	return gui.DockSplit("root", gui.DockSplitHorizontal, 0.2,
		gui.DockPanelGroup("left", []string{"explorer"}, "explorer"),
		gui.DockSplit("right", gui.DockSplitVertical, 0.75,
			gui.DockPanelGroup("top", []string{"editor", "preview"}, "editor"),
			gui.DockPanelGroup("bottom", []string{"console", "problems"}, "console"),
		),
	)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Spacing: gui.NoSpacing,
		Content: []gui.View{
			toolbar(),
			gui.DockLayout(gui.DockLayoutCfg{
				ID:     "dock",
				Root:   app.Root,
				Panels: panels(),
				OnLayoutChange: func(root *gui.DockNode, w *gui.Window) {
					gui.State[App](w).Root = root
				},
				OnPanelSelect: func(groupID, panelID string, w *gui.Window) {
					app := gui.State[App](w)
					app.Root = gui.DockTreeSelectPanel(app.Root, groupID, panelID)
				},
				OnPanelClose: func(panelID string, w *gui.Window) {
					app := gui.State[App](w)
					app.Root = gui.DockTreeRemovePanel(app.Root, panelID)
				},
			}),
		},
	})
}

func toolbar() gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.SomeP(4, 8, 4, 8),
		Spacing: gui.SomeF(8),
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Reset Layout"})},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					gui.State[App](w).Root = initialLayout()
					e.IsHandled = true
				},
			}),
			gui.Button(gui.ButtonCfg{
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Add Properties"})},
				OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
					app := gui.State[App](w)
					if _, ok := gui.DockTreeFindGroupByPanel(app.Root, "properties"); ok {
						e.IsHandled = true
						return
					}
					app.Root = gui.DockTreeAddTab(app.Root, "left", "properties")
					e.IsHandled = true
				},
			}),
		},
	})
}

func panels() []gui.DockPanelDef {
	return []gui.DockPanelDef{
		{ID: "explorer", Label: "Explorer", Content: panelContent("Explorer", "src/\n  main.go\n  app.go\n  util.go\npkg/\n  config.go\n  server.go")},
		{ID: "editor", Label: "Editor", Closable: true, Content: panelContent("Editor", "func main() {\n    fmt.Println(\"Hello\")\n}")},
		{ID: "preview", Label: "Preview", Closable: true, Content: panelContent("Preview", "Live preview area")},
		{ID: "console", Label: "Console", Closable: true, Content: panelContent("Console", "$ go build ./...\nok\n$ go test ./...\nPASS")},
		{ID: "problems", Label: "Problems", Closable: true, Content: panelContent("Problems", "No problems detected.")},
		{ID: "properties", Label: "Properties", Closable: true, Content: panelContent("Properties", "Name: app.go\nSize: 1.2 KB\nModified: today")},
	}
}

func panelContent(title, body string) []gui.View {
	return []gui.View{
		gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Padding: gui.SomeP(8, 12, 8, 12),
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      title,
					TextStyle: gui.CurrentTheme().B2,
				}),
				gui.Text(gui.TextCfg{Text: body}),
			},
		}),
	}
}
