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
	defaultColorFieldWidth = 50
	defaultHexFieldWidth   = 100
	// defaultColorFieldsSwatch matches the hex input's height closely
	// enough that the two read as one row.
	defaultColorFieldsSwatch = 28
	// colorFieldsSwatchAspect widens the swatch relative to SwatchSize,
	// which stays the height so the swatch keeps matching the hex
	// input's line.
	colorFieldsSwatchAspect = 2
)

func (fv *colorFieldsView) GenerateLayout(w *Window) Layout {
	cfg := &fv.cfg
	id := w.EffID(cfg.ID)
	v := cfg.Value.Normalized()

	rows := make([]View, 0, 3)
	if !cfg.HideHex {
		rows = append(rows, colorHexRow(cfg, id, v))
	}
	rows = append(rows, colorRGBARow(cfg, id, v))
	if cfg.ShowHSL {
		rows = append(rows, colorHSLRow(cfg, id, v))
	}

	return generateViewLayout(&containerView{
		cfg: ContainerCfg{
			ID:      id,
			Padding: NoPadding,
			Spacing: Some(SpacingSmall),
			axis:    axisTopToBottom,
			A11YCfg: cfg.A11YCfg,
		},
		content: rows,
	}, w)
}

// colorHexRow builds the hex text field, and — with ShowSwatch — the
// preview swatch beside it. The swatch belongs next to the hex value:
// both are readouts of the same color, one exact and one legible.
func colorHexRow(cfg *ColorFieldsCfg, id string, v HSLA) View {
	hex := colorHexField(cfg, id, v)
	if !cfg.ShowSwatch {
		return hex
	}
	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		VAlign:  VAlignMiddle,
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

func colorHexField(cfg *ColorFieldsCfg, id string, v HSLA) View {
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
		// point and the field tracks its text: "#FFF" and "#3C8CDDD9"
		// give different widths, and the swatch beside it slides
		// around as the user types. MinWidth pins the floor so the
		// row only ever grows, never twitches.
		MinWidth: defaultHexFieldWidth,
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
func colorRGBARow(cfg *ColorFieldsCfg, id string, v HSLA) View {
	c := v.Color()
	l := ActiveLocale
	vals := [4]uint8{c.R, c.G, c.B, c.A}
	labels := [4]string{l.strRed, l.strGreen, l.strBlue, l.strAlpha}

	fields := make([]View, 0, 4)
	for i := range 4 {
		fields = append(fields, colorFieldColumn(cfg,
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
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		Content: fields,
	})
}

// colorHSLRow builds the H/S/L inputs, which edit the value directly
// with no RGBA round-trip and so cannot lose the hue.
func colorHSLRow(cfg *ColorFieldsCfg, id string, v HSLA) View {
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
		fields = append(fields, colorFieldColumn(cfg,
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
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		Content: fields,
	})
}

// colorFieldColumn builds one labeled numeric input.
func colorFieldColumn(
	cfg *ColorFieldsCfg, label string, val int,
	inputID string, apply func(string, EventCtx),
) View {
	return Column(ContainerCfg{
		Padding: NoPadding,
		Spacing: SomeF(2),
		Content: []View{
			Text(TextCfg{
				Text: label,
				TextStyle: TextStyle{
					Color: cfg.TextStyle.Color,
					Size:  cfg.TextStyle.Size,
					Align: TextAlignCenter,
				},
			}),
			Input(InputCfg{
				ID:        inputID,
				Text:      strconv.Itoa(val),
				TextStyle: cfg.TextStyle,
				Width:     cfg.FieldWidth,
				A11YCfg:   A11YCfg{A11YLabel: label},
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
