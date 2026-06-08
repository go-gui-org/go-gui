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

func TestColorPickerLayoutWithHSV(t *testing.T) {
	w := &Window{}
	v := ColorPicker(ColorPickerCfg{
		ID:      "cp-hsv",
		Color:   RGB(0, 128, 255),
		ShowHSV: true,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "cp-hsv" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
}

func TestColorPickerDefaults(t *testing.T) {
	cfg := ColorPickerCfg{}
	applyColorPickerDefaults(&cfg)
	if !cfg.Color.IsSet() {
		t.Error("Color should default to Red")
	}
	if cfg.Style.SVSize == 0 {
		t.Error("SVSize should be set")
	}
	if cfg.Style.SliderHeight == 0 {
		t.Error("SliderHeight should be set")
	}
	if cfg.Style.IndicatorSize == 0 {
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
	v := ColorPicker(ColorPickerCfg{ID: "cp-state", Color: c})
	generateViewLayout(v, w)

	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	hsv, ok := sm.Get("cp-state")
	if !ok {
		t.Fatal("state should be initialized")
	}
	// Pure red → H≈0, S≈1, V≈1.
	if hsv.S < 0.99 || hsv.V < 0.99 {
		t.Errorf("HSV = %v %v %v", hsv.H, hsv.S, hsv.V)
	}
}

func TestCpParseUint8(t *testing.T) {
	tests := []struct {
		input string
		want  uint8
		ok    bool
	}{
		{"0", 0, true},
		{"255", 255, true},
		{"128", 128, true},
		{"-1", 0, false},
		{"256", 0, false},
		{"abc", 0, false},
		{"", 0, false},
	}
	for _, tt := range tests {
		got, ok := cpParseUint8(tt.input)
		if ok != tt.ok || (ok && got != tt.want) {
			t.Errorf("cpParseUint8(%q) = %d, %v; want %d, %v",
				tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestCpAmendSVIndicator(t *testing.T) {
	parent := &Layout{
		Shape: &Shape{X: 10, Y: 20, Width: 200, Height: 200},
	}
	child := &Layout{Shape: &Shape{}, Parent: parent}
	hsv := colorPickerState{H: 0, S: 0.5, V: 0.75}
	cpAmendSVIndicator(child, hsv, 200, 12)

	wantX := float32(10 + 0.5*200 - 6)
	wantY := float32(20 + (1-0.75)*200 - 6)
	if child.Shape.X != wantX {
		t.Errorf("X = %f, want %f", child.Shape.X, wantX)
	}
	if child.Shape.Y != wantY {
		t.Errorf("Y = %f, want %f", child.Shape.Y, wantY)
	}
}

func TestCpAmendSVIndicatorNoParent(t *testing.T) {
	child := &Layout{Shape: &Shape{X: 5, Y: 5}}
	hsv := colorPickerState{H: 0, S: 0.5, V: 0.5}
	cpAmendSVIndicator(child, hsv, 200, 12)
	// Should not modify when no parent.
	if child.Shape.X != 5 || child.Shape.Y != 5 {
		t.Error("should not modify layout without parent")
	}
}

func TestCpAmendHueIndicator(t *testing.T) {
	parent := &Layout{
		Shape: &Shape{X: 10, Y: 20, Width: 30, Height: 200},
	}
	child := &Layout{Shape: &Shape{}, Parent: parent}
	hsv := colorPickerState{H: 180, S: 1, V: 1}
	cpAmendHueIndicator(child, hsv, 200, 12)

	wantX := float32(10 + 30.0/2 - 6)
	wantY := float32(20 + (180.0/360)*200 - 6)
	if child.Shape.X != wantX {
		t.Errorf("X = %f, want %f", child.Shape.X, wantX)
	}
	if child.Shape.Y != wantY {
		t.Errorf("Y = %f, want %f", child.Shape.Y, wantY)
	}
}

func TestCpAmendHueIndicatorNoParent(t *testing.T) {
	child := &Layout{Shape: &Shape{X: 5, Y: 5}}
	hsv := colorPickerState{H: 90, S: 1, V: 1}
	cpAmendHueIndicator(child, hsv, 200, 12)
	if child.Shape.X != 5 || child.Shape.Y != 5 {
		t.Error("should not modify layout without parent")
	}
}

func TestCpSVMouseAction(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set("sv-test", colorPickerState{H: 120, S: 0, V: 0})

	var gotColor Color
	onChange := func(c Color, _ *Event, _ *Window) { gotColor = c }

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 100},
	}
	e := &Event{MouseX: 50, MouseY: 25}
	cpSVMouseAction("sv-test", RGB(0, 0, 0), onChange, layout.Shape, e, w)

	hsv, _ := sm.Get("sv-test")
	if hsv.S < 0.49 || hsv.S > 0.51 {
		t.Errorf("S = %f, want ~0.5", hsv.S)
	}
	if hsv.V < 0.74 || hsv.V > 0.76 {
		t.Errorf("V = %f, want ~0.75", hsv.V)
	}
	if !gotColor.IsSet() {
		t.Error("onChange should be called")
	}
}

func TestCpSVMouseActionWithOffset(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set("sv-off", colorPickerState{H: 120, S: 0, V: 0})

	var gotColor Color
	onChange := func(c Color, _ *Event, _ *Window) { gotColor = c }

	shape := &Shape{X: 100, Y: 200, Width: 100, Height: 100}
	e := &Event{MouseX: 150, MouseY: 225}
	cpSVMouseAction("sv-off", RGB(0, 0, 0), onChange, shape, e, w)

	hsv, _ := sm.Get("sv-off")
	if hsv.S < 0.49 || hsv.S > 0.51 {
		t.Errorf("S = %f, want ~0.5", hsv.S)
	}
	if hsv.V < 0.74 || hsv.V > 0.76 {
		t.Errorf("V = %f, want ~0.75", hsv.V)
	}
	if !gotColor.IsSet() {
		t.Error("onChange should be called")
	}
}

func TestCpSVMouseActionNilOnChange(_ *testing.T) {
	w := &Window{}
	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 100},
	}
	e := &Event{MouseX: 50, MouseY: 50}
	// Should not panic.
	cpSVMouseAction("nil-test", RGB(0, 0, 0), nil, layout.Shape, e, w)
}

func TestCpHueMouseAction(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set("hue-test", colorPickerState{H: 0, S: 1, V: 1})

	var gotColor Color
	onChange := func(c Color, _ *Event, _ *Window) { gotColor = c }

	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 30, Height: 360},
	}
	e := &Event{MouseX: 15, MouseY: 180}
	cpHueMouseAction("hue-test", RGB(255, 0, 0), onChange, layout.Shape, e, w)

	hsv, _ := sm.Get("hue-test")
	if hsv.H < 179 || hsv.H > 181 {
		t.Errorf("H = %f, want ~180", hsv.H)
	}
	if !gotColor.IsSet() {
		t.Error("onChange should be called")
	}
}

func TestCpHueMouseActionNilOnChange(_ *testing.T) {
	w := &Window{}
	layout := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 30, Height: 360},
	}
	e := &Event{MouseX: 15, MouseY: 180}
	cpHueMouseAction("nil-test", RGB(255, 0, 0), nil, layout.Shape, e, w)
}

