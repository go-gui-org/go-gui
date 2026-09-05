package gui

// NumericInputCfg configures a locale-aware numeric input with
// optional step controls.
type NumericInputCfg struct {
	// Label names this field. Empty renders exactly as before: no
	// wrapper and no extra shape. Set, it stacks above the field in
	// the theme's label role, and fills A11YLabel when that is unset.
	// See gui/field_label.go for the convention and why it is one.
	Label            string
	TextStyle        TextStyle
	PlaceholderStyle TextStyle

	// Callbacks
	OnTextChanged func(string, EventCtx)
	OnValueCommit func(Opt[float64], string, EventCtx)

	ID          string `gui:"required,focus"`
	Text        string
	Placeholder string

	// Accessibility
	A11YCfg
	currencyCfg numericCurrencyModeCfg
	percentCfg  numericPercentModeCfg
	Locale      NumericLocaleCfg
	StepCfg     NumericStepCfg
	Value       Opt[float64]
	Min         Opt[float64]
	Max         Opt[float64]

	Decimals int

	// Appearance
	Padding    Padding
	Radius     Opt[float32]
	SizeBorder Opt[float32]
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Width         float32
	Height        float32
	MinWidth      float32
	MaxWidth      float32
	MinHeight     float32
	MaxHeight     float32

	Color            Color
	ColorHover       Color
	ColorBorder      Color
	ColorBorderFocus Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors ColorSet

	// Sizing
	Sizing Sizing
	Mode   numericInputMode

	// ReadOnly blocks value edits while the field stays focusable and
	// selectable, mirroring InputCfg.ReadOnly. Typing is blocked on the
	// inner Input and the step buttons are gated so they cannot mutate
	// the value. Distinct from Disabled, which removes interaction
	// entirely.
	ReadOnly bool

	// SoundDisabled suppresses the field's sound regardless of the
	// theme. There is no Sound field to pair with it: the step
	// buttons already sound through the ordinary click path, so the
	// only cue this control raises on its own is Theme.Sounds.Error
	// for a step refused because the value already sits at Min or
	// Max (issue #468).
	// exportaudit:keep — caller-facing config (issue #468)
	SoundDisabled bool

	Disabled  bool
	Invisible bool
}

// NumericInput creates a locale-aware numeric input.
func NumericInput(cfg NumericInputCfg) View {
	applyNumericInputDefaults(&cfg)
	requireFocusID("NumericInput", cfg.FocusDisabled, cfg.ID)

	// A numeric input is an Input with steppers, so it takes its border
	// and radius from the theme's input style rather than a private copy
	// that no theme could reach (issue #300).
	dn := &defaultInputStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	locale := numericLocaleNormalize(cfg.Locale)
	stepCfg := numericStepCfgNormalize(cfg.StepCfg)

	field := numericInputField(cfg, locale, stepCfg, stepCfg.ShowButtons)
	if !stepCfg.ShowButtons {
		return field
	}

	colorHover := cfg.ColorHover
	colorBorderFocus := cfg.ColorBorderFocus
	focusID := cfg.ID

	content := []View{
		field,
		numericInputStepButtons(cfg, locale, stepCfg),
	}

	cfg.A11YLabel = a11yLabel(cfg.A11YLabel, cfg.Label)
	control := Row(ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleTextField,
		A11YState: a11yReadOnlyState(cfg.ReadOnly),
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Placeholder),
			A11YDescription: cfg.A11YDescription,
		},
		Width:       cfg.Width,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Sizing:      cfg.Sizing,
		Clip:        true,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     NoPadding,
		Invisible:   cfg.Invisible,
		Disabled:    cfg.Disabled,
		VAlign:      VAlignMiddle,
		Spacing:     SomeF(0),
		OnClick: func(ctx EventCtx) {
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
		},
		OnHover: func(ctx EventCtx) {
			if ctx.Window.IsFocus(focusID) {
				ctx.Window.setMouseCursor(CursorIBeam)
			} else {
				ctx.Layout.Shape.Color = colorHover
			}
		},
		AmendLayout: focusRingAmend(Color{}, colorBorderFocus),
		Content:     content,
	})
	return labelledField(cfg.Label, cfg.TextStyle, HAlignLeft, cfg.Sizing, control)
}

