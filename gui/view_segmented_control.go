package gui

// SegmentOption defines one segment of a SegmentedControlCfg.
type SegmentOption struct {
	Label string
	Value string
	// Icon is a glyph in the theme icon font (an Icon* constant).
	// A segment can show an icon, a label, or both.
	Icon     string
	Disabled bool
}

// NewSegmentOption creates a SegmentOption.
func NewSegmentOption(label, value string) SegmentOption {
	return SegmentOption{Label: label, Value: value}
}

// SegmentedControlCfg configures a segmented control: a row of
// segments in one shared track, one of them selected (issue #600).
//
// The API follows RadioButtonGroupCfg (Value, Items, Options, OnSelect)
// so the two controls swap easily. The difference is the look and the
// focus model: the whole control is one tab stop, and the arrow keys
// move the selection.
type SegmentedControlCfg struct {
	// TextStyle styles the segment labels. Zero takes the theme
	// default.
	TextStyle TextStyle
	// TextStyleIcon styles the segment icons. Zero takes the theme's
	// body-size icon role.
	TextStyleIcon TextStyle

	OnSelect func(string, EventCtx)
	Value    string

	A11YCfg
	// Items is a convenience field for simple string lists. Each
	// string becomes a SegmentOption with Label==Value. When set,
	// Items takes precedence over Options.
	Items   []string
	Options []SegmentOption
	ID      string `gui:"required,focus"`
	// Padding is the text inset of one segment. Unset takes the
	// theme's field inset, so the control shares a row height with an
	// Input or a Select.
	Padding    Padding
	SizeBorder Opt[float32]
	Radius     Opt[float32]
	// Colors themes the track: Base is its fill, Border and
	// BorderFocus its outline.
	Colors ColorSet
	// ColorsSegment themes one segment: Base (at rest, transparent by
	// default), Hover, Click, Selected (the pill) and Disabled.
	// exportaudit:keep — caller-facing config (issue #600)
	ColorsSegment ColorSet
	// Sizing is FitFit by default: each segment fits its content.
	// A Fill width (FillFit) stretches the control and gives every
	// segment the same width when there is room. A segment never
	// shrinks below its content, so a long label can stay wider.
	Sizing Sizing
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Disabled      bool

	// Sound overrides the theme's selection cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	Sound SoundCue

	// SoundDisabled suppresses every segment's sound regardless of the
	// theme and of Sound above.
	SoundDisabled bool
}

type segmentedControlView struct {
	cfg SegmentedControlCfg
	// ownDisabledColor records that the caller set ColorsSegment.Disabled.
	// Read before the defaults resolve it, because after that an unset
	// field and the theme value look the same.
	ownDisabledColor bool
}

// SegmentedControl creates a segmented control.
func SegmentedControl(cfg SegmentedControlCfg) View {
	requireFocusID("SegmentedControl", cfg.FocusDisabled, cfg.ID)
	ownDisabledColor := cfg.ColorsSegment.Disabled.IsSet()
	applySegmentedControlDefaults(&cfg)
	if len(cfg.Items) > 0 {
		n := min(len(cfg.Items), maxDataConvLen)
		cfg.Options = make([]SegmentOption, n)
		for i := range n {
			cfg.Options[i] = SegmentOption{
				Label: cfg.Items[i], Value: cfg.Items[i]}
		}
	}
	return &segmentedControlView{cfg: cfg, ownDisabledColor: ownDisabledColor}
}

func applySegmentedControlDefaults(cfg *SegmentedControlCfg) {
	s := &defaultSegmentedControlStyle
	cfg.Colors = cfg.Colors.resolved(Color{}, s.colors)
	cfg.ColorsSegment = cfg.ColorsSegment.resolved(Color{}, s.colorsSegment)
	if !cfg.Padding.IsSet() {
		cfg.Padding = s.paddingSegment
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.textStyle
	} else {
		cfg.TextStyle = mergeTextStyle(cfg.TextStyle, s.textStyle)
	}
	if cfg.TextStyleIcon == (TextStyle{}) {
		cfg.TextStyleIcon = s.textStyleIcon
	} else {
		cfg.TextStyleIcon = mergeTextStyle(cfg.TextStyleIcon, s.textStyleIcon)
	}
	cfg.Sizing = cfg.Sizing.Or(FitFit)
}

