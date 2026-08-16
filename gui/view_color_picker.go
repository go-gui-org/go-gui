package gui

// colorPickerState holds the picker's live HSLA across frames.
//
// The public API is RGBA in and RGBA out, and RGBA cannot represent
// hue at S=0 or at the lightness extremes. Without this, dragging
// lightness to the bottom would discard the hue and the control would
// come back red on the way up. It is a lossy-conversion guard for the
// back-compat surface, not a second color model: the components below
// all speak HSLA directly.
type colorPickerState struct {
	hsla HSLA
	// color is the Color the stored hsla was derived from, so a
	// caller changing Color out from under the picker is detected and
	// wins, while a round-trip of the picker's own output does not
	// clobber the hue it is holding.
	color Color
}

// ColorPickerCfg configures a color picker view.
type ColorPickerCfg struct {
	Style         ColorPickerStyle
	OnColorChange func(Color, EventCtx)
	ID            string `gui:"required"`

	A11YCfg
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Width         float32
	Height        float32

	Color  Color
	Sizing Sizing

	// ShowHSV shows the hue/saturation/lightness input row.
	//
	// Deprecated: the picker edits HSL, not HSV. Use ShowHSL, which
	// names what the row actually contains. Both set the same flag.
	//
	// exportaudit:keep — back-compat for callers outside this repo; the
	// in-repo examples all moved to ShowHSL.
	ShowHSV bool
	// ShowHSL shows the hue/saturation/lightness input row.
	ShowHSL bool
}

type colorPickerView struct {
	cfg ColorPickerCfg
}

// ColorPicker creates a color picker with a saturation × lightness
// plane, hue and alpha sliders, a preview swatch and channel inputs.
//
// It is a composition of the color components — ColorPlane,
// ColorChannelSlider, ColorSwatch, ColorFields — assembled into the
// arrangement most callers want. Reach for those directly to lay the
// same controls out differently, add a ColorWheel, or drive several
// controls from one HSLA value in app state.
func ColorPicker(cfg ColorPickerCfg) View {
	RequireID("ColorPicker", cfg.ID)
	applyColorPickerDefaults(&cfg)
	return &colorPickerView{cfg: cfg}
}

func applyColorPickerDefaults(cfg *ColorPickerCfg) {
	d := &defaultColorPickerStyle
	if cfg.Style == (ColorPickerStyle{}) {
		cfg.Style = *d
	}
	if !cfg.Color.IsSet() {
		cfg.Color = Red
	}
}

func (cv *colorPickerView) GenerateLayout(w *Window) Layout {
	cfg := &cv.cfg

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)
	style := cfg.Style
	v := cpValue(w, cfg.ID, cfg.Color)

	// Every child reports an HSLA; the picker stores it, converts it
	// once, and hands the caller the Color its API promises.
	onChange := func(next HSLA, ctx EventCtx) {
		cpStore(ctx.Window, cfg.ID, next)
		if cfg.OnColorChange != nil {
			cfg.OnColorChange(next.Color(), ctx)
		}
	}

	planeSize := colorPickerPlaneSize(style)
	content := []View{
		// Plane first, then the two channel sliders stood on end beside
		// it: the sliders are as tall as the plane, so the picker is
		// squarer than a stack of full-width tracks would make it.
		Row(ContainerCfg{
			Padding: NoPadding,
			Spacing: Some(colorPickerPlaneGap),
			Content: []View{
				ColorPlane(ColorPlaneCfg{
					ID:       ScopeID(cfg.ID, "plane"),
					Value:    v,
					Size:     planeSize,
					OnChange: onChange,
				}),
				ColorChannelSlider(ColorChannelSliderCfg{
					ID:       ScopeID(cfg.ID, "hue"),
					Channel:  ChannelHue,
					Value:    v,
					Vertical: true,
					Height:   planeSize,
					OnChange: onChange,
				}),
				ColorChannelSlider(ColorChannelSliderCfg{
					ID:       ScopeID(cfg.ID, "alpha"),
					Channel:  ChannelAlpha,
					Value:    v,
					Vertical: true,
					Height:   planeSize,
					OnChange: onChange,
				}),
			},
		}),
		// The swatch rides in the fields block, beside the hex value it
		// previews, rather than standing to the left of the whole
		// numeric column.
		ColorFields(ColorFieldsCfg{
			ID:         ScopeID(cfg.ID, "fields"),
			Value:      v,
			ShowHSL:    cfg.ShowHSL || cfg.ShowHSV,
			ShowSwatch: true,
			SwatchSize: colorPickerSwatchSize,
			TextStyle:  style.TextStyle,
			OnChange:   onChange,
		}),
	}

	ccfg := ContainerCfg{
		ID:       cfg.ID,
		A11YRole: AccessRoleColorWell,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, "Color Picker"),
			A11YDescription: cfg.A11YDescription,
		},
		Color:       style.Color,
		ColorBorder: style.ColorBorder,
		SizeBorder:  Some(style.SizeBorder),
		Radius:      Some(style.Radius),
		Spacing:     Some(SpacingSmall),
		Sizing:      cfg.Sizing,
		Width:       cfg.Width,
		Height:      cfg.Height,
		axis:        axisTopToBottom,
	}
	return generateViewLayout(&containerView{
		cfg:     ccfg,
		content: content,
	}, w)
}

