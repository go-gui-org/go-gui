package gui

// RadioCfg configures a radio button.
type RadioCfg struct {
	TextStyle TextStyle
	OnClick   func(*Layout, *Event, *Window)
	ID        string
	Label     string

	A11YLabel        string
	A11YDescription  string
	Padding          Opt[Padding]
	Size             Opt[float32]
	SizeBorder       Opt[float32]
	IDFocus          uint32
	Color            Color
	ColorHover       Color
	ColorFocus       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	ColorUnselect    Color
	Disabled         bool
	Selected         bool
	Invisible        bool
}

// Radio creates a radio button view.
func Radio(cfg RadioCfg) View {
	applyRadioDefaults(&cfg)

	dr := &DefaultRadioStyle
	size := cfg.Size.Get(dr.Size)
	sizeBorder := cfg.SizeBorder.Get(dr.SizeBorder)

	colorBorderFocus := cfg.ColorBorderFocus
	colorHover := cfg.ColorHover
	colorClick := cfg.ColorClick
	circleColor := cfg.ColorUnselect
	if cfg.Selected {
		circleColor = cfg.ColorSelect
	}

	content := make([]View, 0, 2)
	content = append(content, Circle(ContainerCfg{
		Width:       size,
		Height:      size,
		Color:       circleColor,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Sizing:      FixedFixed,
		HAlign:      HAlignCenter,
		VAlign:      VAlignMiddle,
	}))

	if len(cfg.Label) > 0 {
		content = append(content, Row(ContainerCfg{
			Padding: SomeP(0, PadXSmall, 0, 0),
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
		ID:              cfg.ID,
		IDFocus:         cfg.IDFocus,
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		Padding:         cfg.Padding,
		VAlign:          VAlignMiddle,
		A11YRole:        AccessRoleRadioButton,
		A11YState:       a11yState,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
		A11YDescription: cfg.A11YDescription,
		OnClick:         leftClickOnly(cfg.OnClick),
		OnChar:          spacebarToClick(cfg.OnClick),
		AmendLayout: func(layout *Layout, w *Window) {
			if layout.Shape.Disabled ||
				!layout.Shape.hasEvents() ||
				layout.Shape.events.OnClick == nil {
				return
			}
			if len(layout.Children) == 0 {
				return
			}
			if w.IsFocus(layout.Shape.IDFocus) {
				layout.Children[0].Shape.ColorBorder = colorBorderFocus
			}
		},
		OnHover: func(layout *Layout, e *Event, w *Window) {
			if layout.Shape.Disabled ||
				!layout.Shape.hasEvents() ||
				layout.Shape.events.OnClick == nil {
				return
			}
			w.setMouseCursor(CursorPointingHand)
			if len(layout.Children) == 0 {
				return
			}
			layout.Children[0].Shape.ColorBorder = colorHover
			if e.MouseButton == MouseLeft {
				layout.Children[0].Shape.ColorBorder = colorClick
			}
		},
		Content: content,
	})
}

func applyRadioDefaults(cfg *RadioCfg) {
	d := &DefaultRadioStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorClick.IsSet() {
		cfg.ColorClick = d.ColorClick
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
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
	} else {
		cfg.TextStyle = mergeTextStyle(cfg.TextStyle, d.TextStyleNormal)
	}
}
