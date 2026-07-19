package gui

// ToggleCfg configures a toggle/checkbox button.
type ToggleCfg struct {
	TextStyle      TextStyle
	TextStyleLabel TextStyle
	OnClick        func(*Layout, *Event, *Window)
	ID             string
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
	FocusDisabled    bool
	Color            Color
	ColorFocus       Color
	ColorHover       Color
	ColorClick       Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorSelect      Color
	Disabled         bool
	Invisible        bool
	Selected         bool
}

// Toggle creates a toggle/checkbox view.
func Toggle(cfg ToggleCfg) View {
	applyToggleDefaults(&cfg)

	d := &DefaultToggleStyle
	sizeBorder := cfg.SizeBorder.Get(d.SizeBorder)
	radius := cfg.Radius.Get(d.Radius)
	size := cfg.Size.Get(d.Size)

	boxColor := cfg.Color
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

	colorFocus := cfg.ColorFocus
	colorBorderFocus := cfg.ColorBorderFocus
	colorHover := cfg.ColorHover
	colorClick := cfg.ColorClick

	content := make([]View, 0, 2)
	// Fixed square box: without Fixed sizing the box fits the check
	// glyph, which is narrower than the line height, so it renders as
	// a tall rectangle. Padding stays for the inner text inset.
	content = append(content, Row(ContainerCfg{
		Color:       boxColor,
		ColorBorder: cfg.ColorBorder,
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
			layout.Children[0].Shape.Color = colorHover
			if e.MouseButton == MouseLeft {
				layout.Children[0].Shape.Color = colorClick
			}
		},
		AmendLayout: func(layout *Layout, w *Window) {
			if layout.Shape.Disabled ||
				!layout.Shape.hasEvents() ||
				layout.Shape.events.OnClick == nil {
				return
			}
			if len(layout.Children) == 0 {
				return
			}
			if w.IsFocus(layout.Shape.ID) {
				layout.Children[0].Shape.Color = colorFocus
				layout.Children[0].Shape.ColorBorder = colorBorderFocus
			}
		},
		Content: content,
	})
}

func applyToggleDefaults(cfg *ToggleCfg) {
	d := &DefaultToggleStyle
	if cfg.TextSelect == "" {
		cfg.TextSelect = "✓"
	}
	if cfg.TextUnselect == "" {
		cfg.TextUnselect = " "
	}
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
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
