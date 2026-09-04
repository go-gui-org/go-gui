package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
)

func demoToggle(w *gui.Window) gui.View {
	app := appState(w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Toggle(gui.ToggleCfg{
				ID:       "toggle-a",
				Label:    "Toggle",
				Selected: app.ToggleA,
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.ToggleA = !a.ToggleA
				},
			}),
			gui.Toggle(gui.ToggleCfg{
				ID:       "checkbox-a",
				Label:    "Checkbox",
				Selected: app.CheckboxA,
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.CheckboxA = !a.CheckboxA
				},
			}),
		},
	})
}

func demoSwitch(w *gui.Window) gui.View {
	app := appState(w)
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Switch(gui.SwitchCfg{
				ID:       "switch-a",
				Label:    "Enable feature",
				Selected: app.SwitchA,
				OnClick: func(ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.SwitchA = !a.SwitchA
				},
			}),
		},
	})
}

func demoRadio(w *gui.Window) gui.View {
	app := appState(w)
	options := []struct{ label, value string }{
		{"Go", "go"},
		{"Rust", "rust"},
		{"Zig", "zig"},
	}
	views := make([]gui.View, len(options))
	for i, opt := range options {
		v := opt.value
		views[i] = gui.Radio(gui.RadioCfg{
			ID:       "radio-" + v,
			Label:    opt.label,
			Selected: app.RadioValue == v,
			OnClick: func(ctx gui.EventCtx) {
				appState(ctx.Window).RadioValue = v
			},
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		Content: views,
	})
}

func demoRadioGroup(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Column layout", TextStyle: t.B3}),
			gui.RadioButtonGroupColumn(gui.RadioButtonGroupCfg{
				ID:    "demo_selection_demo_radio_group",
				Value: app.RadioValue,
				Options: []gui.RadioOption{
					gui.NewRadioOption("Go", "go"),
					gui.NewRadioOption("Rust", "rust"),
					gui.NewRadioOption("Zig", "zig"),
				},
				OnSelect: func(v string, ctx gui.EventCtx) {
					appState(ctx.Window).RadioValue = v
				},
			}),
			gui.Text(gui.TextCfg{Text: "Row layout", TextStyle: t.B3}),
			gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
				ID:    "demo_selection_demo_radio_group_2",
				Value: app.RadioValue,
				Options: []gui.RadioOption{
					gui.NewRadioOption("Go", "go"),
					gui.NewRadioOption("Rust", "rust"),
					gui.NewRadioOption("Zig", "zig"),
				},
				OnSelect: func(v string, ctx gui.EventCtx) {
					appState(ctx.Window).RadioValue = v
				},
			}),
		},
	})
}

func demoSelect(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Selected: %v", app.SelectValue),
				TextStyle: t.N3,
			}),
			sectionLabel(t, "Single Select"),
			gui.Select(gui.SelectCfg{
				ID:          "select-single",
				Placeholder: "Pick a language",
				Selected:    app.SelectValue,
				Options:     []string{"Go", "Rust", "Zig", "C", "Python", "TypeScript"},
				OnSelect: func(sel []string, ctx gui.EventCtx) {
					appState(ctx.Window).SelectValue = sel
				},
			}),
			sectionLabel(t, "Multi-Select"),
			gui.Select(gui.SelectCfg{
				ID:             "select-multi",
				Placeholder:    "Pick languages",
				Selected:       app.SelectValue,
				SelectMultiple: true,
				Options:        []string{"Go", "Rust", "Zig", "C", "Python", "TypeScript"},
				OnSelect: func(sel []string, ctx gui.EventCtx) {
					appState(ctx.Window).SelectValue = sel
				},
			}),
		},
	})
}

func demoListBox(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Selected: %v", app.ListBoxSelected),
				TextStyle: t.N3,
			}),
			gui.Text(gui.TextCfg{Text: "Virtualized list (scroll)", TextStyle: t.B3}),
			gui.ListBox(gui.ListBoxCfg{
				ID:          "listbox-demo",
				Scrollable:  true,
				Sizing:      gui.FillFit,
				MaxHeight:   200,
				Multiple:    true,
				SelectedIDs: app.ListBoxSelected,
				Data: []gui.ListBoxOption{
					gui.NewListBoxSubheading("h-compiled", "Compiled"),
					gui.NewListBoxOption("go", "Go", "go"),
					gui.NewListBoxOption("rust", "Rust", "rust"),
					gui.NewListBoxOption("zig", "Zig", "zig"),
					gui.NewListBoxOption("c", "C", "c"),
					gui.NewListBoxOption("cpp", "C++", "cpp"),
					gui.NewListBoxOption("swift", "Swift", "swift"),
					gui.NewListBoxSubheading("h-interp", "Interpreted"),
					gui.NewListBoxOption("python", "Python", "python"),
					gui.NewListBoxOption("js", "JavaScript", "js"),
					gui.NewListBoxOption("ts", "TypeScript", "ts"),
					gui.NewListBoxOption("ruby", "Ruby", "ruby"),
					gui.NewListBoxOption("elixir", "Elixir", "elixir"),
					gui.NewListBoxOption("lua", "Lua", "lua"),
				},
				OnSelect: func(ids []string, ctx gui.EventCtx) {
					appState(ctx.Window).ListBoxSelected = ids
				},
			}),
		},
	})
}

