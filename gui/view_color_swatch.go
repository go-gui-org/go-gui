package gui

// ColorSwatchCfg configures a color preview swatch.
//
// The color is drawn over a checkerboard, so a half-transparent value
// reads as half-transparent instead of as a muted opaque one. That is
// the whole reason this is a widget rather than a Rectangle: the
// checkerboard is a generated buffer, and compositing over it is not
// something a caller should have to assemble.
type ColorSwatchCfg struct {
	ID string

	A11YCfg

	// Color is the value shown. Its alpha is honored.
	Color Color

	Width  float32
	Height float32
	Radius Opt[float32]
}

type colorSwatchView struct {
	cfg ColorSwatchCfg
}

// ColorSwatch creates a color preview over a transparency
// checkerboard.
func ColorSwatch(cfg ColorSwatchCfg) View {
	applyColorSwatchDefaults(&cfg)
	return &colorSwatchView{cfg: cfg}
}

func applyColorSwatchDefaults(cfg *ColorSwatchCfg) {
	d := &defaultColorPickerStyle
	if cfg.Width <= 0 {
		cfg.Width = defaultSwatchWidth
	}
	if cfg.Height <= 0 {
		cfg.Height = defaultSwatchHeight
	}
	if !cfg.Radius.IsSet() {
		cfg.Radius = Some(d.Radius)
	}
}

const (
	defaultSwatchWidth  = 48
	defaultSwatchHeight = 32
	// colorSwatchBorder is a hairline: enough to separate the color
	// from the surface behind it, not enough to read as a frame around
	// it or to eat into a small swatch.
	colorSwatchBorder = 1
	// colorSwatchEdgeAlpha keeps the outline a contour rather than a
	// stroke. Derived from the theme's border color, so it darkens on
	// a light theme and lightens on a dark one instead of being one
	// gray that is wrong against one of them.
	colorSwatchEdgeAlpha = 0.5
)

// colorSwatchEdge is the swatch's outline color, dimmed from the
// theme's own border color.
func colorSwatchEdge() Color {
	return defaultColorPickerStyle.ColorBorder.WithOpacity(colorSwatchEdgeAlpha)
}

func (sv *colorSwatchView) GenerateLayout(w *Window) Layout {
	cfg := &sv.cfg

	return generateViewLayout(&containerView{
		cfg: ContainerCfg{
			ID:       cfg.ID,
			A11YRole: AccessRoleColorWell,
			A11YCfg: A11YCfg{
				A11YLabel: a11yLabel(cfg.A11YLabel,
					// Naming the value, not "swatch": the color is
					// the information, and hex is how it is written.
					cfg.Color.Hex()),
				A11YDescription: cfg.A11YDescription,
			},
			Width:  cfg.Width,
			Height: cfg.Height,
			// Fixed: the checkerboard is generated at this size.
			Sizing:     FixedFixed,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Radius:     cfg.Radius,
			Clip:       true,
			axis:       axisTopToBottom,
		},
		content: []View{
			Image(ImageCfg{
				Src: checkerSrc(
					int(cfg.Width)*imgScale,
					int(cfg.Height)*imgScale),
				Width:  cfg.Width,
				Height: cfg.Height,
			}),
			// The color rides over the checker as a float so it
			// covers it exactly rather than being laid out after it.
			container(ContainerCfg{
				Float:  true,
				Width:  cfg.Width,
				Height: cfg.Height,
				Color:  cfg.Color,
				Radius: cfg.Radius,
				// The outline goes on the color, not on the box
				// beneath it: this container covers the whole
				// swatch, so a border on the parent would be drawn
				// under it. Without one a white value on a light
				// theme is an invisible rectangle -- the swatch
				// vanishes exactly when the color is hardest to
				// read off the hex string.
				ColorBorder: colorSwatchEdge(),
				SizeBorder:  SomeF(colorSwatchBorder),
				Padding:     NoPadding,
			}),
		},
	}, w)
}
