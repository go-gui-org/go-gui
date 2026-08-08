package gui

// ExpandPanelCfg configures an expand panel. It consists of a
// header (always visible) and content (visible when expanded).
type ExpandPanelCfg struct {
	Head     View
	Content  View
	OnToggle func(EventCtx)
	ID       string

	// Accessibility
	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	MinWidth        float32
	MaxWidth        float32
	MinHeight       float32
	MaxHeight       float32

	Color       Color
	ColorHover  Color
	ColorClick  Color
	ColorBorder Color
	Sizing      Sizing
	Open        bool
}

// ExpandPanel creates an expandable panel view.
func ExpandPanel(cfg ExpandPanelCfg) View {
	if !cfg.Color.IsSet() {
		cfg.Color = guiTheme.ExpandPanelStyle.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = guiTheme.ExpandPanelStyle.ColorHover
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = guiTheme.ExpandPanelStyle.ColorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = guiTheme.ExpandPanelStyle.ColorBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(guiTheme.ExpandPanelStyle.Padding)
	}
	sizeBorder := cfg.SizeBorder.Get(guiTheme.ExpandPanelStyle.SizeBorder)
	radius := cfg.Radius.Get(guiTheme.ExpandPanelStyle.Radius)

	onToggle := cfg.OnToggle
	colorHover := cfg.ColorHover
	colorClick := cfg.ColorClick

	a11yState := AccessState(0)
	if cfg.Open {
		a11yState = AccessStateExpanded
	}

	arrowText := "▼"
	if cfg.Open {
		arrowText = "▲"
	}

	return Column(ContainerCfg{
		ID:              cfg.ID,
		A11YRole:        AccessRoleDisclosure,
		A11YState:       a11yState,
		A11YLabel:       cfg.A11YLabel,
		A11YDescription: cfg.A11YDescription,
		Color:           cfg.Color,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      Some(sizeBorder),
		Padding:         cfg.Padding,
		Radius:          Some(radius),
		Sizing:          cfg.Sizing,
		MinWidth:        cfg.MinWidth,
		MaxWidth:        cfg.MaxWidth,
		MinHeight:       cfg.MinHeight,
		MaxHeight:       cfg.MaxHeight,
		Spacing:         SomeF(0),
		Content: []View{
			Row(ContainerCfg{
				Padding: NoPadding,
				Sizing:  FillFit,
				VAlign:  VAlignMiddle,
				Content: []View{
					cfg.Head,
					Row(ContainerCfg{
						Padding: SomeP(0, PadMedium, 0, 0),
						Content: []View{
							Text(TextCfg{
								Text:      arrowText,
								TextStyle: guiTheme.N3,
							}),
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
					if ctx.Event.CharCode != CharSpace || onToggle == nil {
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
