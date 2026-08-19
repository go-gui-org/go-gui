package gui

// ExpandPanelCfg configures an expand panel. It consists of a
// header (always visible) and content (visible when expanded).
type ExpandPanelCfg struct {
	Head     View
	Content  View
	OnToggle func(EventCtx)
	ID       string

	// Accessibility
	A11YCfg
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	MinWidth   float32
	MaxWidth   float32
	MinHeight  float32
	MaxHeight  float32

	Color       Color
	ColorHover  Color
	colorClick  Color
	ColorBorder Color
	// ColorBorderFocus is the header's border while it holds focus.
	// Unset takes the theme's.
	ColorBorderFocus Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors ColorSet
	Sizing Sizing
	Open   bool
}

// ExpandPanel creates an expandable panel view.
func ExpandPanel(cfg ExpandPanelCfg) View {
	applyExpandPanelDefaults(&cfg)
	sizeBorder := cfg.SizeBorder.Get(guiTheme.expandPanelStyle.SizeBorder)
	radius := cfg.Radius.Get(guiTheme.expandPanelStyle.Radius)

	onToggle := cfg.OnToggle
	colorHover := cfg.ColorHover
	colorClick := cfg.colorClick
	colorBorderFocus := cfg.ColorBorderFocus

	a11yState := AccessState(0)
	if cfg.Open {
		a11yState = AccessStateExpanded
	}

	// The header row joins the tab order: Space/Enter toggle the
	// panel (issue #345). The body's own focusables sit after it in
	// tab order because the header row precedes them in the Column.
	headID := ScopeID(cfg.ID, "head")

	return Column(ContainerCfg{
		ID:          cfg.ID,
		A11YRole:    AccessRoleDisclosure,
		A11YState:   a11yState,
		A11YCfg:     cfg.A11YCfg,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Padding:     cfg.Padding,
		Radius:      Some(radius),
		Sizing:      cfg.Sizing,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Spacing:     SomeF(0),
		Content: []View{
			Row(ContainerCfg{
				ID:           headID,
				Padding:      NoPadding,
				Sizing:       FillFit,
				VAlign:       VAlignMiddle,
				Focusable:    true,
				ClickOnSpace: true,
				ClickOnEnter: true,
				AmendLayout:  focusRingAmend(Color{}, colorBorderFocus),
				Content: []View{
					cfg.Head,
					Row(ContainerCfg{
						Padding: NewPadding(0, PadMedium, 0, 0),
						Content: []View{
							disclosureArrow(cfg.Open, guiTheme.N3),
						},
					}),
				},
				OnClick: func(ctx EventCtx) {
					if onToggle != nil {
						onToggle(ctx)
						ctx.Consume()
					}
				},
				OnHover: func(ctx EventCtx) {
					ctx.Window.SetMouseCursorPointingHand()
					ctx.Layout.Shape.Color = colorHover
					if ctx.Event.MouseButton == MouseLeft {
						ctx.Layout.Shape.Color = colorClick
					}
					ctx.Consume()
				},
			}),
			Column(ContainerCfg{
				Invisible: !cfg.Open,
				Padding:   NoPadding,
				Sizing:    FillFit,
				Spacing:   SomeF(0),
				Content: []View{
					cfg.Content,
				},
			}),
		},
	})
}

// applyExpandPanelDefaults fills zero-value color fields from the theme.
func applyExpandPanelDefaults(cfg *ExpandPanelCfg) {
	d := &defaultExpandPanelStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		Color{}, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, &cfg.ColorHover, &cfg.colorClick,
		nil, &cfg.ColorBorder, &cfg.ColorBorderFocus)
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
}
