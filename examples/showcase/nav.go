package main

import "github.com/go-gui-org/go-gui/gui"

// Responsive navigation. A wide window shows the catalog inline beside
// the page, and the reader can collapse it. Below navBreakpoint the
// catalog leaves the row: the page takes the full width under a top
// bar, and the catalog opens as a drawer over it.
const (
	// navBreakpoint is the narrowest window, in logical pixels, that
	// keeps the catalog inline: the catalog plus a page wide enough to
	// read.
	navBreakpoint float32 = 780

	// navDrawerGap is the strip of page the drawer leaves uncovered on
	// a phone, so the reader sees there is a page to go back to and
	// has somewhere to tap to close.
	navDrawerGap float32 = 56

	// catalogScopeID is the catalog's own ID, inline and in the
	// drawer alike, so the widgets inside it keep one effective ID in
	// both.
	catalogScopeID = "showcase-catalog"

	// navOpenID is the top bar's button; navToggleID is the one in the
	// catalog header. Both can be on screen while the drawer is open.
	navOpenID   = "showcase-nav-open"
	navToggleID = "showcase-nav-toggle"
)

// isNarrow reports whether the window is too narrow to keep the
// catalog inline.
func isNarrow(w *gui.Window) bool {
	width, _ := w.WindowSize()
	return float32(width) < navBreakpoint
}

// catalogInline reports whether the catalog shows beside the page.
func catalogInline(w *gui.Window) bool {
	return !isNarrow(w) && !appState(w).CatalogCollapsed
}

func mainView(w *gui.Window) gui.View {
	app := appState(w)
	var content []gui.View
	switch {
	case isNarrow(w):
		content = []gui.View{navTopBar(w), detailPanel(w)}
		if app.CatalogDrawerOpen {
			content = append(content, catalogDrawer(w))
		}
	default:
		// The sidebar stays in the row while collapsed, so it slides
		// shut instead of vanishing; the top bar takes over its menu
		// button.
		if app.CatalogCollapsed {
			content = append(content, navTopBar(w))
		}
		content = append(content, gui.Row(gui.ContainerCfg{
			Sizing:  gui.FillFill,
			Padding: gui.NoPadding,
			Spacing: gui.NoSpacing,
			Content: []gui.View{catalogPanel(w), detailPanel(w)},
		}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.NoPadding,
		Spacing: gui.NoSpacing,
		Content: content,
	})
}

// navTopBar replaces the inline catalog when it is not shown: a menu
// button to bring it back and the current page's name. It sits above
// the scrolling page, so the button never scrolls away.
func navTopBar(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	title := "Showcase"
	if entries := filteredEntries(app); len(entries) > 0 {
		title = selectedEntry(entries, app.SelectedComponent).Label
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Color:   t.ColorPanel,
		Padding: gui.PaddingSmall,
		Spacing: gui.SpacingSmall,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			navToggleButton(navOpenID, "Show component catalog"),
			gui.Text(gui.TextCfg{Text: title, TextStyle: t.TextStyleTitle}),
		},
	})
}

// catalogDrawer floats the catalog over the page on a narrow window.
// The backdrop dims the page and closes the drawer on a tap.
func catalogDrawer(w *gui.Window) gui.View {
	width, height := w.WindowSize()
	drawerW := min(catalogWidth, float32(width)-navDrawerGap)
	return gui.Column(gui.ContainerCfg{
		Float:      true,
		Sizing:     gui.FillFill,
		Color:      gui.RGBA(0, 0, 0, 120),
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignLeft,
		OnClick: func(ctx gui.EventCtx) {
			closeCatalogDrawer(ctx.Window)
			// The tap closes the drawer; it is not for the page under
			// the backdrop.
			ctx.Consume()
		},
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				ID: catalogScopeID,
				// A float has no parent height to fill, so the drawer
				// takes the window's; Fill would grow it to the whole
				// list and push the footer off screen.
				Width:      drawerW,
				Height:     float32(height),
				Sizing:     gui.FixedFixed,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				A11YRole:   gui.AccessRoleDialog,
				A11YCfg:    gui.A11YCfg{A11YLabel: "Component catalog"},
				OnClick: func(ctx gui.EventCtx) {
					// Keep a tap on the drawer's own background from
					// reaching the backdrop, which would close it.
					ctx.Consume()
				},
				Content: []gui.View{catalogContent(w, drawerW)},
			}),
		},
	})
}

// navToggleButton shows or hides the catalog: inline on a wide window,
// as a drawer on a narrow one.
func navToggleButton(id, label string) gui.View {
	t := gui.CurrentTheme()
	return gui.Button(gui.ButtonCfg{
		ID:         id,
		A11YCfg:    gui.A11YCfg{A11YLabel: label},
		Color:      gui.ColorTransparent,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingSmall,
		Radius:     gui.RadiusSmall,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: gui.IconBar, TextStyle: t.TextStyleIconMedium}),
		},
		OnClick: func(ctx gui.EventCtx) {
			toggleCatalog(ctx.Window)
			ctx.Consume()
		},
	})
}

// toggleCatalog shows or hides the catalog and moves focus to the menu
// button that is on screen afterwards, since the one pressed may be
// gone.
func toggleCatalog(w *gui.Window) {
	app := appState(w)
	if isNarrow(w) {
		app.CatalogDrawerOpen = !app.CatalogDrawerOpen
	} else {
		app.CatalogCollapsed = !app.CatalogCollapsed
	}
	if app.CatalogDrawerOpen || catalogInline(w) {
		w.SetFocus(gui.ScopeID(catalogScopeID, navToggleID))
	} else {
		w.SetFocus(navOpenID)
	}
}

// closeCatalogDrawer shuts the drawer and gives focus back to the top
// bar's menu button, which opened it.
func closeCatalogDrawer(w *gui.Window) {
	appState(w).CatalogDrawerOpen = false
	w.SetFocus(navOpenID)
}

// closeCatalogCommand closes the drawer on Escape. It is a fallback
// command, so a focused widget that uses Escape itself keeps it.
func closeCatalogCommand() gui.Command {
	return gui.Command{
		ID:       "sc.closeCatalog",
		Label:    "Close Catalog",
		Shortcut: gui.Shortcut{Key: gui.KeyEscape},
		CanExecute: func(w *gui.Window) bool {
			return appState(w).CatalogDrawerOpen && isNarrow(w)
		},
		Execute: func(_ *gui.Event, w *gui.Window) {
			closeCatalogDrawer(w)
		},
	}
}
