//go:build android

package android

/*
#include <stdlib.h>
#include "gles_android.h"
*/
import "C"

import (
	"log"
	"math"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
)

// renderersDraw iterates render commands and draws them.
func (b *Backend) renderersDraw(w *gui.Window) {
	cmds := w.Renderers()
	for i := range cmds {
		r := &cmds[i]
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderRect:
			b.drawRect(r)
		case gui.RenderStrokeRect:
			b.drawStrokeRect(r)
		case gui.RenderText:
			b.drawText(r)
		case gui.RenderCircle:
			b.drawCircle(r)
		case gui.RenderLine:
			b.drawLine(r)
		case gui.RenderShadow:
			b.drawShadow(r)
		case gui.RenderBlur:
			b.drawBlur(r)
		case gui.RenderGradient:
			b.drawGradient(w, r)
		case gui.RenderGradientBorder:
			b.drawGradientBorder(r)
		case gui.RenderImage:
			b.drawImage(r)
		case gui.RenderSvg:
			b.drawSvg(r)
		case gui.RenderLayout:
			b.drawLayout(r)
		case gui.RenderLayoutTransformed:
			b.drawLayoutTransformed(r)
		case gui.RenderTextPath:
			b.drawTextPath(r)
		case gui.RenderRTF:
			b.drawRtf(r)
		case gui.RenderCustomShader:
			b.drawCustomShader(r)
		case gui.RenderFilterBegin:
			b.beginFilter(r)
		case gui.RenderFilterEnd:
			b.endFilter()

		case gui.RenderStencilBegin:
			b.beginStencilClip(r)
		case gui.RenderStencilEnd:
			b.endStencilClip(r)

		case gui.RenderRotateBegin:
			b.beginRotation(r)
		case gui.RenderRotateEnd:
			b.endRotation()

		case gui.RenderNone,
			gui.RenderFilterComposite,
			gui.RenderLayoutPlaced:
		}
	}
}

// --- Individual draw commands ---

func (b *Backend) drawClip(r *gui.RenderCmd) {
	s := b.DPIScale
	x := int32(r.X * s)
	y := int32(r.Y * s)
	w := int32(r.W * s)
	h := int32(r.H * s)
	C.glesSetScissor(C.int(x), C.int(y), C.int(w), C.int(h),
		C.int(b.PhysH))
}

