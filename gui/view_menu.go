package gui

import "fmt"

// Menu creates a standalone columnar menu (used by
// OverflowPanel and context menus). Supports keyboard
// navigation: Up/Down to move, Enter/Space to select,
// Escape to close, Right/Left for submenus.
func Menu(w *Window, cfg MenubarCfg) View {
	applyMenubarDefaults(&cfg)
	if cfg.IDFocus == 0 {
		cfg.IDFocus = FnvSum32("menu_" + cfg.ID)
	}
	checkForDuplicateMenuIDs(cfg.Items)

	// Auto-select first item on focus.
	if w.IsFocus(cfg.IDFocus) {
		sel := StateReadOr(
			w, nsMenu, cfg.IDFocus, "")
		if sel == "" {
			if first, ok := firstSelectable(cfg.Items); ok {
				sm := StateMap[uint32, string](
					w, nsMenu, capModerate)
				sm.Set(cfg.IDFocus, first.ID)
			}
		}
	}

	return Column(ContainerCfg{
		ID:            cfg.ID,
		A11YRole:      AccessRoleMenu,
		Color:         cfg.Color,
		ColorBorder:   cfg.ColorBorder,
		SizeBorder:    cfg.SizeBorder,
		Radius:        cfg.RadiusBorder,
		MinWidth:      cfg.WidthSubmenuMin.Get(DefaultMenubarStyle.WidthSubmenuMin),
		MaxWidth:      cfg.WidthSubmenuMax.Get(DefaultMenubarStyle.WidthSubmenuMax),
		Spacing:       Some(cfg.SpacingSubmenu.Get(DefaultMenubarStyle.SpacingSubmenu)),
		Padding:       cfg.PaddingSubmenu,
		Float:         cfg.Float,
		FloatAutoFlip: cfg.FloatAutoFlip,
		FloatAnchor:   cfg.FloatAnchor,
		FloatTieOff:   cfg.FloatTieOff,
		FloatOffsetX:  cfg.FloatOffsetX,
		FloatOffsetY:  cfg.FloatOffsetY,
		IDFocus:       cfg.IDFocus,
		OnKeyDown:     makeMenuOnKeyDown(cfg),
		AmendLayout:   makeMenuAmendLayout(cfg.IDFocus),
		Content:       menuBuild(cfg, 1, cfg.Items, w),
	})
}

// makeMenuOnKeyDown returns the keyboard handler for a
// standalone vertical menu.
func makeMenuOnKeyDown(cfg MenubarCfg) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		menuOnKeyDown(cfg, menuMapperVertical, e, w)
	}
}

// menuOnKeyDown is the shared keyboard handler for both
// standalone menus and menubars. The mapper function builds
// the directional navigation graph for arrow-key movement.
func menuOnKeyDown(cfg MenubarCfg,
	mapper func([]MenuItemCfg) MenuIDMap,
	e *Event, w *Window) {

	sm := StateMap[uint32, string](w, nsMenu, capModerate)

	switch e.KeyCode {
	case KeyEscape:
		w.SetIDFocus(0)
		sm.Delete(cfg.IDFocus)
		e.IsHandled = true

	case KeySpace, KeyEnter:
		sel, _ := sm.Get(cfg.IDFocus) // ok ignored: empty string checked immediately below
		if sel == "" {
			return
		}
		item, found := findMenuItemCfg(cfg.Items, sel)
		if !found {
			return
		}
		if item.Action != nil {
			item.Action(&item, e, w)
		}
		if cfg.Action != nil {
			cfg.Action(sel, e, w)
		}
		if len(item.Submenu) == 0 {
			w.SetIDFocus(0)
			sm.Delete(cfg.IDFocus)
		}
		e.IsHandled = true

	case KeyLeft, KeyRight, KeyUp, KeyDown:
		sel, _ := sm.Get(cfg.IDFocus) // ok ignored: empty string checked immediately below
		if sel == "" {
			return
		}
		idMap := mapper(cfg.Items)
		node, ok := idMap[sel]
		if !ok {
			return
		}
		var target string
		switch e.KeyCode {
		case KeyLeft:
			target = node.Left
		case KeyRight:
			target = node.Right
		case KeyUp:
			target = node.Up
		case KeyDown:
			target = node.Down
		}
		if target != "" && target != sel {
			sm.Set(cfg.IDFocus, target)
			w.viewState.menuKeyNav = true
		}
		e.IsHandled = true
	}
}

