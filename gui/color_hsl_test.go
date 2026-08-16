package gui

import "testing"

func TestHSLAColorPrimaries(t *testing.T) {
	cases := []struct {
		name string
		in   HSLA
		want Color
	}{
		{"red", HSLA{0, 1, 0.5, 1}, RGBA(255, 0, 0, 255)},
		{"yellow", HSLA{60, 1, 0.5, 1}, RGBA(255, 255, 0, 255)},
		{"green", HSLA{120, 1, 0.5, 1}, RGBA(0, 255, 0, 255)},
		{"cyan", HSLA{180, 1, 0.5, 1}, RGBA(0, 255, 255, 255)},
		{"blue", HSLA{240, 1, 0.5, 1}, RGBA(0, 0, 255, 255)},
		{"magenta", HSLA{300, 1, 0.5, 1}, RGBA(255, 0, 255, 255)},
		{"black", HSLA{0, 1, 0, 1}, RGBA(0, 0, 0, 255)},
		{"white", HSLA{0, 1, 1, 1}, RGBA(255, 255, 255, 255)},
		{"mid gray", HSLA{0, 0, 0.5, 1}, RGBA(128, 128, 128, 255)},
		{"half alpha", HSLA{0, 1, 0.5, 0.5}, RGBA(255, 0, 0, 128)},
	}
	for _, tc := range cases {
		got := tc.in.Color()
		if got != tc.want {
			t.Errorf("%s: Color() = %v, want %v",
				tc.name, got, tc.want)
		}
	}
}

// Hue must wrap and S/L/A must clamp, because pointer-derived values
// arrive raw from a drag that ran past the edge of the control.
func TestHSLANormalized(t *testing.T) {
	got := HSLA{H: -30, S: 1.4, L: -0.2, A: 2}.Normalized()
	want := HSLA{H: 330, S: 1, L: 0, A: 1}
	if got != want {
		t.Errorf("Normalized() = %+v, want %+v", got, want)
	}
	if got := (HSLA{H: 720}).Normalized().H; got != 0 {
		t.Errorf("720° wrapped to %v, want 0", got)
	}
}

func TestColorToHSLARoundTrip(t *testing.T) {
	cases := []HSLA{
		{0, 1, 0.5, 1},
		{210, 0.7, 0.55, 0.85},
		{45, 0.3, 0.2, 1},
		{300, 1, 0.75, 0.5},
	}
	for _, want := range cases {
		got := ColorToHSLA(want.Color())
		// 8-bit RGB quantization bounds the achievable precision;
		// a degree of hue and a percent of S/L is well inside it.
		if f32Abs(got.H-want.H) > 1 {
			t.Errorf("%v: H = %v", want, got.H)
		}
		if f32Abs(got.S-want.S) > 0.01 {
			t.Errorf("%v: S = %v", want, got.S)
		}
		if f32Abs(got.L-want.L) > 0.01 {
			t.Errorf("%v: L = %v", want, got.L)
		}
		if f32Abs(got.A-want.A) > 0.01 {
			t.Errorf("%v: A = %v", want, got.A)
		}
	}
}

// The degenerate points are exactly where a picker that round-trips
// through Color loses its hue — assert the conversion is at least
// well defined there.
func TestColorToHSLADegenerate(t *testing.T) {
	cases := []struct {
		name    string
		in      Color
		wantS   float32
		wantL   float32
		wantHue float32
	}{
		{"black", RGBA(0, 0, 0, 255), 0, 0, 0},
		{"white", RGBA(255, 255, 255, 255), 0, 1, 0},
		{"gray", RGBA(128, 128, 128, 255), 0, 0.502, 0},
	}
	for _, tc := range cases {
		got := ColorToHSLA(tc.in)
		if f32Abs(got.S-tc.wantS) > 0.001 ||
			f32Abs(got.L-tc.wantL) > 0.005 ||
			got.H != tc.wantHue {
			t.Errorf("%s: got %+v", tc.name, got)
		}
	}
}

func TestHSLAString(t *testing.T) {
	got := HSLA{210, 0.7, 0.55, 0.85}.String()
	want := "hsla(210, 70%, 55%, 0.85)"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func TestColorHexExports(t *testing.T) {
	if got := RGBA(0x3b, 0x8c, 0xdc, 255).Hex(); got != "#3B8CDC" {
		t.Errorf("Hex() = %q", got)
	}
	if got := RGBA(0x3b, 0x8c, 0xdc, 128).Hex(); got != "#3B8CDC80" {
		t.Errorf("Hex() with alpha = %q", got)
	}
	c, ok := ColorFromHex("#3b8cdc")
	if !ok || c != RGBA(0x3b, 0x8c, 0xdc, 255) {
		t.Errorf("ColorFromHex = %v, %v", c, ok)
	}
	if _, ok := ColorFromHex("#nothex"); ok {
		t.Error("ColorFromHex accepted malformed input")
	}
}

// The components convert on every frame of a drag, so the hot
// conversions must stay off the heap.
func TestHSLAConversionsDoNotAllocate(t *testing.T) {
	v := HSLA{210, 0.7, 0.55, 0.85}
	if got := testing.AllocsPerRun(100, func() {
		_ = ColorToHSLA(v.Color())
	}); got != 0 {
		t.Errorf("allocs = %v, want 0", got)
	}
}
