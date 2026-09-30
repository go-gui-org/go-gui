package gui

// InputGroupCfg configures an input group: a row of form controls
// joined into one shape (issue #820). The group draws one rounded
// border around all segments and a divider between each pair. The
// segments have no border and no radius of their own, so they share
// their edges, and only the outer corners are round. This is the
// Bootstrap "input-group".
//
// A titled container is different: it is a frame around separate
// controls, each with its own border.
//
// Build the segments with InputGroupText, InputGroupInput,
// InputGroupSelect and InputGroupButton. Those constructors remove the
// segment's own border, radius and focus glow, so a caller cannot
// build a segment with a doubled seam.
type InputGroupCfg struct {
	A11YCfg
	// ID scopes the segments: a segment with ID "email" in a group with
	// ID "login" has the effective ID "login:email". SetFocus and the
	// other addressing APIs take that effective ID. The group needs its
	// ID to show focus when a segment holds it.
	ID       string `gui:"required"`
	Segments []InputGroupSegment
	// Sizing is FitFit by default. For a group that fills its row, set
	// FillFit here and FillFit on the segment that takes the extra
	// width (usually the input).
	Sizing     Sizing
	SizeBorder Border
	Radius     Radius
	// Colors themes the group: Base is the fill behind the segments,
	// Border the outline and the dividers, BorderFocus the outline
	// while a segment holds focus. Unset takes the theme's input
	// colors.
	Colors   ColorSet
	Disabled bool
}

// InputGroupSegment is one part of an InputGroup. Make it with
// InputGroupText, InputGroupInput, InputGroupSelect or
// InputGroupButton; the zero value is an empty slot the group skips.
type InputGroupSegment struct {
	view View
}

// InputGroupTextCfg configures a text addon of an input group: a
// label that is part of the control, such as "@", "$" or ".00".
type InputGroupTextCfg struct {
	Text string
	// TextStyle styles the text. Zero takes the input's text style.
	TextStyle TextStyle
}

// InputGroupText makes a text addon segment. It has the theme's
// background fill, so it reads as a label and not as a place to type,
// and the field inset, so the group has the height of an Input.
func InputGroupText(cfg InputGroupTextCfg) InputGroupSegment {
	ts := cfg.TextStyle
	if ts == (TextStyle{}) {
		// The same default as Input, so the addon text and the typed
		// text beside it match.
		ts = DefaultTextStyle
	}
	return InputGroupSegment{view: Row(ContainerCfg{
		// ColorBackground, not ColorPanel: the light ladder has
		// ColorPanel == ColorInterior == white, so a panel addon
		// vanished into the input beside it. The background is one
		// step off the field fill in both polarities.
		Color:      guiTheme.ColorBackground,
		Padding:    guiTheme.PaddingField,
		SizeBorder: NoBorder,
		Radius:     NoRadius,
		Sizing:     FitFill,
		VAlign:     VAlignMiddle,
		// Same optical correction a Button label takes, so the addon
		// text sits on the line of the text in the segment beside it.
		AmendLayout: OpticalCenterText,
		Content:     []View{Text(TextCfg{Text: cfg.Text, TextStyle: ts})},
	})}
}

// InputGroupInput makes an Input segment. cfg is an ordinary InputCfg.
// The group removes its border, radius and focus glow, and makes its
// height fill the group. A Label does not render inside a group; it
// becomes the segment's A11YLabel when that is unset.
func InputGroupInput(cfg InputCfg) InputGroupSegment {
	// Not a11yLabel: that treats its fallback as an ID path and keeps
	// only the part after the last IDSep. A Label is prose, and a colon
	// in it ("Price: USD") is part of the name.
	if cfg.A11YLabel == "" {
		cfg.A11YLabel = cfg.Label
	}
	cfg.Label = ""
	cfg.SizeBorder = NoBorder
	cfg.Radius = NoRadius
	cfg.noFocusRing = true
	cfg.Sizing = inputGroupSegmentSizing(cfg.Sizing)
	return InputGroupSegment{view: Input(cfg)}
}

// InputGroupSelect makes a Select segment. The group changes cfg the
// same way as for InputGroupInput. The dropdown is a float, so the
// group's rounded clip does not cut it off.
func InputGroupSelect(cfg SelectCfg) InputGroupSegment {
	// Not a11yLabel: that treats its fallback as an ID path and keeps
	// only the part after the last IDSep. A Label is prose, and a colon
	// in it ("Price: USD") is part of the name.
	if cfg.A11YLabel == "" {
		cfg.A11YLabel = cfg.Label
	}
	cfg.Label = ""
	cfg.SizeBorder = NoBorder
	cfg.Radius = NoRadius
	cfg.noFocusRing = true
	cfg.Sizing = inputGroupSegmentSizing(cfg.Sizing)
	return InputGroupSegment{view: Select(cfg)}
}

