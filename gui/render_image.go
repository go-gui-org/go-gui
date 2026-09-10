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
	}, w)
}
