package gui

// colorDragTrack returns an OnClick handler that reports the pointer
// position normalized to the clicked shape's own box — (0,0) at its
// top-left corner, (1,1) at its bottom-right, clamped — and keeps
// reporting while the button is held.
//
// The three draggable color components (plane, wheel, channel slider)
// need exactly this and nothing else, and the sequence is easy to get
// subtly wrong: OnClick arrives with shape-relative coordinates while
// MouseLock delivers window-absolute ones, so the first report has to
// be converted up before it can share a code path with the drag, and
// the dragged shape has to be re-found by ID each move because the
// layout tree is rebuilt every frame.
//
// id must be the *effective* ID of the shape the handler is installed
// on — FindByID takes effective IDs.
func colorDragTrack(
	id string, onPoint func(nx, ny float32, ctx EventCtx),
) func(EventCtx) {
	report := func(shape *Shape, ctx EventCtx) {
		if shape.Width <= 0 || shape.Height <= 0 || ctx.Event == nil {
			return
		}
		nx := f32Clamp(
			(ctx.Event.MouseX-shape.X)/shape.Width, 0, 1)
		ny := f32Clamp(
			(ctx.Event.MouseY-shape.Y)/shape.Height, 0, 1)
		onPoint(nx, ny, ctx)
	}

	return func(ctx EventCtx) {
		if ctx.Event == nil || ctx.Layout == nil {
			return
		}
		// Lift the click into window-absolute space so it means the
		// same thing as every MouseMove that follows it.
		ev := *ctx.Event
		ev.MouseX += ctx.Layout.Shape.X
		ev.MouseY += ctx.Layout.Shape.Y
		report(ctx.Layout.Shape,
			EventCtx{ctx.Layout, &ev, ctx.Window})

		ctx.Window.MouseLock(MouseLockCfg{
			MouseMove: func(mc EventCtx) {
				// The layout is rebuilt every frame, so the shape
				// captured at press time is stale by now.
				l, ok := mc.Layout.FindByID(id)
				if !ok {
					return
				}
				report(l.Shape, mc)
			},
			MouseUp: func(mc EventCtx) {
				mc.Window.MouseUnlock()
			},
			Cancel: func(w *Window) {
				// Capture revoked with no MouseUp coming. Nothing to
				// commit — the value was written on every move — so
				// just let go.
				w.MouseUnlock()
			},
		})
		// These controls sit inside a focusable container in the
		// composed picker; without consuming, the click would travel
		// on and move focus off the control being dragged.
		ctx.Consume()
	}
}

// colorKeyStep is the fraction of a channel's full span one arrow key
// moves. 1/100 gives a percent per press on saturation, lightness and
// alpha, and a little under 4° on hue — fine enough to be useful,
// coarse enough to cross the range without holding the key forever.
const colorKeyStep = 0.01

// colorKeyDelta maps an arrow key to a step along one axis, as a
// fraction of the axis span. ok=false for any other key, which the
// caller must let travel on rather than consume.
//
// Shift takes the larger step: the same 10× convention the slider and
// numeric input use for a coarse adjustment.
func colorKeyDelta(e *Event) (dx, dy float32, ok bool) {
	if e == nil {
		return 0, 0, false
	}
	step := float32(colorKeyStep)
	if e.Modifiers&ModShift != 0 {
		step *= 10
	}
	switch e.KeyCode {
	case KeyLeft:
		return -step, 0, true
	case KeyRight:
		return step, 0, true
	case KeyUp:
		return 0, -step, true
	case KeyDown:
		return 0, step, true
	}
	return 0, 0, false
}