// InputGroupButton makes a Button segment. The group removes its
// border, radius and focus glow, and makes its height fill the group.
// Unset Padding takes the field inset, so the button does not make the
// group taller than an Input.
func InputGroupButton(cfg ButtonCfg) InputGroupSegment {
	cfg.SizeBorder = NoBorder
	cfg.Radius = NoRadius
	cfg.noFocusRing = true
	if !cfg.Padding.IsSet() {
		cfg.Padding = guiTheme.PaddingField
	}
	cfg.Sizing = inputGroupSegmentSizing(cfg.Sizing)
	return InputGroupSegment{view: Button(cfg)}
}

// inputGroupSegmentSizing keeps the caller's width and makes the
// height fill, so every segment is as tall as the tallest one and the
// dividers run the full height.
func inputGroupSegmentSizing(s Sizing) Sizing {
	s = s.Or(FitFit)
	s.Height = sizingFill
	return s
}

// InputGroup creates an input group. See InputGroupCfg.
func InputGroup(cfg InputGroupCfg) View {
	d := &defaultInputStyle
	sizeBorder := cfg.SizeBorder.Or(d.SizeBorder)
	radius := cfg.Radius.Or(d.Radius)
	// BorderPx and RadiusPx take any float. A NaN width would reach the
	// divider as max(1, NaN) == NaN and poison the row's layout; a
	// negative one means nothing. Both fall back to none.
	if !f32IsFinite(sizeBorder) || sizeBorder < 0 {
		sizeBorder = 0
	}
	if !f32IsFinite(radius) || radius < 0 {
		radius = 0
	}
	colors := cfg.Colors.resolved(Color{}, d.Colors)
	// A borderless theme (SizeBorder 0) keeps a visible divider, as
	// SegmentedControl does: without it adjacent segments with the
	// same fill would merge.
	sizeDivider := max(1, sizeBorder)

	content := make([]View, 0, max(0, 2*len(cfg.Segments)-1))
	for _, seg := range cfg.Segments {
		if seg.view == nil {
			continue
		}
		if len(content) > 0 {
			content = append(content, Rectangle(RectangleCfg{
				Width:  sizeDivider,
				Color:  colors.Border,
				Sizing: FixedFill,
			}))
		}
		content = append(content, seg.view)
	}

	return Row(ContainerCfg{
		ID:       cfg.ID,
		A11YRole: AccessRoleGroup,
		A11YCfg:  cfg.A11YCfg,
		Color:    colors.Base,
		// The group owns the only border. The segments are drawn
		// inside it with none of their own.
		ColorBorder: colors.Border,
		SizeBorder:  BorderPx(sizeBorder),
		Radius:      RadiusPx(radius),
		// ClipContents clips the segments to the rounded area inside
		// the border. So an end segment's square fill does not cover
		// the rounded corner, and no per-corner radius is needed.
		ClipContents: true,
		Padding:      PaddingNone,
		// The dividers are the separation; a gap would open a seam of
		// group fill between a segment and its divider.
		Spacing:     NoSpacing,
		Sizing:      cfg.Sizing.Or(FitFit),
		VAlign:      VAlignMiddle,
		Disabled:    cfg.Disabled,
		AmendLayout: inputGroupFocusAmend(colors.BorderFocus),
		Content:     content,
	})
}

// inputGroupFocusAmend shows focus on the group while one of its
// segments holds it. The segments draw no focus glow of their own: the
// group clips them, so a segment's glow would be cut off at the group
// edge and spill onto the segments beside it.
func inputGroupFocusAmend(colorBorder Color) func(EventCtx) {
	// Captured at generation; see focusRingAmend.
	ring := guiTheme.focusRing
	return func(ctx EventCtx) {
		if ctx.Layout == nil || ctx.Window == nil {
			return
		}
		shape := ctx.Layout.Shape
		if shape == nil || shape.Disabled {
			return
		}
		// The segments' IDs are scoped under the group's, so focus is
		// within the group exactly when the focus ID has the group's
		// effective ID as a path prefix.
		if !targetWithin(ctx.Window.FocusID(), shape.idKey()) {
			return
		}
		if colorBorder.IsSet() {
			shape.ColorBorder = colorBorder
		}
		applyFocusRingShadow(shape, ctx.Window, ring)
	}
}
