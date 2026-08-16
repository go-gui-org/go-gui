package gui

import "strconv"

// ColorFieldsCfg configures the numeric side of a color editor: a hex
// field, the R/G/B/A channels, and optionally the H/S/L channels.
//
// Text entry is where a color is named exactly rather than aimed at, so
// the fields commit on every keystroke that parses and ignore the ones
// that do not — a half-typed "#3b8" leaves the current color alone
// instead of jumping to black.
type ColorFieldsCfg struct {
	OnChange func(HSLA, EventCtx)
	ID       string `gui:"required"`

	A11YCfg

	Value HSLA

	// ShowHSL adds the hue/saturation/lightness row under the RGBA
	// one.
	ShowHSL bool
	// HideHex drops the hex field, for a caller that shows the hex
	// value elsewhere.
	// exportaudit:keep — caller-facing layout choice
	HideHex bool
	// ShowSwatch puts a ColorSwatch of Value to the right of the hex
	// field, where the hex string it previews is. Ignored with HideHex —
	// a swatch with nothing to sit beside belongs in the caller's own
	// layout, as a plain ColorSwatch.
	//
	// exportaudit:keep — caller-facing layout choice; ColorPicker is the
	// only in-repo user.
	ShowSwatch bool
	// SwatchSize is the swatch's height; it is drawn twice as wide.
	// Zero takes the default.
	//
	// exportaudit:keep — caller-facing sizing
	SwatchSize float32

	TextStyle TextStyle
	// FieldWidth is the width of one channel input. Zero takes the
	// default.
	// exportaudit:keep — caller-facing sizing
	FieldWidth float32
}

type colorFieldsView struct {
	cfg ColorFieldsCfg
}

// ColorFields creates the hex and channel inputs for a color.
func ColorFields(cfg ColorFieldsCfg) View {
	RequireID("ColorFields", cfg.ID)
	if cfg.FieldWidth <= 0 {
		cfg.FieldWidth = defaultColorFieldWidth
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = defaultColorPickerStyle.TextStyle
	}
	if cfg.SwatchSize <= 0 {
		cfg.SwatchSize = defaultColorFieldsSwatch
	}
	return &colorFieldsView{cfg: cfg}
}

const (
	// defaultColorFieldWidth holds three digits with room for the caret,
	// and stays just above the widest RGBA label ("Green") so the label
	// never widens the column. It sets the whole picker's width — four
	// of these is its widest row — so the slack here is what shows up as
	// empty space beside the shorter rows.
	defaultColorFieldWidth = 44
	// defaultHexFieldWidth holds the widest value the field ever shows
	// — "#RRGGBBAA", which Hex() emits whenever alpha is not full — so
	// pinning the field to it never clips.
	defaultHexFieldWidth = 115
	// defaultColorFieldsSwatch matches the hex input's height closely
	// enough that the two read as one row.
	defaultColorFieldsSwatch = 28
	// colorFieldsSwatchAspect widens the swatch relative to SwatchSize,
	// which stays the height so the swatch keeps matching the hex
	// input's line.
	colorFieldsSwatchAspect = 2
	// colorFieldLabelDrop takes the channel labels two steps down from
	// the value text — the theme's N3-to-N5 step. The label names the
	// field; the value is what gets read, so the value keeps the larger
	// size. It also stops a long label ("Lightness") from setting the
	// column width, which is what the whole picker's width follows.
	colorFieldLabelDrop = 4
	// colorFieldLabelMin floors the drop so a caller already using a
	// tiny TextStyle does not end up with an unreadable label.
	colorFieldLabelMin = 10
	// colorFieldCapRatio is a digit's cap height as a fraction of the
	// font size. It is the one metric TextMeasurer does not expose, and
	// it varies little across text faces; every other term in the
	// optical-centring correction is measured.
	colorFieldCapRatio = 0.72
	// colorFieldLeadRatio covers half-leading: FontHeight includes line
	// spacing beyond ascent+descent, split above and below the glyphs,
	// and the metrics do not separate it out. Measured from the
	// rendered field — the cap-height term alone left the ink 1.5
	// device pixels high at size 16.
	colorFieldLeadRatio = 0.047
	// colorFieldPadX is the fields' horizontal padding, matching the
	// input default (paddingTwoFour).
	colorFieldPadX = 4
	// colorFieldPadY is its vertical half, the baseline the optical
	// correction adjusts around.
	colorFieldPadY = 2
	// colorFieldLabelAlpha dims the label against its value. Size alone
	// separates them only while both are read in isolation; dropping
	// contrast as well is what makes the column scan as "value, with a
	// name over it" rather than as two competing lines. Kept well above
	// placeholder dimness (~0.4), which reads as disabled.
	colorFieldLabelAlpha = 0.7
)

