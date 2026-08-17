package gui

// ColorPlaneCfg configures a saturation × lightness plane.
//
// The plane paints every color available at the current hue —
// saturation left to right, lightness bottom to top — and a marker at
// the current value. Dragging anywhere in it sets saturation and
// lightness together; hue and alpha pass through untouched.
//
// It is stateless: Value in, OnChange out. Hold the HSLA in app state
// and every other color control reading that same value stays in sync
// with this one.
type ColorPlaneCfg struct {
	OnChange func(HSLA, EventCtx)
	ID       string `gui:"required"`

	A11YCfg
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool

	Value HSLA
	// Size is the plane's width and height in points. Zero takes the
	// theme default.
	Size   float32 // ergonomics-audit:opt-plain — a zero-sized plane is meaningless; 0 falls back to the theme
	Radius Opt[float32]
	// MarkerSize is the diameter of the position marker. Zero takes
	// the theme default.
	MarkerSize float32
}

type colorPlaneView struct {
	cfg ColorPlaneCfg
}

// ColorPlane creates a saturation × lightness plane at Value's hue.
func ColorPlane(cfg ColorPlaneCfg) View {
	requireFocusID("ColorPlane", cfg.FocusDisabled, cfg.ID)
	applyColorPlaneDefaults(&cfg)
	return &colorPlaneView{cfg: cfg}
}

func applyColorPlaneDefaults(cfg *ColorPlaneCfg) {
	d := &defaultColorPickerStyle
	if cfg.Size <= 0 {
		cfg.Size = d.sVSize
	}
	if cfg.MarkerSize <= 0 {
		cfg.MarkerSize = d.indicatorSize
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.Radius)
	}
}

func (pv *colorPlaneView) GenerateLayout(w *Window) Layout {
	cfg := &pv.cfg
	// One resolved identity for the drag lookup and the shape alike.
	id := w.EffID(cfg.ID)
	size := cfg.Size
	v := cfg.Value.Normalized()
	onChange := cfg.OnChange

	// Saturation runs left to right, lightness bottom to top — the
	// same mapping the generated buffer paints, so the marker lands on
	// the color under it.
	apply := func(nx, ny float32, ctx EventCtx) {
		if onChange == nil {
			return
		}
		out := v
		out.S = nx
		out.L = 1 - ny
		onChange(out, ctx)
	}

	px := int(size) * imgScale
	cv := &containerView{
		cfg: ContainerCfg{
			ID:        id,
			Focusable: !cfg.FocusDisabled,
			A11YRole:  AccessRoleColorWell,
			A11YCfg: A11YCfg{
				A11YLabel: a11yLabel(cfg.A11YLabel,
					"Saturation and lightness"),
				A11YDescription: cfg.A11YDescription,
			},
			Width:  size,
			Height: size,
			// Fixed: the plane's size is tied to the buffer generated
			// for it, so it must not stretch to fill a parent.
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Radius:     cfg.Radius,
			axis:       axisTopToBottom,
			OnClick:    colorDragTrack(id, apply),
			OnKeyDown:  colorPlaneKeys(v, onChange),
		},
		content: []View{
			Image(ImageCfg{
				Src:    planeSrc(v, px),
				Width:  size,
				Height: size,
			}),
			colorMarker(cfg.MarkerSize, v.Color(), func(ctx EventCtx) {
				colorAmendMarker(ctx.Layout,
					v.S*size, (1-v.L)*size, cfg.MarkerSize)
			}),
		},
	}
	colorControlFocusRing(&cv.cfg, w.EffID(cv.cfg.ID))
	return generateViewLayout(cv, w)
}

// colorPlaneKeys moves saturation with the horizontal arrows and
// lightness with the vertical ones, matching the axes on screen.
func colorPlaneKeys(
	v HSLA, onChange func(HSLA, EventCtx),
) func(EventCtx) {
	return colorArrowKeys(v, onChange,
		func(v HSLA, dx, dy float32) HSLA {
			v.S = f32Clamp(v.S+dx, 0, 1)
			v.L = f32Clamp(v.L-dy, 0, 1)
			return v
		})
}