// GenerateLayout builds the track and its segments. It resolves the
// effective ID here, not in the factory: the factory runs while the
// parent's Content is built, before the framework enters this
// control's scope (gui/CLAUDE.md, "A widget factory that reads window
// state must defer its build").
func (sv *segmentedControlView) GenerateLayout(w *Window) Layout {
	cfg := &sv.cfg
	s := &defaultSegmentedControlStyle
	effID := w.EffID(cfg.ID)

	sizeBorder := cfg.SizeBorder.Get(s.sizeBorder)
	radius := cfg.Radius.Get(s.radius)
	// Concentric corners: keep the pill's radius the track's less the
	// inset, also when the caller changed the track radius.
	radiusSegment := s.radiusSegment
	if cfg.Radius.IsSet() {
		radiusSegment = max(0, radius-s.padding.Top)
	}

	// The keyboard walks these; a disabled control disables them all.
	n := len(cfg.Options)
	navValues := make([]string, n)
	navDisabled := make([]bool, n)
	selectedIdx := -1
	for i, opt := range cfg.Options {
		navValues[i] = opt.Value
		navDisabled[i] = cfg.Disabled || opt.Disabled
		// Disabled does not hide the selection: a disabled control
		// still shows which value it holds, as a disabled radio does.
		// Only the keyboard skips disabled segments.
		if selectedIdx < 0 && opt.Value == cfg.Value {
			selectedIdx = i
		}
	}

	// A Fill width on the control shares the width out evenly: every
	// segment fills, so they all grow to the same width.
	segSizing := FitFit
	if cfg.Sizing.Width == sizingFill {
		segSizing = FillFit
	}

	// Picking a segment is choosing one of several: the selection
	// role, as for a radio or a tab (issue #467).
	soundCue := resolveSoundCue(
		guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)

	onSelect := cfg.OnSelect
	focusID := ""
	if !cfg.FocusDisabled {
		focusID = effID
	}

	// Segments and the dividers between them: 2n-1 children.
	content := make([]View, 0, max(0, 2*n-1))
	for i, opt := range cfg.Options {
		if i > 0 {
			// A divider touching the pill would draw a line along
			// its edge, so it goes transparent there, as in Apple's
			// control. It keeps its width so the segments do not
			// shift when the selection moves.
			divColor := s.colorDivider
			if i == selectedIdx || i-1 == selectedIdx {
				divColor = ColorTransparent
			}
			content = append(content, Rectangle(RectangleCfg{
				Width:  s.sizeDivider,
				Color:  divColor,
				Sizing: FixedFill,
			}))
		}
		content = append(content, segmentButton(segmentButtonArgs{
			cfg:              cfg,
			opt:              opt,
			id:               ScopeIDN(effID, "opt", i),
			selected:         i == selectedIdx,
			disabled:         navDisabled[i],
			sizing:           segSizing,
			radius:           radiusSegment,
			soundCue:         soundCue,
			onSelect:         onSelect,
			focusID:          focusID,
			textSelected:     s.textStyleSelected,
			textDisabled:     s.textStyleDisabled,
			ownDisabledColor: sv.ownDisabledColor,
		}))
	}

	disabled := cfg.Disabled
	value := cfg.Value
	return generateViewLayout(Row(ContainerCfg{
		ID:        effID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleRadioGroup,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Color:       cfg.Colors.Base,
		ColorBorder: cfg.Colors.Border,
		SizeBorder:  SomeF(sizeBorder),
		Radius:      SomeF(radius),
		Padding:     s.padding,
		// The dividers are the separation; a gap would open a seam of
		// track color between a segment and its divider.
		Spacing:  NoBorder,
		Sizing:   cfg.Sizing,
		VAlign:   VAlignMiddle,
		Disabled: cfg.Disabled,
		// The track is the focus target, so it carries the ring.
		AmendLayout: focusRingAmend(Color{}, cfg.Colors.BorderFocus),
		OnKeyDown: func(ctx EventCtx) {
			// Same model as the tab strip: Left/Right (and Up/Down)
			// move and wrap, Home/End jump, Space/Enter fire the
			// current one again, disabled segments are skipped.
			tabControlOnKeydown(disabled, navValues, navDisabled,
				value, onSelect, focusID, ctx.Event, ctx.Window)
		},
		Content: content,
	}), w)
}

