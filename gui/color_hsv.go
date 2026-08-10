package gui

import "fmt"

// ToHSV converts an RGB Color to HSV components.
// Returns h (0–360), s (0–1), v (0–1).
func (c Color) ToHSV() (h, s, v float32) {
	r := float32(c.R) / 255.0
	g := float32(c.G) / 255.0
	b := float32(c.B) / 255.0

	mx := f32Max(r, f32Max(g, b))
	mn := f32Min(r, f32Min(g, b))
	delta := mx - mn

	v = mx
	if mx == 0 {
		s = 0
	} else {
		s = delta / mx
	}

	if delta != 0 {
		switch mx {
		case r:
			h = 60.0 * f32Mod((g-b)/delta, 6)
		case g:
			h = 60.0 * (((b - r) / delta) + 2.0)
		default:
			h = 60.0 * (((r - g) / delta) + 4.0)
		}
	}
	if h < 0 {
		h += 360.0
	}
	return h, s, v
}

// ColorFromHSV creates a Color from HSV values.
// h: 0–360, s: 0–1, v: 0–1. Alpha defaults to 255.
func ColorFromHSV(h, s, v float32) Color {
	return colorFromHSVA(h, s, v, 255)
}

// ColorFromHSVA creates a Color from HSVA values.
// h: 0–360, s: 0–1, v: 0–1, a: 0–255.
func colorFromHSVA(h, s, v float32, a uint8) Color {
	h = f32Mod(h, 360)
	if h < 0 {
		h += 360
	}
	c := v * s
	hh := f32Mod(h/60.0, 6)
	x := c * (1.0 - f32Abs(f32Mod(hh, 2)-1.0))
	m := v - c

	var r, g, b float32
	switch {
	case hh < 1:
		r, g = c, x
	case hh < 2:
		r, g = x, c
	case hh < 3:
		g, b = c, x
	case hh < 4:
		g, b = x, c
	case hh < 5:
		r, b = x, c
	default:
		r, b = c, x
	}

	return RGBA(uint8((r+m)*255.0+0.5), uint8((g+m)*255.0+0.5), uint8((b+m)*255.0+0.5), a)
}

// HueColor returns the pure color for a given hue (s=1, v=1).
func hueColor(h float32) Color {
	return ColorFromHSV(h, 1.0, 1.0)
}

// ToHexString returns "#RRGGBB" or "#RRGGBBAA" when alpha != 255.
func (c Color) toHexString() string {
	if c.A == 255 {
		return fmt.Sprintf("#%02X%02X%02X", c.R, c.G, c.B)
	}
	return fmt.Sprintf("#%02X%02X%02X%02X", c.R, c.G, c.B, c.A)
}

// ColorFromHexString parses "#RRGGBB" or "#RRGGBBAA".
// Returns (Color, false) on invalid input.
func colorFromHexString(s string) (Color, bool) {
	raw := s
	if len(raw) > 0 && raw[0] == '#' {
		raw = raw[1:]
	}
	if len(raw) != 6 && len(raw) != 8 {
		return Color{}, false
	}
	r, ok := hexPair(raw[0], raw[1])
	if !ok {
		return Color{}, false
	}
	g, ok := hexPair(raw[2], raw[3])
	if !ok {
		return Color{}, false
	}
	b, ok := hexPair(raw[4], raw[5])
	if !ok {
		return Color{}, false
	}
	a := uint8(255)
	if len(raw) == 8 {
		a, ok = hexPair(raw[6], raw[7])
		if !ok {
			return Color{}, false
		}
	}
	return RGBA(r, g, b, a), true
}

func hexPair(hi, lo byte) (uint8, bool) {
	h, ok := hexNibble(hi)
	if !ok {
		return 0, false
	}
	l, ok := hexNibble(lo)
	if !ok {
		return 0, false
	}
	return (h << 4) | l, true
}

func hexNibble(c byte) (uint8, bool) {
	switch {
	case c >= '0' && c <= '9':
		return c - '0', true
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10, true
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10, true
	}
	return 0, false
}
