package main

import (
	"slices"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// catalogPanel is the inline catalog of a wide window. It slides shut
// when the reader collapses it; the content keeps its full width and
// is clipped while it slides, so the rows do not reflow.
func catalogPanel(w *gui.Window) gui.View {
	app := appState(w)
	return w.Sidebar(gui.SidebarCfg{
		ID:      catalogScopeID,
		A11YCfg: gui.A11YCfg{A11YLabel: catalogA11YLabel},
		Width:   catalogWidth,
		Open:    !app.CatalogCollapsed,
		Padding: gui.NoPadding,
		Color:   gui.CurrentTheme().ColorPanel,
		Content: []gui.View{catalogContent(w, catalogWidth)},
	})
}

// catalogContent is the catalog itself, at a fixed width: inline in
// the sidebar on a wide window, or inside the drawer on a narrow one.
func catalogContent(w *gui.Window, width float32) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	entries := filteredEntries(app)

	// Generation must not write state or scroll synchronously, so a
	// selection repair goes through QueueCommand (see detail.go): this
	// frame renders the stale selection and the fix lands next frame.
	fixSelection := len(entries) > 0 && !hasEntry(entries, app.SelectedComponent)
	clearSelection := len(entries) == 0 && app.SelectedComponent != ""
	if fixSelection || clearSelection {
		w.QueueCommand(func(w *gui.Window) {
			fixed := appState(w)
			latest := filteredEntries(fixed)
			switch {
			case len(latest) == 0:
				fixed.SelectedComponent = ""
			case !hasEntry(latest, fixed.SelectedComponent):
				fixed.SelectedComponent = preferredComponentForGroup(latest)
			default:
				return
			}
			w.ScrollVerticalTo(scrollDetail, 0)
			w.ScrollHorizontalTo(scrollDetail, 0)
			w.InvalidateLayout()
		})
	}

	return gui.Column(gui.ContainerCfg{
		Width:   width,
		Sizing:  gui.FixedFill,
		Color:   t.ColorPanel,
		Padding: gui.PaddingMedium,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Spacing:    gui.SpacingSmall,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					navToggleButton(navToggleID, "Hide component catalog"),
					gui.Text(gui.TextCfg{Text: "Component Catalog", TextStyle: t.TextStyleTitle}),
				},
			}),
			searchInput(app),
			line(),
			catalogList(entries, app),
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SpacingMedium,
				HAlign:  gui.HAlignRight,
				VAlign:  gui.VAlignMiddle,
				Content: []gui.View{
					toggleLocale(w, app),
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

// catalogItem is one row of the catalog list: a group header when
// entry is nil, otherwise a demo entry.
type catalogItem struct {
	group *DemoGroup
	entry *DemoEntry
	// first marks the list's first header, which has no gap above it.
	first bool
}

// catalogItems flattens the filtered entries into list rows: each
// non-empty group's header, then its entries sorted by label. Start
// stays first; the rest render in alphabetical order by label.
func catalogItems(entries []DemoEntry) []catalogItem {
	items := make([]catalogItem, 0, len(entries)+len(demoGroups))
	ordered := make([]*DemoGroup, 0, len(demoGroups))
	for gi := range demoGroups {
		ordered = append(ordered, &demoGroups[gi])
	}
	slices.SortFunc(ordered, func(a, b *DemoGroup) int {
		if a.Key == groupStart {
			return -1
		}
		if b.Key == groupStart {
			return 1
		}
		return strings.Compare(a.Label, b.Label)
	})
	for _, group := range ordered {
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
					TextStyle: t.TextStyleSecondary,
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

// catalogHeader is a group's label. Headings and rows both stay at
// full intensity, differing by size and weight. VirtualList puts no
// gap between rows, so the row carries its own: a group gap above it
// unless it is the first, and the tight gap below.
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
				TextStyle: t.TextStyleBodySmall.Bold(),
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

// catalogButton is the clickable part of a catalog row. Row text
// stays at full intensity for readability.
func catalogButton(entry DemoEntry, color gui.Color) gui.View {
	t := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		ID:      gui.ScopeID("showcase", "cat", entry.ID),
		Sizing:  gui.FillFit,
		Color:   color,
		Colors:  gui.ColorSet{Hover: t.ColorHover, Click: t.ColorActive, Focus: color, Border: gui.ColorTransparent, BorderFocus: gui.ColorTransparent},
		Radius:  gui.RadiusSmall,
		Padding: gui.PaddingSmall,
		HAlign:  gui.Some(gui.HAlignLeft),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: entry.Label, TextStyle: t.TextStyleBody}),
		},
		OnClick: func(ctx gui.EventCtx) {
			showcaseApp := appState(ctx.Window)
			showcaseApp.SelectedComponent = entry.ID
			ctx.Window.ScrollVerticalTo(scrollDetail, 0)
			ctx.Window.ScrollHorizontalTo(scrollDetail, 0)
			// A pick in the drawer is the reader done choosing: show
			// the page they picked.
			if showcaseApp.CatalogDrawerOpen {
				closeCatalogDrawer(ctx.Window)
			}
		},
	})
}

// localeButtonID is the leaf ID of the catalog's locale button.
const localeButtonID = "showcase-locale"

// localeTooltipID is the leaf ID of the locale button's tooltip.
// WithTooltip falls back to its text for an ID, and that text holds a
// colon, which would make the key an absolute ID.
const localeTooltipID = "showcase-locale-tip"

// localeButtonTooltip says what the locale button changes. Without it
// a press looks like it did nothing: the showcase's own text is not
// translated, so the change shows only on a page that formats numbers,
// dates or widget strings.
const localeButtonTooltip = "Switch the locale: number and date formats, " +
	"widget strings and text direction. The showcase text is not " +
	"translated. See App & Window > Locale."

// localeButtonA11YLabel names the action and the locale the button is
// on now. The visible text is a two-letter code, which a screen reader
// reads as letters.
func localeButtonA11YLabel(idx int) string {
	id := "en-US"
	if idx >= 0 && idx < len(showcaseLocaleIDs) {
		id = showcaseLocaleIDs[idx]
	}
	return "Switch locale, now " + id
}

func toggleLocale(w *gui.Window, app *ShowcaseApp) gui.View {
	label := localeLabel(app.LocaleIndex)
	return gui.WithTooltip(w, gui.WithTooltipCfg{
		ID:   localeTooltipID,
		Text: localeButtonTooltip,
		Content: []gui.View{
			gui.Button(gui.ButtonCfg{
				ID:      localeButtonID,
				A11YCfg: gui.A11YCfg{A11YLabel: localeButtonA11YLabel(app.LocaleIndex)},
				Color:   gui.ColorTransparent,
				Colors:  gui.ColorSet{Border: gui.ColorTransparent, BorderFocus: gui.CurrentTheme().ColorActive},
				Padding: gui.PaddingSmall,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      label,
						TextStyle: gui.CurrentTheme().TextStyleBody.Bold(),
					}),
				},
				OnClick: func(ctx gui.EventCtx) {
					next := (appState(ctx.Window).LocaleIndex + 1) % localeCount()
					setShowcaseLocale(ctx.Window, next)
					ctx.Consume()
				},
			}),
		},
	})
}
