package main

import (
	"slices"

	"github.com/go-gui-org/go-gui/gui"
)

func catalogPanel(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	entries := filteredEntries(app)

	// Generation must not write state or scroll synchronously, so a
	// selection repair goes through QueueCommand (see detail.go): this
	// frame renders the stale selection and the fix lands next frame.
	fixSelection := len(entries) > 0 && !hasEntry(entries, app.SelectedComponent)
	clearSelection := len(entries) == 0 && (app.SelectedComponent != "" || app.ShowDocs)
	if fixSelection || clearSelection {
		w.QueueCommand(func(w *gui.Window) {
			fixed := appState(w)
			latest := filteredEntries(fixed)
			switch {
			case len(latest) == 0:
				fixed.SelectedComponent = ""
				fixed.ShowDocs = false
			case !hasEntry(latest, fixed.SelectedComponent):
				fixed.SelectedComponent = preferredComponentForGroup(latest)
				fixed.ShowDocs = false
			default:
				return
			}
			w.ScrollVerticalTo(scrollDetail, 0)
			w.ScrollHorizontalTo(scrollDetail, 0)
			w.InvalidateLayout()
		})
	}

	return gui.Column(gui.ContainerCfg{
		Width:   catalogWidth,
		Sizing:  gui.FixedFill,
		Color:   t.ColorPanel,
		Padding: gui.PaddingMedium,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Component Catalog", TextStyle: t.TextStyleTitleSmall}),
			searchInput(app),
			groupPicker(app),
			line(),
			catalogList(entries, app),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					toggleLocale(app),
					gui.ThemePicker(gui.ThemePickerCfg{
						ID:          "showcase-theme",
						Focusable:   true,
						FloatAnchor: gui.FloatTopRight,
						FloatTieOff: gui.FloatBottomRight,
						OnSelect: func(name string, ctx gui.EventCtx) {
							theme, ok := gui.ThemeGet(name)
							if !ok {
								return
							}
							syncThemeGenFromCfg(appState(ctx.Window), theme.Cfg)
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
		Sizing:      gui.FillFit,
		Text:        app.NavQuery,
		Placeholder: "Search controls...",
		OnTextChanged: func(text string, ctx gui.EventCtx) {
			appState(ctx.Window).NavQuery = text
		},
	})
}

// groupPicker shows "All", then one item per catalog group. It reads
// demoGroups, the same list the catalog headers read, so the two
// cannot drift apart in order or label (#1025).
func groupPicker(app *ShowcaseApp) gui.View {
	items := make([]gui.View, 0, len(demoGroups)+1)
	items = append(items, groupPickerItem("All", groupAll, app))
	for i := range demoGroups {
		items = append(items, groupPickerItem(demoGroups[i].Label, demoGroups[i].Key, app))
	}
	return gui.Wrap(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SpacingTight,
		Content: items,
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
		ID:      gui.ScopeID("showcase", "grp", key),
		Color:   color,
		Colors:  gui.ColorSet{Border: color},
		Radius:  gui.RadiusSmall,
		Padding: gui.PaddingTwoFive,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: label, TextStyle: t.TextStyleCaption}),
		},
		OnClick: func(ctx gui.EventCtx) {
			showcaseApp := appState(ctx.Window)
			showcaseApp.SelectedGroup = key
			showcaseApp.ShowDocs = false
			showcaseApp.NavQuery = ""
			entries := filteredEntries(showcaseApp)
			showcaseApp.SelectedComponent = preferredComponentForGroup(entries)
			ctx.Window.ScrollToIndex(scrollCatalog, 0)
			ctx.Window.ScrollVerticalTo(scrollDetail, 0)
			ctx.Window.ScrollHorizontalTo(scrollDetail, 0)
		},
	})
}

// catalogItem is one row of the catalog list: a group header when
// entry is nil, otherwise a demo entry.
type catalogItem struct {
	group *DemoGroup
	entry *DemoEntry
	// first marks the list's first header, which has no gap above it.
	first bool
}

// catalogItems flattens the filtered entries into list rows: each
// non-empty group's header, then its entries sorted by label.
func catalogItems(entries []DemoEntry) []catalogItem {
	items := make([]catalogItem, 0, len(entries)+len(demoGroups))
	for gi := range demoGroups {
		group := &demoGroups[gi]
		start := len(items) + 1
		for ei := range entries {
			if entries[ei].Group == group.Key {
				items = append(items, catalogItem{entry: &entries[ei]})
			}
		}
		if len(items) < start {
			continue
		}
		// Make room for the header in front of the group's entries. The
		// entries were appended first so a group with none adds nothing.
		items = slices.Insert(items, start-1, catalogItem{group: group, first: start == 1})
		slices.SortStableFunc(items[start:], func(a, b catalogItem) int {
			return entryCompare(*a.entry, *b.entry)
		})
	}
	return items
}

