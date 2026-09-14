package gui

// renderImage renders an image shape by emitting a RenderImage
// command with the shape's resource path and clip radius.
func renderImage(shape *Shape, clip drawClip, w *Window) {
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}

	// Hide Color from renderContainer so it doesn't draw a
	// redundant bg rect; the backend handles the fill itself.
	// Opacity already rides on shape.Color via renderShape's
	// mutation; only the disabled dim is left to apply here.
	// The dim goes to a local: restoring it onto shape.Color would
	// halve again on the next frame, because renderShape only
	// restores the shape when Opacity < 1.
	origColor := shape.Color
	bgColor := origColor
	if shape.Disabled {
		bgColor = dimAlpha(bgColor)
	}
	shape.Color = ColorTransparent
	renderContainer(shape, ColorTransparent, clip, w)
	shape.Color = origColor

	emitRenderer(RenderCmd{
		Kind:       RenderImage,
		X:          shape.X,
		Y:          shape.Y,
		W:          shape.Width,
		H:          shape.Height,
		Color:      bgColor,
		Resource:   shape.Resource,
		ClipRadius: w.clipRadius,
		Opacity:    imageAlpha(shape.Opacity, shape.Disabled),
	}, w)
}

// imageAlpha folds widget opacity and the disabled dim into one
// 0..1 multiplier for image texels. Built on dimColor so images
// dim identically to text, gradients and fills; a NaN opacity
// applies nothing, matching renderShape.
func imageAlpha(opacity float32, disabled bool) float32 {
	return float32(dimColor(White, opacity, disabled).A) / 255
}
