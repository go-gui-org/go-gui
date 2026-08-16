package gui

import "strconv"

// HSLA is a color in hue/saturation/lightness form with alpha. It is
// the value the color components (ColorPlane, ColorWheel,
// ColorChannelSlider, ColorSwatch) read and write.
//
// Why a value the *app* owns rather than a Color: RGBA cannot
// represent hue at S=0 (grays) or at L=0/L=1 (black and white), so a
// picker that round-trips through Color loses the hue as soon as the
// pointer touches an edge. The old monolithic ColorPicker hid an H in
// window state to paper over that; components that each hid their own
// copy would drift apart instead. Holding one HSLA in app state makes
// every control read the same value, which is the whole point of
// splitting them up.
//
// Ranges: H in degrees 0–360 (wrapping), S, L and A in 0–1. Nothing
// clamps on construction — Normalized does, at the point of use.
type HSLA struct {
	H float32 // hue, degrees
	S float32 // saturation, 0–1
	L float32 // lightness, 0–1
	A float32 // alpha, 0–1
}

// Normalized returns v with H wrapped into [0, 360) and S, L, A
// clamped to [0, 1]. Conversions call it, so callers may hand over raw
// pointer-derived values.
func (v HSLA) Normalized() HSLA {
	h := f32Mod(v.H, 360)
	if h < 0 {
		h += 360
	}
	return HSLA{
		H: h,
		S: f32Clamp(v.S, 0, 1),
		L: f32Clamp(v.L, 0, 1),
		A: f32Clamp(v.A, 0, 1),
	}
}

// Color converts to an RGBA Color. The conversion is lossy in the one
// direction that matters — hue and saturation collapse at the
// lightness extremes — which is why the HSLA value, not the Color, is
// what a picker keeps.
func (v HSLA) Color() Color {
	n := v.Normalized()
	a := uint8(n.A*255 + 0.5)

	// c is the chroma the lightness leaves room for; m re-centers the
	// result on that lightness. Standard HSL→RGB.
	c := (1 - f32Abs(2*n.L-1)) * n.S
	hh := f32Mod(n.H/60, 6)
	x := c * (1 - f32Abs(f32Mod(hh, 2)-1))
	m := n.L - c/2

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
	return RGBA(
		uint8((r+m)*255+0.5),
		uint8((g+m)*255+0.5),
		uint8((b+m)*255+0.5),
		a)
}

// ColorToHSLA converts an RGBA Color to HSLA. Hue is 0 for grays,
// where it is genuinely undefined — a caller tracking a live HSLA
// value should keep its own H rather than round-tripping through this.
func ColorToHSLA(c Color) HSLA {
	r := float32(c.R) / 255
	g := float32(c.G) / 255
	b := float32(c.B) / 255

	mx := f32Max(r, f32Max(g, b))
	mn := f32Min(r, f32Min(g, b))
	delta := mx - mn

	v := HSLA{L: (mx + mn) / 2, A: float32(c.A) / 255}
	if delta == 0 {
		return v // gray: hue undefined, saturation zero
	}

	den := 1 - f32Abs(2*v.L-1)
	if den > 0 {
		v.S = f32Clamp(delta/den, 0, 1)
	}
	switch mx {
	case r:
		v.H = 60 * f32Mod((g-b)/delta, 6)
	case g:
		v.H = 60 * (((b - r) / delta) + 2)
	default:
		v.H = 60 * (((r - g) / delta) + 4)
	}
	if v.H < 0 {
		v.H += 360
	}
	return v
}

// String formats the value as CSS hsla(), the notation the readout
// shows: hsla(210, 70%, 55%, 0.85).
func (v HSLA) String() string {
	n := v.Normalized()
	b := make([]byte, 0, 32)
	b = append(b, "hsla("...)
	b = strconv.AppendInt(b, int64(n.H+0.5), 10)
	b = append(b, ", "...)
	b = strconv.AppendInt(b, int64(n.S*100+0.5), 10)
	b = append(b, "%, "...)
	b = strconv.AppendInt(b, int64(n.L*100+0.5), 10)
	b = append(b, "%, "...)
	b = strconv.AppendFloat(b, float64(n.A), 'f', 2, 32)
	b = append(b, ')')
	return string(b)
}

// Hex returns "#RRGGBB", or "#RRGGBBAA" when alpha is not opaque.
func (c Color) Hex() string { return c.toHexString() }

// ColorFromHex parses "#RRGGBB" or "#RRGGBBAA"; the leading "#" is
// optional. It returns ok=false on malformed input, leaving the
// caller's current color untouched — which is what a hex field wants
// while the user is still typing.
//
// exportaudit:keep — public counterpart of Color.Hex, for app code
// reading colors from config or user input.
func ColorFromHex(s string) (Color, bool) {
	return colorFromHexString(s)
}
