package main

import (
	"slices"

	"github.com/go-gui-org/go-gui/gui"
)

func catalogPanel(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)
	entries := filteredEntries(app)

	switch {
	case len(entries) == 0:
		app.SelectedComponent = ""
		app.ShowDocs = false
		w.ScrollVerticalTo(scrollDetail, 0)
		w.ScrollHorizontalTo(scrollDetail, 0)
	case !hasEntry(entries, app.SelectedComponent):
		app.SelectedComponent = preferredComponentForGroup(entries)
		app.ShowDocs = false
		w.ScrollVerticalTo(scrollDetail, 0)
		w.ScrollHorizontalTo(scrollDetail, 0)
	}

	return gui.Column(gui.ContainerCfg{
		Width:   catalogWidth,
		Sizing:  gui.FixedFill,
		Color:   t.ColorPanel,
		Padding: gui.SomeP(12, 12, 12, 12),
		Spacing: gui.SomeF(8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Component Catalog", TextStyle: t.B3}),
			searchInput(app),
			groupPicker(app),
			line(),
			gui.Column(gui.ContainerCfg{
				ID:         scrollCatalog,
				Scrollable: true,
				Sizing:     gui.FillFill,
				Padding: gui.Some(gui.Padding{
					Right: t.ScrollbarStyle.Size + 4,
				}),
				Spacing:       gui.SomeF(2),
				ScrollbarCfgY: &gui.ScrollbarCfg{GapEdge: 3},
				Content:       catalogRows(entries, app),
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SomeF(8),
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					toggleLocale(app),
					gui.ThemePicker(gui.ThemePickerCfg{
						ID:          "showcase-theme",
						Focusable:   true,
						FloatAnchor: gui.FloatTopRight,
						FloatTieOff: gui.FloatBottomRight,
						OnSelect: func(name string, _ *gui.Event, w *gui.Window) {
							theme, ok := gui.ThemeGet(name)
							if !ok {
								return
							}
							syncThemeGenFromCfg(gui.State[ShowcaseApp](w), theme.Cfg)
						},
					}),
				},
			}),
		},
	})
}

func searchInput(app *ShowcaseApp) gui.View {
	return gui.Input(gui.InputCfg{
		ID:          "showcase-nav-search",
		Focusable:   true,
		Sizing:      gui.FillFit,
		Text:        app.NavQuery,
		Placeholder: "Search controls...",
		OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
			gui.State[ShowcaseApp](w).NavQuery = text
		},
	})
}

func groupPicker(app *ShowcaseApp) gui.View {
	return gui.Wrap(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(3),
		Content: []gui.View{
			groupPickerItem("Welcome", groupWelcome, app),
			groupPickerItem("All", groupAll, app),
			groupPickerItem("Text", groupText, app),
			groupPickerItem("Buttons", groupButtons, app),
			groupPickerItem("Input", groupInput, app),
			groupPickerItem("Selection", groupSelection, app),
			groupPickerItem("Data", groupData, app),
			groupPickerItem("Graphics", groupGraphics, app),
			groupPickerItem("Nav", groupNav, app),
			groupPickerItem("Layout", groupLayout, app),
			groupPickerItem("Feedback", groupFeedback, app),
			groupPickerItem("Overlays", groupOverlays, app),
		},
	})
}

func groupPickerItem(label, key string, app *ShowcaseApp) gui.View {
	t := gui.CurrentTheme()
	selected := app.SelectedGroup == key
	color := t.ColorBackground
	if selected {
		color = t.ColorActive
	}

	return gui.Button(gui.ButtonCfg{
		ID:          "grp-" + key,
		Color:       color,
		ColorBorder: color,
		Radius:      gui.SomeF(3),
		Padding:     gui.SomeP(3, 6, 3, 6),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.N5}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			showcaseApp := gui.State[ShowcaseApp](w)
			showcaseApp.SelectedGroup = key
			showcaseApp.ShowDocs = false
			showcaseApp.NavQuery = ""
			entries := filteredEntries(showcaseApp)
			showcaseApp.SelectedComponent = preferredComponentForGroup(entries)
			w.ScrollVerticalTo(scrollCatalog, 0)
			w.ScrollVerticalTo(scrollDetail, 0)
			w.ScrollHorizontalTo(scrollDetail, 0)
			e.IsHandled = true
		},
	})
}

func catalogRows(entries []DemoEntry, app *ShowcaseApp) []gui.View {
	t := gui.CurrentTheme()
	if len(entries) == 0 {
		return []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "No matching components",
				TextStyle: t.N4,
			}),
		}
	}

	rows := make([]gui.View, 0, len(entries)+len(demoGroups)*2)
	for _, group := range demoGroups {
		groupEntries := make([]DemoEntry, 0, 8)
		for _, entry := range entries {
			if entry.Group == group.Key {
				groupEntries = append(groupEntries, entry)
			}
		}
		if len(groupEntries) == 0 {
			continue
		}
		slices.SortStableFunc(groupEntries, entryCompare)
		if len(rows) > 0 {
			rows = append(rows, gui.Row(gui.ContainerCfg{
				Height:  6,
				Sizing:  gui.FillFixed,
				Padding: gui.NoPadding,
			}))
		}
		rows = append(rows, gui.Text(gui.TextCfg{
			Text:      group.Label,
			TextStyle: t.B5,
		}))
		for _, entry := range groupEntries {
			rows = append(rows, catalogRow(entry, app))
		}
	}
	return rows
}

func catalogRow(entry DemoEntry, app *ShowcaseApp) gui.View {
	t := gui.CurrentTheme()
	selected := app.SelectedComponent == entry.ID
	color := gui.ColorTransparent
	if selected {
		color = t.ColorActive
	}

	return gui.Button(gui.ButtonCfg{
		ID:               "cat-" + entry.ID,
		Sizing:           gui.FillFit,
		Color:            color,
		ColorHover:       t.MenubarStyle.ColorSelect,
		ColorClick:       t.ColorActive,
		ColorFocus:       color,
		ColorBorder:      gui.ColorTransparent,
		ColorBorderFocus: gui.ColorTransparent,
		Radius:           gui.SomeF(4),
		Padding:          gui.SomeP(3, 6, 3, 6),
		HAlign:           gui.Some(gui.HAlignLeft),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: entry.Label, TextStyle: t.N4}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			showcaseApp := gui.State[ShowcaseApp](w)
			showcaseApp.SelectedComponent = entry.ID
			showcaseApp.ShowDocs = false
			w.ScrollVerticalTo(scrollDetail, 0)
			w.ScrollHorizontalTo(scrollDetail, 0)
			e.IsHandled = true
		},
	})
}

func toggleLocale(app *ShowcaseApp) gui.View {
	label := localeLabel(app.LocaleIndex)
	return gui.Button(gui.ButtonCfg{
		ID:               "showcase-locale",
		Color:            gui.ColorTransparent,
		ColorBorder:      gui.ColorTransparent,
		ColorBorderFocus: gui.CurrentTheme().ColorActive,
		Padding:          gui.SomeP(6, 8, 6, 8),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: gui.CurrentTheme().B4,
			}),
		},
		OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
			showcaseApp := gui.State[ShowcaseApp](w)
			showcaseApp.LocaleIndex = (showcaseApp.LocaleIndex + 1) % localeCount()
			if locale, ok := showcaseLocaleAt(showcaseApp.LocaleIndex); ok {
				w.SetLocale(locale)
			}
			e.IsHandled = true
		},
	})
}