func TestCpSVMouseActionZeroSize(t *testing.T) {
	w := &Window{}
	called := false
	onChange := func(_ Color, _ *Event, _ *Window) { called = true }
	shape := &Shape{X: 0, Y: 0, Width: 0, Height: 0}
	e := &Event{MouseX: 50, MouseY: 50}
	cpSVMouseAction("zero", RGB(0, 0, 0), onChange, shape, e, w)
	if called {
		t.Error("onChange should not be called with zero-size shape")
	}
}

func TestCpHueMouseActionZeroHeight(t *testing.T) {
	w := &Window{}
	called := false
	onChange := func(_ Color, _ *Event, _ *Window) { called = true }
	shape := &Shape{X: 0, Y: 0, Width: 30, Height: 0}
	e := &Event{MouseX: 15, MouseY: 50}
	cpHueMouseAction("zero", RGB(255, 0, 0), onChange, shape, e, w)
	if called {
		t.Error("onChange should not be called with zero-height shape")
	}
}

func TestCpApplyRGB(t *testing.T) {
	w := &Window{}
	var got Color
	onChange := func(c Color, _ *Event, _ *Window) { got = c }
	cpApplyRGB("128", 0, RGB(0, 50, 100), "rgb-test", onChange, w)
	if got.R != 128 || got.G != 50 || got.B != 100 {
		t.Errorf("color = %v, want R=128 G=50 B=100", got)
	}
	// Verify HSV state updated.
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	if _, ok := sm.Get("rgb-test"); !ok {
		t.Error("HSV state should be set")
	}
}

func TestCpApplyRGBInvalid(t *testing.T) {
	w := &Window{}
	called := false
	onChange := func(_ Color, _ *Event, _ *Window) { called = true }
	cpApplyRGB("abc", 0, RGB(0, 0, 0), "inv", onChange, w)
	if called {
		t.Error("onChange should not be called for invalid input")
	}
}

func TestCpApplyHSV(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set("hsv-test", colorPickerState{H: 0, S: 1, V: 1})

	var got Color
	onChange := func(c Color, _ *Event, _ *Window) { got = c }
	cpApplyHSV("180", 0, 360, "hsv-test", 255, onChange, w)
	hsv, _ := sm.Get("hsv-test")
	if hsv.H != 180 {
		t.Errorf("H = %f, want 180", hsv.H)
	}
	if !got.IsSet() {
		t.Error("onChange should be called")
	}
}

func TestCpApplyHSVClamp(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	sm.Set("hsv-clamp", colorPickerState{H: 0, S: 1, V: 1})

	var got Color
	onChange := func(c Color, _ *Event, _ *Window) { got = c }
	cpApplyHSV("-10", 0, 360, "hsv-clamp", 255, onChange, w)
	hsv, _ := sm.Get("hsv-clamp")
	if hsv.H != 0 {
		t.Errorf("H = %f, want 0 (clamped)", hsv.H)
	}
	if !got.IsSet() {
		t.Error("onChange should be called")
	}
}

func TestCpApplyHex(t *testing.T) {
	w := &Window{}
	var got Color
	onChange := func(c Color, _ *Event, _ *Window) { got = c }
	cpApplyHex("#FF8000", "hex-test", onChange, w)
	if got.R != 255 || got.G != 128 || got.B != 0 {
		t.Errorf("color = %v, want R=255 G=128 B=0", got)
	}
	sm := StateMap[string, colorPickerState](
		w, nsColorPicker, capModerate)
	if _, ok := sm.Get("hex-test"); !ok {
		t.Error("HSV state should be set")
	}
}

func TestCpApplyHexInvalid(t *testing.T) {
	w := &Window{}
	called := false
	onChange := func(_ Color, _ *Event, _ *Window) { called = true }
	cpApplyHex("not-hex", "inv", onChange, w)
	if called {
		t.Error("onChange should not be called for invalid hex")
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
		got := ColorFromHSVA(h, s, v, c.A)
		if got.R != c.R || got.G != c.G || got.B != c.B {
			t.Errorf("round-trip %v: HSV(%f,%f,%f) → %v",
				c, h, s, v, got)
		}
	}
}
