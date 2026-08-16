package gui

import (
	"slices"
	"testing"
)

// shapeIDs collects every ID in a generated tree, so a component's
// composed identities can be asserted without reaching into layout
// internals at each call site.
func shapeIDs(l *Layout) []string {
	var out []string
	var walk func(*Layout)
	walk = func(n *Layout) {
		if n.Shape != nil && n.Shape.ID != "" {
			out = append(out, n.Shape.ID)
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(l)
	return out
}

func hasID(ids []string, want string) bool {
	return slices.Contains(ids, want)
}

func TestColorPlaneLayout(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorPlane(ColorPlaneCfg{
		ID:    "plane",
		Value: HSLA{H: 210, S: 0.7, L: 0.55, A: 1},
		Size:  100,
	}), w)

	if l.Shape.ID != "plane" {
		t.Errorf("ID = %q", l.Shape.ID)
	}
	if !l.Shape.Focusable {
		t.Error("plane should be focusable by default")
	}
	if l.Shape.Width != 100 || l.Shape.Height != 100 {
		t.Errorf("size = %vx%v, want 100x100",
			l.Shape.Width, l.Shape.Height)
	}
	// The plane's imagery must be a registered buffer, not a file.
	var img *Shape
	var walk func(*Layout)
	walk = func(n *Layout) {
		if n.Shape != nil && n.Shape.shapeType == shapeImage {
			img = n.Shape
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(&l)
	if img == nil {
		t.Fatal("no image shape in the plane")
	}
	if !isMemImage(img.Resource) {
		t.Errorf("Resource = %q, want a mem: source", img.Resource)
	}
}

func TestColorPlaneFocusDisabled(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorPlane(ColorPlaneCfg{
		ID: "plane-nf", Size: 50, FocusDisabled: true,
	}), w)
	if l.Shape.Focusable {
		t.Error("FocusDisabled should opt out of focus")
	}
}

func TestColorWheelLayout(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorWheel(ColorWheelCfg{
		ID:    "wheel",
		Value: HSLA{H: 90, S: 0.5, L: 0.5, A: 1},
		Size:  120,
	}), w)
	if l.Shape.ID != "wheel" {
		t.Errorf("ID = %q", l.Shape.ID)
	}
	if l.Shape.Width != 120 {
		t.Errorf("width = %v, want 120", l.Shape.Width)
	}
}

func TestColorChannelSliderLayout(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(
		ColorChannelSlider(ColorChannelSliderCfg{
			ID:      "hue",
			Channel: ChannelHue,
			Value:   HSLA{H: 10, S: 1, L: 0.5, A: 1},
			Width:   200,
		}), w)
	if l.Shape.ID != "hue" {
		t.Errorf("ID = %q", l.Shape.ID)
	}
	if l.Shape.Width != 200 {
		t.Errorf("width = %v, want 200", l.Shape.Width)
	}
	if l.Shape.A11YRole != AccessRoleSlider {
		t.Errorf("role = %v, want slider", l.Shape.A11YRole)
	}
}

// A vertical slider swaps its axes: Height is the length, and the
// control is only as wide as its thumb.
func TestColorChannelSliderVerticalLayout(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(
		ColorChannelSlider(ColorChannelSliderCfg{
			ID:          "hue",
			Channel:     ChannelHue,
			Value:       HSLA{H: 10, S: 1, L: 0.5, A: 1},
			Vertical:    true,
			Height:      200,
			ThumbSize:   16,
			TrackHeight: 10,
		}), w)
	if l.Shape.Height != 200 {
		t.Errorf("height = %v, want 200", l.Shape.Height)
	}
	// Thickness is the larger of thumb and track, as it is horizontally.
	if l.Shape.Width != 16 {
		t.Errorf("width = %v, want 16 (thumb)", l.Shape.Width)
	}
}

// The vertical strip runs bottom to top: the minimum belongs at the
// bottom of the screen, where the thumb sits at value 0.
func TestVerticalStripSweepsUpward(t *testing.T) {
	const w, h, inset = 8, 64, 4
	v := HSLA{H: 0, S: 1, L: 0.5, A: 1}
	pix := channelStripPixels(ChannelLightness, v, w, h, inset, true)
	at := func(y int) uint8 { return pix[(y*w)*4] } // red of column 0

	if top, bot := at(0), at(h-1); top <= bot {
		t.Errorf("top=%d bottom=%d: lightness must increase upward",
			top, bot)
	}
}

// A vertical strip is a different buffer from a horizontal one of the
// same dimensions, or one orientation would draw the other's pixels.
func TestStripKeyIncludesOrientation(t *testing.T) {
	resetMemImages(t)
	v := HSLA{H: 120, S: 0.5, L: 0.5, A: 1}
	horiz := channelStripSrc(ChannelHue, v, 32, 32, 4, false)
	vert := channelStripSrc(ChannelHue, v, 32, 32, 4, true)
	if horiz == vert {
		t.Error("orientation missing from the strip's content key")
	}
}

func TestColorSwatchLayout(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorSwatch(ColorSwatchCfg{
		ID:     "swatch",
		Color:  RGBA(255, 0, 0, 128),
		Width:  40,
		Height: 20,
	}), w)
	if l.Shape.ID != "swatch" {
		t.Errorf("ID = %q", l.Shape.ID)
	}
	// The checkerboard is the point: a half-transparent color must
	// sit over a generated pattern, not over the theme background.
	found := false
	var walk func(*Layout)
	walk = func(n *Layout) {
		if n.Shape != nil && n.Shape.shapeType == shapeImage &&
			isMemImage(n.Shape.Resource) {
			found = true
		}
		for i := range n.Children {
			walk(&n.Children[i])
		}
	}
	walk(&l)
	if !found {
		t.Error("swatch has no checkerboard buffer")
	}
}

