package gui

// OverflowItem defines an item in an overflow panel.
type OverflowItem struct {
	View   View
	Action func(*MenuItemCfg, EventCtx)
	ID     string
	Text   string
}

// OverflowPanelCfg configures an overflow panel that hides
// items that don't fit and shows them in a dropdown menu.
type OverflowPanelCfg struct {
	ID           string
	Items        []OverflowItem
	Trigger      []View
	FloatZIndex  int
	Padding      Opt[Padding]
	Spacing      Opt[float32]
	Focusable    bool
	FloatOffsetX float32
	FloatOffsetY float32
	FloatAnchor  floatAttach
	FloatTieOff  floatAttach
	Disabled     bool
}

// OverflowPanel creates a row that hides items that don't fit
// and shows a trigger button to reveal them in a dropdown.
func OverflowPanel(w *Window, cfg OverflowPanelCfg) View {
	applyOverflowDefaults(&cfg)

	// The overflow count and the menu-open flag are keyed by the panel's
	// effective ID — the same key layoutOverflow writes from the shape.
	id := w.EffID(cfg.ID)
	visibleCount, ok := w.overflow().Get(id)
	if !ok {
		visibleCount = len(cfg.Items)
	}
	isOpen := StateReadOr(
		w, nsSelect, id, false)

	content := make([]View, 0, len(cfg.Items)+2)

	// Add all item views — layoutOverflow will hide those
	// that don't fit.
	for _, item := range cfg.Items {
		content = append(content, item.View)
	}

	// Trigger button (last non-float child).
	triggerContent := cfg.Trigger
	if len(triggerContent) == 0 {
		triggerContent = []View{
			Text(TextCfg{
				Text:      "\u22EE",
				TextStyle: DefaultTextStyle,
			}),
		}
	}

	content = append(content, Button(ButtonCfg{
		// Namespaced by the panel's ID: a toolbar can hold several
		// overflow panels, each with its own trigger.
		ID:       ScopeID(id, "trigger"),
		Color:    ColorTransparent,
		Padding:  cfg.Padding,
		Disabled: cfg.Disabled,
		Content:  triggerContent,
		OnClick: func(ctx EventCtx) {
			ss := StateMap[string, bool](
				ctx.Window, nsSelect, capModerate)
			cur := StateReadOr(
				ctx.Window, nsSelect, id, false)
			ss.Set(id, !cur)
		},
	}))

	// Floating dropdown for overflow items.
	if isOpen && visibleCount < len(cfg.Items) {
		overflow := cfg.Items[visibleCount:]
		menuItems := make([]MenuItemCfg, 0, len(overflow))
		for _, oi := range overflow {
			text := oi.Text
			if text == "" {
				text = oi.ID
			}
			menuItems = append(menuItems, MenuItemCfg{
				ID:     oi.ID,
				Text:   text,
				Action: oi.Action,
			})
		}

		content = append(content, menu(w, MenubarCfg{
			ID:           ScopeID(id, "menu"),
			Items:        menuItems,
			Float:        true,
			FloatAnchor:  cfg.FloatAnchor,
			FloatTieOff:  cfg.FloatTieOff,
			FloatOffsetX: cfg.FloatOffsetX,
			FloatOffsetY: cfg.FloatOffsetY,
			FloatZIndex:  cfg.FloatZIndex,
		}))
	}

	return Row(ContainerCfg{
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleGroup,
		Sizing:    FillFit,
		Padding:   NoPadding,
		Spacing:   cfg.Spacing,
		Overflow:  true,
		Disabled:  cfg.Disabled,
		Content:   content,
	})
}

func applyOverflowDefaults(cfg *OverflowPanelCfg) {
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(defaultButtonStyle.Padding)
	}
	if cfg.FloatAnchor == 0 {
		cfg.FloatAnchor = FloatBottomRight
	}
	if cfg.FloatTieOff == 0 {
		cfg.FloatTieOff = FloatTopRight
	}
	if !cfg.Spacing.IsSet() {
		cfg.Spacing = SomeF(guiTheme.SpacingSmall)
	}
}
