package gui

// Menu item sentinel IDs.
const (
	MenuSeparatorID  = "__separator__"
	MenuSubtitleID   = "__subtitle__"
	submenuIndicator = "  \u203A"
)

// MenuItemCfg configures a single menu item. Items may be
// text, separators, subtitles, or submenus.
type MenuItemCfg struct {
	textStyle  TextStyle
	CustomView View
	Action     func(*MenuItemCfg, *Event, *Window)

	// Public configuration.
	ID        string
	Text      string
	CommandID string // auto-fill from registered command

	// Internal — resolved shortcut hint text.
	shortcutText string
	Submenu      []MenuItemCfg
	level        int
	Padding      Opt[Padding]
	radius       float32
	spacing      float32
	// Internal — set by menuBuild from theme/context.
	colorSelect Color
	sizing      Sizing
	disabled    bool
	selected    bool

	Separator bool
}

// MenuItemText creates a simple text menu item.
func MenuItemText(id, text string) MenuItemCfg {
	return MenuItemCfg{
		ID:   id,
		Text: text,
	}
}

// MenuSeparator creates a separator line.
func MenuSeparator() MenuItemCfg {
	return MenuItemCfg{
		ID:        MenuSeparatorID,
		Separator: true,
	}
}

// MenuSubtitle creates a disabled subtitle item.
func MenuSubtitle(text string) MenuItemCfg {
	return MenuItemCfg{
		ID:       MenuSubtitleID,
		Text:     text,
		disabled: true,
	}
}

// MenuSubmenu creates an item with a submenu. A "›" indicator
// is appended for nested submenus (not top-level menubar items).
func MenuSubmenu(id, text string, submenu []MenuItemCfg) MenuItemCfg {
	return MenuItemCfg{
		ID:      id,
		Text:    text,
		Submenu: submenu,
	}
}

// menuItem builds the View for a single menu item.
func menuItem(menubarCfg MenubarCfg, itemCfg MenuItemCfg, extra ...View) View {
	if itemCfg.Separator {
		return Column(ContainerCfg{
			Sizing:  FillFit,
			Padding: SomeP(2, 0, 2, 0),
			Content: []View{
				Rectangle(RectangleCfg{
					Height: 1,
					Sizing: FillFit,
					Color:  menubarCfg.ColorBorder,
				}),
			},
		})
	}

	itemColor := ColorTransparent
	if itemCfg.selected {
		itemColor = itemCfg.colorSelect
	}

	var content View
	if itemCfg.CustomView != nil {
		content = itemCfg.CustomView
	} else {
		textContent := itemCfg.Text
		if len(itemCfg.Submenu) > 0 && itemCfg.level > 0 {
			textContent += submenuIndicator
		}
		mode := TextModeSingleLine
		if itemCfg.sizing == FillFit {
			mode = TextModeWrap
		}
		label := Text(TextCfg{
			Text:      textContent,
			TextStyle: itemCfg.textStyle,
			Mode:      mode,
		})
		if itemCfg.shortcutText != "" && itemCfg.level > 0 {
			hintStyle := itemCfg.textStyle
			hintStyle.Color = dimAlpha(hintStyle.Color)
			content = Row(ContainerCfg{
				Sizing:     FillFit,
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content: []View{
					label,
					Rectangle(RectangleCfg{
						Sizing: FillFit,
					}),
					Text(TextCfg{
						Text:      itemCfg.shortcutText,
						TextStyle: hintStyle,
						Mode:      TextModeSingleLine,
					}),
				},
			})
		} else {
			content = label
		}
	}

	itemID := itemCfg.ID
	cfgIDFocus := menubarCfg.IDFocus

	var onHover func(*Layout, *Event, *Window)
	if !itemCfg.disabled {
		onHover = func(_ *Layout, _ *Event, w *Window) {
			if !w.IsFocus(cfgIDFocus) {
				return
			}
			if w.viewState.menuKeyNav {
				return
			}
			w.SetMouseCursor(CursorPointingHand)
			sm := StateMap[uint32, string](
				w, nsMenu, capModerate)
			cur, _ := sm.Get(cfgIDFocus)
			if cur != itemID {
				sm.Set(cfgIDFocus, itemID)
			}
		}
	}

	itemContent := make([]View, 0, 1+len(extra))
	itemContent = append(itemContent, content)
	itemContent = append(itemContent, extra...)

	return Column(ContainerCfg{
		ID:        itemCfg.ID,
		A11YRole:  AccessRoleMenuItem,
		A11YLabel: a11yLabel("", itemCfg.Text),
		Color:     itemColor,
		Sizing:    itemCfg.sizing,
		Padding:   itemCfg.Padding,
		Radius:    Some(itemCfg.radius),
		Disabled:  itemCfg.disabled,
		OnClick:   menuItemClick(menubarCfg, itemCfg),
		OnHover:   onHover,
		Content:   itemContent,
	})
}

// menuItemClick returns the OnClick handler for a menu item.
func menuItemClick(cfg MenubarCfg, itemCfg MenuItemCfg) func(*Layout, *Event, *Window) {
	return leftClickOnly(func(_ *Layout, e *Event, w *Window) {
		w.SetIDFocus(cfg.IDFocus)

		if !isSelectableMenuID(itemCfg.ID) {
			return
		}

		sm := StateMap[uint32, string](
			w, nsMenu, capModerate)
		sm.Set(cfg.IDFocus, itemCfg.ID)

		if itemCfg.Action != nil {
			itemCfg.Action(&itemCfg, e, w)
		}
		focusBeforeAction := w.IDFocus()
		if cfg.Action != nil {
			cfg.Action(itemCfg.ID, e, w)
		}

		// Close menu if leaf item (no submenu). Only reset focus to
		// zero if neither action callback changed it — an action that
		// restores a previous focus should win.
		if len(itemCfg.Submenu) == 0 {
			if w.IDFocus() == focusBeforeAction {
				w.SetIDFocus(0)
			}
			sm.Delete(cfg.IDFocus)
		}

		e.IsHandled = true
	})
}
