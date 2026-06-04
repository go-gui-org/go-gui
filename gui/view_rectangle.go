package gui

// RectangleCfg configures a rectangle. Rectangles can be
// filled, outlined, colored, and have radius corners.
type RectangleCfg struct {
	Gradient       *GradientDef
	BorderGradient *GradientDef
	Shadow         *BoxShadow
	Shader         *Shader
	ID             string
	Width          float32
	Height         float32
	MinWidth       float32
	MinHeight      float32
	MaxHeight      float32
	Radius         float32
	BlurRadius     float32
	SizeBorder     float32
	Color          Color
	ColorBorder    Color
	Sizing         Sizing
	Disabled       bool
	Invisible      bool
}

// Rectangle draws a rectangle. Technically a container with no
// children, axis, or padding.
func Rectangle(cfg RectangleCfg) View {
	return container(ContainerCfg{
		ID:             cfg.ID,
		Width:          cfg.Width,
		Height:         cfg.Height,
		MinWidth:       cfg.Width,
		MinHeight:      cfg.Height,
		Sizing:         cfg.Sizing,
		Disabled:       cfg.Disabled,
		Invisible:      cfg.Invisible,
		Color:          cfg.Color,
		ColorBorder:    cfg.ColorBorder,
		Gradient:       cfg.Gradient,
		BorderGradient: cfg.BorderGradient,
		Shadow:         cfg.Shadow,
		BlurRadius:     cfg.BlurRadius,
		Shader:         cfg.Shader,
		Padding:        NoPadding,
		Radius:         Some(cfg.Radius),
		SizeBorder:     Some(cfg.SizeBorder),
		Spacing:        Opt[float32]{},
	})
}