// colorFieldLabelSize derives a channel label's size from the value
// text's, never going below the floor.
func colorFieldLabelSize(valueSize float32) float32 {
	return f32Max(valueSize-colorFieldLabelDrop, colorFieldLabelMin)
}

// colorFieldLabelColor dims a channel label relative to its value.
//
// Scaling the text color's alpha rather than picking a gray keeps one
// source of truth: the label tracks whatever color the caller or theme
// set, and it stays legible on a light theme and a dark one alike. A
// flat gray chosen against one background is wrong against the other.
// The same trick sets placeholder text in ThemeMaker.
func colorFieldLabelColor(c Color) Color {
	return RGBA(c.R, c.G, c.B, uint8(float32(c.A)*colorFieldLabelAlpha))
}

func (fv *colorFieldsView) GenerateLayout(w *Window) Layout {
	cfg := &fv.cfg
	id := w.EffID(cfg.ID)
	v := cfg.Value.Normalized()

	// One measurement for every field in the block; see
	// colorFieldPadding.
	pad := colorFieldPadding(w, cfg.TextStyle)

	rows := make([]View, 0, 3)
	if !cfg.HideHex {
		rows = append(rows, colorHexRow(cfg, id, v, pad))
	}
	rows = append(rows, colorRGBARow(cfg, id, v, pad))
	if cfg.ShowHSL {
		rows = append(rows, colorHSLRow(cfg, id, v, pad))
	}

	return generateViewLayout(&containerView{
		cfg: ContainerCfg{
			ID:      id,
			Padding: NoPadding,
			// Structural: these containers group fields, they are not
			// boxes. Without opting out they inherit the theme border
			// under a bordered theme, and every nesting level then adds
			// its width to the row and shifts its children.
			SizeBorder: NoBorder,
			Spacing:    Some(SpacingSmall),
			axis:       axisTopToBottom,
			A11YCfg:    cfg.A11YCfg,
		},
		content: rows,
	}, w)
}

// colorHexRow builds the hex text field, and — with ShowSwatch — the
// preview swatch beside it. The swatch belongs next to the hex value:
// both are readouts of the same color, one exact and one legible.
func colorHexRow(
	cfg *ColorFieldsCfg, id string, v HSLA, pad Padding,
) View {
	hex := colorHexField(cfg, id, v, pad)
	if !cfg.ShowSwatch {
		return hex
	}
	return Row(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder, // structural; see colorFieldsView
		Spacing:    Some(SpacingSmall),
		VAlign:     VAlignMiddle,
		Content: []View{
			hex,
			ColorSwatch(ColorSwatchCfg{
				ID:    ScopeID(id, "swatch"),
				Color: v.Color(),
				// Twice as wide as it is tall. A square beside a
				// text field reads as a button; a wide bar reads
				// as a sample of the value next to it, and gives
				// the checkerboard room to show under a
				// translucent color.
				Width:  cfg.SwatchSize * colorFieldsSwatchAspect,
				Height: cfg.SwatchSize,
			}),
		},
	})
}