// segmentButtonArgs carries one segment's inputs to segmentButton.
type segmentButtonArgs struct {
	cfg              *SegmentedControlCfg
	opt              SegmentOption
	id               string
	selected         bool
	disabled         bool
	sizing           Sizing
	radius           float32
	soundCue         SoundCue
	onSelect         func(string, EventCtx)
	focusID          string
	textSelected     TextStyle
	textDisabled     TextStyle
	ownDisabledColor bool
}

// segmentButton builds one segment. It is a Button so hover, press
// and the selected fill come from the same ColorSet.pick path as a
// tab (#741); it never takes focus, because the track is the one tab
// stop.
func segmentButton(a segmentButtonArgs) View {
	// Only the color changes with the state. The face and size stay
	// the caller's, or a pill label in a different size would move the
	// segment widths each time the selection moves.
	ts := a.cfg.TextStyle
	if a.disabled {
		// The disabled role's alpha and its marker, so layoutDisables
		// does not dim the label a second time.
		ts = withRoleAlpha(ts, a.textDisabled)
	} else if a.selected {
		ts.Color = a.textSelected.Color
	}

	content := make([]View, 0, 2)
	if a.opt.Icon != "" {
		// An icon on the pill takes the label's on-accent color, or
		// it would draw body-colored ink on the accent fill.
		iconStyle := a.cfg.TextStyleIcon
		if a.selected && !a.disabled {
			iconStyle.Color = a.textSelected.Color
		}
		content = append(content, Text(TextCfg{
			Text: a.opt.Icon, TextStyle: iconStyle}))
	}
	if a.opt.Label != "" {
		content = append(content, Text(TextCfg{
			Text: a.opt.Label, TextStyle: ts}))
	}

	a11yState := AccessStateNone
	if a.selected {
		a11yState = AccessStateSelected
	}

	var onClick func(EventCtx)
	if !a.disabled {
		value := a.opt.Value
		onSelect := a.onSelect
		focusID := a.focusID
		onClick = func(ctx EventCtx) {
			if onSelect != nil {
				onSelect(value, ctx)
			}
			// The segment consumes the press, so the track below it
			// never sees the press that would focus it. Focus it
			// here. EventFn dispatch holds no frame lock.
			if focusID != "" {
				ctx.Window.SetFocus(focusID)
			}
			ctx.Consume()
		}
	}

	// A disabled segment rests transparent, which would also erase a
	// disabled pill. The pill keeps its fill at the dimmed alpha: an
	// explicit Disabled replaces the render dim (#741), so it states
	// the dim itself. A caller's own ColorsSegment.Disabled wins.
	colors := a.cfg.ColorsSegment
	if a.selected && !a.ownDisabledColor {
		colors.Disabled = dimAlpha(colors.Selected)
	}

	// A segment with only an icon is named by its value, so a screen
	// reader still has something to say.
	label := a.opt.Label
	if label == "" {
		label = a.opt.Value
	}

	return Button(ButtonCfg{
		ID:            a.id,
		FocusDisabled: true,
		A11YRole:      AccessRoleRadioButton,
		A11YState:     a11yState,
		A11YCfg:       A11YCfg{A11YLabel: label},
		Colors:        colors,
		selected:      a.selected,
		Padding:       a.cfg.Padding,
		SizeBorder:    NoBorder,
		Radius:        SomeF(a.radius),
		Sizing:        a.sizing,
		Disabled:      a.disabled,
		OnClick:       onClick,
		// SoundDisabled as well as Sound: Button reads a resolved
		// SoundNone as "unset" and falls back to its click cue.
		Sound:         a.soundCue,
		SoundDisabled: a.soundCue == SoundNone,
		Content:       content,
	})
}