// colorPickerSwatchSize is the preview swatch's edge length.
const colorPickerSwatchSize = 32

// colorPickerPlaneGap separates the plane from the two vertical
// sliders beside it. Wider than the picker's usual small spacing: the
// sliders carry their own imagery, so at small spacing they read as
// part of the plane's edge rather than as separate controls.
//
// colorPickerPlaneSize subtracts it, so widening the gap narrows the
// plane and the row's total width does not move.
const colorPickerPlaneGap = SpacingMedium

// colorPickerMinPlane floors the derived plane size. A theme with wide
// sliders and narrow fields could otherwise drive the plane down to
// something unusable; below this the picker stops shrinking and simply
// runs wider than its fields.
const colorPickerMinPlane = 120

// colorPickerPlaneSize derives the plane's edge length instead of
// taking the theme's sVSize directly.
//
// The picker is as wide as its widest row, and that row is the four
// RGBA fields. A plane at the full sVSize makes the top row — plane
// plus the two vertical sliders — wider than the fields, which shows
// up as dead space to the right of the alpha input. Subtract exactly
// what the sliders occupy so the two rows come out the same width.
//
// It is only ever a shrink: sVSize stays the upper bound, so a theme
// asking for a small plane is never grown to fill the row.
func colorPickerPlaneSize(style ColorPickerStyle) float32 {
	sliderThick := f32Max(style.sliderHeight, style.indicatorSize)
	// The fields block's own width, mirroring colorRGBARow: four
	// inputs at the default width with small spacing between them.
	fieldsW := 4*defaultColorFieldWidth + 3*SpacingSmall
	size := fieldsW - 2*(sliderThick+colorPickerPlaneGap)
	if size > style.sVSize {
		size = style.sVSize
	}
	return f32Max(size, colorPickerMinPlane)
}

// cpValue returns the picker's live HSLA for this frame.
//
// The stored value wins while the caller keeps handing back the Color
// the picker produced — that is a round trip, and re-deriving from it
// would drop the hue at the extremes. A Color the picker did not
// produce is a caller setting the value, and it wins instead.
func cpValue(w *Window, id string, c Color) HSLA {
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	if st, ok := sm.Get(id); ok && st.color == c {
		return st.hsla
	}
	v := ColorToHSLA(c)
	sm.Set(id, colorPickerState{hsla: v, color: c})
	return v
}

// cpStore records the value a child just produced, along with the
// Color the caller will see, so the next frame recognizes the round
// trip.
func cpStore(w *Window, id string, v HSLA) {
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set(id, colorPickerState{hsla: v, color: v.Color()})
}
