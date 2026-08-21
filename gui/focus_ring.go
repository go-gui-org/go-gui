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

// focusRingBorderWidth is the stroke width of the keyboard-focus ring
// a container paints on its border.
//
// It is a const, never a theme's SizeBorder: a borderless theme sets
// that to 0 (ThemeLight does, issue #390), and the keyboard cursor
// has to be visible in every theme.
const focusRingBorderWidth = 1

// focusRingBorderAmend returns the AmendLayout hook that strokes a
// focus ring — SizeBorder + ColorBorder — on the shape that currently
// holds key.
//
// Key is an effective ID, tested via IsFocus. It may be a foreign ID:
// a list row has no identity of its own, so its ring tests the owning
// list's key; the color swatch's ring lives on its color layer, so it
// captures the control's effective ID at generation instead.
//
// The ring is applied from AmendLayout, never from the Cfg: a border in
// the config insets content and would shift what the ring is meant to
// outline (the plane's gradient would move 1px off its marker
// positions). AmendLayout runs after arrange, so a ring set there
// strokes the edge without moving anything inside it.
//
// Returns nil when there is nothing to paint — no key, no width, no
// color — so a caller that cannot hold focus pays no closure. A
// disabled shape never shows focus: it cannot hold it, and the dimmed
// fill already reads as inert.
func focusRingBorderAmend(
	key string, width float32, color Color,
) func(EventCtx) {
	if key == "" || width <= 0 || !color.IsSet() {
		return nil
	}
	return func(ctx EventCtx) {
		if ctx.Layout == nil || ctx.Window == nil {
			return
		}
		shape := ctx.Layout.Shape
		if shape == nil || shape.Disabled {
			return
		}
		if !ctx.Window.IsFocus(key) {
			return
		}
		shape.SizeBorder = width
		shape.ColorBorder = color
	}
}

// colorControlFocusRing fills in the focus ring for a color-picker
// control — the plane, wheel, channel slider and swatch.
//
// All four are focusable and draggable and drew no focus indication of
// any kind, so a keyboard user had no way to tell which one they were
// on (issue #335, audit section 6). They are also the awkward case: a
// gradient surface cannot tint on hover or focus the way a button fill
// can, because the fill *is* the value being edited. The ring goes on
// the border instead, via focusRingBorderAmend.
//
// A theme that never decided a focus colour yields a nil hook, which
// amendAll drops — the control's own AmendLayout survives untouched.
func colorControlFocusRing(cfg *ContainerCfg, key string) {
	cfg.AmendLayout = amendAll(cfg.AmendLayout, focusRingBorderAmend(
		key, focusRingBorderWidth, defaultColorPickerStyle.ColorBorderFocus))
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
	// Captured at generation, which is the only correct time to read a
	// theme: the bare guiTheme honours a surrounding Themed scope, while
	// w.Theme() at amend time would ignore it.
	ring := guiTheme.focusRing
	if !colorFill.IsSet() && !colorBorder.IsSet() && ring == nil {
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
		applyFocusRingShadow(shape, ctx.Window, ring)
	}
}

// applyFocusRingShadow hangs the theme's focus ring on a shape that
// currently holds focus.
//
// The ring is a zero-offset tinted shadow rather than a border: macOS
// draws focus as a soft glow *outside* the control, and a border insets
// content, which would shift a control's interior the moment it took
// focus. renderContainer emits the shadow before the fill, so the glow
// sits behind the control rather than over it.
//
// Allocating here, rather than reserving a shapeEffects at generation
// for every focusable widget, means only the one focused shape in the
// window pays. The pool hands out individually allocated objects and
// grows a slice of pointers, so a pointer taken here stays valid even
// if the pool grows again later in the frame; it is invalidated only by
// the next view-phase reset, which is also when the shape itself dies.
//
// ring is theme-owned and shared by every shape in the window — assign
// it, never write through it.
func applyFocusRingShadow(shape *Shape, w *Window, ring *BoxShadow) {
	if ring == nil {
		return
	}
	// Shapes are rebuilt from their Cfg every frame, so an unfocused
	// shape never carries a stale ring and there is nothing to clear.
	if shape.fx == nil {
		shape.fx = w.allocEffects(shapeEffects{Shadow: ring})
		return
	}
	// A widget that already paints its own shadow keeps it: the ring is
	// focus indication, not elevation, and a popover that owns both
	// would otherwise lose the elevation it was given.
	if shape.fx.Shadow == nil {
		shape.fx.Shadow = ring
	}
}
