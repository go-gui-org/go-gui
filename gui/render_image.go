package gui

// renderImage renders an image shape by emitting a RenderImage
// command with the shape's resource path and clip radius.
func renderImage(shape *Shape, clip drawClip, w *Window) {
	if !rectsOverlap(shapeBounds(shape), clip) {
		return
	}

	// Hide Color from renderContainer so it doesn't draw a
	// redundant bg rect; the backend handles the fill itself.
	bgColor := shape.Color
	shape.Color = ColorTransparent
	renderContainer(shape, ColorTransparent, clip, w)
	shape.Color = bgColor

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
