package gui

import "math"

// ColorFilter holds a 4x4 column-major color transform matrix.
// Applied as a post-processing pass on container content.
// Operates on premultiplied-alpha pixels from the FBO.
// exportaudit:keep — reachable from an exported signature
type ColorFilter struct {
	matrix [16]float32
}

// Package-level singletons for constant filters (immutable, zero alloc).
var (
	colorFilterIdentity = ColorFilter{matrix: [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}}
	colorFilterGrayscale = ColorFilter{matrix: [16]float32{
		0.2126, 0.2126, 0.2126, 0,
		0.7152, 0.7152, 0.7152, 0,
		0.0722, 0.0722, 0.0722, 0,
		0, 0, 0, 1,
	}}
	colorFilterSepia = ColorFilter{matrix: [16]float32{
		0.393, 0.349, 0.272, 0,
		0.769, 0.686, 0.534, 0,
		0.189, 0.168, 0.131, 0,
		0, 0, 0, 1,
	}}
	colorFilterInvert = ColorFilter{matrix: [16]float32{
		-1, 0, 0, 0,
		0, -1, 0, 0,
		0, 0, -1, 0,
		1, 1, 1, 1,
	}}
)

// ColorFilterIdentity returns a no-op color filter. The result is a
// fresh copy: callers never alias the package singleton, so no future
// in-package mutation can leak across users.
// exportaudit:keep — collides with the colorFilterIdentity singleton var
func ColorFilterIdentity() *ColorFilter {
	f := colorFilterIdentity
	return &f
}

// ColorFilterGrayscale converts to luminance-weighted grayscale.
// Returns a copy, as ColorFilterIdentity does.
func ColorFilterGrayscale() *ColorFilter {
	f := colorFilterGrayscale
	return &f
}

// ColorFilterSepia applies a warm sepia tone.
// Returns a copy, as ColorFilterIdentity does.
func ColorFilterSepia() *ColorFilter {
	f := colorFilterSepia
	return &f
}

// ColorFilterSaturate adjusts saturation. 0=grayscale, 1=identity,
// >1=oversaturated.
func ColorFilterSaturate(amount float32) *ColorFilter {
	const lr, lg, lb = 0.2126, 0.7152, 0.0722
	s := amount
	return &ColorFilter{matrix: [16]float32{
		lr*(1-s) + s, lr * (1 - s), lr * (1 - s), 0,
		lg * (1 - s), lg*(1-s) + s, lg * (1 - s), 0,
		lb * (1 - s), lb * (1 - s), lb*(1-s) + s, 0,
		0, 0, 0, 1,
	}}
}

// ColorFilterBrightness scales RGB channels. 1=identity, <1=dim,
// >1=bright.
func ColorFilterBrightness(amount float32) *ColorFilter {
	return &ColorFilter{matrix: [16]float32{
		amount, 0, 0, 0,
		0, amount, 0, 0,
		0, 0, amount, 0,
		0, 0, 0, 1,
	}}
}

// ColorFilterContrast scales RGB around 0.5 midpoint. 1=identity,
// 0=all gray, >1=higher contrast. Bias injected via alpha column
// (correct for premultiplied-alpha FBO content).
func ColorFilterContrast(amount float32) *ColorFilter {
	bias := 0.5 * (1 - amount)
	return &ColorFilter{matrix: [16]float32{
		amount, 0, 0, 0,
		0, amount, 0, 0,
		0, 0, amount, 0,
		bias, bias, bias, 1,
	}}
}

// ColorFilterHueRotate rotates hue by the given angle in degrees.
// Rodrigues rotation around (1,1,1)/sqrt(3) in RGB space.
func ColorFilterHueRotate(degrees float32) *ColorFilter {
	rad := float64(degrees) * math.Pi / 180
	c := float32(math.Cos(rad))
	s := float32(math.Sin(rad))
	const k = 1.0 / 3.0
	sq := float32(1.0 / math.Sqrt(3))
	// Column-major 4x4. Top-left 3x3 is Rodrigues formula.
	return &ColorFilter{matrix: [16]float32{
		k + c*(1-k), k*(1-c) + s*sq, k*(1-c) - s*sq, 0,
		k*(1-c) - s*sq, k + c*(1-k), k*(1-c) + s*sq, 0,
		k*(1-c) + s*sq, k*(1-c) - s*sq, k + c*(1-k), 0,
		0, 0, 0, 1,
	}}
}

// ColorFilterInvert negates RGB, keeps alpha. Uses the alpha
// column to inject bias (output.rgb = alpha - input.rgb).
// Returns a copy, as ColorFilterIdentity does.
// exportaudit:keep — collides with the colorFilterInvert singleton var
func ColorFilterInvert() *ColorFilter {
	f := colorFilterInvert
	return &f
}

// ColorFilterCompose multiplies two color filters (a applied
// first, then b). Returns a new filter representing b*a; a nil side
// yields a copy of the other side, never an alias of it.
func colorFilterCompose(a, b *ColorFilter) *ColorFilter {
	if a == nil && b == nil {
		return nil
	}
	if a == nil {
		out := *b
		return &out
	}
	if b == nil {
		out := *a
		return &out
	}
	var out ColorFilter
	for col := range 4 {
		for row := range 4 {
			var sum float32
			for k := range 4 {
				sum += b.matrix[k*4+row] * a.matrix[col*4+k]
			}
			out.matrix[col*4+row] = sum
		}
	}
	return &out
}