func colorHexField(
	cfg *ColorFieldsCfg, id string, v HSLA, pad Padding,
) View {
	onChange := cfg.OnChange
	apply := func(text string, ctx EventCtx) {
		c, ok := ColorFromHex(text)
		if !ok || onChange == nil {
			return // still typing, or malformed: keep the current color
		}
		out := ColorToHSLA(c)
		// Hex carries no hue for a gray, so keep the hue the user was
		// already on rather than snapping the other controls to red.
		if out.S == 0 {
			out.H = v.H
		}
		onChange(out, ctx)
	}
	return Input(InputCfg{
		ID:        ScopeID(id, "hex"),
		Text:      v.Color().Hex(),
		TextStyle: cfg.TextStyle,
		Width:     defaultHexFieldWidth,
		// An Input is Fit-sized, so Width alone is only a starting
		// point and the field tracks its text. Hex() emits "#RRGGBB"
		// at full alpha and "#RRGGBBAA" otherwise, and even at one
		// length the glyphs differ in width, so the field — and the
		// swatch beside it — would move on nearly every edit. Both
		// bounds pin it: a floor alone still lets a long value grow.
		MinWidth: defaultHexFieldWidth,
		MaxWidth: defaultHexFieldWidth,
		Padding:  pad,
		A11YCfg:  A11YCfg{A11YLabel: "Hex color"},
		OnTextChanged: func(text string, ctx EventCtx) {
			apply(text, ctx)
		},
		OnTextCommit: func(
			text string, _ InputCommitReason, ctx EventCtx,
		) {
			apply(text, ctx)
		},
	})
}

// colorRGBARow builds the R/G/B/A inputs. They edit the Color the HSLA
// converts to, then convert back — the one place in these components
// where a value round-trips through RGBA, because that is exactly what
// the user asked to edit.
func colorRGBARow(
	cfg *ColorFieldsCfg, id string, v HSLA, pad Padding,
) View {
	c := v.Color()
	l := ActiveLocale
	vals := [4]uint8{c.R, c.G, c.B, c.A}
	labels := [4]string{l.strRed, l.strGreen, l.strBlue, l.strAlpha}

	fields := make([]View, 0, 4)
	for i := range 4 {
		fields = append(fields, colorFieldColumn(cfg, pad,
			labels[i], int(vals[i]), ScopeIDN(id, "rgb", i),
			func(text string, ctx EventCtx) {
				n, err := strconv.ParseUint(text, 10, 8)
				if err != nil || cfg.OnChange == nil {
					return
				}
				out := c
				switch i {
				case 0:
					out.R = uint8(n)
				case 1:
					out.G = uint8(n)
				case 2:
					out.B = uint8(n)
				case 3:
					out.A = uint8(n)
				}
				next := ColorToHSLA(out)
				// Same gray caveat as hex: RGB cannot carry the hue
				// of a color it has just desaturated.
				if next.S == 0 {
					next.H = v.H
				}
				cfg.OnChange(next, ctx)
			}))
	}
	return Row(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder, // structural; see colorFieldsView
		Spacing:    Some(SpacingSmall),
		Content:    fields,
	})
}

// colorHSLRow builds the H/S/L inputs, which edit the value directly
// with no RGBA round-trip and so cannot lose the hue.
func colorHSLRow(
	cfg *ColorFieldsCfg, id string, v HSLA, pad Padding,
) View {
	l := ActiveLocale
	type field struct {
		label string
		val   int
		maxV  int
	}
	fs := [3]field{
		{l.strHue, int(v.H + 0.5), 360},
		{l.strSat, int(v.S*100 + 0.5), 100},
		{l.strLightness, int(v.L*100 + 0.5), 100},
	}

	fields := make([]View, 0, 3)
	for i := range 3 {
		f := fs[i]
		fields = append(fields, colorFieldColumn(cfg, pad,
			f.label, f.val, ScopeIDN(id, "hsl", i),
			func(text string, ctx EventCtx) {
				n, err := strconv.Atoi(text)
				if err != nil || cfg.OnChange == nil {
					return
				}
				n = intClamp(n, 0, f.maxV)
				out := v
				switch i {
				case 0:
					out.H = float32(n)
				case 1:
					out.S = float32(n) / 100
				default:
					out.L = float32(n) / 100
				}
				cfg.OnChange(out, ctx)
			}))
	}
	return Row(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder, // structural; see colorFieldsView
		Spacing:    Some(SpacingSmall),
		Content:    fields,
	})
}

