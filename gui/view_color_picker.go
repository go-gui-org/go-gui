package gui

import (
	"strconv"
)

// colorPickerState stores persistent HSV to preserve hue when
// saturation or value goes to zero.
type colorPickerState struct {
	H float32
	S float32
	V float32
}

// ColorPickerCfg configures a color picker view.
type ColorPickerCfg struct {
	Style         ColorPickerStyle
	OnColorChange func(Color, EventCtx)
	ID            string `gui:"required"`

	A11YLabel       string
	A11YDescription string
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Width         float32
	Height        float32

	Color   Color
	Sizing  Sizing
	ShowHSV bool
}

type colorPickerView struct {
	cfg ColorPickerCfg
}

// ColorPicker creates a color picker view with SV area, hue slider,
// alpha slider, hex input, and RGBA/HSV channel inputs.
func ColorPicker(cfg ColorPickerCfg) View {
	RequireID("ColorPicker", cfg.ID)
	applyColorPickerDefaults(&cfg)
	return &colorPickerView{cfg: cfg}
}

func (cv *colorPickerView) Content() []View { return nil }

func (cv *colorPickerView) GenerateLayout(w *Window) Layout {
	cfg := &cv.cfg

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)
	style := cfg.Style

	// Get or init HSV state.
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	hsv, ok := sm.Get(cfg.ID)
	if !ok {
		h, s, v := cfg.Color.ToHSV()
		hsv = colorPickerState{H: h, S: s, V: v}
		sm.Set(cfg.ID, hsv)
	}

	svSize := style.SVSize
	sliderH := style.SliderHeight
	indicatorSize := style.IndicatorSize

	content := make([]View, 0, 5)

	// SV area + hue slider side by side.
	content = append(content, cpSVAndHueRow(cfg, hsv,
		svSize, sliderH, indicatorSize))

	// Alpha slider.
	content = append(content, cpAlphaSlider(cfg))

	// Preview + hex row.
	content = append(content, cpPreviewRow(cfg))

	// RGBA inputs.
	content = append(content, cpRGBAInputs(cfg))

	// Optional HSV inputs.
	if cfg.ShowHSV {
		content = append(content, cpHSVInputs(cfg, hsv))
	}

	ccfg := ContainerCfg{
		ID:          cfg.ID,
		Focusable:   !cfg.FocusDisabled,
		A11YRole:    AccessRoleColorWell,
		A11YLabel:   a11yLabel(cfg.A11YLabel, "Color Picker"),
		Color:       style.Color,
		ColorBorder: style.ColorBorder,
		SizeBorder:  Some(style.SizeBorder),
		Radius:      Some(style.Radius),
		Spacing:     Some(SpacingSmall),
		Sizing:      cfg.Sizing,
		Width:       cfg.Width,
		Height:      cfg.Height,
		axis:        AxisTopToBottom,
		AmendLayout: func(ctx EventCtx) {
			if ctx.Window.IsFocus(cfg.ID) {
				ctx.Layout.Shape.ColorBorder = style.ColorBorderFocus
			}
		},
	}
	return generateViewLayout(&containerView{
		cfg:     ccfg,
		content: content,
	}, w)
}

// cpSVAndHueRow builds the SV area and hue slider row.
func cpSVAndHueRow(
	cfg *ColorPickerCfg, hsv colorPickerState,
	svSize, sliderH, indicatorSize float32,
) View {
	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		Content: []View{
			cpSVArea(cfg, hsv, svSize, indicatorSize),
			cpHueSlider(cfg, hsv, sliderH, svSize,
				indicatorSize),
		},
	})
}

