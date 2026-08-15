package gui

// RadioCfg configures a radio button.
type RadioCfg struct {
	TextStyle TextStyle
	OnClick   func(EventCtx)
	ID        string `gui:"required,focus"`
	Label     string

	A11YCfg
	Padding    Padding
	Size       Opt[float32]
	SizeBorder Opt[float32]
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Color         Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it.
	Colors        ColorSet
	ColorSelect   Color
	colorUnselect Color
	Disabled      bool
	Selected      bool
	Invisible     bool
}

// Radio creates a radio button view.
func Radio(cfg RadioCfg) View {
	applyRadioDefaults(&cfg)
	requireFocusID("Radio", cfg.FocusDisabled, cfg.ID)

	dr := &defaultRadioStyle
	size := cfg.Size.Get(dr.Size)
	sizeBorder := cfg.SizeBorder.Get(dr.SizeBorder)

	colorBorderFocus := cfg.Colors.BorderFocus
	colorHover := cfg.Colors.Hover
	colorClick := cfg.Colors.Click
	circleColor := cfg.colorUnselect
	if cfg.Selected {
		circleColor = cfg.ColorSelect
	}

	content := make([]View, 0, 2)
	content = append(content, Circle(ContainerCfg{
		Width:       size,
		Height:      size,
		Color:       circleColor,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  Some(sizeBorder),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Sizing:      FixedFixed,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
	}))

	if len(cfg.Label) > 0 {
		content = append(content, Row(ContainerCfg{
			Padding: NewPadding(0, PadXSmall, 0, 0),
			Content: []View{
				Text(TextCfg{Text: cfg.Label, TextStyle: cfg.TextStyle}),
			},
		}))
	}

	a11yState := AccessStateNone
	if cfg.Selected {
		a11yState = AccessStateSelected
	}

	return Row(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		Disabled:  cfg.Disabled,
		Invisible: cfg.Invisible,
		Padding:   cfg.Padding,
		VAlign:    VAlignMiddle,
		A11YRole:  AccessRoleRadioButton,
		A11YState: a11yState,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
			A11YDescription: cfg.A11YDescription,
		},
		OnClick:      cfg.OnClick,
		clickButton:  MouseLeft,
		clickOnSpace: true,
		AmendLayout: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled ||
				!ctx.Layout.Shape.hasEvents() ||
				ctx.Layout.Shape.events.OnClick == nil {
				return
			}
			if len(ctx.Layout.Children) == 0 {
				return
			}
			if ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
				ctx.Layout.Children[0].Shape.ColorBorder = colorBorderFocus
			}
		},
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
			ctx.Layout.Children[0].Shape.ColorBorder = colorHover
			if ctx.Event.MouseButton == MouseLeft {
				ctx.Layout.Children[0].Shape.ColorBorder = colorClick
			}
		},
		Content: content,
	})
}

func applyRadioDefaults(cfg *RadioCfg) {
	d := &defaultRadioStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.colorUnselect.IsSet() {
		cfg.colorUnselect = d.colorUnselect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.textStyleNormal
	} else {
		cfg.TextStyle = mergeTextStyle(cfg.TextStyle, d.textStyleNormal)
	}
}
