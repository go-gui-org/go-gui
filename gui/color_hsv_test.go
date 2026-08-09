package gui

import (
	"math"
	"testing"
)

func TestToHSVPrimaries(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		wantH float32
		wantS float32
		wantV float32
	}{
		{"red", Color{255, 0, 0, 255, true}, 0, 1, 1},
		{"green", Color{0, 255, 0, 255, true}, 120, 1, 1},
		{"blue", Color{0, 0, 255, 255, true}, 240, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, v := tt.color.ToHSV()
			if !closeEnough(h, tt.wantH) || !closeEnough(s, tt.wantS) || !closeEnough(v, tt.wantV) {
				t.Errorf("ToHSV() = (%f,%f,%f), want (%f,%f,%f)",
					h, s, v, tt.wantH, tt.wantS, tt.wantV)
			}
		})
	}
}

func TestToHSVBlackWhiteGray(t *testing.T) {
	tests := []struct {
		name  string
		color Color
		wantH float32
		wantS float32
		wantV float32
	}{
		{"black", Color{0, 0, 0, 255, true}, 0, 0, 0},
		{"white", Color{255, 255, 255, 255, true}, 0, 0, 1},
		{"gray", Color{128, 128, 128, 255, true}, 0, 0, 128.0 / 255.0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, s, v := tt.color.ToHSV()
			if !closeEnough(h, tt.wantH) || !closeEnough(s, tt.wantS) || !closeEnough(v, tt.wantV) {
				t.Errorf("ToHSV() = (%f,%f,%f), want (%f,%f,%f)",
					h, s, v, tt.wantH, tt.wantS, tt.wantV)
			}
		})
	}
}

func TestColorFromHSVRoundTrip(t *testing.T) {
	colors := []Color{
		{255, 0, 0, 255, true},
		{0, 255, 0, 255, true},
		{0, 0, 255, 255, true},
		{255, 255, 0, 255, true},
		{0, 255, 255, 255, true},
		{255, 0, 255, 255, true},
		{128, 64, 32, 255, true},
	}
	for _, c := range colors {
		h, s, v := c.ToHSV()
		got := ColorFromHSV(h, s, v)
		if absDiff(got.R, c.R) > 1 || absDiff(got.G, c.G) > 1 || absDiff(got.B, c.B) > 1 {
			t.Errorf("round-trip %v: got %v", c, got)
		}
	}
}

func TestColorFromHSVAAlpha(t *testing.T) {
	c := colorFromHSVA(0, 1, 1, 128)
	if c.A != 128 {
		t.Errorf("A = %d, want 128", c.A)
	}
}

func TestHueColorPrimaries(t *testing.T) {
	tests := []struct {
		hue  float32
		want Color
	}{
		{0, Color{255, 0, 0, 255, true}},
		{120, Color{0, 255, 0, 255, true}},
		{240, Color{0, 0, 255, 255, true}},
	}
	for _, tt := range tests {
		got := hueColor(tt.hue)
		if absDiff(got.R, tt.want.R) > 1 || absDiff(got.G, tt.want.G) > 1 || absDiff(got.B, tt.want.B) > 1 {
			t.Errorf("HueColor(%f) = %v, want %v", tt.hue, got, tt.want)
		}
	}
}

func TestToHexStringOpaque(t *testing.T) {
	c := Color{255, 0, 128, 255, true}
	if got := c.toHexString(); got != "#FF0080" {
		t.Errorf("got %q, want #FF0080", got)
	}
}

func TestToHexStringAlpha(t *testing.T) {
	c := Color{255, 0, 128, 200, true}
	if got := c.toHexString(); got != "#FF0080C8" {
		t.Errorf("got %q, want #FF0080C8", got)
	}
}

func TestColorFromHexStringValid(t *testing.T) {
	tests := []struct {
		input string
		want  Color
	}{
		{"#FF0000", Color{255, 0, 0, 255, true}},
		{"#ff0000", Color{255, 0, 0, 255, true}},
		{"FF0000", Color{255, 0, 0, 255, true}},
		{"#AABBCCDD", Color{170, 187, 204, 221, true}},
	}
	for _, tt := range tests {
		got, ok := colorFromHexString(tt.input)
		if !ok {
			t.Errorf("ColorFromHexString(%q) returned false", tt.input)
			continue
		}
		if got.R != tt.want.R || got.G != tt.want.G || got.B != tt.want.B || got.A != tt.want.A {
			t.Errorf("ColorFromHexString(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestColorFromHexStringInvalid(t *testing.T) {
	invalids := []string{"", "#GG0000", "#12345", "nope"}
	for _, s := range invalids {
		if _, ok := colorFromHexString(s); ok {
			t.Errorf("ColorFromHexString(%q) should return false", s)
		}
	}
}

func TestHexRoundTrip(t *testing.T) {
	colors := []Color{
		{255, 0, 0, 255, true},
		{0, 128, 255, 255, true},
		{10, 20, 30, 40, true},
	}
	for _, c := range colors {
		got, ok := colorFromHexString(c.toHexString())
		if !ok {
			t.Errorf("round-trip failed for %v", c)
			continue
		}
		if got.R != c.R || got.G != c.G || got.B != c.B || got.A != c.A {
			t.Errorf("round-trip %v: got %v", c, got)
		}
	}
}

func TestHexNibbleValid(t *testing.T) {
	tests := []struct {
		c    byte
		want uint8
	}{
		{'0', 0}, {'9', 9}, {'a', 10}, {'f', 15}, {'A', 10}, {'F', 15},
	}
	for _, tt := range tests {
		got, ok := hexNibble(tt.c)
		if !ok || got != tt.want {
			t.Errorf("hexNibble(%c) = (%d,%v), want (%d,true)", tt.c, got, ok, tt.want)
		}
	}
}

func TestHexNibbleInvalid(t *testing.T) {
	for _, c := range []byte{'g', 'G', ' ', 0xFF} {
		if _, ok := hexNibble(c); ok {
			t.Errorf("hexNibble(%c) should return false", c)
		}
	}
}

func TestHexPairValid(t *testing.T) {
	got, ok := hexPair('F', 'F')
	if !ok || got != 255 {
		t.Errorf("hexPair('F','F') = (%d,%v), want (255,true)", got, ok)
	}
	got, ok = hexPair('0', '0')
	if !ok || got != 0 {
		t.Errorf("hexPair('0','0') = (%d,%v), want (0,true)", got, ok)
	}
}

func TestHexPairInvalid(t *testing.T) {
	if _, ok := hexPair('G', '0'); ok {
		t.Error("hexPair('G','0') should return false")
	}
}

func closeEnough(a, b float32) bool {
	return float32(math.Abs(float64(a-b))) < 0.02
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}
