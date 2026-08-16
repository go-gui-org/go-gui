package gui

// ColorChannelSliderCfg configures one HSLA channel slider.
//
// The track is not a theme fill: it shows the colors this slider can
// pick, holding the other channels where they are. Drag the hue slider
// and the saturation and lightness tracks repaint under it; the alpha
// track composites over a checkerboard so transparency is visible
// rather than implied.
//
// It is not built on SliderCfg. That widget's track fill and thumb
// color are unexported and it has no gradient or image track, so a
// channel slider built on it could not show what it picks — which is
// the entire point of this control.
type ColorChannelSliderCfg struct {
	OnChange func(HSLA, EventCtx)
	ID       string `gui:"required"`

	A11YCfg
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool

	// Channel selects which component of Value this slider edits:
	// ChannelHue, ChannelSaturation, ChannelLightness or ChannelAlpha.
	Channel ColorChannel
	Value   HSLA

	// Vertical lays the track out bottom (minimum) to top (maximum)
	// instead of left to right. Height then gives its length, and Width
	// is ignored.
	//
	// exportaudit:keep — caller-facing orientation; ColorPicker is the
	// only in-repo user.
	Vertical bool

	// Width is the control's length in points, or its thickness when
	// Vertical. Zero takes defaultColorSliderWidth.
	Width float32
	// Height is the control's length in points when Vertical. Zero takes
	// defaultColorSliderWidth. Ignored when horizontal, where the
	// thickness follows the thumb and track.
	Height float32
	// TrackHeight is the colored track's thickness. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing sizing
	TrackHeight float32
	// ThumbSize is the diameter of the ring handle. Zero takes the
	// theme default. The control's height is the larger of this and
	// TrackHeight.
	ThumbSize float32
}

type colorChannelSliderView struct {
	cfg ColorChannelSliderCfg
}

// ColorChannelSlider creates a slider for one HSLA channel, with a
// track showing the colors it can pick.
func ColorChannelSlider(cfg ColorChannelSliderCfg) View {
	requireFocusID("ColorChannelSlider", cfg.FocusDisabled, cfg.ID)
	applyColorChannelSliderDefaults(&cfg)
	return &colorChannelSliderView{cfg: cfg}
}

func applyColorChannelSliderDefaults(cfg *ColorChannelSliderCfg) {
	d := &defaultColorPickerStyle
	if cfg.TrackHeight <= 0 {
		cfg.TrackHeight = d.sliderHeight
	}
	if cfg.ThumbSize <= 0 {
		cfg.ThumbSize = d.indicatorSize
	}
}

