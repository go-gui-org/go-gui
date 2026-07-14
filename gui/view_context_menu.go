package gui

// contextMenuState is the internal state for a ContextMenu widget.
type contextMenuState struct {
	Open bool
	X    float32
	Y    float32
}

// ContextMenuCfg configures a ContextMenu widget.
type ContextMenuCfg struct {
	TextStyle         TextStyle
	TextStyleSubtitle TextStyle
	Action            func(string, *Event, *Window)

	// User click handler — fires before context menu logic.
	OnAnyClick func(*Layout, *Event, *Window)

	ID    string `gui:"required"`
	Items []MenuItemCfg

	Content []View

	FloatZIndex int

	Padding Opt[Padding]

	PaddingMenuItem Opt[Padding]
	PaddingSubmenu  Opt[Padding]
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	RadiusMenuItem  Opt[float32]
	SpacingSubmenu  Opt[float32]
	WidthSubmenuMin Opt[float32]
	WidthSubmenuMax Opt[float32]

	Width  float32
	Height float32

	// Menu styling — optional, defaults from theme.
	Color       Color
	ColorBorder Color
	ColorSelect Color

	// Container passthrough (outer wrapper).
	Sizing Sizing
	HAlign HorizontalAlign
	VAlign VerticalAlign
}

// ContextMenu creates a container that opens a floating menu
// on right-click at the cursor position.
func ContextMenu(w *Window, cfg ContextMenuCfg) View {
	RequireID("ContextMenu", cfg.ID)
	checkForDuplicateMenuIDs(cfg.Items)
	applyContextMenuDefaults(&cfg)

	st := StateReadOr(
		w, nsContextMenu, cfg.ID, contextMenuState{})

	content := cfg.Content
	if st.Open && w.IsFocus(cfg.ID+"_popup") {
		content = make([]View, len(cfg.Content)+1)
		copy(content, cfg.Content)
		content[len(cfg.Content)] = contextMenuPopup(w, cfg, st.X, st.Y)
	}

	focusID := cfg.ID + "_popup"
	return Column(ContainerCfg{
		ID:      cfg.ID,
		Sizing:  cfg.Sizing,
		Width:   cfg.Width,
		Height:  cfg.Height,
		HAlign:  cfg.HAlign,
		VAlign:  cfg.VAlign,
		Padding: cfg.Padding,
		OnAnyClick: func(l *Layout, e *Event, w *Window) {
			if cfg.OnAnyClick != nil {
				cfg.OnAnyClick(l, e, w)
				if e.IsHandled {
					return
				}
			}
			if e.MouseButton == MouseRight {
				// Save current focus in a namespace dismissPopups doesn't clear.
				// Skip the save if the context menu already owns focus (repeated
				// right-click) so we don't overwrite the original caller's focus.
				fs := StateMap[string, string](w, nsContextMenuFocus, capFew)
				if w.FocusID() != focusID {
					fs.Set(cfg.ID, w.FocusID())
				}
				sm := StateMap[string, contextMenuState](
					w, nsContextMenu, capFew)
				sm.Set(cfg.ID, contextMenuState{
					Open: true,
					X:    e.MouseX,
					Y:    e.MouseY,
				})
				w.SetFocus(focusID)
			} else {
				sm := StateMap[string, contextMenuState](
					w, nsContextMenu, capFew)
				sm.Set(cfg.ID, contextMenuState{})
			}
			e.IsHandled = true
		},
		AmendLayout: func(_ *Layout, w *Window) {
			if !w.IsFocus(focusID) {
				sm := StateMapRead[string, contextMenuState](
					w, nsContextMenu)
				if sm != nil {
					sm.Delete(cfg.ID)
				}
				fs := StateMapRead[string, string](
					w, nsContextMenuFocus)
				if fs != nil {
					if prev, ok := fs.Get(cfg.ID); ok && prev != "" {
						w.setFocusLocked(prev)
					}
					fs.Delete(cfg.ID)
				}
			}
		},
		Content: content,
	})
}

// contextMenuPopup builds the floating menu popup.
func contextMenuPopup(w *Window, cfg ContextMenuCfg, mx, my float32) View {
	action := func(id string, e *Event, w *Window) {
		// Restore focus saved before the menu opened. nsContextMenuFocus
		// survives dismissPopups (which only clears nsContextMenu/nsMenu).
		fs := StateMap[string, string](w, nsContextMenuFocus, capFew)
		if prev, ok := fs.Get(cfg.ID); ok && prev != "" {
			w.SetFocus(prev)
		}
		fs.Delete(cfg.ID)
		sm := StateMap[string, contextMenuState](
			w, nsContextMenu, capFew)
		sm.Set(cfg.ID, contextMenuState{})
		if cfg.Action != nil {
			cfg.Action(id, e, w)
		}
	}

	return Menu(w, MenubarCfg{
		ID:                cfg.ID + "_popup",
		Items:             cfg.Items,
		Action:            action,
		Color:             cfg.Color,
		ColorBorder:       cfg.ColorBorder,
		ColorSelect:       cfg.ColorSelect,
		SizeBorder:        cfg.SizeBorder,
		RadiusBorder:      cfg.Radius,
		RadiusMenuItem:    cfg.RadiusMenuItem,
		TextStyle:         cfg.TextStyle,
		TextStyleSubtitle: cfg.TextStyleSubtitle,
		PaddingMenuItem:   cfg.PaddingMenuItem,
		PaddingSubmenu:    cfg.PaddingSubmenu,
		SpacingSubmenu:    cfg.SpacingSubmenu,
		WidthSubmenuMin:   cfg.WidthSubmenuMin,
		WidthSubmenuMax:   cfg.WidthSubmenuMax,
		Float:             true,
		FloatAutoFlip:     true,
		FloatAnchor:       FloatTopLeft,
		FloatTieOff:       FloatTopLeft,
		FloatOffsetX:      mx,
		FloatOffsetY:      my,
		FloatZIndex:       cfg.FloatZIndex,
	})
}

// applyContextMenuDefaults fills zero-value styling fields
// from DefaultMenubarStyle.
func applyContextMenuDefaults(cfg *ContextMenuCfg) {
	d := &DefaultMenubarStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.SizeBorder.IsSet() {
		cfg.SizeBorder = Some(d.SizeBorder)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.TextStyleSubtitle == (TextStyle{}) {
		cfg.TextStyleSubtitle = d.TextStyleSubtitle
	}
	if !cfg.PaddingMenuItem.IsSet() {
		cfg.PaddingMenuItem = Some(d.PaddingMenuItem)
	}
	if !cfg.PaddingSubmenu.IsSet() {
		cfg.PaddingSubmenu = Some(d.PaddingSubmenu)
	}
}
