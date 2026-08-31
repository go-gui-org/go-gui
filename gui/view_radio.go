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
	Colors      ColorSet
	ColorSelect Color
	// ColorUnselect paints the ring in the off state. Unset takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorUnselect Color
	Disabled      bool
	Selected      bool
	Invisible     bool

	// Sound overrides the theme's selection cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses this radio's sound regardless of the theme
	// and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
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
	circleColor := cfg.ColorUnselect
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
		content = append(content,
			trailingLabel(cfg.Label, cfg.TextStyle))
	}

	a11yState := AccessStateNone
	if cfg.Selected {
		a11yState = AccessStateSelected
	}

	// A radio picks one option out of several, so it takes the
	// selection role rather than the plain click one (issue #467).
	soundCue := resolveSoundCue(
		guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)

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
		Sound:        soundCue,
		clickButton:  MouseLeft,
		ClickOnSpace: true,
		AmendLayout: amendAll(
			func(ctx EventCtx) {
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
			// Ring shadow on the focusable row, but no colour change:
			// the ring is the row's focus indication while the pill
			// keeps its own accent border (visual-refresh § 5.4).
			focusRingAmend(Color{}, Color{})),
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
	if !cfg.ColorUnselect.IsSet() {
		cfg.ColorUnselect = d.colorUnselect
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
