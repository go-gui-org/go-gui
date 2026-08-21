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

// focusRingAmend with both colors unset must produce no hook under a
// ringless theme, so a widget that never fills either keeps its plain
// AmendLayout slot there; under a theme whose FocusRing is set, the
// ring alone justifies the hook. Windows and GNOME build from baseCfg
// with no ring; the default presets carry one (visual-refresh § 5.4).
func TestFocusRingAmendUnsetIsNil(t *testing.T) {
	saved := guiTheme
	t.Cleanup(func() { guiTheme = saved })

	cfg := baseDarkCfg()
	cfg.Name = "ringless"
	cfg.FocusRing = nil
	guiTheme = ThemeMaker(cfg)

	if got := focusRingAmend(Color{}, Color{}); got != nil {
		t.Error("no ring, both colors unset: want nil hook")
	}
	if got := focusRingAmend(RGB(1, 2, 3), Color{}); got == nil {
		t.Error("no ring, fill set: want non-nil hook")
	}
	if got := focusRingAmend(Color{}, RGB(1, 2, 3)); got == nil {
		t.Error("no ring, border set: want non-nil hook")
	}

	guiTheme = ThemeDark
	if got := focusRingAmend(Color{}, Color{}); got == nil {
		t.Error("themed ring, colors unset: want ring-only hook")
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
	cfg.FocusRing = ring
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

// The widgets wired in phase 5b (visual-refresh.md § 5.4) all attach
// their ring through focusRingAmend, but a hook contract test does not
// prove the hook is attached — each widget must actually emit the ring
// when focused and stay shadow-free when it is not. The list is the
// wiring inventory: a widget added or removed here is the diff review
// noticing the spec's wiring list changed.
func TestWiredFocusablesEmitRingWhenFocused(t *testing.T) {
	cases := []struct {
		name  string
		id    string
		build func() View
	}{
		{"combobox", "cb", func() View {
			return Combobox(ComboboxCfg{ID: "cb", Options: []string{"a"}})
		}},
		{"date_picker", "dp", func() View {
			return DatePicker(DatePickerCfg{ID: "dp"})
		}},
		{"input_date", "id", func() View {
			return InputDate(InputDateCfg{ID: "id"})
		}},
		{"numeric_input", "num", func() View {
			return NumericInput(NumericInputCfg{ID: "num"})
		}},
		{"tree", "tree", func() View {
			return Tree(TreeCfg{ID: "tree", Nodes: []TreeNodeCfg{{Text: "node"}}})
		}},
		{"radio", "radio", func() View {
			return Radio(RadioCfg{ID: "radio"})
		}},
		{"switch", "swt", func() View {
			return Switch(SwitchCfg{ID: "swt"})
		}},
		{"toggle", "tg", func() View {
			return Toggle(ToggleCfg{ID: "tg"})
		}},
		{"slider", "sld", func() View {
			return Slider(SliderCfg{ID: "sld"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			build := func(*Window) View { return tc.build() }
			if got := countShadows(frameCmds(t, ThemeDark, build, tc.id, nil)); got != 1 {
				t.Errorf("focused: emitted %d shadows, want 1 (the ring)", got)
			}
			if got := countShadows(frameCmds(t, ThemeDark, build, "", nil)); got != 0 {
				t.Errorf("unfocused: emitted %d shadows, want 0", got)
			}
		})
	}
}

// focusRingBorderAmend returns nil when there is nothing to paint, so
// a caller that cannot hold focus pays no closure — the ListBox passes
// an empty key for an ID-less list, the colour picker relies on the
// width never being 0, and an unset colour means the theme never
// decided what focus looks like.
func TestFocusRingBorderAmendNilWhenNothingToPaint(t *testing.T) {
	color := RGB(1, 2, 3)
	if got := focusRingBorderAmend("", focusRingBorderWidth, color); got != nil {
		t.Error("empty key: want nil hook")
	}
	if got := focusRingBorderAmend("k", 0, color); got != nil {
		t.Error("zero width: want nil hook")
	}
	if got := focusRingBorderAmend("k", focusRingBorderWidth, Color{}); got != nil {
		t.Error("unset colour: want nil hook")
	}
	if got := focusRingBorderAmend("k", focusRingBorderWidth, color); got == nil {
		t.Error("full arguments: want a hook")
	}
}

// The hook strokes width + colour on the shape only while the keyed
// control holds focus; a disabled shape and a nil layout get nothing.
func TestFocusRingBorderAmendStrokesOnlyWhenFocused(t *testing.T) {
	t.Parallel()
	ringColor := RGB(1, 2, 3)
	hook := focusRingBorderAmend("cp", focusRingBorderWidth, ringColor)
	if hook == nil {
		t.Fatal("want a hook")
	}

	run := func(disabled bool, layoutNil bool) *Shape {
		w := makeWindowWithScratch()
		w.SetFocus("cp")
		s := &Shape{shapeType: shapeRectangle, Disabled: disabled}
		ctx := EventCtx{Layout: &Layout{Shape: s}, Window: w}
		if layoutNil {
			ctx.Layout = nil
		}
		hook(ctx)
		return s
	}

	if s := run(false, false); s.SizeBorder != focusRingBorderWidth ||
		s.ColorBorder != ringColor {
		t.Errorf("focused shape: border = %v/%v, want %v/%v",
			s.SizeBorder, s.ColorBorder, focusRingBorderWidth, ringColor)
	}
	if s := run(false, true); s.SizeBorder != 0 || s.ColorBorder.IsSet() {
		t.Error("nil layout: shape must be untouched")
	}
	if s := run(true, false); s.SizeBorder != 0 || s.ColorBorder.IsSet() {
		t.Error("disabled shape must not take a ring")
	}

	w := makeWindowWithScratch()
	s := &Shape{shapeType: shapeRectangle}
	hook(EventCtx{Layout: &Layout{Shape: s}, Window: w})
	if s.SizeBorder != 0 || s.ColorBorder.IsSet() {
		t.Error("unfocused window: shape must be untouched")
	}
}

// colorControlFocusRing is the colour picker's entry point to the
// border-ring mechanism: it sources the colour from the theme and
// composes the ring into any AmendLayout the control already carries,
// ring last so it wins any colour a prior hook set (amendAll's
// contract). A theme that never decided a focus colour must leave the
// control's hook alone entirely.
func TestColorControlFocusRingComposesAndSkipsUnset(t *testing.T) {
	savedTheme := guiTheme
	savedStyle := defaultColorPickerStyle
	t.Cleanup(func() {
		guiTheme = savedTheme
		defaultColorPickerStyle = savedStyle
	})

	// Composition under a theme that decides the focus colour.
	guiTheme = ThemeDark
	prior := func(ctx EventCtx) {
		ctx.Layout.Shape.ColorBorder = RGB(9, 9, 9)
	}
	cfg := ContainerCfg{AmendLayout: prior}
	colorControlFocusRing(&cfg, "cp")
	if cfg.AmendLayout == nil {
		t.Fatal("ring not composed into the control's AmendLayout")
	}
	w := makeWindowWithScratch()
	w.SetFocus("cp")
	shape := &Shape{shapeType: shapeRectangle}
	cfg.AmendLayout(EventCtx{Layout: &Layout{Shape: shape}, Window: w})
	if shape.SizeBorder != focusRingBorderWidth {
		t.Errorf("ring width = %v, want %v",
			shape.SizeBorder, focusRingBorderWidth)
	}
	if want := defaultColorPickerStyle.ColorBorderFocus; shape.ColorBorder != want {
		t.Errorf("ring colour = %v, want the theme's %v", shape.ColorBorder, want)
	}

	// No focus colour decided: the control keeps its own hook, the
	// wrapper adds nothing.
	defaultColorPickerStyle = ColorPickerStyle{}
	cfg = ContainerCfg{AmendLayout: prior}
	colorControlFocusRing(&cfg, "cp")
	shape = &Shape{shapeType: shapeRectangle}
	cfg.AmendLayout(EventCtx{Layout: &Layout{Shape: shape}, Window: w})
	if shape.SizeBorder != 0 {
		t.Error("unset theme colour still installed a ring")
	}
	if shape.ColorBorder != RGB(9, 9, 9) {
		t.Error("prior hook lost: its colour stamp must survive")
	}
}

// The colour half of the colour-picker ring is theme-sourced; the
// width is a const (focusRingBorderWidth) so no preset can delete it.
// Every registered theme must still decide the colour, or a future
// borderless preset silently removes the indicator again (issue #390).
func TestRegisteredThemesCarryColorPickerFocusColor(t *testing.T) {
	names := themeRegisteredNames()
	if len(names) == 0 {
		t.Fatal("no registered themes")
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			th, ok := ThemeGet(name)
			if !ok {
				t.Fatalf("theme %q not found", name)
			}
			color := th.colorPickerStyle.ColorBorderFocus
			if !color.IsSet() {
				t.Error("colorPickerStyle.ColorBorderFocus is unset")
			}
			if got := focusRingBorderAmend("k", focusRingBorderWidth, color); got == nil {
				t.Error("focus ring helper yielded no hook for this theme")
			}
		})
	}
}

// With no ring in the theme, the hook keeps its old contract: unset
// colours mean no hook at all. Covered by the ringless half of
// TestFocusRingAmendUnsetIsNil; this pins the ring-bearing default
// presets against losing the ring when the contract is next touched.
func TestFocusRingDefaultsCarryARing(t *testing.T) {
	if ThemeDark.focusRing == nil {
		t.Error("ThemeDark carries no focus ring (visual-refresh § 5.4)")
	}
	if ThemeLight.focusRing == nil {
		t.Error("ThemeLight carries no focus ring (visual-refresh § 5.4)")
	}
}