// centeredText centres a copy of a style, leaving the caller's
// unchanged.
func centeredText(s TextStyle) TextStyle {
	s.Align = TextAlignCenter
	return s
}

// colorFieldPadding centres a digit string optically rather than
// metrically inside its field.
//
// An Input centres the text's *line box*, which is correct and still
// looks wrong here: the line box reserves descent space, and digits and
// hex have no descenders, so the ink hangs above the box's middle by
// half the descent. Measured on the RGBA row, the gap below the digits
// was twice the gap above.
//
// The correction moves the line box down by shifting padding from the
// bottom to the top. Working from the box's own centre, the ink sits
// off by (ascent - cap - descent)/2, so top-minus-bottom must make up
// twice that. Ascent and descent are measured; only cap height is a
// ratio, and it is the smallest term.
func colorFieldPadding(w *Window, style TextStyle) Padding {
	even := NewPadding(
		colorFieldPadY, colorFieldPadX, colorFieldPadY, colorFieldPadX)
	if w == nil || w.textMeasurer == nil {
		return even // no metrics: metric centring is the best available
	}
	ascent := w.textMeasurer.FontAscent(style)
	descent := w.textMeasurer.FontHeight(style) - ascent
	shift := descent - ascent +
		style.Size*(colorFieldCapRatio+colorFieldLeadRatio)
	if shift <= 0 {
		return even // a face whose ink already sits centred
	}
	// Take from the bottom first so the field grows only by whatever
	// the bottom padding cannot cover.
	bottom := f32Max(colorFieldPadY-shift, 0)
	return NewPadding(bottom+shift, colorFieldPadX, bottom, colorFieldPadX)
}

// colorFieldColumn builds one labeled numeric input.
func colorFieldColumn(
	cfg *ColorFieldsCfg, pad Padding, label string, val int,
	inputID string, apply func(string, EventCtx),
) View {
	return Column(ContainerCfg{
		Padding:    NoPadding,
		SizeBorder: NoBorder, // structural; see colorFieldsView
		// Centered: the label is narrower than its field, and left
		// alignment hangs it off the field's left edge instead of
		// reading as a heading over it. TextAlignCenter alone cannot
		// do this — a Fit text shape is only as wide as its glyphs, so
		// there is nothing to center within.
		HAlign:  HAlignCenter,
		Spacing: SomeF(2),
		Content: []View{
			Text(TextCfg{
				Text: label,
				TextStyle: TextStyle{
					Color: colorFieldLabelColor(cfg.TextStyle.Color),
					Size:  colorFieldLabelSize(cfg.TextStyle.Size),
					Align: TextAlignCenter,
				},
			}),
			Input(InputCfg{
				ID:   inputID,
				Text: strconv.Itoa(val),
				// Centred under a centred label, so the pair reads
				// as one unit -- and so "60" and "140" do not sit
				// ragged against each other down the row.
				TextStyle: centeredText(cfg.TextStyle),
				Padding:   pad,
				Width:     cfg.FieldWidth,
				// Pinned both ways like the hex field: an Input is
				// Fit-sized, so "0" and "255" would resolve to
				// different widths and the whole column of fields
				// would shift as the user drags a control.
				MinWidth: cfg.FieldWidth,
				MaxWidth: cfg.FieldWidth,
				A11YCfg:  A11YCfg{A11YLabel: label},
				OnTextChanged: func(text string, ctx EventCtx) {
					apply(text, ctx)
				},
				OnTextCommit: func(
					text string, _ InputCommitReason, ctx EventCtx,
				) {
					apply(text, ctx)
				},
			}),
		},
	})
}
