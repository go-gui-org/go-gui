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
	Sizing      Sizing
	Open        bool
}

// ExpandPanel creates an expandable panel view.
func ExpandPanel(cfg ExpandPanelCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.expandPanelStyle.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = guiTheme.expandPanelStyle.ColorHover
	}
	if !cfg.colorClick.IsSet() {
		cfg.colorClick = guiTheme.expandPanelStyle.colorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = guiTheme.expandPanelStyle.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.expandPanelStyle.Padding
	}
	sizeBorder := cfg.SizeBorder.Get(guiTheme.expandPanelStyle.SizeBorder)
	radius := cfg.Radius.Get(guiTheme.expandPanelStyle.Radius)

	onToggle := cfg.OnToggle
	colorHover := cfg.ColorHover
	colorClick := cfg.colorClick

	a11yState := AccessState(0)
	if cfg.Open {
		a11yState = AccessStateExpanded
	}

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
				Padding: NoPadding,
				Sizing:  FillFit,
				VAlign:  VAlignMiddle,
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
				OnChar: func(ctx EventCtx) {
					// Only the spacebar activates the header; every
					// other character has to keep travelling.
					if ctx.Event.CharCode != charSpace || onToggle == nil {
						return
					}
					onToggle(ctx)
					ctx.Consume()
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
