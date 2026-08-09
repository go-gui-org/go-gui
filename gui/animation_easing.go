package gui

import "math"

// EasingFn maps progress t (0.0 to 1.0) to an eased value.
type EasingFn = func(float32) float32

// Lerp linearly interpolates between a and b by t.
func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

// EaseLinear returns constant-speed motion.
func EaseLinear(t float32) float32 { return t }

// EaseInQuad starts slow and accelerates (quadratic).
func easeInQuad(t float32) float32 { return t * t }

// EaseOutQuad starts fast and decelerates (quadratic).
func EaseOutQuad(t float32) float32 { return 1 - (1-t)*(1-t) }

// EaseInOutQuad accelerates then decelerates (quadratic).
func EaseInOutQuad(t float32) float32 {
	if t < 0.5 {
		return 2 * t * t
	}
	v := -2*t + 2
	return 1 - (v*v)/2
}

// EaseInCubic starts slow and accelerates (cubic).
func easeInCubic(t float32) float32 { return t * t * t }

// EaseOutCubic starts fast and decelerates (cubic).
func EaseOutCubic(t float32) float32 {
	u := 1 - t
	return 1 - u*u*u
}

// EaseInOutCubic accelerates then decelerates (cubic).
func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	v := -2*t + 2
	return 1 - v*v*v/2
}

// EaseInBack pulls back slightly before accelerating forward.
func easeInBack(t float32) float32 {
	const c1 = float32(1.70158)
	c3 := c1 + 1
	return c3*t*t*t - c1*t*t
}

// EaseOutBack overshoots the target then settles back.
func easeOutBack(t float32) float32 {
	const c1 = float32(1.70158)
	c3 := c1 + 1
	u := t - 1
	return 1 + c3*u*u*u + c1*u*u
}

// EaseOutElastic overshoots and oscillates like a released spring.
func EaseOutElastic(t float32) float32 {
	if t == 0 {
		return 0
	}
	if t == 1 {
		return 1
	}
	c4 := float32(2*math.Pi) / 3
	return float32(math.Pow(2, float64(-10*t)))*
		float32(math.Sin(float64((t*10-0.75)*c4))) + 1
}

// EaseOutBounce simulates a bouncing ball settling to rest.
func EaseOutBounce(t float32) float32 {
	const n1 = float32(7.5625)
	const d1 = float32(2.75)
	switch {
	case t < 1/d1:
		return n1 * t * t
	case t < 2/d1:
		t -= 1.5 / d1
		return n1*t*t + 0.75
	case t < 2.5/d1:
		t -= 2.25 / d1
		return n1*t*t + 0.9375
	default:
		t -= 2.625 / d1
		return n1*t*t + 0.984375
	}
}

// bezierLUTSize is the LUT size for bezier precomputation.
const bezierLUTSize = 257

// bezierLUT stores precomputed bezier values for O(1) lookup.
type bezierLUT struct {
	values [bezierLUTSize]float32
}

func buildBezierLUT(x1, y1, x2, y2 float32) bezierLUT {
	var lut bezierLUT
	for i := range bezierLUTSize {
		t := float32(i) / float32(bezierLUTSize-1)
		lut.values[i] = bezierCalc(t, x1, y1, x2, y2)
	}
	return lut
}

func (lut *bezierLUT) lookup(t float32) float32 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	idxF := t * float32(bezierLUTSize-1)
	idx := int(idxF)
	frac := idxF - float32(idx)
	if idx >= bezierLUTSize-1 {
		return lut.values[bezierLUTSize-1]
	}
	return lut.values[idx] + frac*(lut.values[idx+1]-lut.values[idx])
}

// Precomputed LUTs for common CSS bezier curves.
var (
	easeLUT      = buildBezierLUT(0.25, 0.1, 0.25, 1.0)
	easeInLUT    = buildBezierLUT(0.42, 0, 1.0, 1.0)
	easeOutLUT   = buildBezierLUT(0, 0, 0.58, 1.0)
	easeInOutLUT = buildBezierLUT(0.42, 0, 0.58, 1.0)
)

// EaseCSS returns CSS "ease" curve. Uses precomputed LUT.
func easeCSS(t float32) float32 { return easeLUT.lookup(t) }

// EaseInCSS returns CSS "ease-in" curve. Uses precomputed LUT.
func easeInCSS(t float32) float32 { return easeInLUT.lookup(t) }

// EaseOutCSS returns CSS "ease-out" curve. Uses precomputed LUT.
func easeOutCSS(t float32) float32 { return easeOutLUT.lookup(t) }

// EaseInOutCSS returns CSS "ease-in-out" curve. Uses precomputed LUT.
func easeInOutCSS(t float32) float32 { return easeInOutLUT.lookup(t) }

// CubicBezier creates a custom easing function from bezier
// control points. Works like CSS cubic-bezier().
func cubicBezier(x1, y1, x2, y2 float32) EasingFn {
	return func(t float32) float32 {
		return bezierCalc(t, x1, y1, x2, y2)
	}
}

// bezierCalc approximates cubic bezier curve value using
// Newton-Raphson iteration.
func bezierCalc(t, x1, y1, x2, y2 float32) float32 {
	guess := t
	for range 8 {
		x := bezierX(guess, x1, x2) - t
		if f32Abs(x) < 0.001 {
			break
		}
		dx := bezierDX(guess, x1, x2)
		if f32Abs(dx) < 0.000001 {
			break
		}
		guess -= x / dx
	}
	return bezierY(guess, y1, y2)
}

func bezierX(t, x1, x2 float32) float32 {
	u := 1 - t
	return 3*u*u*t*x1 + 3*u*t*t*x2 + t*t*t
}

func bezierY(t, y1, y2 float32) float32 {
	u := 1 - t
	return 3*u*u*t*y1 + 3*u*t*t*y2 + t*t*t
}

func bezierDX(t, x1, x2 float32) float32 {
	u := 1 - t
	return 3*u*u*x1 + 6*u*t*(x2-x1) + 3*t*t*(1-x2)
}
