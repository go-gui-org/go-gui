package gui

import "testing"

// amendAll composes the one AmendLayout slot a shape has. The
// composition contract: nil hooks are dropped, nothing live means no
// hook at all, and a single live hook runs without a wrapper.
func TestAmendAllComposition(t *testing.T) {
	var order []string
	a := func(EventCtx) { order = append(order, "a") }
	b := func(EventCtx) { order = append(order, "b") }

	if got := amendAll(); got != nil {
		t.Error("empty call: want nil hook")
	}
	if got := amendAll(nil, nil); got != nil {
		t.Error("all-nil: want nil hook")
	}

	if got := amendAll(nil, a); got == nil {
		t.Fatal("one live hook: want non-nil")
	} else {
		got(EventCtx{})
	}
	if len(order) != 1 || order[0] != "a" {
		t.Errorf("single hook ran %v, want [a]", order)
	}

	order = nil
	h := amendAll(a, nil, b)
	if h == nil {
		t.Fatal("mixed hooks: want non-nil")
	}
	h(EventCtx{})
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("mixed hooks ran %v, want [a b] in call order", order)
	}
}

// focusRingAmend with both colors unset must produce no hook, so a
// widget that never fills either keeps its plain AmendLayout slot.
func TestFocusRingAmendUnsetIsNil(t *testing.T) {
	if got := focusRingAmend(Color{}, Color{}); got != nil {
		t.Error("both colors unset: want nil hook")
	}
	if got := focusRingAmend(RGB(1, 2, 3), Color{}); got == nil {
		t.Error("fill set: want non-nil hook")
	}
	if got := focusRingAmend(Color{}, RGB(1, 2, 3)); got == nil {
		t.Error("border set: want non-nil hook")
	}
}

// applyFocusRingShadow is the whole focus-ring mechanism: it hangs a
// theme-owned shadow on the focused shape and allocates the effects
// record if the shape had none. The cases below are the ones that can
// silently corrupt shared state or lose a widget's own appearance.
func TestApplyFocusRingShadowAllocatesWhenAbsent(t *testing.T) {
	t.Parallel()
	w := makeWindowWithScratch()
	ring := &BoxShadow{Color: RGB(0, 122, 255), BlurRadius: 3}
	s := &Shape{shapeType: shapeRectangle}

	applyFocusRingShadow(s, w, ring)

	if s.fx == nil {
		t.Fatal("effects not allocated for a shape that had none")
	}
	if s.fx.Shadow != ring {
		t.Errorf("shadow: got %p, want the theme ring %p", s.fx.Shadow, ring)
	}
}

// A theme with no ring must leave the shape exactly as it found it —
// in particular it must not allocate an effects record, because that
// is a per-frame allocation for every focusable widget in the window.
func TestApplyFocusRingShadowNilRingIsInert(t *testing.T) {
	t.Parallel()
	w := makeWindowWithScratch()
	s := &Shape{shapeType: shapeRectangle}

	applyFocusRingShadow(s, w, nil)

	if s.fx != nil {
		t.Error("nil ring allocated an effects record")
	}
}

// A shape that already carries an effects record — a gradient, say —
// but no shadow of its own must still receive the ring, without
// losing the effects it does have.
func TestApplyFocusRingShadowFillsEmptyExistingEffects(t *testing.T) {
	t.Parallel()
	w := makeWindowWithScratch()
	ring := &BoxShadow{Color: RGB(0, 122, 255), BlurRadius: 3}
	s := &Shape{
		shapeType: shapeRectangle,
		fx:        &shapeEffects{Gradient: &GradientDef{}},
	}

	applyFocusRingShadow(s, w, ring)

	if s.fx.Shadow != ring {
		t.Errorf("shadow: got %p, want the theme ring %p", s.fx.Shadow, ring)
	}
	if s.fx.Gradient == nil {
		t.Error("existing effects lost when the ring was applied")
	}
}

// A widget that already paints its own shadow keeps it. The ring is
// focus indication, not elevation: an elevated popover that takes
// focus must not trade its elevation for a glow.
func TestApplyFocusRingShadowKeepsExistingShadow(t *testing.T) {
	t.Parallel()
	w := makeWindowWithScratch()
	own := &BoxShadow{Color: RGBA(0, 0, 0, 80), BlurRadius: 20}
	ring := &BoxShadow{Color: RGB(0, 122, 255), BlurRadius: 3}
	s := &Shape{shapeType: shapeRectangle, fx: &shapeEffects{Shadow: own}}

	applyFocusRingShadow(s, w, ring)

	if s.fx.Shadow != own {
		t.Errorf("shadow: got %p, want the widget's own %p", s.fx.Shadow, own)
	}
}

// The ring is one value shared by every shape in the window. Writing
// through the pointer would restyle every other control at once, so
// the mechanism must only ever assign it.
func TestApplyFocusRingShadowDoesNotMutateTheme(t *testing.T) {
	t.Parallel()
	w := makeWindowWithScratch()
	ring := &BoxShadow{Color: RGB(0, 122, 255), BlurRadius: 3}
	before := *ring

	for range 3 {
		applyFocusRingShadow(&Shape{shapeType: shapeRectangle}, w, ring)
	}

	if *ring != before {
		t.Errorf("theme ring mutated: got %+v, want %+v", *ring, before)
	}
}

// Under a theme with a ring, only the focused shape gets one, and a
// disabled shape gets none even while it holds the focus key —
// matching the existing colour behaviour of the same hook.
func TestFocusRingAmendAppliesShadowOnlyWhenFocused(t *testing.T) {
	saved := guiTheme
	t.Cleanup(func() { guiTheme = saved })

	ring := &BoxShadow{Color: RGB(0, 122, 255), BlurRadius: 3}
	cfg := baseDarkCfg()
	cfg.Name = "ring-test"
	cfg.focusRing = ring
	guiTheme = ThemeMaker(cfg)

	hook := focusRingAmend(Color{}, RGB(1, 2, 3))
	if hook == nil {
		t.Fatal("themed ring: want a hook even with colours unset")
	}

	run := func(id string, disabled bool) *Shape {
		w := makeWindowWithScratch()
		w.SetFocus("focused")
		s := &Shape{shapeType: shapeRectangle, ID: id, Disabled: disabled}
		hook(EventCtx{Layout: &Layout{Shape: s}, Window: w})
		return s
	}

	if s := run("focused", false); s.fx == nil || s.fx.Shadow != ring {
		t.Error("focused shape did not receive the ring")
	}
	if s := run("other", false); s.fx != nil {
		t.Error("unfocused shape received a ring")
	}
	if s := run("focused", true); s.fx != nil {
		t.Error("disabled shape received a ring")
	}
}

// With no ring in the theme, the hook keeps its old contract: unset
// colours mean no hook at all.
func TestFocusRingAmendNoRingKeepsNilContract(t *testing.T) {
	saved := guiTheme
	t.Cleanup(func() { guiTheme = saved })
	guiTheme = ThemeDark

	if got := focusRingAmend(Color{}, Color{}); got != nil {
		t.Error("no ring and no colours: want nil hook")
	}
}
