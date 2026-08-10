package gui

import (
	"math"
	"slices"
)

// render_gradient.go — pure-Go gradient math ported from V's
// render_gradient.v. No GPU calls.

const gradientShaderStopLimit = 5

func clampUnit(v float32) float32 {
	// NaN compares false both ways; fold to 0 instead of propagating.
	if v != v || v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// angleToDirection converts a CSS angle (degrees) to a unit
// direction vector. CSS: 0deg=top, clockwise.
func angleToDirection(cssDeg float32) (dx, dy float32) {
	rad := (90.0 - cssDeg) * math.Pi / 180.0
	return float32(math.Cos(float64(rad))), -float32(math.Sin(float64(rad)))
}

// compareStops orders gradient stops by position.
func compareStops(a, b GradientStop) int {
	if a.Pos < b.Pos {
		return -1
	}
	if a.Pos > b.Pos {
		return 1
	}
	return 0
}

// GradientDir computes direction vector from GradientDef.
func GradientDir(g *GradientDef, w, h float32) (dx, dy float32) {
	if g == nil {
		return 0, -1 // default: top
	}
	if g.hasAngle {
		return angleToDirection(g.angle)
	}
	var cssDeg float32
	switch g.Direction {
	case GradientToTop:
		cssDeg = 0
	case GradientToRight:
		cssDeg = 90
	case GradientToBottom:
		cssDeg = 180
	case GradientToLeft:
		cssDeg = 270
	case GradientToTopRight:
		cssDeg = 90.0 - float32(math.Atan2(float64(h), float64(w)))*180.0/math.Pi
	case GradientToBottomRight:
		cssDeg = 90.0 + float32(math.Atan2(float64(h), float64(w)))*180.0/math.Pi
	case GradientToBottomLeft:
		cssDeg = 270.0 - float32(math.Atan2(float64(h), float64(w)))*180.0/math.Pi
	case GradientToTopLeft:
		cssDeg = 270.0 + float32(math.Atan2(float64(h), float64(w)))*180.0/math.Pi
	}
	return angleToDirection(cssDeg)
}

// PackRGB packs R, G, B into a single float32 for GPU uniforms.
func PackRGB(c Color) float32 {
	return float32(c.R) + float32(c.G)*256.0 + float32(c.B)*65536.0
}

// PackAlphaPos packs Alpha and gradient position into one float32.
func PackAlphaPos(c Color, pos float32) float32 {
	return float32(c.A) + float32(math.Floor(float64(pos)*10000.0))*256.0
}

// f32ToU8Saturated clamps a float32 to [0,255] and rounds.
func f32ToU8Saturated(v float32) uint8 {
	clamped := max(0.0, min(float64(v), 255.0))
	return uint8(math.Round(clamped))
}

// lerpColorPremultiplied linearly interpolates two colors in
// premultiplied-alpha space.
func lerpColorPremultiplied(a, b Color, t float32) Color {
	ct := clampUnit(t)
	aAlpha := float32(a.A) / 255.0
	bAlpha := float32(b.A) / 255.0
	aR := (float32(a.R) / 255.0) * aAlpha
	aG := (float32(a.G) / 255.0) * aAlpha
	aB := (float32(a.B) / 255.0) * aAlpha
	bR := (float32(b.R) / 255.0) * bAlpha
	bG := (float32(b.G) / 255.0) * bAlpha
	bB := (float32(b.B) / 255.0) * bAlpha
	alpha := aAlpha + (bAlpha-aAlpha)*ct
	pR := aR + (bR-aR)*ct
	pG := aG + (bG-aG)*ct
	pB := aB + (bB-aB)*ct
	if alpha <= 0.0001 {
		return RGBA(0, 0, 0, 0)
	}
	r := (pR / alpha) * 255.0
	g := (pG / alpha) * 255.0
	bl := (pB / alpha) * 255.0
	return RGBA(f32ToU8Saturated(r), f32ToU8Saturated(g), f32ToU8Saturated(bl), f32ToU8Saturated(alpha*255.0))
}

// SampleGradientStopColor returns the interpolated color at the
// given position along the gradient stops.
func SampleGradientStopColor(stops []GradientStop, pos float32) Color {
	if len(stops) == 0 {
		return RGBA(0, 0, 0, 0)
	}
	if pos <= stops[0].Pos {
		return stops[0].Color
	}
	for i := 1; i < len(stops); i++ {
		left := stops[i-1]
		right := stops[i]
		if pos > right.Pos {
			continue
		}
		span := right.Pos - left.Pos
		if span <= 0.0001 {
			return right.Color
		}
		localT := (pos - left.Pos) / span
		return lerpColorPremultiplied(left.Color, right.Color, localT)
	}
	return stops[len(stops)-1].Color
}

// NormalizeGradientStopsInto is the non-allocating variant that
// reuses caller-provided slices.
func NormalizeGradientStopsInto(stops []GradientStop, norm, sampled *[]GradientStop) []GradientStop {
	if len(stops) == 0 {
		*norm = (*norm)[:0]
		*sampled = (*sampled)[:0]
		return nil
	}
	*norm = (*norm)[:0]
	if cap(*norm) < len(stops) {
		*norm = make([]GradientStop, 0, len(stops))
	}
	for _, s := range stops {
		*norm = append(*norm, GradientStop{Color: s.Color, Pos: clampUnit(s.Pos)})
	}
	slices.SortFunc(*norm, compareStops)
	if len(*norm) <= gradientShaderStopLimit {
		*sampled = (*sampled)[:0]
		return *norm
	}
	*sampled = (*sampled)[:0]
	if cap(*sampled) < gradientShaderStopLimit {
		*sampled = make([]GradientStop, 0, gradientShaderStopLimit)
	}
	for i := range gradientShaderStopLimit {
		samplePos := float32(i) / float32(gradientShaderStopLimit-1)
		*sampled = append(*sampled, GradientStop{
			Color: SampleGradientStopColor(*norm, samplePos),
			Pos:   samplePos,
		})
	}
	return *sampled
}
