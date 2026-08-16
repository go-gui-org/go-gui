package gui

import "testing"

func TestColorPickerLayout(t *testing.T) {
	w := &Window{}
	v := ColorPicker(ColorPickerCfg{
		ID:    "cp1",
		Color: RGB(255, 0, 0),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "cp1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if layout.Shape.shapeType != shapeRectangle {
		t.Errorf("type = %d", layout.Shape.shapeType)
	}
}

// ShowHSV is the deprecated spelling of ShowHSL and must keep working:
// callers in other repos still set it.
func TestColorPickerShowHSVStillAddsRow(t *testing.T) {
	countInputs := func(cfg ColorPickerCfg) int {
		w := &Window{}
		layout := generateViewLayout(ColorPicker(cfg), w)
		n := 0
		var walk func(*Layout)
		walk = func(l *Layout) {
			if l.Shape != nil && l.Shape.shapeType == shapeText {
				n++
			}
			for i := range l.Children {
				walk(&l.Children[i])
			}
		}
		walk(&layout)
		return n
	}
	base := countInputs(ColorPickerCfg{
		ID: "cp-plain", Color: RGB(0, 128, 255),
	})
	hsv := countInputs(ColorPickerCfg{
		ID: "cp-hsv", Color: RGB(0, 128, 255), ShowHSV: true,
	})
	hsl := countInputs(ColorPickerCfg{
		ID: "cp-hsl", Color: RGB(0, 128, 255), ShowHSL: true,
	})
	if hsv <= base {
		t.Errorf("ShowHSV added no row: %d vs %d", hsv, base)
	}
	if hsl != hsv {
		t.Errorf("ShowHSL = %d, ShowHSV = %d; want the same row",
			hsl, hsv)
	}
}

// FocusDisabled must reach the focusable parts of the composition — the
// plane and the two sliders — or a picker opted out of focus still joins
// the tab order three times over. The text fields' inputs stay focusable:
// they always were, under the old picker too.
func TestColorPickerFocusDisabledPropagates(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorPicker(ColorPickerCfg{
		ID: "cp-fd", Color: RGB(0, 128, 255), FocusDisabled: true,
	}), w)

	for _, id := range []string{"cp-fd:plane", "cp-fd:hue", "cp-fd:alpha"} {
		if s := findShapeByID(&l, id); s == nil {
			t.Fatalf("missing shape %q", id)
		} else if s.Shape.Focusable {
			t.Errorf("%q still focusable under FocusDisabled", id)
		}
	}
}

func TestColorPickerDefaults(t *testing.T) {
	cfg := ColorPickerCfg{}
	applyColorPickerDefaults(&cfg)
	if !cfg.Color.IsSet() {
		t.Error("Color should default to Red")
	}
	if cfg.Style.sVSize == 0 {
		t.Error("SVSize should be set")
	}
	if cfg.Style.sliderHeight == 0 {
		t.Error("SliderHeight should be set")
	}
	if cfg.Style.indicatorSize == 0 {
		t.Error("IndicatorSize should be set")
	}
}

func TestColorPickerDefaultColor(t *testing.T) {
	cfg := ColorPickerCfg{}
	applyColorPickerDefaults(&cfg)
	if cfg.Color != Red {
		t.Errorf("default color = %v, want Red", cfg.Color)
	}
}

func TestColorPickerStateInit(t *testing.T) {
	w := &Window{}
	c := RGB(255, 0, 0) // pure red
	generateViewLayout(
		ColorPicker(ColorPickerCfg{ID: "cp-state", Color: c}), w)

	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	st, ok := sm.Get("cp-state")
	if !ok {
		t.Fatal("state should be initialized")
	}
	// Pure red → H≈0, S≈1, L≈0.5 in HSL.
	if st.hsla.S < 0.99 || f32Abs(st.hsla.L-0.5) > 0.01 {
		t.Errorf("HSLA = %+v", st.hsla)
	}
}

