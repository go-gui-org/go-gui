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
		// Both ends are fully transparent, so the premultiplied RGB
		// holds no hue to divide back out. Black would be the easy
		// answer and is wrong: any consumer that interpolates in
		// straight-alpha space — a vertex-colored triangle mesh, on
		// every backend — reads that black as a real color, and a fade
		// to transparent white darkens as it fades instead of just
		// thinning. Carry the nearer end's hue at zero alpha.
		hue := a
		if ct >= 0.5 {
			hue = b
		}
		return RGBA(hue.R, hue.G, hue.B, 0)
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
		right := &stops[i]
		if pos > right.Pos {
			continue
		}
		left := &stops[i-1]
		span := right.Pos - left.Pos
		if span <= 0.0001 {
			return right.Color
		}
		localT := (pos - left.Pos) / span
		return lerpColorPremultiplied(left.Color, right.Color, localT)
	}
	return stops[len(stops)-1].Color
}

// gradRampSegment holds the per-interval data that is invariant across
// all vertices of one fill. Premultiplying once and reusing it is the
// exact variant from issue #434: no accuracy change, goldens stay
// byte-identical, but the per-vertex path drops from six divisions by
// 255 and two alpha multiplies per end to a single lerp and one
// unpremultiply.
type gradRampSegment struct {
	leftPos        float32
	rightPos       float32
	span           float32
	aA, aR, aG, aB float32
	bA, bR, bG, bB float32
	left           Color
	right          Color
}

// prepareGradRamp fills segs from normalized stops. segs is a caller
// scratch buffer so the fill allocates nothing after the first frame.
func prepareGradRamp(stops []GradientStop, segs *[]gradRampSegment) []gradRampSegment {
	if segs == nil {
		return nil
	}
	if len(stops) < 2 {
		*segs = (*segs)[:0]
		return nil
	}
	// Cap absurd stop counts to bound allocation and per-vertex
	// scan. A ramp with thousands of stops has no visible fidelity
	// gain and would turn the per-vertex linear scan into a DoS.
	const maxGradRampStops = 2048
	if len(stops) > maxGradRampStops {
		stops = stops[:maxGradRampStops]
	}
	*segs = (*segs)[:0]
	if cap(*segs) < len(stops)-1 {
		*segs = make([]gradRampSegment, 0, len(stops)-1)
	}
	for i := 1; i < len(stops); i++ {
		left := &stops[i-1]
		right := &stops[i]
		span := right.Pos - left.Pos
		s := gradRampSegment{
			leftPos:  left.Pos,
			rightPos: right.Pos,
			span:     span,
			left:     left.Color,
			right:    right.Color,
		}
		if span > 0.0001 {
			aA := float32(left.Color.A) / 255.0
			bA := float32(right.Color.A) / 255.0
			s.aA = aA
			s.bA = bA
			s.aR = (float32(left.Color.R) / 255.0) * aA
			s.aG = (float32(left.Color.G) / 255.0) * aA
			s.aB = (float32(left.Color.B) / 255.0) * aA
			s.bR = (float32(right.Color.R) / 255.0) * bA
			s.bG = (float32(right.Color.G) / 255.0) * bA
			s.bB = (float32(right.Color.B) / 255.0) * bA
		}
		*segs = append(*segs, s)
	}
	return *segs
}

// sampleGradRamp is the per-vertex fast path that reuses the
// premultiplied endpoints prepared by prepareGradRamp. The math is
// identical to SampleGradientStopColor → lerpColorPremultiplied, only
// the per-segment work is hoisted.
func sampleGradRamp(stops []GradientStop, segs []gradRampSegment, pos float32) Color {
	if len(stops) == 0 {
		return RGBA(0, 0, 0, 0)
	}
	// Non-finite positions come from degenerate geometry and must not
	// propagate NaN through the lerp. No guard of their own is needed:
	// NaN fails every comparison and -Inf fails this one, so both fold
	// to the ramp start, and +Inf falls through to the end test below.
	// That saves a float64 conversion and an IsInf on every vertex of
	// every gradient mesh.
	if !(pos > stops[0].Pos) {
		return stops[0].Color
	}
	if pos >= stops[len(stops)-1].Pos {
		return stops[len(stops)-1].Color
	}
	for i := range segs {
		s := &segs[i]
		if pos > s.rightPos {
			continue
		}
		if s.span <= 0.0001 {
			return s.right
		}
		localT := (pos - s.leftPos) / s.span
		ct := clampUnit(localT)
		alpha := s.aA + (s.bA-s.aA)*ct
		pR := s.aR + (s.bR-s.aR)*ct
		pG := s.aG + (s.bG-s.aG)*ct
		pB := s.aB + (s.bB-s.aB)*ct
		if alpha <= 0.0001 {
			hue := s.left
			if ct >= 0.5 {
				hue = s.right
			}
			return RGBA(hue.R, hue.G, hue.B, 0)
		}
		r := (pR / alpha) * 255.0
		g := (pG / alpha) * 255.0
		bl := (pB / alpha) * 255.0
		return RGBA(f32ToU8Saturated(r), f32ToU8Saturated(g), f32ToU8Saturated(bl), f32ToU8Saturated(alpha*255.0))
	}
	return stops[len(stops)-1].Color
}