// Inner IDs must be composed under the component's own ID, or two
// pickers on one screen collide.
func TestColorFieldsComposesInnerIDs(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorFields(ColorFieldsCfg{
		ID:      "fields",
		Value:   HSLA{H: 0, S: 1, L: 0.5, A: 1},
		ShowHSL: true,
	}), w)
	ids := shapeIDs(&l)
	for _, want := range []string{
		"fields", "fields:hex", "fields:rgb:0", "fields:hsl:2",
	} {
		if !hasID(ids, want) {
			t.Errorf("missing composed ID %q; got %v", want, ids)
		}
	}
}

// The swatch rides beside the hex value, so it must appear under the
// fields' own scope rather than the caller's.
func TestColorFieldsShowSwatch(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorFields(ColorFieldsCfg{
		ID:         "fields",
		Value:      HSLA{H: 0, S: 1, L: 0.5, A: 1},
		ShowSwatch: true,
	}), w)
	if ids := shapeIDs(&l); !hasID(ids, "fields:swatch") {
		t.Errorf("ShowSwatch rendered no swatch; got %v", ids)
	}
}

func TestColorFieldsHideHexAndHSL(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorFields(ColorFieldsCfg{
		ID:      "f2",
		Value:   HSLA{H: 0, S: 1, L: 0.5, A: 1},
		HideHex: true,
	}), w)
	ids := shapeIDs(&l)
	if hasID(ids, "f2:hex") {
		t.Error("HideHex still rendered the hex field")
	}
	if hasID(ids, "f2:hsl:0") {
		t.Error("HSL row rendered without ShowHSL")
	}
}

// The composed picker must produce unique effective IDs — the whole
// reason inner IDs are scoped.
func TestColorPickerNoDuplicateIDs(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(ColorPicker(ColorPickerCfg{
		ID: "cp-dup", Color: RGB(0, 128, 255), ShowHSL: true,
	}), w)
	seen := map[string]bool{}
	for _, id := range shapeIDs(&l) {
		if seen[id] {
			t.Errorf("duplicate ID %q", id)
		}
		seen[id] = true
	}
}

// The thumb must select the color under its centre, which is what the
// inset mapping buys.
func TestTrackFraction(t *testing.T) {
	const width, inset = 100, 10
	cases := []struct {
		nx   float32
		want float32
	}{
		{0, 0},     // left edge pins to the minimum
		{0.1, 0},   // start of the track
		{0.5, 0.5}, // centre
		{0.9, 1},   // end of the track
		{1, 1},     // right edge pins to the maximum
	}
	for _, tc := range cases {
		got := trackFraction(tc.nx, width, inset)
		if f32Abs(got-tc.want) > 0.001 {
			t.Errorf("trackFraction(%v) = %v, want %v",
				tc.nx, got, tc.want)
		}
	}
	// A control narrower than its thumb must not divide by zero.
	if got := trackFraction(0.5, 10, 20); got != 0.5 {
		t.Errorf("degenerate track = %v, want the raw fraction", got)
	}
}

func TestColorKeyDelta(t *testing.T) {
	cases := []struct {
		key          KeyCode
		mods         Modifier
		wantX, wantY float32
		wantOK       bool
	}{
		{KeyLeft, 0, -colorKeyStep, 0, true},
		{KeyRight, 0, colorKeyStep, 0, true},
		{KeyUp, 0, 0, -colorKeyStep, true},
		{KeyDown, 0, 0, colorKeyStep, true},
		{KeyRight, ModShift, colorKeyStep * 10, 0, true},
		{KeyTab, 0, 0, 0, false},
	}
	for _, tc := range cases {
		dx, dy, ok := colorKeyDelta(
			&Event{KeyCode: tc.key, Modifiers: tc.mods})
		if ok != tc.wantOK {
			t.Errorf("key %v: ok = %v, want %v", tc.key, ok, tc.wantOK)
			continue
		}
		if f32Abs(dx-tc.wantX) > 1e-6 || f32Abs(dy-tc.wantY) > 1e-6 {
			t.Errorf("key %v: delta = %v,%v want %v,%v",
				tc.key, dx, dy, tc.wantX, tc.wantY)
		}
	}
	if _, _, ok := colorKeyDelta(nil); ok {
		t.Error("nil event should not produce a delta")
	}
}

