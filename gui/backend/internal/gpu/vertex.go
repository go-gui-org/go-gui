package gpu

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// Vertex is the CPU-side vertex for quad uploads, matching the SDF
// shader input layout: position(vec3) + texcoord(vec2) + color(vec4).
// 9 float32s = 36 bytes.
type Vertex struct {
	X, Y, Z    float32 // position; Z = packed params
	U, V       float32 // texcoord (-1..1 for SDF quads)
	R, G, B, A float32 // color (0..1)
}

// NormColor normalizes 0–255 color components to 0..1 floats.
func NormColor(r, g, b, a uint8) (rf, gf, bf, af float32) {
	return float32(r) / 255, float32(g) / 255,
		float32(b) / 255, float32(a) / 255
}

// PackParams packs radius and thickness into a single float32
// matching the shader unpacking: radius = ⌊p/4096⌋/4,
// thickness = mod(p,4096)/4.
// NaN/Inf inputs are clamped to 0 to avoid corrupting GPU uniforms.
func PackParams(radius, thickness float32) float32 {
	if isBadFloat(radius) {
		radius = 0
	}
	if isBadFloat(thickness) {
		thickness = 0
	}
	r := float32(math.Floor(float64(radius)*4)) * 4096
	t := float32(math.Floor(float64(thickness) * 4))
	return r + t
}

// isBadFloat reports whether f is NaN or ±Inf.
func isBadFloat(f float32) bool {
	return math.IsNaN(float64(f)) || math.IsInf(float64(f), 0)
}

// BuildQuad returns 4 vertices forming a screen-aligned SDF quad.
// UVs span -1..1 for SDF calculations in shaders.
func BuildQuad(x, y, w, h float32, c gui.Color,
	radius, thickness float32) [4]Vertex {
	z := PackParams(radius, thickness)
	r, g, b, a := NormColor(c.R, c.G, c.B, c.A)
	return [4]Vertex{
		{x, y, z, -1, -1, r, g, b, a},       // TL
		{x + w, y, z, 1, -1, r, g, b, a},    // TR
		{x + w, y + h, z, 1, 1, r, g, b, a}, // BR
		{x, y + h, z, -1, 1, r, g, b, a},    // BL
	}
}