// NormalizeGradientStops clamps every stop position to [0,1] and
// sorts by position, preserving the full stop count. Backends whose
// gradient path has no stop limit — the web backend's canvas
// gradients — use this instead of NormalizeGradientStopsInto so
// fidelity is never resampled away.
func NormalizeGradientStops(stops []GradientStop, norm *[]GradientStop) []GradientStop {
	if norm == nil {
		return nil
	}
	if len(stops) == 0 {
		*norm = (*norm)[:0]
		return nil
	}
	// Bound allocation for hostile input; 8k stops already exceed
	// any visible fidelity.
	const maxNormalizeStops = 8192
	if len(stops) > maxNormalizeStops {
		stops = stops[:maxNormalizeStops]
	}
	*norm = (*norm)[:0]
	if cap(*norm) < len(stops) {
		*norm = make([]GradientStop, 0, len(stops))
	}
	for _, s := range stops {
		*norm = append(*norm, GradientStop{Color: s.Color, Pos: clampUnit(s.Pos)})
	}
	slices.SortFunc(*norm, compareStops)
	return *norm
}

// NormalizeGradientStopsInto is the non-allocating variant that
// reuses caller-provided slices. Stops beyond gradientShaderStopLimit
// are resampled down to the limit by resampleStopsInto;
// NormalizeGradientStops is the no-resample variant for backends
// without a stop limit.
func NormalizeGradientStopsInto(stops []GradientStop, norm, sampled *[]GradientStop) []GradientStop {
	if norm == nil || sampled == nil {
		return nil
	}
	result := NormalizeGradientStops(stops, norm)
	if result == nil {
		*sampled = (*sampled)[:0]
		return nil
	}
	if len(result) <= gradientShaderStopLimit {
		*sampled = (*sampled)[:0]
		return result
	}
	return resampleStopsInto(result, sampled)
}

// premulChannels returns the four channels a compositor actually
// receives: RGB scaled by alpha, plus alpha, in 0..255 units.
//
// Stop placement is judged here rather than in straight-alpha space
// because a hue error underneath a near-zero alpha never reaches the
// framebuffer, and a straight-alpha metric would spend stops chasing
// it — exactly the tail of a glow, where the budget is scarcest.
func premulChannels(c Color) [4]float32 {
	a := float32(c.A) / 255
	return [4]float32{
		float32(c.R) * a, float32(c.G) * a, float32(c.B) * a,
		float32(c.A),
	}
}

// segmentWorstStop finds the stop strictly inside [lo,hi] that the
// straight line from lo to hi misses by the most, and reports that
// error. Returns -1 when the span holds no interior stop.
func segmentWorstStop(stops []GradientStop, lo, hi int) (int, float32) {
	a := premulChannels(stops[lo].Color)
	b := premulChannels(stops[hi].Color)
	span := stops[hi].Pos - stops[lo].Pos
	idx, worst := -1, float32(0)
	for m := lo + 1; m < hi; m++ {
		var u float32
		if span > 0 {
			u = (stops[m].Pos - stops[lo].Pos) / span
		}
		cur := premulChannels(stops[m].Color)
		var e float32
		for ch := range 4 {
			d := a[ch] + (b[ch]-a[ch])*u - cur[ch]
			if d < 0 {
				d = -d
			}
			if d > e {
				e = d
			}
		}
		if idx < 0 || e > worst {
			idx, worst = m, e
		}
	}
	return idx, worst
}

// resampleStopsInto cuts an over-long stop list down to
// gradientShaderStopLimit by keeping the stops that carry the shape,
// not by spacing them evenly.
//
// Even spacing is the wrong default for the curves that actually run
// over the limit. A glow's opacity piles into the innermost third of
// its ramp, so five evenly spaced samples put four of them in the flat
// part and one in the falloff — measured against the analytic curve
// that is up to 80/255 off in the composited channels, while the same
// five stops placed by error land within 18 and eight within 8, which
// is the source list's own sampling error. Soft and web honour the
// full list, so this divergence was GPU-only and no golden could see
// it.
//
// The placement is a Douglas-Peucker split: keep the two ends, then
// repeatedly promote the stop that the current piecewise line misses
// by the most. O(n*k), the same order as the even path it replaces
// (which scanned the list once per sample), and it keeps the source
// stops' own colors and positions rather than resampling them.
func resampleStopsInto(result []GradientStop, sampled *[]GradientStop) []GradientStop {
	*sampled = (*sampled)[:0]
	if cap(*sampled) < gradientShaderStopLimit {
		*sampled = make([]GradientStop, 0, gradientShaderStopLimit)
	}

	// A list that does not span the whole ramp needs a flat stop at
	// each end it misses. SampleGradientStopColor clamps outside the
	// stop range, but the GPU shader interpolates from the first
	// stop's position outward, so without these it extrapolates past
	// the ramp instead of holding the end color. The even path got
	// this for free by always emitting positions 0 and 1.
	lead := result[0].Pos > 0
	tail := result[len(result)-1].Pos < 1
	budget := gradientShaderStopLimit
	if lead {
		budget--
	}
	if tail {
		budget--
	}
	budget = max(budget, 2)

	var chosen [gradientShaderStopLimit]int
	n := 2
	chosen[0] = 0
	chosen[1] = len(result) - 1
	for n < budget {
		bestErr := float32(0)
		bestSeg, bestIdx := -1, -1
		for s := 0; s+1 < n; s++ {
			idx, e := segmentWorstStop(result, chosen[s], chosen[s+1])
			if idx >= 0 && (bestIdx < 0 || e > bestErr) {
				bestErr, bestSeg, bestIdx = e, s, idx
			}
		}
		if bestIdx < 0 {
			// Every remaining stop already lies on a chosen line.
			break
		}
		copy(chosen[bestSeg+2:n+1], chosen[bestSeg+1:n])
		chosen[bestSeg+1] = bestIdx
		n++
	}

	if lead {
		*sampled = append(*sampled,
			GradientStop{Color: result[0].Color, Pos: 0})
	}
	for _, i := range chosen[:n] {
		*sampled = append(*sampled, result[i])
	}
	if tail {
		*sampled = append(*sampled, GradientStop{
			Color: result[len(result)-1].Color, Pos: 1})
	}
	return *sampled
}
