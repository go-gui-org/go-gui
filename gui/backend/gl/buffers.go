//go:build !js

package gl

import (
	"math"
	"unsafe"

	gogl "github.com/go-gl/gl/v3.3-core/gl"

	"github.com/go-gui-org/go-gui/gui"
)

// colF holds normalized (0..1) RGBA color components.
type colF struct{ r, g, b, a float32 }

// normColor normalizes 0-255 color components to 0..1 floats.
func normColor(r, g, b, a uint8) colF {
	return colF{float32(r) / 255, float32(g) / 255,
		float32(b) / 255, float32(a) / 255}
}

// Vertex layout: position(vec3) + texcoord(vec2) + color(vec4)
// 9 floats * 4 bytes = 36 bytes per vertex.
const vertexStride = 9 * 4

// vertex is the CPU-side vertex for quad uploads.
type vertex struct {
	X, Y, Z    float32 // position; Z = packed params
	U, V       float32 // texcoord (-1..1 for SDF quads)
	R, G, B, A float32 // color (0..1)
}

func (b *Backend) initQuadBuffers() {
	gogl.GenVertexArrays(1, &b.quadVAO)
	gogl.GenBuffers(1, &b.quadVBO)
	gogl.GenBuffers(1, &b.quadIBO)

	gogl.BindVertexArray(b.quadVAO)

	// Allocate VBO for 4 vertices.
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.quadVBO)
	gogl.BufferData(gogl.ARRAY_BUFFER, 4*vertexStride,
		nil, gogl.DYNAMIC_DRAW)

	// Index buffer: two triangles forming a quad.
	indices := [6]uint16{0, 1, 2, 0, 2, 3}
	gogl.BindBuffer(gogl.ELEMENT_ARRAY_BUFFER, b.quadIBO)
	gogl.BufferData(gogl.ELEMENT_ARRAY_BUFFER,
		int(unsafe.Sizeof(indices)),
		unsafe.Pointer(&indices[0]), gogl.STATIC_DRAW)

	setupVertexAttribs()
	gogl.BindVertexArray(0)
}

func (b *Backend) initSvgBuffers() {
	gogl.GenVertexArrays(1, &b.svgVAO)
	gogl.GenBuffers(1, &b.svgVBO)

	gogl.BindVertexArray(b.svgVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.svgVBO)
	gogl.BufferData(gogl.ARRAY_BUFFER, 1024*vertexStride,
		nil, gogl.DYNAMIC_DRAW)
	b.svgCap = 1024

	setupVertexAttribs()
	gogl.BindVertexArray(0)
}

func setupVertexAttribs() {
	// location=0: position (vec3)
	gogl.EnableVertexAttribArray(0)
	gogl.VertexAttribPointerWithOffset(0, 3, gogl.FLOAT, false,
		vertexStride, 0)
	// location=1: texcoord0 (vec2)
	gogl.EnableVertexAttribArray(1)
	gogl.VertexAttribPointerWithOffset(1, 2, gogl.FLOAT, false,
		vertexStride, 3*4)
	// location=2: color0 (vec4)
	gogl.EnableVertexAttribArray(2)
	gogl.VertexAttribPointerWithOffset(2, 4, gogl.FLOAT, false,
		vertexStride, 5*4)
}

// packParams packs radius and thickness into a single float32
// matching the shader unpacking: radius = floor(p/4096)/4,
// thickness = mod(p,4096)/4.
func packParams(radius, thickness float32) float32 {
	r := float32(math.Floor(float64(radius)*4)) * 4096
	t := float32(math.Floor(float64(thickness) * 4))
	return r + t
}

// drawQuad uploads 4 vertices and draws an indexed quad.
// UVs span -1..1 for SDF calculations in shaders.
func (b *Backend) drawQuad(x, y, w, h float32, c gui.Color,
	radius, thickness float32) {
	z := packParams(radius, thickness)
	nc := normColor(c.R, c.G, c.B, c.A)

	verts := [4]vertex{
		{x, y, z, -1, -1, nc.r, nc.g, nc.b, nc.a},       // TL
		{x + w, y, z, 1, -1, nc.r, nc.g, nc.b, nc.a},    // TR
		{x + w, y + h, z, 1, 1, nc.r, nc.g, nc.b, nc.a}, // BR
		{x, y + h, z, -1, 1, nc.r, nc.g, nc.b, nc.a},    // BL
	}

	gogl.BindVertexArray(b.quadVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.quadVBO)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawElements(gogl.TRIANGLES, 6, gogl.UNSIGNED_SHORT,
		nil)
	gogl.BindVertexArray(0)
}

// drawQuadUV draws a quad with custom UV coordinates (0..1 range
// for texture sampling).
func (b *Backend) drawQuadUV(x, y, w, h float32, c gui.Color,
	radius float32) {
	z := packParams(radius, 0)
	nc := normColor(c.R, c.G, c.B, c.A)

	verts := [4]vertex{
		{x, y, z, -1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y, z, 1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y + h, z, 1, 1, nc.r, nc.g, nc.b, nc.a},
		{x, y + h, z, -1, 1, nc.r, nc.g, nc.b, nc.a},
	}

	gogl.BindVertexArray(b.quadVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.quadVBO)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawElements(gogl.TRIANGLES, 6, gogl.UNSIGNED_SHORT,
		nil)
	gogl.BindVertexArray(0)
}

// drawQuadTex draws a quad with texture UVs in 0..1 range, for
// compositing FBO textures or images.
func (b *Backend) drawQuadTex(x, y, w, h float32, c gui.Color) {
	nc := normColor(c.R, c.G, c.B, c.A)

	verts := [4]vertex{
		{x, y, 0, 0, 1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y, 0, 1, 1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y + h, 0, 1, 0, nc.r, nc.g, nc.b, nc.a},
		{x, y + h, 0, 0, 0, nc.r, nc.g, nc.b, nc.a},
	}

	gogl.BindVertexArray(b.quadVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.quadVBO)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		int(unsafe.Sizeof(verts)), unsafe.Pointer(&verts[0]))
	gogl.DrawElements(gogl.TRIANGLES, 6, gogl.UNSIGNED_SHORT,
		nil)
	gogl.BindVertexArray(0)
}

// uploadSvgVerts uploads an arbitrary vertex array for SVG
// triangle rendering.
func (b *Backend) uploadSvgVerts(verts []vertex) {
	n := len(verts)
	if n == 0 {
		return
	}
	gogl.BindVertexArray(b.svgVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.svgVBO)
	size := n * vertexStride
	if n > b.svgCap {
		gogl.BufferData(gogl.ARRAY_BUFFER, size,
			unsafe.Pointer(&verts[0]), gogl.DYNAMIC_DRAW)
		b.svgCap = n
	} else {
		gogl.BufferSubData(gogl.ARRAY_BUFFER, 0, size,
			unsafe.Pointer(&verts[0]))
	}
	gogl.DrawArrays(gogl.TRIANGLES, 0, int32(n))
	gogl.BindVertexArray(0)
}
