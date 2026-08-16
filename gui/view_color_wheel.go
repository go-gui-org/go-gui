package gui

// ColorWheelCfg configures a hue × saturation wheel.
//
// Angle is hue — 0° at the right, increasing counterclockwise — and
// radius is saturation, drawn at the current lightness. Dragging sets
// hue and saturation together; lightness and alpha pass through
// untouched.
//
// The disc cannot be drawn with gradients (there is no conic gradient),
// so it is a generated buffer keyed on lightness. That is the reason
// the in-memory image registry exists; see gui/color_buffers.go.
type ColorWheelCfg struct {
	OnChange func(HSLA, EventCtx)
	ID       string `gui:"required"`

	A11YCfg
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool

	Value HSLA
	// Size is the wheel's diameter in points. Zero takes the theme
	// default.
	Size float32 // ergonomics-audit:opt-plain — a zero-diameter wheel is meaningless; 0 falls back to the theme
	// MarkerSize is the diameter of the position marker. Zero takes
	// the theme default.
	MarkerSize float32
}

type colorWheelView struct {
	cfg ColorWheelCfg
}

// ColorWheel creates a hue × saturation wheel at Value's lightness.
func ColorWheel(cfg ColorWheelCfg) View {
	requireFocusID("ColorWheel", cfg.FocusDisabled, cfg.ID)
	applyColorWheelDefaults(&cfg)
	return &colorWheelView{cfg: cfg}
}

func applyColorWheelDefaults(cfg *ColorWheelCfg) {
	d := &defaultColorPickerStyle
	if cfg.Size <= 0 {
		cfg.Size = d.sVSize
	}
	if cfg.MarkerSize <= 0 {
		cfg.MarkerSize = d.indicatorSize
	}
}

func (wv *colorWheelView) GenerateLayout(w *Window) Layout {
	cfg := &wv.cfg
	id := w.EffID(cfg.ID)
	size := cfg.Size
	radius := size / 2
	v := cfg.Value.Normalized()
	onChange := cfg.OnChange

	apply := func(nx, ny float32, ctx EventCtx) {
		if onChange == nil {
			return
		}
		// Back from the unit box the drag helper reports to a vector
		// out of the wheel's centre.
		dx := nx*size - radius
		dy := ny*size - radius
		out := v
		out.H = hueOf(dx, dy)
		// A drag past the rim pins to full saturation rather than
		// snapping the hue back, which is what makes the edge of the
		// disc feel like a wall instead of a cliff.
		out.S = f32Clamp(f32Sqrt(dx*dx+dy*dy)/radius, 0, 1)
		onChange(out, ctx)
	}

	// Marker position: hue is the angle, saturation the distance out.
	// Screen y grows downward, so the sine term is subtracted.
	rad := v.H * f32Pi / 180
	mr := v.S * radius
	mx := radius + f32Cos(rad)*mr
	my := radius - f32Sin(rad)*mr

	px := int(size) * imgScale
	cv := &containerView{
		cfg: ContainerCfg{
			ID:        id,
			Focusable: !cfg.FocusDisabled,
			A11YRole:  AccessRoleColorWell,
			A11YCfg: A11YCfg{
				A11YLabel: a11yLabel(cfg.A11YLabel,
					"Hue and saturation wheel"),
				A11YDescription: cfg.A11YDescription,
			},
			Width:  size,
			Height: size,
			// Fixed: the disc's size is tied to the buffer generated
			// for it, so it must not stretch to fill a parent.
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			axis:       axisTopToBottom,
			OnClick:    colorDragTrack(id, apply),
			OnKeyDown:  colorWheelKeys(v, onChange),
		},
		content: []View{
			Image(ImageCfg{
				Src:    wheelSrc(v, px),
				Width:  size,
				Height: size,
			}),
			colorMarker(cfg.MarkerSize, v.Color(), func(ctx EventCtx) {
				colorAmendMarker(ctx.Layout, mx, my, cfg.MarkerSize)
			}),
		},
	}
	colorControlFocusRing(&cv.cfg)
	return generateViewLayout(cv, w)
}

// colorWheelKeys steps hue with the horizontal arrows and saturation
// with the vertical ones — the two axes the wheel edits, mapped to the
// arrows a user can reach without a pointer.
func colorWheelKeys(
	v HSLA, onChange func(HSLA, EventCtx),
) func(EventCtx) {
	return colorArrowKeys(v, onChange,
		func(v HSLA, dx, dy float32) HSLA {
			v.H += dx * 360
			v.S = f32Clamp(v.S-dy, 0, 1)
			return v.Normalized()
		})
}