// catalogList shows the catalog in a VirtualList, so a frame builds
// only the rows in view rather than all of them. The catalog is in
// every frame, so this is most of the showcase's per-frame cost.
func catalogList(entries []DemoEntry, app *ShowcaseApp) gui.View {
	t := gui.CurrentTheme()
	if len(entries) == 0 {
		return gui.Column(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Padding: gui.NoPadding,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text:      "No matching components",
					TextStyle: t.TextStyleBodySmall,
				}),
			},
		})
	}

	items := catalogItems(entries)
	return gui.VirtualList(gui.VirtualListCfg{
		ID:         scrollCatalog,
		Sizing:     gui.FillFill,
		Color:      gui.ColorTransparent,
		Colors:     gui.ColorSet{Border: gui.ColorTransparent},
		Padding:    gui.NewPadding(0, t.ScrollbarStyle.Size+4, 0, 0),
		SizeBorder: gui.NoBorder,
		// The rows are buttons and take focus themselves; the list
		// stays out of the tab order, as the old Column did.
		FocusDisabled: true,
		ItemCount:     len(items),
		// Measured heights are stored under the key, so they follow a
		// row when the search or the group filter moves it. The first
		// header is shorter (no gap above it), so it has its own key.
		ItemKey: func(i int) string {
			it := items[i]
			switch {
			case it.entry != nil:
				return it.entry.ID
			case it.first:
				return it.group.firstHeaderKey
			default:
				return it.group.headerKey
			}
		},
		ItemView: func(i int, _ float32) gui.View {
			it := items[i]
			if it.entry != nil {
				return catalogRow(*it.entry, app)
			}
			return catalogHeader(it.group, it.first)
		},
	})
}

// catalogHeader is a group's label. VirtualList puts no gap between
// rows, so the row carries its own: a group gap above it unless it is
// the first, and the tight gap below.
func catalogHeader(group *DemoGroup, first bool) gui.View {
	t := gui.CurrentTheme()
	var top float32
	if !first {
		top = 6 + t.SpacingTight
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NewPadding(top, 0, t.SpacingTight, 0),
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      group.Label,
				TextStyle: t.TextStyleCaption.Bold(),
			}),
		},
	})
}

func catalogRow(entry DemoEntry, app *ShowcaseApp) gui.View {
	t := gui.CurrentTheme()
	selected := app.SelectedComponent == entry.ID
	color := gui.ColorTransparent
	if selected {
		color = t.ColorActive
	}

	// VirtualList puts no gap between rows, so the row carries the
	// tight gap below itself.
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Padding:    gui.NewPadding(0, 0, t.SpacingTight, 0),
		SizeBorder: gui.NoBorder,
		Content:    []gui.View{catalogButton(entry, color)},
	})
}

// catalogButton is the clickable part of a catalog row.
func catalogButton(entry DemoEntry, color gui.Color) gui.View {
	t := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		ID:      gui.ScopeID("showcase", "cat", entry.ID),
		Sizing:  gui.FillFit,
		Color:   color,
		Colors:  gui.ColorSet{Hover: t.ColorHover, Click: t.ColorActive, Focus: color, Border: gui.ColorTransparent, BorderFocus: gui.ColorTransparent},
		Radius:  gui.RadiusSmall,
		Padding: gui.PaddingTwoFive,
		HAlign:  gui.Some(gui.HAlignLeft),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: entry.Label, TextStyle: t.TextStyleBodySmall}),
		},
		OnClick: func(ctx gui.EventCtx) {
			showcaseApp := appState(ctx.Window)
			showcaseApp.SelectedComponent = entry.ID
			showcaseApp.ShowDocs = false
			ctx.Window.ScrollVerticalTo(scrollDetail, 0)
			ctx.Window.ScrollHorizontalTo(scrollDetail, 0)
		},
	})
}

func toggleLocale(app *ShowcaseApp) gui.View {
	label := localeLabel(app.LocaleIndex)
	return gui.Button(gui.ButtonCfg{
		ID:      "showcase-locale",
		Color:   gui.ColorTransparent,
		Colors:  gui.ColorSet{Border: gui.ColorTransparent, BorderFocus: gui.CurrentTheme().ColorActive},
		Padding: gui.PaddingSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: gui.CurrentTheme().TextStyleBodySmall.Bold(),
			}),
		},
		OnClick: func(ctx gui.EventCtx) {
			showcaseApp := appState(ctx.Window)
			showcaseApp.LocaleIndex = (showcaseApp.LocaleIndex + 1) % localeCount()
			if locale, ok := showcaseLocaleAt(showcaseApp.LocaleIndex); ok {
				ctx.Window.SetLocale(locale)
			}
		},
	})
}
