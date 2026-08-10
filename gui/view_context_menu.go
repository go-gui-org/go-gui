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
	textStyleSubtitle TextStyle
	Action            func(string, EventCtx)

	// User click handler — fires before context menu logic.
	OnAnyClick func(EventCtx)

	ID    string `gui:"required"`
	Items []MenuItemCfg

	Content []View

	FloatZIndex int

	Padding Padding

	paddingMenuItem Padding
	paddingSubmenu  Padding
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	radiusMenuItem  Opt[float32]
	spacingSubmenu  Opt[float32]
	widthSubmenuMin Opt[float32]
	widthSubmenuMax Opt[float32]

	Width  float32
	Height float32

	// Menu styling — optional, defaults from theme.
	Color       Color
	ColorBorder Color
	ColorSelect Color

	// Container passthrough (outer wrapper).
	Sizing Sizing
	HAlign HorizontalAlign // ergonomics-audit:opt-plain — zero (HAlignStart) is the natural default; no distinct unset behavior
	VAlign verticalAlign   // ergonomics-audit:opt-plain — zero (VAlignTop) is the natural default; no distinct unset behavior
}

// ContextMenu creates a container that opens a floating menu
// on right-click at the cursor position.
func ContextMenu(w *Window, cfg ContextMenuCfg) View {
	RequireID("ContextMenu", cfg.ID)
	checkForDuplicateMenuIDs(cfg.Items)
	applyContextMenuDefaults(&cfg)

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	st := StateReadOr(
		w, nsContextMenu, cfg.ID, contextMenuState{})

	content := cfg.Content
	if st.Open && w.IsFocus(ScopeID(cfg.ID, "popup")) {
		content = make([]View, len(cfg.Content)+1)
		copy(content, cfg.Content)
		content[len(cfg.Content)] = contextMenuPopup(w, cfg, st.X, st.Y)
	}

	focusID := ScopeID(cfg.ID, "popup")
	return Column(ContainerCfg{
		ID:      cfg.ID,
		Sizing:  cfg.Sizing,
		Width:   cfg.Width,
		Height:  cfg.Height,
		HAlign:  cfg.HAlign,
		VAlign:  cfg.VAlign,
		Padding: cfg.Padding,
		OnAnyClick: func(ctx EventCtx) {
			if cfg.OnAnyClick != nil {
				cfg.OnAnyClick(ctx)
				if ctx.Event.IsHandled {
					return
				}
			}
			if ctx.Event.MouseButton == MouseRight {
				// Save current focus in a namespace dismissPopups doesn't clear.
				// Skip the save if the context menu already owns focus (repeated
				// right-click) so we don't overwrite the original caller's focus.
				fs := StateMap[string, string](ctx.Window, nsContextMenuFocus, capFew)
				if ctx.Window.FocusID() != focusID {
					fs.Set(cfg.ID, ctx.Window.FocusID())
				}
				sm := StateMap[string, contextMenuState](
					ctx.Window, nsContextMenu, capFew)
				sm.Set(cfg.ID, contextMenuState{
					Open: true,
					X:    ctx.Event.MouseX,
					Y:    ctx.Event.MouseY,
				})
				ctx.Window.SetFocus(focusID)
			} else {
				sm := StateMap[string, contextMenuState](
					ctx.Window, nsContextMenu, capFew)
				sm.Set(cfg.ID, contextMenuState{})
			}
			ctx.Consume()
		},
		AmendLayout: func(ctx EventCtx) {
			if !ctx.Window.IsFocus(focusID) {
				sm := StateMapRead[string, contextMenuState](
					ctx.Window, nsContextMenu)
				if sm != nil {
					sm.Delete(cfg.ID)
				}
				fs := StateMapRead[string, string](
					ctx.Window, nsContextMenuFocus)
				if fs != nil {
					if prev, ok := fs.Get(cfg.ID); ok && prev != "" {
						ctx.Window.setFocusLocked(prev)
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
	action := func(id string, ctx EventCtx) {
		// Restore focus saved before the menu opened. nsContextMenuFocus
		// survives dismissPopups (which only clears nsContextMenu/nsMenu).
		fs := StateMap[string, string](ctx.Window, nsContextMenuFocus, capFew)
		if prev, ok := fs.Get(cfg.ID); ok && prev != "" {
			ctx.Window.SetFocus(prev)
		}
		fs.Delete(cfg.ID)
		sm := StateMap[string, contextMenuState](
			ctx.Window, nsContextMenu, capFew)
		sm.Set(cfg.ID, contextMenuState{})
		if cfg.Action != nil {
			cfg.Action(id, EventCtx{nil, ctx.Event, ctx.Window})
		}
	}

	return menu(w, MenubarCfg{
		ID:                ScopeID(cfg.ID, "popup"),
		Items:             cfg.Items,
		Action:            action,
		Color:             cfg.Color,
		ColorBorder:       cfg.ColorBorder,
		ColorSelect:       cfg.ColorSelect,
		SizeBorder:        cfg.SizeBorder,
		radiusBorder:      cfg.Radius,
		radiusMenuItem:    cfg.radiusMenuItem,
		TextStyle:         cfg.TextStyle,
		textStyleSubtitle: cfg.textStyleSubtitle,
		paddingMenuItem:   cfg.paddingMenuItem,
		paddingSubmenu:    cfg.paddingSubmenu,
		spacingSubmenu:    cfg.spacingSubmenu,
		widthSubmenuMin:   cfg.widthSubmenuMin,
		widthSubmenuMax:   cfg.widthSubmenuMax,
		Float:             true,
		floatAutoFlip:     true,
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
	d := &defaultMenubarStyle
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
	if cfg.textStyleSubtitle == (TextStyle{}) {
		cfg.textStyleSubtitle = d.textStyleSubtitle
	}
	if !cfg.paddingMenuItem.IsSet() {
		cfg.paddingMenuItem = d.paddingMenuItem
	}
	if !cfg.paddingSubmenu.IsSet() {
		cfg.paddingSubmenu = d.paddingSubmenu
	}
}
