//go:build !js

package gl

import (
	"unsafe"

	gogl "github.com/go-gl/gl/v3.3-core/gl"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// vertex is a local alias for gpu.Vertex to avoid unkeyed
// composite literal warnings across packages.
type vertex = gpu.Vertex

// Vertex layout: position(vec3) + texcoord(vec2) + color(vec4)
// 9 float32s * 4 bytes = 36 bytes per vertex.
const vertexStride = 36

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

// drawQuad uploads 4 vertices and draws an indexed quad.
// UVs span -1..1 for SDF calculations in shaders.
func (b *Backend) drawQuad(x, y, w, h float32, c gui.Color,
	radius, thickness float32) {
	verts := gpu.BuildQuad(x, y, w, h, c, radius, thickness)

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
	z := gpu.PackParams(radius, 0)
	cr, cg, cb, ca := gpu.NormColor(c.R, c.G, c.B, c.A)

	verts := [4]gpu.Vertex{
		{X: x, Y: y, Z: z, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y, Z: z, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y + h, Z: z, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x, Y: y + h, Z: z, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
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
	cr, cg, cb, ca := gpu.NormColor(c.R, c.G, c.B, c.A)

	verts := [4]gpu.Vertex{
		{X: x, Y: y, Z: 0, U: 0, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y, Z: 0, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y + h, Z: 0, U: 1, V: 0, R: cr, G: cg, B: cb, A: ca},
		{X: x, Y: y + h, Z: 0, U: 0, V: 0, R: cr, G: cg, B: cb, A: ca},
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
func (b *Backend) uploadSvgVerts(verts []gpu.Vertex) {
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
