package gui

// Focus appearance for container-shaped widgets.
//
// Button reaches its focus colors through shapeButtonColors and
// buttonAmendLayout. Everything else that is focusable but not a button
// — Select, ListBox, Table — has to restyle its own shape once focus
// lands on it, because focus is resolved after layout and the shape is
// already built by then.
//
// That was open-coded per widget, which is why ListBox and Table simply
// had no focus ring: nothing forced the question at the point a widget
// was written (issue #335, audit section 6). One helper makes the
// convention reachable, so a new focusable container gets a ring by
// asking for one rather than by remembering the mechanism.

// colorControlFocusRing fills in the focus ring for a color-picker
// control — the plane, wheel, channel slider and swatch.
//
// All four are focusable and draggable and drew no focus indication of
// any kind, so a keyboard user had no way to tell which one they were
// on (issue #335, audit section 6). They are also the awkward case: a
// gradient surface cannot tint on hover or focus the way a button fill
// can, because the fill *is* the value being edited. The ring goes on
// the border instead.
//
// Both the width and the color are applied from AmendLayout, which is
// the whole reason this is separate from focusRingAmend. These controls
// carry no border at rest, and a border in the *config* insets the
// content: reserving one shifted the plane's gradient 1.5px away from
// the marker positions, which are computed from the control's own size.
// AmendLayout runs after arrange, so a ring set there strokes the edge
// without moving anything inside it.
// key is the effective ID the ring tests focus against. The control's
// own shape keys via idKey() at hook time; the swatch's ring lives on
// its color layer, which has no identity of its own, so the swatch
// captures its effective ID at generation instead. AmendLayout runs
// after arrange, so a ring set there strokes the edge without moving
// anything inside it.
func colorControlFocusRing(cfg *ContainerCfg, key string) {
	d := &defaultColorPickerStyle
	width, color := d.SizeBorder, d.ColorBorderFocus
	if width <= 0 || !color.IsSet() {
		return
	}
	cfg.AmendLayout = amendAll(cfg.AmendLayout, func(ctx EventCtx) {
		shape := ctx.Layout.Shape
		if shape == nil || shape.Disabled || ctx.Window == nil {
			return
		}
		if !ctx.Window.IsFocus(key) {
			return
		}
		shape.SizeBorder = width
		shape.ColorBorder = color
	})
}

// amendAll runs several AmendLayout hooks in order, skipping nil ones,
// and returns nil when nothing is left to run.
//
// A shape has one AmendLayout slot, so a widget that already uses it for
// something else — ListBox captures its arranged height there — cannot
// also take a focus ring without composing. Order is call order: the
// ring goes last so it wins any color a prior hook set.
func amendAll(fns ...func(EventCtx)) func(EventCtx) {
	live := fns[:0]
	for _, fn := range fns {
		if fn != nil {
			live = append(live, fn)
		}
	}
	switch len(live) {
	case 0:
		return nil
	case 1:
		return live[0]
	}
	hooks := make([]func(EventCtx), len(live))
	copy(hooks, live)
	return func(ctx EventCtx) {
		for _, fn := range hooks {
			fn(ctx)
		}
	}
}

// focusRingAmend returns an AmendLayout hook that applies a focus
// appearance to its own shape.
//
// Unset colors are skipped, so a widget can take a border ring without
// also restating a fill. A disabled shape never shows focus — it cannot
// hold it in the first place, and the dimmed fill reads as inert, so a
// ring there would contradict the rest of the control.
//
// Keying is via idKey(), the effective ID, so a widget dropped inside an
// ID-bearing container still matches the focus the window recorded.
func focusRingAmend(colorFill, colorBorder Color) func(EventCtx) {
	if !colorFill.IsSet() && !colorBorder.IsSet() {
		return nil
	}
	return func(ctx EventCtx) {
		shape := ctx.Layout.Shape
		if shape == nil || shape.Disabled || ctx.Window == nil {
			return
		}
		if !ctx.Window.IsFocus(shape.idKey()) {
			return
		}
		if colorFill.IsSet() {
			shape.Color = colorFill
		}
		if colorBorder.IsSet() {
			shape.ColorBorder = colorBorder
		}
	}
}
