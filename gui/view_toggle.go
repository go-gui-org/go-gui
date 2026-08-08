package gui

// ToggleCfg configures a toggle/checkbox button.
type ToggleCfg struct {
	TextStyle      TextStyle
	TextStyleLabel TextStyle
	OnClick        func(EventCtx)
	ID             string `gui:"required,focus"`
	Label          string
	TextSelect     string
	TextUnselect   string

	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	// Size overrides the square edge length of the check box.
	Size       Opt[float32]
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	MinWidth   float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Color         Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors      ColorSet
	ColorSelect Color
	Disabled    bool
	Invisible   bool
	Selected    bool
}

// Toggle creates a toggle/checkbox view.
func Toggle(cfg ToggleCfg) View {
	applyToggleDefaults(&cfg)
	requireFocusID("Toggle", cfg.FocusDisabled, cfg.ID)

	d := &DefaultToggleStyle
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	radius := cfg.Radius.Get(d.Radius)
	size := cfg.Size.Get(d.Size)

	boxColor := cfg.Colors.Base
	if cfg.Selected {
		boxColor = cfg.ColorSelect
	}

	txt := cfg.TextSelect
	txtStyle := cfg.TextStyle
	if !cfg.Selected {
		if cfg.TextUnselect == " " {
			txtStyle.Color = ColorTransparent
		} else {
			txt = cfg.TextUnselect
		}
	}

	colorFocus := cfg.Colors.Focus
	colorBorderFocus := cfg.Colors.BorderFocus
	colorHover := cfg.Colors.Hover
	colorClick := cfg.Colors.Click

	content := make([]View, 0, 2)
	// Fixed square box: without Fixed sizing the box fits the check
	// glyph, which is narrower than the line height, so it renders as
	// a tall rectangle. Padding stays for the inner text inset.
	content = append(content, Row(ContainerCfg{
		Color:       boxColor,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  Some(sizeBorder),
		Padding:     cfg.Padding,
		Radius:      Some(radius),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Width:       size,
		Height:      size,
		Sizing:      FixedFixed,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
		Content: []View{
			Text(TextCfg{Text: txt, TextStyle: txtStyle}),
		},
	}))

	if len(cfg.Label) > 0 {
		content = append(content,
			Text(TextCfg{Text: cfg.Label, TextStyle: cfg.TextStyleLabel}))
	}

	a11yState := AccessStateNone
	if cfg.Selected {
		a11yState = AccessStateChecked
	}

	return Row(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       !cfg.FocusDisabled,
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		SizeBorder:      NoBorder,
		Padding:         NoPadding,
		VAlign:          VAlignMiddle,
		A11YRole:        AccessRoleCheckbox,
		A11YState:       a11yState,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
		A11YDescription: cfg.A11YDescription,
		ClickOnSpace:    true,
		OnClick:         cfg.OnClick,
		ClickButton:     MouseLeft,
		MinWidth:        cfg.MinWidth,
		OnHover: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled ||
				!ctx.Layout.Shape.hasEvents() ||
				ctx.Layout.Shape.events.OnClick == nil {
				return
			}
			ctx.Window.setMouseCursor(CursorPointingHand)
			if len(ctx.Layout.Children) == 0 {
				return
			}
			ctx.Layout.Children[0].Shape.Color = colorHover
			if ctx.Event.MouseButton == MouseLeft {
				ctx.Layout.Children[0].Shape.Color = colorClick
			}
		},
		AmendLayout: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled ||
				!ctx.Layout.Shape.hasEvents() ||
				ctx.Layout.Shape.events.OnClick == nil {
				return
			}
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

func applyToggleDefaults(cfg *ToggleCfg) {
	d := &DefaultToggleStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.ColorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if cfg.TextSelect == "" {
		cfg.TextSelect = "✓"
	}
	if cfg.TextUnselect == "" {
		cfg.TextUnselect = " "
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}

	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyleNormal
	} else {
		cfg.TextStyle = mergeTextStyle(cfg.TextStyle, d.TextStyleNormal)
	}
	if cfg.TextStyleLabel == (TextStyle{}) {
		cfg.TextStyleLabel = d.TextStyleLabel
	} else {
		cfg.TextStyleLabel = mergeTextStyle(cfg.TextStyleLabel, d.TextStyleLabel)
	}
}
