package gui

// ToggleCfg configures a toggle/checkbox button.
type ToggleCfg struct {
	TextStyle TextStyle
	// TextStyleLabel styles the trailing label. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleLabel TextStyle
	OnClick        func(EventCtx)
	ID             string `gui:"required,focus"`
	Label          string
	TextSelect     string
	TextUnselect   string

	A11YCfg
	Padding Padding
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

	// Sound overrides the theme's toggle cue for this instance.
	// SoundNone (the zero value) takes Theme.Sounds.ToggleOn or
	// ToggleOff depending on what the next click will do (issue #446).
	// exportaudit:keep — caller-facing config (issue #446)
	Sound SoundCue

	// SoundDisabled suppresses this toggle's sound regardless of the
	// theme and of Sound above.
	// exportaudit:keep — caller-facing config (issue #446)
	SoundDisabled bool
}

// LabeledToggle is the thin form of Toggle for the common case: a
// label next to a checkbox in its initial selected state. Callers
// needing TextStyle, Colors, Size, or the rest of ToggleCfg use
// Toggle directly.
// exportaudit:keep — convenience form; no example uses it yet
func LabeledToggle(id, label string, selected bool, onClick func(EventCtx)) View {
	return Toggle(ToggleCfg{
		ID:       id,
		Label:    label,
		Selected: selected,
		OnClick:  onClick,
	})
}

// Toggle creates a toggle/checkbox view.
func Toggle(cfg ToggleCfg) View {
	applyToggleDefaults(&cfg)
	requireFocusID("Toggle", cfg.FocusDisabled, cfg.ID)

	d := &defaultToggleStyle
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
		AmendLayout: centerGlyphOnInk(txt, txtStyle),
		Content: []View{
			Text(TextCfg{Text: txt, TextStyle: txtStyle}),
		},
	}))

	if len(cfg.Label) > 0 {
		content = append(content,
			trailingLabel(cfg.Label, cfg.TextStyleLabel))
	}

	a11yState := AccessStateNone
	if cfg.Selected {
		a11yState = AccessStateChecked
	}

	// The cue names what the click will do, not the current state: a
	// selected toggle is about to turn off. Resolved at generation
	// time, where the state is already known, so no runtime branch and
	// no closure (issue #446).
	themeCue := guiTheme.Sounds.ToggleOn
	if cfg.Selected {
		themeCue = guiTheme.Sounds.ToggleOff
	}
	soundCue := resolveSoundCue(themeCue, cfg.Sound, cfg.SoundDisabled)

	return Row(ContainerCfg{
		ID:         cfg.ID,
		Focusable:  !cfg.FocusDisabled,
		Disabled:   cfg.Disabled,
		Invisible:  cfg.Invisible,
		SizeBorder: NoBorder,
		Padding:    NoPadding,
		VAlign:     VAlignMiddle,
		A11YRole:   AccessRoleCheckbox,
		A11YState:  a11yState,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Label),
			A11YDescription: cfg.A11YDescription,
		},
		ClickOnSpace: true,
		OnClick:      cfg.OnClick,
		Sound:        soundCue,
		clickButton:  MouseLeft,
		MinWidth:     cfg.MinWidth,
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
					ctx.Layout.Children[0].Shape.Color = colorFocus
					ctx.Layout.Children[0].Shape.ColorBorder = colorBorderFocus
				}
			},
			// Ring shadow on the focusable row; the box keeps its own
			// accent fill and border (visual-refresh § 5.4).
			focusRingAmend(Color{}, Color{})),
		Content: content,
	})
}

func applyToggleDefaults(cfg *ToggleCfg) {
	d := &defaultToggleStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
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
		cfg.Padding = d.Padding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.textStyleNormal
	} else {
		cfg.TextStyle = mergeTextStyle(cfg.TextStyle, d.textStyleNormal)
	}
	if cfg.TextStyleLabel == (TextStyle{}) {
		cfg.TextStyleLabel = d.textStyleLabel
	} else {
		cfg.TextStyleLabel = mergeTextStyle(cfg.TextStyleLabel, d.textStyleLabel)
	}
}