func numericInputField(
	cfg NumericInputCfg, locale NumericLocaleCfg,
	stepCfg NumericStepCfg, fillParent bool,
) View {
	sizing := cfg.Sizing
	var width, height, minWidth, maxWidth, minHeight, maxHeight float32
	if fillParent {
		sizing = FillFill
	} else {
		width = cfg.Width
		height = cfg.Height
		minWidth = cfg.MinWidth
		maxWidth = cfg.MaxWidth
		minHeight = cfg.MinHeight
		maxHeight = cfg.MaxHeight
	}
	inputID := cfg.ID
	if fillParent && len(cfg.ID) > 0 {
		inputID = ScopeID(cfg.ID, "field")
	}
	color := cfg.Color
	colorHover := cfg.ColorHover
	colorBorder := cfg.ColorBorder
	colorBorderFocus := cfg.ColorBorderFocus
	sizeBorder := cfg.SizeBorder
	radius := cfg.Radius
	if fillParent {
		color = ColorTransparent
		colorHover = ColorTransparent
		colorBorder = ColorTransparent
		colorBorderFocus = ColorTransparent
		sizeBorder = Opt[float32]{}
		radius = Opt[float32]{}
	}

	modeCfg := numericModeCfgFromInput(cfg)

	return Input(InputCfg{
		ID: inputID,
		// NumericInput focus is default-on; propagate FocusDisabled
		// intent to the inner Input.
		FocusDisabled:   cfg.FocusDisabled,
		ReadOnly:        cfg.ReadOnly,
		Text:            cfg.Text,
		Placeholder:     cfg.Placeholder,
		A11YCfg:         cfg.A11YCfg,
		Sizing:          sizing,
		Width:           width,
		Height:          height,
		MinWidth:        minWidth,
		MaxWidth:        maxWidth,
		noMinWidthFloor: true,
		MinHeight:       minHeight,
		MaxHeight:       maxHeight,
		Padding:         cfg.Padding,
		Radius:          radius,
		SizeBorder:      sizeBorder,
		// The pre-commit transform admits digits, the locale's group and
		// decimal separators and a sign, so the reserved descent is
		// provably empty and the value can be centred on its ink rather
		// than on its line box (issue #346).
		opticalDigitCenter: true,
		Color:              color,
		ColorHover:         colorHover,
		ColorBorder:        colorBorder,
		ColorBorderFocus:   colorBorderFocus,
		TextStyle:          cfg.TextStyle,
		PlaceholderStyle:   cfg.PlaceholderStyle,
		Disabled:           cfg.Disabled,
		Invisible:          cfg.Invisible,
		OnTextChanged:      cfg.OnTextChanged,
		OnKeyDown:          numericInputOnKeyDown(cfg, locale, stepCfg),
		onMouseScroll:      numericInputOnWheel(cfg, locale, stepCfg),
		PreTextChange: func(current, proposed string) (string, bool) {
			return numericInputPreCommitTransformMode(
				current, proposed, cfg.Decimals, locale, modeCfg)
		},
		PostCommitNormalize: func(text string, _ InputCommitReason) string {
			_, committed := numericInputCommitResultMode(
				text, cfg.Value, cfg.Min, cfg.Max,
				cfg.Decimals, locale, modeCfg)
			return committed
		},
		OnTextCommit: func(text string, _ InputCommitReason, ctx EventCtx) {
			// A read-only inner Input still fires OnTextCommit on Enter
			// (with text unchanged); do not surface it as a value commit.
			if cfg.ReadOnly {
				return
			}
			value, committed := numericInputCommitResultMode(
				text, cfg.Value, cfg.Min, cfg.Max,
				cfg.Decimals, locale, modeCfg)
			if cfg.OnValueCommit != nil {
				cfg.OnValueCommit(value, committed, ctx)
			}
		},
	})
}

