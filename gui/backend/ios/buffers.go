//go:build ios

package ios

import (
	"math"

	"github.com/go-gui-org/go-gui/gui"
)

// colF holds normalized (0..1) RGBA color components.
type colF struct{ r, g, b, a float32 }

// normColor normalizes 0-255 color components to 0..1 floats.
func normColor(r, g, b, a uint8) colF {
	return colF{float32(r) / 255, float32(g) / 255,
		float32(b) / 255, float32(a) / 255}
}

// vertex is the CPU-side vertex for quad uploads.
type vertex struct {
	X, Y, Z    float32 // position; Z = packed params
	U, V       float32 // texcoord (-1..1 for SDF quads)
	R, G, B, A float32 // color (0..1)
}

// packParams packs radius and thickness into a single float32
// matching the shader unpacking: radius = floor(p/4096)/4,
// thickness = mod(p,4096)/4.
func packParams(radius, thickness float32) float32 {
	r := float32(math.Floor(float64(radius)*4)) * 4096
	t := float32(math.Floor(float64(thickness) * 4))
	return r + t
}

// buildQuad fills a 4-vertex array for SDF quad rendering.
// UVs span -1..1 for SDF calculations in shaders.
func buildQuad(x, y, w, h float32, c gui.Color,
	radius, thickness float32) [4]vertex {
	z := packParams(radius, thickness)
	nc := normColor(c.R, c.G, c.B, c.A)
	return [4]vertex{
		{x, y, z, -1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y, z, 1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y + h, z, 1, 1, nc.r, nc.g, nc.b, nc.a},
		{x, y + h, z, -1, 1, nc.r, nc.g, nc.b, nc.a},
	}
}

// identityTM returns a 4x4 identity matrix for the TM uniform.
func identityTM() [16]float32 {
	return [16]float32{0: 1, 5: 1, 10: 1, 15: 1}
}

// ortho builds an orthographic projection matrix (column-major).
func ortho(m *[16]float32, l, r, b, t, n, f float32) {
	m[0] = 2 / (r - l)
	m[1] = 0
	m[2] = 0
	m[3] = 0

	m[4] = 0
	m[5] = 2 / (t - b)
	m[6] = 0
	m[7] = 0

	m[8] = 0
	m[9] = 0
	m[10] = -2 / (f - n)
	m[11] = 0

	m[12] = -(r + l) / (r - l)
	m[13] = -(t + b) / (t - b)
	m[14] = -(f + n) / (f - n)
	m[15] = 1
}
