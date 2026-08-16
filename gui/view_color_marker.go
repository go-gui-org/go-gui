package gui

// colorMarker builds the ring that shows where a color control's
// current value sits. It is a circle filled with that color and ringed
// in white, so it reads against both ends of the imagery underneath —
// the same treatment on the plane, the wheel, and the slider thumbs.
//
// amend positions it: the marker is a child of the control, and layout
// alone cannot express "at this fraction across the parent", so every
// caller repositions it in AmendLayout against absolute coordinates.
func colorMarker(
	size float32, fill Color, amend func(EventCtx),
) View {
	return Circle(ContainerCfg{
		Width:  size,
		Height: size,
		// Fixed on both axes: a marker is a circle, not a box that may
		// be resolved per axis. Left Fit, the shrink pass squeezes it
		// against the imagery it floats over — the control's content is
		// taller than the control — and the circle renders as an
		// ellipse.
		Sizing:      FixedFixed,
		Color:       fill,
		ColorBorder: White,
		SizeBorder:  SomeF(colorMarkerRing),
		Padding:     NoPadding,
		// Float with a z-index above the imagery: a channel slider
		// floats its track to centre it in a control the thumb makes
		// taller, and a marker drawn under that track is invisible.
		Float:       true,
		FloatZIndex: colorMarkerZ,
		AmendLayout: amend,
	})
}

// colorMarkerZ keeps markers above the generated imagery they sit on.
const colorMarkerZ = 1

// colorMarkerRing is the marker's white outline width in points.
const colorMarkerRing = 2

// colorAmendMarker centres a marker on (x, y), given relative to the
// parent's top-left corner. AmendLayout works in absolute coordinates,
// so the parent's origin is added here rather than by every caller.
func colorAmendMarker(
	layout *Layout, x, y, size float32,
) {
	if layout == nil || layout.Parent == nil {
		return
	}
	r := size / 2
	layout.Shape.X = layout.Parent.Shape.X + x - r
	layout.Shape.Y = layout.Parent.Shape.Y + y - r
}