// cpSVArea builds the saturation/value area with gradients.
func cpSVArea(
	cfg *ColorPickerCfg, hsv colorPickerState,
	size, indicatorSize float32,
) View {
	pureHue := HueColor(hsv.H)
	cfgID := cfg.ID
	cfgColor := cfg.Color
	onChange := cfg.OnColorChange

	return container(ContainerCfg{
		ID:     ScopeID(cfg.ID, "sv"),
		Width:  size,
		Height: size,
		axis:   AxisTopToBottom,
		Gradient: &GradientDef{
			Direction: GradientToRight,
			Stops: []GradientStop{
				{Color: White, Pos: 0},
				{Color: pureHue, Pos: 1},
			},
		},
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Radius:     Some(cfg.Style.Radius),
		Content: []View{
			container(ContainerCfg{
				Sizing: FillFill,
				Gradient: &GradientDef{
					Direction: GradientToBottom,
					Stops: []GradientStop{
						{Color: ColorTransparent, Pos: 0},
						{Color: Black, Pos: 1},
					},
				},
				Padding: NoPadding,
				Content: []View{
					Circle(ContainerCfg{
						Width:       indicatorSize,
						Height:      indicatorSize,
						Color:       cfgColor,
						ColorBorder: White,
						SizeBorder:  SomeF(2),
						Padding:     NoPadding,
						AmendLayout: func(ctx EventCtx) {
							cpAmendSVIndicator(ctx.Layout, hsv,
								size, indicatorSize)
						},
					}),
				},
				OnClick: func(ctx EventCtx) {
					// Convert local mouse coords to absolute.
					ev := *ctx.Event
					ev.MouseX += ctx.Layout.Shape.X
					ev.MouseY += ctx.Layout.Shape.Y
					cpSVMouseAction(cfgID, cfgColor,
						onChange, ctx.Layout.Shape, &ev, ctx.Window)
					ctx.Window.MouseLock(MouseLockCfg{
						MouseMove: func(ctx EventCtx) {
							sv, ok := ctx.Layout.FindByID(
								ScopeID(cfgID, "sv"))
							if !ok {
								return
							}
							cpSVMouseAction(cfgID, cfgColor,
								onChange, sv.Shape, ctx.Event, ctx.Window)
						},
						MouseUp: func(ctx EventCtx) {
							ctx.Window.MouseUnlock()
							ctx.Window.SetMouseCursorArrow()
						},
					})
					// Same as the hue strip: nested in the focusable
					// picker root, so consume to keep the focus where
					// it is once spec §4.3b removes the pre-mark.
					ctx.Consume()
				},
			}),
		},
	})
}

// cpHueSlider builds the vertical hue slider.
func cpHueSlider(
	cfg *ColorPickerCfg, hsv colorPickerState,
	sliderWidth, sliderHeight, indicatorSize float32,
) View {
	cfgID := cfg.ID
	cfgColor := cfg.Color
	onChange := cfg.OnColorChange

	return container(ContainerCfg{
		ID:     ScopeID(cfg.ID, "hue"),
		Width:  sliderWidth,
		Height: sliderHeight,
		Gradient: &GradientDef{
			Direction: GradientToBottom,
			Stops: []GradientStop{
				{Color: RGB(255, 0, 0), Pos: 0.0},
				{Color: RGB(255, 255, 0), Pos: 1.0 / 6},
				{Color: RGB(0, 255, 0), Pos: 2.0 / 6},
				{Color: RGB(0, 255, 255), Pos: 3.0 / 6},
				{Color: RGB(0, 0, 255), Pos: 4.0 / 6},
				{Color: RGB(255, 0, 255), Pos: 5.0 / 6},
				{Color: RGB(255, 0, 0), Pos: 1.0},
			},
		},
		Padding: NoPadding,
		Radius:  Some(cfg.Style.Radius),
		Content: []View{
			Circle(ContainerCfg{
				Width:       indicatorSize,
				Height:      indicatorSize,
				Color:       HueColor(hsv.H),
				ColorBorder: White,
				SizeBorder:  SomeF(2),
				Padding:     NoPadding,
				AmendLayout: func(ctx EventCtx) {
					cpAmendHueIndicator(ctx.Layout, hsv,
						sliderHeight, indicatorSize)
				},
			}),
		},
		OnClick: func(ctx EventCtx) {
			// Convert local mouse coords to absolute.
			ev := *ctx.Event
			ev.MouseX += ctx.Layout.Shape.X
			ev.MouseY += ctx.Layout.Shape.Y
			cpHueMouseAction(cfgID, cfgColor, onChange,
				ctx.Layout.Shape, &ev, ctx.Window)
			ctx.Window.MouseLock(MouseLockCfg{
				MouseMove: func(ctx EventCtx) {
					hue, ok := ctx.Layout.FindByID(
						ScopeID(cfgID, "hue"))
					if !ok {
						return
					}
					cpHueMouseAction(cfgID, cfgColor,
						onChange, hue.Shape, ctx.Event, ctx.Window)
				},
				MouseUp: func(ctx EventCtx) {
					ctx.Window.MouseUnlock()
				},
			})
			// The hue strip is inside the focusable picker root, which
			// would take focus off this click once spec §4.3b removes
			// the consume-class pre-mark.
			ctx.Consume()
		},
	})
}

