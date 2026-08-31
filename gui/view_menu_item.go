package gui

// Menu item sentinel IDs.
const (
	menuSeparatorID  = "__separator__"
	menuSubtitleID   = "__subtitle__"
	submenuIndicator = "  \u203A"
)

// MenuItemCfg configures a single menu item. Items may be
// text, separators, subtitles, or submenus.
type MenuItemCfg struct {
	textStyle  TextStyle
	CustomView View
	Action     func(*MenuItemCfg, EventCtx)

	// Public configuration.
	ID        string
	Text      string
	CommandID string // auto-fill from registered command

	// Internal — resolved shortcut hint text.
	shortcutText string
	Submenu      []MenuItemCfg
	level        int
	Padding      Padding
	radius       float32
	spacing      float32
	// Internal — set by menuBuild from theme/context.
	colorSelect Color
	// Internal — text color over the selected item's fill, set by
	// menuBuild from the theme (issue #373).
	colorTextOnSelect Color
	sizing            Sizing
	disabled          bool
	selected          bool

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
		ID:        menuSeparatorID,
		Separator: true,
	}
}

// MenuSubtitle creates a disabled subtitle item.
func MenuSubtitle(text string) MenuItemCfg {
	return MenuItemCfg{
		ID:       menuSubtitleID,
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
		return Separator(SeparatorCfg{
			Color: menubarCfg.ColorBorder,
			Inset: NewPadding(2, 0, 2, 0),
		})
	}

	itemColor := ColorTransparent
	if itemCfg.selected {
		itemColor = itemCfg.colorSelect
	}

	// A menu item's label is the app's, and static per call site, so the
	// ownership rule admits the correction — but the band is the face's,
	// not each run's. Items stack in a list at a regular pitch, so
	// measuring per run would move a descender-free label down while
	// leaving its descending neighbour where it was, and the unevenness
	// reads down the whole menu. Cap band, whatever the item says
	// (issue #346). Set below only where the widget owns the text — a
	// CustomView's content is the app's to place.
	var opticalAmend func(EventCtx)

	// The selected fill and the text on it travel together: a
	// selected item draws its label — and the shortcut hint derived
	// from it — in the paired foreground (issue #373).
	textStyle := textOnFill(itemCfg.textStyle,
		itemCfg.selected, itemCfg.colorTextOnSelect)

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
		// A wrapping label is included, unlike Select's and Input's.
		// The exclusion there is about a *block* whose later lines the
		// text layout places against a top-aligned box; a menu item's
		// box hugs its label, so the reserved descent that goes unused
		// is the last line's either way, and the offset is the same
		// one.
		opticalAmend = opticalCenterLabelText
		label := Text(TextCfg{
			Text:      textContent,
			TextStyle: textStyle,
			Mode:      mode,
		})
		if itemCfg.shortcutText != "" && itemCfg.level > 0 {
			// A shortcut hint is supporting text on a live item.
			// It used to borrow dimAlpha, the disabled dim, which
			// made an enabled item read as a dead one (issue #335).
			hintStyle := withRoleAlpha(
				textStyle, guiTheme.TextStyleSecondary)
			// The label and its shortcut hint are direct children of
			// this row, so the correction goes here and reaches both —
			// they must move together or the pair reads skewed. The
			// outer column then has no text child of its own left to
			// correct.
			rowAmend := opticalAmend
			opticalAmend = nil
			content = Row(ContainerCfg{
				Sizing:      FillFit,
				Padding:     NoPadding,
				SizeBorder:  NoBorder,
				AmendLayout: rowAmend,
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
	cfgFocusID := menubarCfg.ID

	var onHover func(EventCtx)
	if !itemCfg.disabled {
		onHover = func(ctx EventCtx) {
			if !ctx.Window.IsFocus(cfgFocusID) {
				return
			}
			if ctx.Window.viewState.menuKeyNav {
				return
			}
			ctx.Window.setMouseCursor(CursorPointingHand)
			sm := StateMap[string, string](
				ctx.Window, nsMenu, capModerate)
			// Default empty string: absent means no hover; compared
			// with target itemID.
			cur := sm.GetOr(cfgFocusID, "")
			if cur != itemID {
				sm.Set(cfgFocusID, itemID)
			}
		}
	}

	// A separator and a subtitle are not activations — they carry no
	// Action and a subtitle is disabled — so they stay silent. A live
	// item is a momentary activation, which is the click role, not the
	// selection one: a menu is a list of commands, not of options
	// (issue #467).
	itemSound := SoundNone
	if !itemCfg.Separator && !itemCfg.disabled {
		itemSound = resolveSoundCue(
			guiTheme.Sounds.Click, menubarCfg.Sound, menubarCfg.SoundDisabled)
	}

	itemContent := make([]View, 0, 1+len(extra))
	itemContent = append(itemContent, content)
	itemContent = append(itemContent, extra...)

	return Column(ContainerCfg{
		ID:       itemCfg.ID,
		A11YRole: AccessRoleMenuItem,
		A11YCfg:  A11YCfg{A11YLabel: a11yLabel("", itemCfg.Text)},
		Color:    itemColor,
		Sizing:   itemCfg.sizing,
		Padding:  itemCfg.Padding,
		Radius:   Some(itemCfg.radius),
		Disabled: itemCfg.disabled,
		Sound:    itemSound,
		OnClick:  menuItemClick(menubarCfg, itemCfg),
		OnHover:  onHover,
		// Reaches the label only: an attached submenu is a container,
		// not a text shape, so the hook passes over it.
		AmendLayout: opticalAmend,
		Content:     itemContent,
	})
}

// menuItemClick returns the OnClick handler for a menu item.
func menuItemClick(cfg MenubarCfg, itemCfg MenuItemCfg) func(EventCtx) {
	return func(ctx EventCtx) {
		ctx.Window.SetFocus(cfg.ID)

		if !isSelectableMenuID(itemCfg.ID) {
			return
		}

		sm := StateMap[string, string](
			ctx.Window, nsMenu, capModerate)
		sm.Set(cfg.ID, itemCfg.ID)

		if itemCfg.Action != nil {
			itemCfg.Action(&itemCfg, EventCtx{nil, ctx.Event, ctx.Window})
		}
		focusBeforeAction := ctx.Window.FocusID()
		if cfg.Action != nil {
			cfg.Action(itemCfg.ID, EventCtx{nil, ctx.Event, ctx.Window})
		}

		// Close menu if leaf item (no submenu). Only reset focus to
		// zero if neither action callback changed it — an action that
		// restores a previous focus should win.
		if len(itemCfg.Submenu) == 0 {
			if ctx.Window.FocusID() == focusBeforeAction {
				ctx.Window.ClearFocus()
			}
			sm.Delete(cfg.ID)
		}

		ctx.Consume()
	}
}