func numericInputStepButtons(cfg NumericInputCfg, locale NumericLocaleCfg, stepCfg NumericStepCfg) View {
	// The step triangle sits below the field text by a fixed 4pt, not by
	// a rung: the field text is caller-supplied and lands anywhere, so
	// there is no rung to step from. At the default 16 the drop spans
	// two rungs (16 -> 14 -> 12). This is the §2 step issue #335 left
	// open — a named handle would have to be a "one rung down from an
	// arbitrary size" operation the ladder does not offer, so the
	// arithmetic stays marked and visible to the gate rather than
	// disguised (issue #335 §2).
	//
	// Bounds are both named rungs of the installed theme: never below
	// N6 (tiny), the smallest a triangle stays legible at, and never
	// above the text it decorates — a theme with a large SizeTextTiny
	// can otherwise floor the triangle bigger than the field.
	triangleSize := f32Min(
		f32Max(cfg.TextStyle.Size-4, guiTheme.N6.Size), // ergonomics-audit:visual
		cfg.TextStyle.Size,
	)
	// A triangle is a glyph, not a label: it keeps its own ink
	// rather than taking the face's cap band (issue #346).
	triangleStyle := glyphStyle(TextStyle{
		Color:  cfg.TextStyle.Color,
		Size:   triangleSize,
		Family: cfg.TextStyle.Family,
	})
	baseColor := cfg.Color

	stepUpID := ""
	if len(cfg.ID) > 0 {
		stepUpID = ScopeID(cfg.ID, "step_up")
	}
	stepDownID := ""
	if len(cfg.ID) > 0 {
		stepDownID = ScopeID(cfg.ID, "step_down")
	}

	// Read-only fields disable the steppers so they cannot mutate the
	// value (numericInputApplyStep is the load-bearing gate; this is the
	// affordance).
	stepDisabled := cfg.Disabled || cfg.ReadOnly

	return Column(ContainerCfg{
		Spacing:   SomeF(0),
		Sizing:    FitFill,
		Disabled:  stepDisabled,
		Invisible: cfg.Invisible,
		Padding:   NewPadding(0, PadSmall, 0, 0),
		Content: []View{
			Button(ButtonCfg{
				ID:         stepUpID,
				Disabled:   stepDisabled,
				Sizing:     FillFill,
				Padding:    NoPadding,
				Color:      baseColor,
				Colors:     ColorSet{Hover: cfg.Colors.Hover, Click: cfg.Colors.BorderFocus, Focus: cfg.Colors.Hover, Border: ColorTransparent},
				SizeBorder: SomeF(0),
				Radius:     SomeF(0),
				OnClick: func(ctx EventCtx) {
					numericInputApplyStep(
						ctx.Layout, cfg, locale, stepCfg,
						1.0, ctx.Event, ctx.Window)
				},
				Content: []View{
					Text(TextCfg{
						Text:      "\u25B2",
						TextStyle: triangleStyle,
					}),
				},
			}),
			Button(ButtonCfg{
				ID:         stepDownID,
				Disabled:   stepDisabled,
				Sizing:     FillFill,
				Padding:    NoPadding,
				Color:      baseColor,
				Colors:     ColorSet{Hover: cfg.Colors.Hover, Click: cfg.Colors.BorderFocus, Focus: cfg.Colors.Hover, Border: ColorTransparent},
				SizeBorder: SomeF(0),
				Radius:     SomeF(0),
				OnClick: func(ctx EventCtx) {
					numericInputApplyStep(
						ctx.Layout, cfg, locale, stepCfg,
						-1.0, ctx.Event, ctx.Window)
				},
				Content: []View{
					Text(TextCfg{
						Text:      "\u25BC",
						TextStyle: triangleStyle,
					}),
				},
			}),
		},
	})
}

// numericInputOnKeyDown returns the Up/Down stepping handler, or nil
// when the caller opted out. Stepping is default-on because arrow keys
// are the spinbox convention (issue #503); before this the field
// carried a Keyboard flag that nothing read.
func numericInputOnKeyDown(
	cfg NumericInputCfg, locale NumericLocaleCfg, stepCfg NumericStepCfg,
) func(EventCtx) {
	if stepCfg.KeyboardDisabled {
		return nil
	}
	return func(ctx EventCtx) {
		if ctx.Event == nil {
			return
		}
		var dir float64
		switch ctx.Event.KeyCode {
		case KeyUp:
			dir = 1
		case KeyDown:
			dir = -1
		default:
			return
		}
		// A read-only field declines the key rather than swallowing it,
		// so an enclosing list still moves its selection. Consume only
		// on the paths that actually step.
		if cfg.ReadOnly {
			return
		}
		numericInputApplyStep(ctx.Layout, cfg, locale, stepCfg,
			dir, ctx.Event, ctx.Window)
		ctx.Consume()
	}
}