func demoCombobox(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + app.ComboboxValue,
				TextStyle: t.N3,
			}),
			gui.Combobox(gui.ComboboxCfg{
				ID:          "combobox-demo",
				Placeholder: "Type to search...",
				Value:       app.ComboboxValue,
				Options:     []string{"Go", "Rust", "Zig", "C", "C++", "Python", "TypeScript", "JavaScript", "Ruby", "Elixir"},
				Sizing:      gui.FillFit,
				OnSelect: func(v string, ctx gui.EventCtx) {
					appState(ctx.Window).ComboboxValue = v
				},
			}),
		},
	})
}

func demoDragReorder(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()

	// Tab content panels (simple label per tab).
	tabItems := make([]gui.TabItemCfg, len(app.DragTabItems))
	for i, tab := range app.DragTabItems {
		tabItems[i] = gui.NewTabItem(tab.ID, tab.Label, []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NewPadding(12, 12, 12, 12),
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      tab.Label + " content",
						TextStyle: t.N3,
					}),
				},
			}),
		})
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(16),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Drag items to reorder, or use Alt+Arrow keys. Escape cancels.",
				TextStyle: t.N3,
			}),

			gui.Text(gui.TextCfg{Text: "List Box", TextStyle: t.B3}),
			gui.ListBox(gui.ListBoxCfg{
				ID:          "drag-listbox",
				Sizing:      gui.FillFit,
				MaxHeight:   200,
				Scrollable:  true,
				Data:        app.DragListItems,
				Reorderable: true,
				OnReorder: func(movedID string, beforeID string, ctx gui.EventCtx) {
					a := appState(ctx.Window)
					from, to := gui.ReorderIndices(
						dragListIDs(a.DragListItems), movedID, beforeID)
					if from >= 0 {
						dragSliceMove(&a.DragListItems, from, to)
					}
				},
				OnSelect: func(_ []string, ctx gui.EventCtx) {},
			}),

			gui.Text(gui.TextCfg{Text: "Tab Control", TextStyle: t.B3}),
			gui.TabControl(gui.TabControlCfg{
				ID:          "drag-tabs",
				Items:       tabItems,
				Selected:    app.DragTabSel,
				Sizing:      gui.FillFit,
				Reorderable: true,
				OnReorder: func(movedID string, beforeID string, ctx gui.EventCtx) {
					a := appState(ctx.Window)
					from, to := gui.ReorderIndices(
						dragTabIDs(a.DragTabItems), movedID, beforeID)
					if from >= 0 {
						dragSliceMove(&a.DragTabItems, from, to)
					}
				},
				OnSelect: func(id string, ctx gui.EventCtx) {
					appState(ctx.Window).DragTabSel = id
				},
			}),

			gui.Text(gui.TextCfg{Text: "Tree View", TextStyle: t.B3}),
			gui.Tree(gui.TreeCfg{
				ID:          "drag-tree",
				Nodes:       app.DragTreeNodes,
				Sizing:      gui.FillFit,
				MaxHeight:   250,
				Scrollable:  true,
				Reorderable: true,
				OnReorder: func(movedID string, beforeID string, ctx gui.EventCtx) {
					a := appState(ctx.Window)
					dragTreeReorder(a, movedID, beforeID)
				},
			}),
		},
	})
}

func dragListIDs(items []gui.ListBoxOption) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	return ids
}

func dragTabIDs(items []gui.TabItemCfg) []string {
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	return ids
}

func dragSliceMove[T any](s *[]T, from, to int) {
	item := (*s)[from]
	*s = append((*s)[:from], (*s)[from+1:]...)
	*s = append((*s)[:to], append([]T{item}, (*s)[to:]...)...)
}

// dragTreeReorder reorders siblings under the same parent.
func dragTreeReorder(app *ShowcaseApp, movedID, beforeID string) {
	dragTreeReorderNodes(&app.DragTreeNodes, movedID, beforeID)
}

func dragTreeReorderNodes(
	nodes *[]gui.TreeNodeCfg, movedID, beforeID string,
) bool {
	ids := make([]string, len(*nodes))
	for i := range *nodes {
		ids[i] = (*nodes)[i].ID
	}
	from, to := gui.ReorderIndices(ids, movedID, beforeID)
	if from >= 0 {
		dragSliceMove(nodes, from, to)
		return true
	}
	for i := range *nodes {
		if len((*nodes)[i].Nodes) > 0 {
			if dragTreeReorderNodes(&(*nodes)[i].Nodes, movedID, beforeID) {
				return true
			}
		}
	}
	return false
}

func demoSlider(w *gui.Window) gui.View {
	app := appState(w)
	t := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Value: %.0f", app.RangeValue),
				TextStyle: t.N3,
			}),
			gui.Slider(gui.SliderCfg{
				ID:     "slider-demo",
				Value:  app.RangeValue,
				Min:    0,
				Max:    100,
				Sizing: gui.FillFit,
				OnChange: func(v float32, ctx gui.EventCtx) {
					appState(ctx.Window).RangeValue = v
				},
			}),
		},
	})
}
