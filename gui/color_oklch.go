package gui

import "math"

// oklch is a color in the OKLCH perceptual space: lightness,
// chroma, hue plus alpha. It exists for one job — deriving the
// accent and danger ramps (issue #732) — so it stays private: the
// color picker components are HSLA-based and a second public space
// with no consumer fails the export audit.
//
// Why OKLCH rather than the HSL the ramps used: HSL lightness is
// not perceptual. The same 0.12 step reads large on a blue accent
// and small on a yellow or green one, so hover and pressed states
// looked stronger or weaker depending on the accent a theme picked.
// OKLCH L is perceptually uniform, so one step reads the same on
// every hue; C and H carry through unchanged, so a state reads as
// the same color, lighter or darker.
//
// Matrices are the sRGB pair from Björn Ottosson's OKLab derivation.
// Ranges mirror HSLA: H in degrees 0–360 (wrapping), L, C and A
// clamped at the point of use by normalize.
type oklch struct {
	L float32 // lightness, 0–1
	C float32 // chroma, 0 and up
	H float32 // hue, degrees
	A float32 // alpha, 0–1
}

// normalize wraps H into [0, 360) and clamps L, C and A, following
// the HSLA.Normalized precedent: non-finite components become zero
// rather than surviving into color math.
func (v oklch) normalize() oklch {
	if !f32IsFinite(v.L) {
		v.L = 0
	}
	if !f32IsFinite(v.C) {
		v.C = 0
	}
	if !f32IsFinite(v.H) {
		v.H = 0
	}
	if !f32IsFinite(v.A) {
		v.A = 0
	}
	h := f32Mod(v.H, 360)
	if h < 0 {
		h += 360
	}
	if v.C < 0 {
		v.C = 0
	}
	return oklch{
		L: f32Clamp(v.L, 0, 1),
		C: v.C,
		H: h,
		A: f32Clamp(v.A, 0, 1),
	}
}

// oklchFromLinear maps a linear-light channel to sRGB.
func oklchFromLinear(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// oklchToLinear maps an sRGB channel to linear light.
func oklchToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// colorToOKLCH converts an RGBA Color to OKLCH, carrying alpha
// through untouched.
func colorToOKLCH(c Color) oklch {
	r := oklchToLinear(float64(c.R) / 255)
	g := oklchToLinear(float64(c.G) / 255)
	b := oklchToLinear(float64(c.B) / 255)

	l := 0.4122214708*r + 0.5363325363*g + 0.0514459929*b
	m := 0.2119034982*r + 0.6806995451*g + 0.1073969566*b
	s := 0.0883024619*r + 0.2817188376*g + 0.6299787005*b
	lp, mp, sp := math.Cbrt(l), math.Cbrt(m), math.Cbrt(s)

	ol := 0.2104542553*lp + 0.7936177850*mp - 0.0040720468*sp
	a := 1.9779984951*lp - 2.4285922050*mp + 0.4505937099*sp
	bb := 0.0259040371*lp + 0.7827717662*mp - 0.8086757660*sp

	h := float32(math.Atan2(bb, a)) * 360 / (2 * float32(math.Pi))
	if h < 0 {
		h += 360
	}
	return oklch{
		L: float32(ol),
		C: float32(math.Hypot(a, bb)),
		H: h,
		A: float32(c.A) / 255,
	}
}

// color converts back to an RGBA Color. A shifted color can leave
// the sRGB gamut — a lighter blue at full chroma has no sRGB
// address — so chroma halves until the color fits, keeping the hue
// a clip would otherwise shear off. Twelve rounds cover any input;
// the achromatic fallback cannot miss.
func (v oklch) color() Color {
	n := v.normalize()
	c := n.C
	for range 12 {
		if r, g, b := oklchToLinearRGB(n.L, c, n.H); inGamut(r, g, b) {
			return oklchToColor(r, g, b, n.A)
		}
		c /= 2
	}
	r, g, b := oklchToLinearRGB(n.L, 0, n.H)
	return oklchToColor(
		f64Clamp(r, 0, 1),
		f64Clamp(g, 0, 1),
		f64Clamp(b, 0, 1),
		n.A)
}

// oklchToLinearRGB maps OKLCH to linear-light sRGB, which the
// caller gamut-checks before quantizing.
func oklchToLinearRGB(l, c, h float32) (float64, float64, float64) {
	hr := float64(h) * math.Pi / 180
	a := float64(c) * math.Cos(hr)
	b := float64(c) * math.Sin(hr)
	fl := float64(l)
	lp := fl + 0.3963377774*a + 0.2158037573*b
	mp := fl - 0.1055613458*a - 0.0638541728*b
	sp := fl - 0.0894841775*a - 1.2914855480*b
	l3, m3, s3 := lp*lp*lp, mp*mp*mp, sp*sp*sp
	return 4.0767416621*l3 - 3.3077115913*m3 + 0.2309699292*s3,
		-1.2684380046*l3 + 2.6097574011*m3 - 0.3413193965*s3,
		-0.0041960863*l3 - 0.7034186147*m3 + 1.7076147010*s3
}

// inGamut reports whether linear-light r, g, b all fit in sRGB.
// The epsilon is float32 rounding, not leniency: a round-trip lands
// within 1e-6 of the gamut wall, while a real excursion (a lighter
// blue at full chroma) overshoots by tenths.
func inGamut(r, g, b float64) bool {
	const eps = 1e-4
	return r >= -eps && r <= 1+eps && g >= -eps && g <= 1+eps && b >= -eps && b <= 1+eps
}

// oklchToColor quantizes linear-light sRGB to an RGBA Color,
// clamping the epsilon inGamut tolerates back onto the wall.
func oklchToColor(r, g, b float64, a float32) Color {
	return RGBA(
		uint8(oklchFromLinear(f64Clamp(r, 0, 1))*255+0.5),
		uint8(oklchFromLinear(f64Clamp(g, 0, 1))*255+0.5),
		uint8(oklchFromLinear(f64Clamp(b, 0, 1))*255+0.5),
		uint8(a*255+0.5))
}

// oklchRampDelta is the lightness step separating a fill from its
// hover and pressed states. Calibrated against the HSL ±0.12 rule it
// replaces: the old hover/pressed sat ~0.11/0.10 OKLCH-L off their
// accents, so 0.10 keeps the same perceptual magnitude on the
// default accents while reading identically on every other hue
// (issue #732). Stated once so ThemeMaker, the danger ramp and
// WithColors cannot drift apart.
const oklchRampDelta = float32(0.10)

// oklchShift moves c by delta on the OKLCH lightness axis, keeping
// chroma and hue: a state reads as the same color, lighter or
// darker. The engine behind accentShift, which is the single ramp
// spelling every derivation site uses. Alpha carries through
// untouched.
func oklchShift(c Color, delta float32) Color {
	v := colorToOKLCH(c)
	v.L = f32Clamp(v.L+delta, 0, 1)
	return v.color()
}