// numericInputOnWheel returns the wheel stepping handler, or nil unless
// the caller opted in. Opt-in by design: a field that eats the wheel
// stops the form under the pointer from scrolling.
func numericInputOnWheel(
	cfg NumericInputCfg, locale NumericLocaleCfg, stepCfg NumericStepCfg,
) func(EventCtx) {
	if !stepCfg.MouseWheel || cfg.ReadOnly {
		return nil
	}
	return func(ctx EventCtx) {
		if ctx.Event == nil {
			return
		}
		// ScrollY carries lines for a wheel and points for a trackpad
		// (see the Event doc), so only the sign is portable here: one
		// step per event, with Shift/Alt applying their multipliers
		// through numericInputStepResultClamped.
		var dir float64
		switch {
		case ctx.Event.ScrollY > 0:
			dir = 1
		case ctx.Event.ScrollY < 0:
			dir = -1
		default:
			return
		}
		numericInputApplyStep(ctx.Layout, cfg, locale, stepCfg,
			dir, ctx.Event, ctx.Window)
		ctx.Consume()
	}
}

func numericInputApplyStep(
	layout *Layout,
	cfg NumericInputCfg,
	locale NumericLocaleCfg,
	stepCfg NumericStepCfg,
	dir float64,
	e *Event,
	w *Window,
) {
	// Choke point for every step mutation. Read-only fields must not
	// change value, so gate here by construction — even if a disabled
	// step button's OnClick were somehow reached.
	if cfg.ReadOnly {
		return
	}
	modeCfg := numericModeCfgFromInput(cfg)
	modifiers := ModNone
	if e != nil {
		modifiers = e.Modifiers
	}
	value, committed, clamped := numericInputStepResultClamped(
		cfg.Text, cfg.Value, cfg.Min, cfg.Max,
		cfg.Decimals, stepCfg, locale, dir,
		modifiers, modeCfg)
	if clamped {
		// The step was understood but the value could not move: it
		// already sat at Min or Max. The button's own click cue, if
		// the theme sets one, still fires through dispatch; this
		// adds the refusal on top (issue #468).
		playSoundCue(
			resolveSoundCue(guiTheme.Sounds.Error, SoundNone,
				cfg.SoundDisabled), w)
	}
	if cfg.OnValueCommit != nil {
		cfg.OnValueCommit(value, committed, EventCtx{layout, nil, w})
	}
}

func numericModeCfgFromInput(cfg NumericInputCfg) numericModeCfg {
	switch cfg.Mode {
	case NumericCurrency:
		return numericModeCfg{
			mode:              NumericCurrency,
			affix:             cfg.currencyCfg.Symbol,
			affixPosition:     cfg.currencyCfg.Position,
			affixSpacing:      cfg.currencyCfg.symbolSpacing,
			displayMultiplier: 1.0,
		}
	case NumericPercent:
		return numericModeCfg{
			mode:              NumericPercent,
			affix:             cfg.percentCfg.Symbol,
			affixPosition:     cfg.percentCfg.Position,
			affixSpacing:      cfg.percentCfg.symbolSpacing,
			displayMultiplier: 100.0,
		}
	default:
		return numericModeCfg{
			mode:              numericNumber,
			displayMultiplier: 1.0,
		}
	}
}

func applyNumericInputDefaults(cfg *NumericInputCfg) {
	d := &defaultInputStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, d.colorClick,
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, &cfg.ColorHover, nil, nil,
		&cfg.ColorBorder, &cfg.ColorBorderFocus)
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.PaddingField
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = DefaultTextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = defaultInputStyle.PlaceholderStyle
	}
	cfg.MinWidth = fieldMinWidth(cfg.MinWidth, cfg.Width)
	if cfg.currencyCfg == (numericCurrencyModeCfg{}) {
		cfg.currencyCfg = numericCurrencyModeCfg{
			Symbol:   "$",
			Position: affixPrefix,
		}
	}
	if cfg.percentCfg == (numericPercentModeCfg{}) {
		cfg.percentCfg = numericPercentModeCfg{
			Symbol:   "%",
			Position: affixSuffix,
		}
	}
}
