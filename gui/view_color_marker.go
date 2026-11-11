package gui

// colorMarker builds the ring that shows where a color control's
// current value sits. It is a circle filled with that color and ringed
// in white, so it reads against both ends of the imagery underneath —
// the same treatment on the plane, the wheel, and the slider thumbs.
//
// amend positions it: the marker is a child of the control, and layout
// alone cannot express "at this fraction across the parent", so every
// caller repositions it in AmendLayout against absolute coordinates.
//
// The marker is the control's last child, and the control takes it out
// of the layout flow after generation (colorMarkerLeaveFlow). It stays in
// the control's layer: drawn after the imagery, under every popup, and
// inside every clip around the control. As a Float it was drawn over
// popups and tooltips and escaped scroll clips (#1244).
func colorMarker(
	size float32, fill Color, amend func(EventCtx),
) View {
	return Circle(ContainerCfg{
		Width:  size,
		Height: size,
		// Fixed on both axes: a marker is a circle, not a box that may
		// be resolved per axis. Left Fit, the shrink pass squeezes it
		// against the imagery it sits on — the control's content is
		// taller than the control — and the circle renders as an
		// ellipse.
		Sizing:      FixedFixed,
		Color:       fill,
		ColorBorder: White,
		SizeBorder:  BorderPx(colorMarkerRing),
		Padding:     NoPadding,
		AmendLayout: amend,
	})
}

// colorMarkerLeaveFlow takes the marker, the last child of a generated
// color control, out of the layout flow, so it adds nothing to the
// control's size and does not push the imagery aside. It is set after
// generation, as the slider's Look parts are (#664), because a View has
// no field for it. A layout with no children is left alone.
func colorMarkerLeaveFlow(layout *Layout) {
	if n := len(layout.Children); n > 0 {
		layout.Children[n-1].Shape.outOfFlow = true
	}
}

// colorKnobRoom generates control, a plane or a wheel, inside half a
// marker of padding on every side. The marker is centred on the value's
// point, which can lie on the control's edge, so half of it reaches past
// the control. Without this room, a clipping container the control sits
// flush in cuts that half off: a scroll view clips its content inside
// its padding (#1244). A channel slider needs none: it is as thick as
// its thumb, and its gradient is inset by half a thumb at each end.
//
// The wrapper has no ID and no handlers, so identity, focus, keys and
// the drag stay on the control. A press on the room does nothing, also
// on the half of a marker that lies in it. That is unchanged: when the
// marker was a float, that half took no press either (checked against
// the float version for a plane and a wheel, 3 and 6 px past the edge).
func colorKnobRoom(control View, marker float32, w *Window) Layout {
	return generateViewLayout(&containerView{
		cfg: ContainerCfg{
			Padding:    PadAll(marker / 2),
			SizeBorder: NoBorder,
			axis:       axisTopToBottom,
		},
		content: []View{control},
	}, w)
}

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