func (sv *colorChannelSliderView) GenerateLayout(w *Window) Layout {
	cfg := &sv.cfg
	id := w.EffID(cfg.ID)
	v := cfg.Value.Normalized()
	ch := cfg.Channel
	onChange := cfg.OnChange

	vert := cfg.Vertical
	// length is the swept axis; thick is the other one, which is only
	// as wide as the larger of the thumb and the track.
	length := cfg.Width
	if vert {
		length = cfg.Height
	}
	if length <= 0 {
		length = defaultColorSliderWidth
	}
	thick := f32Max(cfg.ThumbSize, cfg.TrackHeight)
	inset := cfg.ThumbSize / 2

	ctlW, ctlH := length, thick
	trackW, trackH := length, cfg.TrackHeight
	if vert {
		ctlW, ctlH = thick, length
		trackW, trackH = cfg.TrackHeight, length
	}

	// The track's gradient is inset by half a thumb at each end, so
	// the thumb's centre — not its edge — sits on the color it picks.
	// The drag maps the same way, or clicking a color would select a
	// different one.
	apply := func(nx, ny float32, ctx EventCtx) {
		if onChange == nil {
			return
		}
		n := nx
		if vert {
			n = 1 - ny // bottom is the minimum
		}
		onChange(ch.with(v, trackFraction(n, length, inset)*ch.span()),
			ctx)
	}

	frac := ch.value(v) / ch.span()
	if vert {
		frac = 1 - frac
	}
	along := inset + frac*(length-2*inset)
	thumbX, thumbY := along, thick/2
	if vert {
		thumbX, thumbY = thick/2, along
	}

	// The track is thinner than the control when the thumb is the wider
	// of the two, so it is centred on the cross axis.
	trackOffX, trackOffY := float32(0), (thick-cfg.TrackHeight)/2
	if vert {
		trackOffX, trackOffY = (thick-cfg.TrackHeight)/2, 0
	}

	cv := &containerView{
		cfg: ContainerCfg{
			ID:        id,
			Focusable: !cfg.FocusDisabled,
			A11YRole:  AccessRoleSlider,
			A11YCfg: A11YCfg{
				A11YLabel:       a11yLabel(cfg.A11YLabel, ch.name()),
				A11YDescription: cfg.A11YDescription,
			},
			Width:  ctlW,
			Height: ctlH,
			// Fixed: the track image is generated at this size.
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			axis:       axisTopToBottom,
			OnClick:    colorDragTrack(id, apply),
			OnKeyDown:  colorChannelKeys(ch, v, onChange),
		},
		content: []View{
			container(ContainerCfg{
				Float:        true,
				FloatOffsetX: trackOffX,
				FloatOffsetY: trackOffY,
				Width:        trackW,
				Height:       trackH,
				Padding:      NoPadding,
				SizeBorder:   NoBorder,
				Radius:       SomeF(cfg.TrackHeight / 2),
				Clip:         true,
				Content: []View{
					Image(ImageCfg{
						Src: channelStripSrc(ch, v,
							int(trackW)*imgScale,
							int(trackH)*imgScale,
							int(inset)*imgScale, vert),
						Width:  trackW,
						Height: trackH,
					}),
				},
			}),
			colorMarker(cfg.ThumbSize, v.Color(), func(ctx EventCtx) {
				colorAmendMarker(ctx.Layout,
					thumbX, thumbY, cfg.ThumbSize)
			}),
		},
	}
	colorControlFocusRing(&cv.cfg)
	return generateViewLayout(cv, w)
}

// defaultColorSliderWidth is the track length used when Width is
// unset. A slider whose length came from the parent would have to know
// its pixel width at buffer-generation time, which happens before
// arrange — so the width is explicit here rather than Fill.
const defaultColorSliderWidth = 200

// trackFraction converts a pointer position normalized to the whole
// control into a fraction of the inset track, so the value under the
// thumb's centre is the value the thumb picks.
func trackFraction(nx, width, inset float32) float32 {
	track := width - 2*inset
	if track <= 0 {
		return f32Clamp(nx, 0, 1)
	}
	return f32Clamp((nx*width-inset)/track, 0, 1)
}

// name is the channel's accessible label.
func (ch ColorChannel) name() string {
	switch ch {
	case ChannelHue:
		return ActiveLocale.strHue
	case ChannelSaturation:
		return ActiveLocale.strSat
	case ChannelLightness:
		return ActiveLocale.strLightness
	default:
		return ActiveLocale.strAlpha
	}
}

// colorChannelKeys steps the channel with the left and right arrows.
// Up and down move it too: on a one-axis control both pairs mean "more"
// and "less", and requiring the horizontal pair would be a needless
// trap for keyboard users.
func colorChannelKeys(
	ch ColorChannel, v HSLA, onChange func(HSLA, EventCtx),
) func(EventCtx) {
	return colorArrowKeys(v, onChange,
		func(v HSLA, dx, dy float32) HSLA {
			d := dx - dy // up/right increase, down/left decrease
			span := ch.span()
			next := ch.value(v) + d*span
			if ch == ChannelHue {
				// Hue wraps: stepping past 360 lands back at 0 rather
				// than sticking at the end of the strip.
				next = f32Mod(next+360, 360)
			} else {
				next = f32Clamp(next, 0, span)
			}
			return ch.with(v, next)
		})
}
