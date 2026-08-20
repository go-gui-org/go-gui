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