func (b *Backend) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	s := b.DPIScale
	b.SetPipeline(pipeSolid)
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawStrokeRect(r *gui.RenderCmd) {
	s := b.DPIScale
	b.SetPipeline(pipeSolid)
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, r.Thickness*s)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	s := b.DPIScale
	rad := r.Radius * s
	b.SetPipeline(pipeSolid)
	verts := gpu.BuildQuad(
		(r.X-r.Radius)*s,
		(r.Y-r.Radius)*s,
		2*rad, 2*rad,
		r.Color, rad, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawLine(r *gui.RenderCmd) {
	s := b.DPIScale
	x0 := r.X * s
	y0 := r.Y * s
	x1 := r.OffsetX * s
	y1 := r.OffsetY * s

	dx := x1 - x0
	dy := y1 - y0
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 0.001 {
		return
	}
	thick := max(r.Thickness*s, 1.0)
	nx := -dy / length * thick * 0.5
	ny := dx / length * thick * 0.5

	cr, cg, cb, ca := gpu.NormColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)

	verts := [4]vertex{
		{X: x0 + nx, Y: y0 + ny, Z: 0, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 + nx, Y: y1 + ny, Z: 0, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 - nx, Y: y1 - ny, Z: 0, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x0 - nx, Y: y0 - ny, Z: 0, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}

	b.SetPipeline(pipeSolid)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawShadow(r *gui.RenderCmd) {
	s := b.DPIScale
	x := (r.X + r.OffsetX) * s
	y := (r.Y + r.OffsetY) * s
	w := r.W * s
	h := r.H * s
	blur := r.BlurRadius * s
	rad := r.Radius * s
	spread := r.Spread * s

	// The quad must cover the ring beyond the caster, and the vertex
	// radius carries the inflated corner (rad+spread); the fragment
	// shader subtracts spread back out for the caster cut-out.
	expand := blur*1.5 + spread
	qx := x - expand
	qy := y - expand
	qw := w + 2*expand
	qh := h + 2*expand

	b.SetPipeline(pipeShadow)

	tm := gpu.IdentityTM()
	tm[12] = r.OffsetX * s
	tm[13] = r.OffsetY * s
	tm[14] = spread
	C.glesSetTM((*C.float)(&tm[0]))

	verts := gpu.BuildQuad(qx, qy, qw, qh, r.Color, rad+spread, blur)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawBlur(r *gui.RenderCmd) {
	s := b.DPIScale
	blur := r.BlurRadius * s
	rad := r.Radius * s
	expand := blur * 1.5

	b.SetPipeline(pipeBlur)
	tm := gpu.IdentityTM()
	C.glesSetTM((*C.float)(&tm[0]))

	verts := gpu.BuildQuad(
		r.X*s-expand, r.Y*s-expand,
		r.W*s+2*expand, r.H*s+2*expand,
		r.Color, rad+expand, blur)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawGradient(w *gui.Window, r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 ||
		r.W <= 0 || r.H <= 0 {
		return
	}
	s := b.DPIScale
	x := r.X * s
	y := r.Y * s
	width := r.W * s
	h := r.H * s
	rad := r.Radius * s

	stops := gui.NormalizeGradientStopsInto(
		r.Gradient.Stops, &b.NormBuf, &b.SampledBuf)
	if len(stops) == 0 {
		return
	}
	if len(stops) < len(r.Gradient.Stops) {
		w.DebugGradientResampled(r.X, r.Y, len(stops), len(r.Gradient.Stops))
	}

	tm := gpu.PackGradientUniforms(r.Gradient, stops, width, h)

	b.SetPipeline(pipeGradient)
	C.glesSetTM((*C.float)(&tm[0]))

	verts := gpu.BuildQuad(x, y, width, h, gui.White, rad, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	s := b.DPIScale
	rects := gui.GradientBorderRects(r)
	b.SetPipeline(pipeSolid)
	for i := range 4 {
		rc := &rects[i]
		verts := gpu.BuildQuad(rc.X*s, rc.Y*s, rc.W*s, rc.H*s,
			rc.Color, 0, 0)
		C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	}
}

func (b *Backend) drawImage(r *gui.RenderCmd) {
	tex, ok := b.resolveImageTexture(r.Resource)
	if !ok || tex.id == 0 {
		return
	}
	b.drawImageTex(r, tex)
}

// resolveImageTexture returns the uploaded texture for a render
// command's Resource, uploading on first use.
//
// A mem: source names a buffer in gui's in-memory registry: it has no
// path, so it skips path resolution and the AllowedImageRoots sandbox
// (see gui.LookupImage on why that is safe). The texture is cached
// under the Resource string itself, which callers content-key, so a
// changed buffer arrives as a new key and never reuses a stale upload.
func (b *Backend) resolveImageTexture(res string) (glesTexture, bool) {
	if iw, ih, pix, ok := gui.LookupImage(res); ok {
		tex, hit := b.textures.Get(res)
		if !hit {
			tex = createGLESTexture(int32(iw), int32(ih), pix)
			b.textures.Set(res, tex)
		}
		return tex, true
	}

	path := b.ResolveImagePath(res, "android")
	if path == "-" {
		return glesTexture{}, false
	}

	tex, ok := b.textures.Get(path)
	if !ok {
		var err error
		tex, err = b.loadImageTexture(path)
		if err != nil {
			log.Printf("android: drawImage: %v", err)
		}
		b.textures.Set(path, tex)
	}
	return tex, true
}

// drawImageTex paints an already-resolved texture into the command's
// rect, filling BgColor behind it first.
func (b *Backend) drawImageTex(
	r *gui.RenderCmd, tex glesTexture,
) {
	s := b.DPIScale
	x := r.X * s
	y := r.Y * s
	w := r.W * s
	h := r.H * s

	// Fill background.
	if r.Color.A > 0 {
		b.SetPipeline(pipeSolid)
		verts := gpu.BuildQuad(x, y, w, h, r.Color, 0, 0)
		C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	}

	b.SetPipeline(pipeImageClip)
	C.glesBindTexture(C.int(tex.id))

	z := gpu.PackParams(r.ClipRadius*s, 0)
	cr, cg, cb, ca := gpu.NormColor(255, 255, 255, 255)
	verts := [4]vertex{
		{X: x, Y: y, Z: z, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y, Z: z, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y + h, Z: z, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x, Y: y + h, Z: z, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawSvg(r *gui.RenderCmd) {
	if r.IsClipMask {
		return
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 {
		return
	}
	s := b.DPIScale
	numVerts := len(r.Triangles) / 2
	hasVCols := len(r.VertexColors) == numVerts
	vAlpha := float32(1)
	if r.HasVertexAlpha {
		vAlpha = max(0, min(r.VertexAlphaScale, 1))
	}

	hasXform := r.HasXform
	var sx, sy, tx, ty float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
	}
	hasRot := r.RotAngle != 0
	var sinA, cosA, rcx, rcy float32
	if hasRot {
		rad := float64(r.RotAngle) * math.Pi / 180
		sinA = float32(math.Sin(rad))
		cosA = float32(math.Cos(rad))
		rcx, rcy = r.RotCX, r.RotCY
	}

	if cap(b.SVGVerts) < numVerts {
		b.SVGVerts = make([]gpu.Vertex, numVerts)
	}
	verts := b.SVGVerts[:numVerts]
	var flatR, flatG, flatB, flatA float32
	if !hasVCols {
		flatR, flatG, flatB, flatA = gpu.NormColor(r.Color.R, r.Color.G,
			r.Color.B, r.Color.A)
	}
	for i := range numVerts {
		vx := r.Triangles[i*2]
		vy := r.Triangles[i*2+1]
		if hasXform {
			vx = vx*sx + tx
			vy = vy*sy + ty
		}
		if hasRot {
			dx := vx - rcx
			dy := vy - rcy
			vx = rcx + dx*cosA - dy*sinA
			vy = rcy + dx*sinA + dy*cosA
		}
		v := &verts[i]
		v.X = (r.X + vx*r.Scale) * s
		v.Y = (r.Y + vy*r.Scale) * s
		v.U = 0
		v.V = 0
		if hasVCols {
			vc := r.VertexColors[i]
			alpha := vc.A
			if r.HasVertexAlpha {
				alpha = uint8(float32(alpha) * vAlpha)
			}
			cr, cg, cb, ca := gpu.NormColor(vc.R, vc.G, vc.B, alpha)
			v.R = cr
			v.G = cg
			v.B = cb
			v.A = ca
		} else {
			v.R = flatR
			v.G = flatG
			v.B = flatB
			v.A = flatA
		}
	}

	b.SetPipeline(pipeSolid)
	C.glesDrawTriangles(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(numVerts))
}

func (b *Backend) drawText(r *gui.RenderCmd) {
	if b.textSys == nil || len(r.Text) == 0 {
		return
	}
	cfg := glyphconv.GuiTextConfigFromRender(r)

	b.UseGlyphPipeline()
	b.TextQueued = true
	if err := b.textSys.DrawText(r.X, r.Y, r.Text, cfg); err != nil && !b.textErrLogged {
		// Warn once per backend: a persistent failure (e.g. missing
		// font) would otherwise log every frame.
		b.textErrLogged = true
		log.Printf("android: DrawText: %v", err)
	}
	b.InvalidatePipelineState()
}

func (b *Backend) drawTextPath(r *gui.RenderCmd) {
	layout, placements, err := gui.ComputeTextPathPlacements(
		r, b.textSys, &b.TextPathPlacements,
		guiStyleToGlyphConfig)
	if err != nil {
		log.Printf("android: drawTextPath: %v", err)
		return
	}
	if len(placements) == 0 {
		return
	}
	b.TextPathPlacements = placements
	b.UseGlyphPipeline()
	b.TextQueued = true
	b.textSys.DrawLayoutPlaced(layout, placements)
	b.InvalidatePipelineState()
}

func (b *Backend) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	b.UseGlyphPipeline()
	b.TextQueued = true
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient,
		)
	} else {
		b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
	}
	b.InvalidatePipelineState()
}

func (b *Backend) drawLayoutTransformed(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil ||
		r.LayoutTransform == nil {
		return
	}
	b.UseGlyphPipeline()
	b.TextQueued = true
	if r.TextGradient != nil {
		b.textSys.DrawLayoutTransformedWithGradient(
			*r.LayoutPtr, r.X, r.Y,
			*r.LayoutTransform, r.TextGradient,
		)
	} else {
		b.textSys.DrawLayoutTransformed(
			*r.LayoutPtr, r.X, r.Y, *r.LayoutTransform,
		)
	}
	b.InvalidatePipelineState()
}

func (b *Backend) drawRtf(r *gui.RenderCmd) {
	b.drawLayout(r)
}

func (b *Backend) drawCustomShader(r *gui.RenderCmd) {
	if r.Shader == nil || r.Shader.GLSL == "" {
		return
	}

	h := gui.ShaderHash(r.Shader)
	idx, ok := b.customCache.Get(h)
	if !ok {
		if b.customCache.Len() >= maxCustomPipelines {
			b.customCache.EvictOldest()
		}
		glslES := buildCustomGLES3Fragment(r.Shader.GLSL)
		cstr := C.CString(glslES)
		idx = C.int(C.glesBuildCustomPipeline(cstr))
		C.free(unsafe.Pointer(cstr))
		if idx < 0 {
			return
		}
		b.customCache.Set(h, idx)
	}

	s := b.DPIScale
	C.glesSetCustomPipeline(idx)
	C.glesSetMVP((*C.float)(&b.MVP[0]))

	var tm [16]float32
	for i := range min(len(r.Shader.Params), 16) {
		tm[i] = r.Shader.Params[i]
	}
	C.glesSetTM((*C.float)(&tm[0]))

	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	b.InvalidatePipelineState()
}

func buildCustomGLES3Fragment(body string) string {
	return `#version 300 es
precision highp float;
precision highp int;

uniform sampler2D tex;
in vec2 uv;
in vec4 color;
in float params;
in vec4 p0;
in vec4 p1;
in vec4 p2;
in vec4 p3;

out vec4 _frag_out;

void main() {
    float radius = floor(params / 4096.0) / 4.0;

    vec2 uv_to_px = 1.0 / (vec2(fwidth(uv.x), fwidth(uv.y)) + 1e-6);
    vec2 half_size = uv_to_px;
    vec2 pos = uv * half_size;

    vec2 q = abs(pos) - half_size + vec2(radius);
    float d = length(max(q, 0.0)) + min(max(q.x, q.y), 0.0) - radius;

    float grad_len = length(vec2(dFdx(d), dFdy(d)));
    d = d / max(grad_len, 0.001);
    float sdf_alpha = 1.0 - smoothstep(-0.59, 0.59, d);

    // --- user body ---
    ` + body + `
    // --- end user body ---

    frag_color = vec4(frag_color.rgb, frag_color.a * sdf_alpha);

    if (frag_color.a < 0.0) {
        frag_color += texture(tex, uv);
    }
    _frag_out = frag_color;
}
`
}

// --- Stencil clip ---

func (b *Backend) beginStencilClip(r *gui.RenderCmd) {
	s := b.DPIScale
	b.SetPipeline(pipeSolid)
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	C.glesBeginStencilClip(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(r.StencilDepth))
	b.InvalidatePipelineState()
	b.SetPipeline(pipeSolid)
}

func (b *Backend) endStencilClip(r *gui.RenderCmd) {
	s := b.DPIScale
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	C.glesEndStencilClip(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(r.StencilDepth))
	b.InvalidatePipelineState()
	b.SetPipeline(pipeSolid)
}

// --- Rotation ---

func (b *Backend) beginRotation(r *gui.RenderCmd) {
	s := b.DPIScale
	b.BeginRotation(r.RotAngle, r.RotCX*s, r.RotCY*s)
}

func (b *Backend) endRotation() {
	b.EndRotation()
}

// --- Filter (glow) ---

func (b *Backend) beginFilter(r *gui.RenderCmd) {
	b.FilterBlur = r.BlurRadius * b.DPIScale
	b.FilterLayer = r.Layers
	b.FilterColorMatrix = r.ColorMatrix

	b.SetPipeline(pipeSolid)

	rc := C.glesBeginFilter(C.int(b.PhysW), C.int(b.PhysH))
	if rc != 0 {
		return
	}
	b.InvalidatePipelineState()
	b.SetPipeline(pipeSolid)
}

func (b *Backend) endFilter() {
	var cmPtr *C.float
	if b.FilterColorMatrix != nil {
		cmPtr = (*C.float)(&b.FilterColorMatrix[0])
	}
	C.glesEndFilter(C.float(b.FilterBlur),
		C.int(b.FilterLayer), cmPtr)
	b.InvalidatePipelineState()
	b.SetPipeline(pipeSolid)
}
