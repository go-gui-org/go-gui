package gui

// SwitchCfg configures a pill-shaped toggle switch.
type SwitchCfg struct {
	TextStyle TextStyle
	OnClick   func(EventCtx)
	ID        string `gui:"required,focus"`
	Label     string

	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Width           Opt[float32]
	Height          Opt[float32]
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Color         Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors        ColorSet
	ColorSelect   Color
	ColorUnselect Color
	Disabled      bool
	Invisible     bool
	Selected      bool
}

// Switch creates a pill-shaped toggle switch.
func Switch(cfg SwitchCfg) View {
	applySwitchDefaults(&cfg)
	requireFocusID("Switch", cfg.FocusDisabled, cfg.ID)

	d := &DefaultSwitchStyle
	width := cfg.Width.Get(d.SizeWidth)
	height := cfg.Height.Get(d.SizeHeight)
	radius := height / 2
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)

	thumbColor := cfg.ColorUnselect
	if cfg.Selected {
		thumbColor = cfg.ColorSelect
	}
	circleSize := height - cfg.Padding.Get(Padding{}).Height() - (sizeBorder * 2)

	colorFocus := cfg.Colors.Focus
	colorBorderFocus := cfg.Colors.BorderFocus
	colorHover := cfg.Colors.Hover
	colorClick := cfg.Colors.Click

	hAlign := HAlignStart
	if cfg.Selected {
		hAlign = HAlignEnd
	}

	content := make([]View, 0, 2)
	// No ID here: the focusable outer row owns cfg.ID, and IDs must be
	// unique per window.
	content = append(content, Row(ContainerCfg{
		Width:       width,
		Height:      height,
		Sizing:      FixedFit,
		Color:       cfg.Colors.Base,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Padding:     cfg.Padding,
		HAlign:      hAlign,
		VAlign:      VAlignMiddle,
		Content: []View{
			Circle(ContainerCfg{
				Color:  thumbColor,
				Width:  circleSize,
				Height: circleSize,
				Sizing: FixedFixed,
			}),
		},
	}))
	if len(cfg.Label) > 0 {
		content = append(content,
			Text(TextCfg{Text: cfg.Label, TextStyle: cfg.TextStyle}))
	}

	a11yState := AccessStateNone
	if cfg.Selected {
		a11yState = AccessStateChecked
	}

	return Row(ContainerCfg{
		ID:         cfg.ID,
		Focusable:  !cfg.FocusDisabled,
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		SizeBorder: NoBorder,
		Padding:    NoPadding,
		// Centre the pill and its label on the row's cross axis so the
		// label sits vertically middle-aligned with the switch.
		VAlign:          VAlignMiddle,
		A11YRole:        AccessRoleSwitchToggle,
		A11YState:       a11yState,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
		A11YDescription: cfg.A11YDescription,
		ClickOnSpace:    true,
		OnClick:         cfg.OnClick,
		ClickButton:     MouseLeft,
		OnHover: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled ||
				!ctx.Layout.Shape.hasEvents() ||
				ctx.Layout.Shape.events.OnClick == nil {
				return
			}
			ctx.Window.setMouseCursor(CursorPointingHand)
			if len(ctx.Layout.Children) > 0 {
				ctx.Layout.Children[0].Shape.Color = colorHover
				if ctx.Event.MouseButton == MouseLeft {
					ctx.Layout.Children[0].Shape.Color = colorClick
				}
			}
		},
		AmendLayout: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled ||
				!ctx.Layout.Shape.hasEvents() ||
				ctx.Layout.Shape.events.OnClick == nil {
				return
			}
			// Highlight only the pill (child 0), not the outer row —
			// the outer row also spans the label.
			if len(ctx.Layout.Children) == 0 {
				return
			}
			if ctx.Window.IsFocus(ctx.Layout.Shape.ID) {
				ctx.Layout.Children[0].Shape.Color = colorFocus
				ctx.Layout.Children[0].Shape.ColorBorder = colorBorderFocus
			}
		},
		Content: content,
	})
}

func applySwitchDefaults(cfg *SwitchCfg) {
	d := &DefaultSwitchStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.ColorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.ColorUnselect.IsSet() {
		cfg.ColorUnselect = d.ColorUnselect
	}

	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyleNormal
	}
}