// cpAlphaSlider builds the alpha channel slider.
func cpAlphaSlider(cfg *ColorPickerCfg) View {
	onChange := cfg.OnColorChange
	c := cfg.Color
	thumbSize := cfg.Style.IndicatorSize
	trackSize := float32(6)
	return Slider(SliderCfg{
		ID:           ScopeID(cfg.ID, "alpha"),
		Value:        float32(c.A),
		Min:          0,
		Max:          255,
		Step:         1,
		Sizing:       FillFit,
		Size:         trackSize,
		SizeBorder:   SomeF(1),
		ThumbSize:    thumbSize,
		Height:       thumbSize,
		RadiusBorder: SomeF(trackSize / 2),
		OnChange: func(v float32, ctx EventCtx) {
			if onChange != nil {
				nc := c
				nc.A = uint8(f32Clamp(v, 0, 255))
				onChange(nc, EventCtx{nil, ctx.Event, ctx.Window})
			}
		},
		RoundValue: true,
	})
}

// cpPreviewRow builds the color swatch + hex input row.
func cpPreviewRow(cfg *ColorPickerCfg) View {
	c := cfg.Color
	onChange := cfg.OnColorChange
	cfgID := cfg.ID

	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		VAlign:  VAlignMiddle,
		Content: []View{
			Rectangle(RectangleCfg{
				Width:  32,
				Height: 32,
				Color:  c,
				Radius: cfg.Style.Radius,
			}),
			Input(InputCfg{
				ID:        ScopeID(cfgID, "hex"),
				Text:      c.ToHexString(),
				TextStyle: cfg.Style.TextStyle,
				Width:     100,
				OnTextChanged: func(text string, ctx EventCtx) {
					cpApplyHex(text, cfgID, onChange, ctx.Window)
				},
				OnTextCommit: func(text string, _ InputCommitReason, ctx EventCtx) {
					cpApplyHex(text, cfgID, onChange, ctx.Window)
				},
			}),
		},
	})
}

// cpRGBAInputs builds the R/G/B/A channel inputs row.
func cpRGBAInputs(cfg *ColorPickerCfg) View {
	c := cfg.Color
	l := ActiveLocale
	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		Content: []View{
			cpChannelInput(cfg, l.StrRed, c.R, 0),
			cpChannelInput(cfg, l.StrGreen, c.G, 1),
			cpChannelInput(cfg, l.StrBlue, c.B, 2),
			cpChannelInput(cfg, l.StrAlpha, c.A, 3),
		},
	})
}

