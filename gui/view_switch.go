package gui

// SwitchCfg configures a pill-shaped toggle switch.
type SwitchCfg struct {
	TextStyle TextStyle
	OnClick   func(*Layout, *Event, *Window)
	ID        string
	Label     string

	A11YLabel       string
	A11YDescription string
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Width           Opt[float32]
	Height          Opt[float32]
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
	ColorUnselect    Color
	Disabled         bool
	Invisible        bool
	Selected         bool
}

// Switch creates a pill-shaped toggle switch.
func Switch(cfg SwitchCfg) View {
	applySwitchDefaults(&cfg)

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

	colorFocus := cfg.ColorFocus
	colorBorderFocus := cfg.ColorBorderFocus
	colorHover := cfg.ColorHover
	colorClick := cfg.ColorClick

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
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
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
		OnHover: func(layout *Layout, e *Event, w *Window) {
			if layout.Shape.Disabled ||
				!layout.Shape.hasEvents() ||
				layout.Shape.events.OnClick == nil {
				return
			}
			w.setMouseCursor(CursorPointingHand)
			if len(layout.Children) > 0 {
				layout.Children[0].Shape.Color = colorHover
				if e.MouseButton == MouseLeft {
					layout.Children[0].Shape.Color = colorClick
				}
			}
		},
		AmendLayout: func(layout *Layout, w *Window) {
			if layout.Shape.Disabled ||
				!layout.Shape.hasEvents() ||
				layout.Shape.events.OnClick == nil {
				return
			}
			// Highlight only the pill (child 0), not the outer row —
			// the outer row also spans the label.
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

func applySwitchDefaults(cfg *SwitchCfg) {
	d := &DefaultSwitchStyle
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
