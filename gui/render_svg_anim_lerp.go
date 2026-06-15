package gui

// lerpColorKeyframes interpolates packed RGBA stops at frac in
// [0,1]. Uses locateSeg for consistent discrete/spline/keyTimes
// behavior. sRGB Lerp per channel — cheap and visually acceptable
// for short tweens (premultiplied gamma-correct Lerp is a follow-up).
func lerpColorKeyframes(vals []uint32, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32) SvgColor {
	if len(vals) == 0 {
		return SvgColor{}
	}
	n := len(vals)
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return unpackRGBA(vals[n-1])
	}
	if mode == SvgAnimCalcDiscrete {
		return unpackRGBA(vals[idx])
	}
	a := unpackRGBA(vals[idx])
	b := unpackRGBA(vals[idx+1])
	return SvgColor{
		R: lerpU8(a.R, b.R, t),
		G: lerpU8(a.G, b.G, t),
		B: lerpU8(a.B, b.B, t),
		A: lerpU8(a.A, b.A, t),
	}
}

func unpackRGBA(v uint32) SvgColor {
	return SvgColor{
		R: uint8(v >> 24),
		G: uint8(v >> 16),
		B: uint8(v >> 8),
		A: uint8(v),
	}
}

func lerpU8(a, b uint8, t float32) uint8 {
	if t != t || t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	v := float32(a) + (float32(b)-float32(a))*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v)
}

// lerpKeyframes interpolates keyframe scalars at frac ∈ [0,1].
// Linear by default; spline bends per-segment t via cubic-bezier;
// discrete returns the covering keyframe. keyTimes, when non-nil,
// overrides uniform i/(n-1) spacing.
func lerpKeyframes(
	vals, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32,
) float32 {
	n := len(vals)
	if n == 0 {
		return 1
	}
	if n == 1 {
		return vals[0]
	}
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return vals[n-1]
	}
	if mode == SvgAnimCalcDiscrete {
		return vals[idx]
	}
	return vals[idx] + (vals[idx+1]-vals[idx])*t
}

// lerpKeyframes2D interpolates a paired [x0,y0, x1,y1, ...]
// keyframe stream at frac ∈ [0,1].
func lerpKeyframes2D(
	vals, splines, keyTimes []float32,
	mode SvgAnimCalcMode, frac float32,
) (float32, float32) {
	n := len(vals) / 2
	if n == 0 {
		return 0, 0
	}
	if n == 1 {
		return vals[0], vals[1]
	}
	idx, t, atEnd := locateSeg(n, frac, splines, keyTimes, mode)
	if atEnd {
		return vals[(n-1)*2], vals[(n-1)*2+1]
	}
	if mode == SvgAnimCalcDiscrete {
		return vals[idx*2], vals[idx*2+1]
	}
	x0, y0 := vals[idx*2], vals[idx*2+1]
	x1, y1 := vals[(idx+1)*2], vals[(idx+1)*2+1]
	return x0 + (x1-x0)*t, y0 + (y1-y0)*t
}