// cpHSVInputs builds the H/S/V channel inputs row.
func cpHSVInputs(
	cfg *ColorPickerCfg, hsv colorPickerState,
) View {
	l := ActiveLocale
	return Row(ContainerCfg{
		Padding: NoPadding,
		Spacing: Some(SpacingSmall),
		Content: []View{
			cpHSVChannelInput(cfg, l.StrHue,
				int(hsv.H+0.5), 360, 0),
			cpHSVChannelInput(cfg, l.StrSat,
				int(hsv.S*100+0.5), 100, 1),
			cpHSVChannelInput(cfg, l.StrValue,
				int(hsv.V*100+0.5), 100, 2),
		},
	})
}

// cpChannelInput builds a labeled RGB channel input.
func cpChannelInput(
	cfg *ColorPickerCfg, ch string, val uint8, idx int,
) View {
	onChange := cfg.OnColorChange
	cfgID := cfg.ID
	c := cfg.Color
	applyFn := func(text string, w *Window) {
		cpApplyRGB(text, idx, c, cfgID, onChange, w)
	}
	inputID := ScopeIDN(cfgID, "rgb", idx)
	return cpInputColumn(cfg, ch, int(val), inputID, applyFn)
}

// cpHSVChannelInput builds a labeled HSV channel input.
func cpHSVChannelInput(
	cfg *ColorPickerCfg, ch string,
	val, maxVal int, idx int,
) View {
	onChange := cfg.OnColorChange
	cfgID := cfg.ID
	alpha := cfg.Color.A
	applyFn := func(text string, w *Window) {
		cpApplyHSV(text, idx, maxVal, cfgID, alpha, onChange, w)
	}
	inputID := ScopeIDN(cfgID, "hsv", idx)
	return cpInputColumn(cfg, ch, val, inputID, applyFn)
}

// cpInputColumn builds a labeled channel input column shared by
// RGB and HSV inputs.
func cpInputColumn(
	cfg *ColorPickerCfg, label string, val int,
	inputID string, applyFn func(string, *Window),
) View {
	return Column(ContainerCfg{
		Padding: NoPadding,
		Spacing: SomeF(2),
		Content: []View{
			Text(TextCfg{
				Text: label,
				TextStyle: TextStyle{
					Color: cfg.Style.TextStyle.Color,
					Size:  cfg.Style.TextStyle.Size,
					Align: TextAlignCenter,
				},
			}),
			Input(InputCfg{
				ID:        inputID,
				Text:      strconv.Itoa(val),
				TextStyle: cfg.Style.TextStyle,
				Width:     50,
				OnTextChanged: func(text string, ctx EventCtx) {
					applyFn(text, ctx.Window)
				},
				OnTextCommit: func(text string, _ InputCommitReason, ctx EventCtx) {
					applyFn(text, ctx.Window)
				},
			}),
		},
	})
}

// cpSVMouseAction handles mouse interaction in the SV area.
func cpSVMouseAction(
	id string, color Color,
	onChange func(Color, EventCtx),
	shape *Shape, e *Event, w *Window,
) {
	if onChange == nil ||
		shape.Width <= 0 || shape.Height <= 0 {
		return
	}
	s := f32Clamp((e.MouseX-shape.X)/shape.Width, 0, 1)
	v := 1 - f32Clamp((e.MouseY-shape.Y)/shape.Height, 0, 1)

	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	// Default zero: HSV(0,0,0) is black; state seeded before callbacks
	// fire so this default is never reached in practice.
	hsv := sm.GetOr(id, colorPickerState{})
	hsv.S = s
	hsv.V = v
	sm.Set(id, hsv)

	nc := ColorFromHSVA(hsv.H, s, v, color.A)
	onChange(nc, EventCtx{nil, e, w})
}