// menuBuild recursively builds menu item views.
func menuBuild(cfg MenubarCfg, level int, items []MenuItemCfg, w *Window) []View {
	sizing := FillFit
	if level == 0 {
		sizing = FitFit
	}

	selectedID := StateReadOr(
		w, nsMenu, cfg.IDFocus, "")

	views := make([]View, 0, len(items))
	for _, item := range items {
		// Determine padding.
		pad := item.Padding
		if !pad.IsSet() {
			if item.CustomView != nil {
				pad = NoPadding
			} else if item.ID == menuSubtitleID {
				pad = cfg.PaddingSubtitle
			} else {
				pad = cfg.PaddingMenuItem
			}
		}

		// Determine text style.
		ts := cfg.TextStyle
		if item.ID == menuSubtitleID {
			ts = cfg.TextStyleSubtitle
		}

		// Build the configured item.
		configured := item
		configured.colorSelect = cfg.ColorSelect
		configured.Padding = pad
		configured.selected = (selectedID == item.ID)
		configured.sizing = sizing
		configured.radius = cfg.RadiusMenuItem.Get(DefaultMenubarStyle.RadiusMenuItem)
		configured.spacing = cfg.SpacingSubmenu.Get(DefaultMenubarStyle.SpacingSubmenu)
		configured.level = level
		configured.textStyle = ts

		// Resolve CommandID from registered commands.
		if configured.CommandID != "" {
			if cmd, ok := w.CommandByID(configured.CommandID); ok {
				if configured.Text == "" {
					configured.Text = cmd.Label
				}
				configured.shortcutText = cmd.Shortcut.String()
				if cmd.CanExecute != nil && !cmd.CanExecute(w) {
					configured.disabled = true
				}
				if configured.Action == nil {
					cmdExec := cmd.Execute
					cID := configured.CommandID
					configured.Action = func(
						_ *MenuItemCfg, e *Event, w *Window,
					) {
						if w.CommandCanExecute(cID) && cmdExec != nil {
							cmdExec(e, w)
						}
					}
				}
			}
		}

		// Attach submenu as child of menu item so float
		// positioning is relative to the item, not the bar.
		if len(item.Submenu) > 0 &&
			(selectedID == item.ID ||
				isMenuIDInTree(item.Submenu, selectedID)) {

			anchor := FloatTopRight
			tieOff := FloatTopLeft
			if level == 0 {
				anchor = FloatBottomLeft
				tieOff = FloatTopLeft
			}

			subViews := menuBuild(cfg, level+1,
				item.Submenu, w)

			submenu := Column(ContainerCfg{
				Color:         cfg.Color,
				ColorBorder:   cfg.ColorBorder,
				SizeBorder:    cfg.SizeBorder,
				Radius:        cfg.RadiusSubmenu,
				MinWidth:      cfg.WidthSubmenuMin.Get(DefaultMenubarStyle.WidthSubmenuMin),
				MaxWidth:      cfg.WidthSubmenuMax.Get(DefaultMenubarStyle.WidthSubmenuMax),
				Spacing:       Some(cfg.SpacingSubmenu.Get(DefaultMenubarStyle.SpacingSubmenu)),
				Padding:       cfg.PaddingSubmenu,
				Float:         true,
				FloatAutoFlip: true,
				FloatAnchor:   anchor,
				FloatTieOff:   tieOff,
				Content:       subViews,
			})
			views = append(views, menuItem(cfg, configured, submenu))
		} else {
			views = append(views, menuItem(cfg, configured))
		}
	}
	return views
}

// makeMenuAmendLayout clears menu selection when the widget
// loses focus.
func makeMenuAmendLayout(idFocus uint32) func(*Layout, *Window) {
	return func(_ *Layout, w *Window) {
		if !w.IsFocus(idFocus) {
			// StateMapRead returns nil if map not yet created.
			sm := StateMapRead[uint32, string](w, nsMenu)
			if sm != nil {
				sm.Delete(idFocus)
			}
		}
	}
}

// findMenuItemCfg recursively searches for a menu item by ID.
func findMenuItemCfg(items []MenuItemCfg, id string) (MenuItemCfg, bool) {
	for _, item := range items {
		if item.ID == id {
			return item, true
		}
		if found, ok := findMenuItemCfg(item.Submenu, id); ok {
			return found, true
		}
	}
	return MenuItemCfg{}, false
}

// checkForDuplicateMenuIDs panics if duplicate IDs exist
// (ignoring sentinel IDs).
func checkForDuplicateMenuIDs(items []MenuItemCfg) {
	seen := make(map[string]bool)
	if dup, ok := checkMenuIDs(items, seen); ok {
		panic(fmt.Sprintf("gui: duplicate menu item ID %q", dup))
	}
}

func checkMenuIDs(items []MenuItemCfg, seen map[string]bool) (string, bool) {
	for _, item := range items {
		if item.ID == menuSeparatorID || item.ID == menuSubtitleID {
			continue
		}
		if seen[item.ID] {
			return item.ID, true
		}
		seen[item.ID] = true
		if dup, ok := checkMenuIDs(item.Submenu, seen); ok {
			return dup, true
		}
	}
	return "", false
}

// isMenuIDInTree checks if an ID exists in a submenu tree.
func isMenuIDInTree(submenu []MenuItemCfg, id string) bool {
	for _, item := range submenu {
		if item.ID == id {
			return true
		}
		if isMenuIDInTree(item.Submenu, id) {
			return true
		}
	}
	return false
}