// A key the control does not handle must travel on: consuming it would
// swallow Tab and trap focus inside the control.
func TestColorKeysIgnoreUnrelatedKeys(t *testing.T) {
	v := HSLA{H: 100, S: 0.5, L: 0.5, A: 1}
	called := false
	onChange := func(HSLA, EventCtx) { called = true }

	handlers := []func(EventCtx){
		colorPlaneKeys(v, onChange),
		colorWheelKeys(v, onChange),
		colorChannelKeys(ChannelHue, v, onChange),
	}
	for i, h := range handlers {
		e := &Event{KeyCode: KeyTab}
		h(EventCtx{nil, e, nil})
		if called {
			t.Errorf("handler %d acted on Tab", i)
		}
		if e.IsHandled {
			t.Errorf("handler %d consumed Tab", i)
		}
	}
}

func TestColorPlaneKeysMoveSaturationAndLightness(t *testing.T) {
	v := HSLA{H: 100, S: 0.5, L: 0.5, A: 1}
	var got HSLA
	h := colorPlaneKeys(v, func(next HSLA, _ EventCtx) { got = next })

	e := &Event{KeyCode: KeyRight}
	h(EventCtx{nil, e, nil})
	if got.S <= v.S {
		t.Errorf("right arrow: S = %v, want > %v", got.S, v.S)
	}
	if !e.IsHandled {
		t.Error("a handler that acted must consume")
	}

	// Up increases lightness: the plane paints light at the top.
	h(EventCtx{nil, &Event{KeyCode: KeyUp}, nil})
	if got.L <= v.L {
		t.Errorf("up arrow: L = %v, want > %v", got.L, v.L)
	}
}

func TestColorChannelKeysHueWraps(t *testing.T) {
	v := HSLA{H: 359, S: 1, L: 0.5, A: 1}
	var got HSLA
	h := colorChannelKeys(ChannelHue, v,
		func(next HSLA, _ EventCtx) { got = next })
	// Shift steps 10% of 360 = 36°, past the end of the strip.
	h(EventCtx{nil, &Event{
		KeyCode: KeyRight, Modifiers: ModShift,
	}, nil})
	if got.H > 359 {
		t.Errorf("H = %v, want wrapped below 359", got.H)
	}
}

func TestColorChannelKeysClampNonHue(t *testing.T) {
	v := HSLA{H: 0, S: 0.99, L: 0.5, A: 1}
	var got HSLA
	h := colorChannelKeys(ChannelSaturation, v,
		func(next HSLA, _ EventCtx) { got = next })
	h(EventCtx{nil, &Event{
		KeyCode: KeyRight, Modifiers: ModShift,
	}, nil})
	if got.S != 1 {
		t.Errorf("S = %v, want clamped to 1", got.S)
	}
}

// A nil OnChange must be inert rather than panic: a read-only display
// of a color is a legitimate use.
func TestColorComponentsNilOnChange(t *testing.T) {
	w := &Window{}
	generateViewLayout(ColorPlane(ColorPlaneCfg{ID: "p", Size: 20}), w)
	generateViewLayout(ColorWheel(ColorWheelCfg{ID: "w", Size: 20}), w)
	generateViewLayout(ColorChannelSlider(ColorChannelSliderCfg{
		ID: "s", Width: 40,
	}), w)

	for _, h := range []func(EventCtx){
		colorPlaneKeys(HSLA{}, nil),
		colorWheelKeys(HSLA{}, nil),
		colorChannelKeys(ChannelHue, HSLA{}, nil),
	} {
		h(EventCtx{nil, &Event{KeyCode: KeyRight}, nil})
	}
}

// The hex field is Fit-sized, so without a floor it would track its
// own text and the swatch beside it would shift as the value changed
// length.
func TestColorFieldsHexMinWidth(t *testing.T) {
	w := &Window{}
	for _, v := range []HSLA{
		{H: 0, S: 0, L: 1, A: 1},           // "#FFFFFF", short form
		{H: 210, S: 0.7, L: 0.55, A: 0.85}, // "#3C8CDDD9", long form
	} {
		l := generateViewLayout(ColorFields(ColorFieldsCfg{
			ID:         "hexw",
			Value:      v,
			ShowSwatch: true,
		}), w)
		found := findShapeByID(&l, "hexw:hex")
		if found == nil {
			t.Fatalf("no hex field for %v", v)
		}
		s := found.Shape
		if s.MinWidth != defaultHexFieldWidth {
			t.Errorf("hex MinWidth = %v, want %v",
				s.MinWidth, defaultHexFieldWidth)
		}
	}
}