// cpHueMouseAction handles mouse interaction on the hue slider.
func cpHueMouseAction(
	id string, color Color,
	onChange func(Color, EventCtx),
	shape *Shape, e *Event, w *Window,
) {
	if onChange == nil || shape.Height <= 0 {
		return
	}
	h := f32Clamp(
		(e.MouseY-shape.Y)/shape.Height, 0, 1) * 360

	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	// Default zero: HSV(0,0,0) is black; state seeded before callbacks
	// fire so this default is never reached in practice.
	hsv := sm.GetOr(id, colorPickerState{})
	hsv.H = h
	sm.Set(id, hsv)

	nc := ColorFromHSVA(h, hsv.S, hsv.V, color.A)
	onChange(nc, EventCtx{nil, e, w})
}

// cpAmendSVIndicator positions the SV indicator circle.
func cpAmendSVIndicator(
	layout *Layout, hsv colorPickerState,
	svSize, indicatorSize float32,
) {
	if layout.Parent == nil {
		return
	}
	parent := layout.Parent
	radius := indicatorSize / 2
	layout.Shape.X = parent.Shape.X +
		hsv.S*svSize - radius
	layout.Shape.Y = parent.Shape.Y +
		(1-hsv.V)*svSize - radius
}

// cpAmendHueIndicator positions the hue slider indicator.
func cpAmendHueIndicator(
	layout *Layout, hsv colorPickerState,
	sliderHeight, indicatorSize float32,
) {
	if layout.Parent == nil {
		return
	}
	parent := layout.Parent
	radius := indicatorSize / 2
	y := (hsv.H / 360) * sliderHeight
	layout.Shape.X = parent.Shape.X +
		parent.Shape.Width/2 - radius
	layout.Shape.Y = parent.Shape.Y + y - radius
}

func applyColorPickerDefaults(cfg *ColorPickerCfg) {
	d := &DefaultColorPickerStyle
	if cfg.Style == (ColorPickerStyle{}) {
		cfg.Style = *d
	}
	if !cfg.Color.IsSet() {
		cfg.Color = Red
	}
}

// cpApplyRGB applies an RGB channel change from text input.
func cpApplyRGB(
	text string, idx int, c Color, cfgID string,
	onChange func(Color, EventCtx), w *Window,
) {
	parsed, err := strconv.ParseUint(text, 10, 8)
	if err != nil || onChange == nil {
		return
	}
	v := uint8(parsed)
	nc := c
	switch idx {
	case 0:
		nc.R = v
	case 1:
		nc.G = v
	case 2:
		nc.B = v
	case 3:
		nc.A = v
	}
	h, s, vv := nc.ToHSV()
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set(cfgID, colorPickerState{H: h, S: s, V: vv})
	onChange(nc, EventCtx{nil, nil, w})
}

func cpApplyHSV(
	text string, idx, maxVal int, cfgID string,
	alpha uint8, onChange func(Color, EventCtx),
	w *Window,
) {
	n, err := strconv.Atoi(text)
	if err != nil || onChange == nil {
		return
	}
	n = intClamp(n, 0, maxVal)
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	// Default zero: HSV(0,0,0) is black; state seeded before callbacks
	// fire so this default is never reached in practice.
	hsv := sm.GetOr(cfgID, colorPickerState{})
	switch idx {
	case 0:
		hsv.H = float32(n)
	case 1:
		hsv.S = float32(n) / 100
	case 2:
		hsv.V = float32(n) / 100
	}
	sm.Set(cfgID, hsv)
	nc := ColorFromHSVA(hsv.H, hsv.S, hsv.V, alpha)
	onChange(nc, EventCtx{nil, nil, w})
}

func cpApplyHex(
	text, cfgID string,
	onChange func(Color, EventCtx), w *Window,
) {
	nc, ok := ColorFromHexString(text)
	if !ok || onChange == nil {
		return
	}
	h, s, v := nc.ToHSV()
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set(cfgID, colorPickerState{H: h, S: s, V: v})
	onChange(nc, EventCtx{nil, nil, w})
}

// cpParseUint8 parses a string as a uint8 (0–255).
func cpParseUint8(s string) (uint8, bool) {
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 255 {
		return 0, false
	}
	return uint8(n), true
}