// The picker's whole reason for holding state: a value the user drove
// to a gray or to black must not lose the hue it was on, even though
// the caller only ever sees a Color.
func TestCpValuePreservesHueThroughRoundTrip(t *testing.T) {
	w := &Window{}
	const id = "cp-hue"

	// Start on cyan, then drag lightness to zero.
	cpStore(w, id, HSLA{H: 180, S: 1, L: 0.5, A: 1})
	black := HSLA{H: 180, S: 1, L: 0, A: 1}
	cpStore(w, id, black)

	// The caller hands back exactly the Color the picker produced —
	// a round trip, so the stored hue must survive.
	got := cpValue(w, id, black.Color())
	if got.H != 180 {
		t.Errorf("H = %v after round trip, want 180 preserved", got.H)
	}
}

// A Color the picker did not produce is the caller setting the value,
// and it must win over the stored one.
func TestCpValueCallerSetColorWins(t *testing.T) {
	w := &Window{}
	const id = "cp-set"

	cpStore(w, id, HSLA{H: 180, S: 1, L: 0.5, A: 1})
	got := cpValue(w, id, RGB(255, 0, 0)) // caller assigns red
	if got.H != 0 || got.S < 0.99 {
		t.Errorf("got %+v, want the caller's red", got)
	}
}

func TestHSVRoundTrip(t *testing.T) {
	colors := []Color{
		RGB(255, 0, 0),
		RGB(0, 255, 0),
		RGB(0, 0, 255),
		RGB(255, 255, 0),
		RGB(0, 255, 255),
		RGB(255, 0, 255),
		RGB(128, 64, 32),
		RGB(0, 0, 0),
		RGB(255, 255, 255),
	}
	for _, c := range colors {
		h, s, v := c.ToHSV()
		got := colorFromHSVA(h, s, v, c.A)
		if got.R != c.R || got.G != c.G || got.B != c.B {
			t.Errorf("round-trip %v: HSV(%f,%f,%f) → %v",
				c, h, s, v, got)
		}
	}
}

// TestColorPickerPlaneFitsFields asserts the derived plane size makes
// the plane-plus-sliders row exactly as wide as the RGBA fields row.
// Any surplus renders as empty space to the right of the alpha input.
func TestColorPickerPlaneFitsFields(t *testing.T) {
	cfg := ColorPickerCfg{}
	applyColorPickerDefaults(&cfg)
	style := cfg.Style

	size := colorPickerPlaneSize(style)
	if size > style.sVSize {
		t.Errorf("plane = %v, must not exceed sVSize %v",
			size, style.sVSize)
	}

	thick := f32Max(style.sliderHeight, style.indicatorSize)
	top := size + 2*(thick+colorPickerPlaneGap)
	fields := colorFieldsBlockWidth()
	if top != fields {
		t.Errorf("top row = %v, fields row = %v; want equal",
			top, fields)
	}
}

// A bordered theme gives every container a border unless it opts out.
// The picker's rows are structural, so a border there would widen the
// fields block past the plane row and offset it — the two must stay the
// same width and start at the same x under either theme.
func TestColorPickerRowsAlignUnderBorders(t *testing.T) {
	for _, borders := range []bool{false, true} {
		w := &Window{}
		w.SetTheme(ThemeDark.WithBorders(borders))
		// A real measurer is needed: the labels above each field are
		// zero-width without one, so the defect would not show.
		w.textMeasurer = &stubTextMeasurer{charWidth: 7, fontHeight: 16}
		l := w.TestRender(func(*Window) View {
			return ColorPicker(ColorPickerCfg{ID: "p", ShowHSL: true})
		})

		fields := findShapeByID(l, "p:fields")
		plane := findShapeByID(l, "p:plane")
		alpha := findShapeByID(l, "p:alpha")
		if fields == nil || plane == nil || alpha == nil {
			t.Fatalf("borders=%v: missing shapes", borders)
		}
		if fields.Shape.X != plane.Shape.X {
			t.Errorf("borders=%v: fields x = %v, plane x = %v",
				borders, fields.Shape.X, plane.Shape.X)
		}
		// The plane row ends at the alpha slider's right edge.
		rowEnd := alpha.Shape.X + alpha.Shape.Width
		if got := fields.Shape.X + fields.Shape.Width; got != rowEnd {
			t.Errorf("borders=%v: fields right = %v, plane row = %v",
				borders, got, rowEnd)
		}
	}
}
