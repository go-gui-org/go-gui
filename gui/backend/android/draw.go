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

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
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
			b.drawGradient(r)
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
	s := b.dpiScale
	x := int32(r.X * s)
	y := int32(r.Y * s)
	w := int32(r.W * s)
	h := int32(r.H * s)
	C.glesSetScissor(C.int(x), C.int(y), C.int(w), C.int(h),
		C.int(b.physH))
}

func (b *Backend) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	s := b.dpiScale
	b.setPipeline(pipeSolid)
	verts := buildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawStrokeRect(r *gui.RenderCmd) {
	s := b.dpiScale
	b.setPipeline(pipeSolid)
	verts := buildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, r.Thickness*s)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	s := b.dpiScale
	rad := r.Radius * s
	b.setPipeline(pipeSolid)
	verts := buildQuad(
		(r.X-r.Radius)*s,
		(r.Y-r.Radius)*s,
		2*rad, 2*rad,
		r.Color, rad, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawLine(r *gui.RenderCmd) {
	s := b.dpiScale
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

	nc := normColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)

	verts := [4]vertex{
		{x0 + nx, y0 + ny, 0, -1, -1, nc.r, nc.g, nc.b, nc.a},
		{x1 + nx, y1 + ny, 0, 1, -1, nc.r, nc.g, nc.b, nc.a},
		{x1 - nx, y1 - ny, 0, 1, 1, nc.r, nc.g, nc.b, nc.a},
		{x0 - nx, y0 - ny, 0, -1, 1, nc.r, nc.g, nc.b, nc.a},
	}

	b.setPipeline(pipeSolid)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawShadow(r *gui.RenderCmd) {
	s := b.dpiScale
	x := (r.X + r.OffsetX) * s
	y := (r.Y + r.OffsetY) * s
	w := r.W * s
	h := r.H * s
	blur := r.BlurRadius * s
	rad := r.Radius * s

	expand := blur * 1.5
	qx := x - expand
	qy := y - expand
	qw := w + 2*expand
	qh := h + 2*expand

	b.setPipeline(pipeShadow)

	tm := identityTM()
	tm[12] = r.OffsetX * s
	tm[13] = r.OffsetY * s
	C.glesSetTM((*C.float)(&tm[0]))

	verts := buildQuad(qx, qy, qw, qh, r.Color, rad, blur)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawBlur(r *gui.RenderCmd) {
	s := b.dpiScale
	blur := r.BlurRadius * s
	rad := r.Radius * s
	expand := blur * 1.5

	b.setPipeline(pipeBlur)
	tm := identityTM()
	C.glesSetTM((*C.float)(&tm[0]))

	verts := buildQuad(
		r.X*s-expand, r.Y*s-expand,
		r.W*s+2*expand, r.H*s+2*expand,
		r.Color, rad+expand, blur)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawGradient(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 ||
		r.W <= 0 || r.H <= 0 {
		return
	}
	s := b.dpiScale
	x := r.X * s
	y := r.Y * s
	w := r.W * s
	h := r.H * s
	rad := r.Radius * s

	stops := gui.NormalizeGradientStopsInto(
		r.Gradient.Stops, &b.normBuf, &b.sampledBuf)
	if len(stops) == 0 {
		return
	}

	var tm [16]float32
	for i := range min(len(stops), 5) {
		col := i / 2
		row := (i % 2) * 2
		tm[col*4+row] = gui.PackRGB(stops[i].Color)
		tm[col*4+row+1] = gui.PackAlphaPos(
			stops[i].Color, stops[i].Pos)
	}

	if r.Gradient.Type == gui.GradientRadial {
		tm[2*4+3] = max(w/2, h/2)
		tm[3*4+2] = 1.0
	} else {
		dx, dy := gui.GradientDir(r.Gradient, r.W, r.H)
		tm[2*4+2] = dx
		tm[2*4+3] = dy
		tm[3*4+2] = 0.0
	}

	tm[3*4+0] = w / 2
	tm[3*4+1] = h / 2
	tm[3*4+3] = float32(len(stops))

	b.setPipeline(pipeGradient)
	C.glesSetTM((*C.float)(&tm[0]))

	verts := buildQuad(x, y, w, h, gui.White, rad, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
}

func (b *Backend) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	s := b.dpiScale
	th := r.Thickness * s
	positions := [4]float32{0.0, 0.25, 0.5, 0.75}
	type rect struct{ x, y, w, h float32 }
	rects := [4]rect{
		{r.X * s, r.Y * s, r.W * s, th},
		{r.X * s, (r.Y+r.H)*s - th, r.W * s, th},
		{r.X * s, r.Y * s, th, r.H * s},
		{(r.X+r.W)*s - th, r.Y * s, th, r.H * s},
	}
	b.setPipeline(pipeSolid)
	for i := range 4 {
		c := gui.SampleGradientStopColor(
			r.Gradient.Stops, positions[i])
		rc := rects[i]
		verts := buildQuad(rc.x, rc.y, rc.w, rc.h, c, 0, 0)
		C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	}
}

func (b *Backend) drawImage(r *gui.RenderCmd) {
	path, ok := b.imagePathCache.Get(r.Resource)
	if !ok {
		var err error
		path, err = imgload.ResolveValidatedPath(
			r.Resource, b.allowedImageRoots)
		if err != nil {
			log.Printf("android: drawImage: %v",
				err)
			path = "-"
		}
		b.imagePathCache.Set(r.Resource, path)
	}
	if path == "-" {
		return
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
	if tex.id == 0 {
		return
	}

	s := b.dpiScale
	x := r.X * s
	y := r.Y * s
	w := r.W * s
	h := r.H * s

	// Fill background.
	if r.Color.A > 0 {
		b.setPipeline(pipeSolid)
		verts := buildQuad(x, y, w, h, r.Color, 0, 0)
		C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	}

	b.setPipeline(pipeImageClip)
	C.glesBindTexture(C.int(tex.id))

	z := packParams(r.ClipRadius*s, 0)
	nc := normColor(255, 255, 255, 255)
	verts := [4]vertex{
		{x, y, z, -1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y, z, 1, -1, nc.r, nc.g, nc.b, nc.a},
		{x + w, y + h, z, 1, 1, nc.r, nc.g, nc.b, nc.a},
		{x, y + h, z, -1, 1, nc.r, nc.g, nc.b, nc.a},
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
	s := b.dpiScale
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

	if cap(b.svgVerts) < numVerts {
		b.svgVerts = make([]vertex, numVerts)
	}
	verts := b.svgVerts[:numVerts]
	var flatNC colF
	if !hasVCols {
		flatNC = normColor(r.Color.R, r.Color.G,
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
			nc := normColor(vc.R, vc.G, vc.B, alpha)
			v.R = nc.r
			v.G = nc.g
			v.B = nc.b
			v.A = nc.a
		} else {
			v.R = flatNC.r
			v.G = flatNC.g
			v.B = flatNC.b
			v.A = flatNC.a
		}
	}

	b.setPipeline(pipeSolid)
	C.glesDrawTriangles(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(numVerts))
}

func (b *Backend) drawText(r *gui.RenderCmd) {
	if b.textSys == nil || len(r.Text) == 0 {
		return
	}
	var cfg glyph.TextConfig
	if r.TextStylePtr != nil {
		cfg = guiStyleToGlyphConfig(*r.TextStylePtr)
		cfg.Gradient = r.TextGradient
	} else {
		cfg = glyph.TextConfig{
			Style: glyph.TextStyle{
				FontName: r.FontName,
				Size:     r.FontSize,
				Color: glyph.Color{
					R: r.Color.R,
					G: r.Color.G,
					B: r.Color.B,
					A: r.Color.A,
				},
			},
			Block: glyph.DefaultBlockStyle(),
		}
	}
	if r.W > 0 {
		cfg.Block.Wrap = glyph.WrapWord
		cfg.Block.Width = r.W
	}

	b.useGlyphPipeline()
	b.textQueued = true
	if err := b.textSys.DrawText(r.X, r.Y, r.Text, cfg); err != nil {
		log.Printf("android: DrawText: %v", err)
	}
	b.invalidatePipelineState()
}

func (b *Backend) drawTextPath(r *gui.RenderCmd) {
	if b.textSys == nil || r.TextPath == nil ||
		r.TextStylePtr == nil {
		return
	}
	tp := r.TextPath
	cfg := guiStyleToGlyphConfig(*r.TextStylePtr)
	layout, err := b.textSys.LayoutTextCached(r.Text, cfg)
	if err != nil {
		log.Printf("android: drawTextPath: %v", err)
		return
	}
	positions := layout.GlyphPositions()
	if len(positions) == 0 {
		return
	}

	var totalAdvance float32
	for _, p := range positions {
		totalAdvance += p.Advance
	}

	offset := tp.Offset
	if tp.Anchor == gui.SvgTextAnchorMiddle {
		offset -= totalAdvance / 2
	} else if tp.Anchor == gui.SvgTextAnchorEnd {
		offset -= totalAdvance
	}

	advScale := float32(1)
	if tp.Method == gui.SvgTextPathMethodStretch && totalAdvance > 0 {
		remaining := tp.TotalLen - offset
		if remaining > 0 {
			advScale = remaining / totalAdvance
		}
	}

	n := len(layout.Glyphs)
	if cap(b.textPathPlacements) < n {
		b.textPathPlacements = make([]glyph.GlyphPlacement, n)
	}
	placements := b.textPathPlacements[:n]
	for i := range placements {
		placements[i] = glyph.GlyphPlacement{
			X: -9999, Y: -9999,
		}
	}

	cumAdv := float32(0)
	for _, p := range positions {
		advance := p.Advance * advScale
		centerDist := offset + cumAdv + advance/2
		px, py, angle := gui.SamplePathAt(
			tp.Polyline, tp.Table, centerDist)

		halfAdv := advance / 2
		cosAngle := float32(math.Cos(float64(angle)))
		sinAngle := float32(math.Sin(float64(angle)))
		gx := px + r.X - halfAdv*cosAngle
		gy := py + r.Y - halfAdv*sinAngle

		placements[p.Index] = glyph.GlyphPlacement{
			X: gx, Y: gy, Angle: angle,
		}
		cumAdv += advance
	}

	b.useGlyphPipeline()
	b.textQueued = true
	b.textSys.DrawLayoutPlaced(layout, placements)
	b.invalidatePipelineState()
}

func (b *Backend) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	b.useGlyphPipeline()
	b.textQueued = true
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient,
		)
	} else {
		b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
	}
	b.invalidatePipelineState()
}

func (b *Backend) drawLayoutTransformed(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil ||
		r.LayoutTransform == nil {
		return
	}
	b.useGlyphPipeline()
	b.textQueued = true
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
	b.invalidatePipelineState()
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

	s := b.dpiScale
	C.glesSetCustomPipeline(idx)
	C.glesSetMVP((*C.float)(&b.mvp[0]))

	var tm [16]float32
	for i := range min(len(r.Shader.Params), 16) {
		tm[i] = r.Shader.Params[i]
	}
	C.glesSetTM((*C.float)(&tm[0]))

	verts := buildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
	C.glesDrawQuad((*C.float)(unsafe.Pointer(&verts[0])))
	b.invalidatePipelineState()
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
	s := b.dpiScale
	b.setPipeline(pipeSolid)
	verts := buildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	C.glesBeginStencilClip(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(r.StencilDepth))
	b.invalidatePipelineState()
	b.setPipeline(pipeSolid)
}

func (b *Backend) endStencilClip(r *gui.RenderCmd) {
	s := b.dpiScale
	verts := buildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	C.glesEndStencilClip(
		(*C.float)(unsafe.Pointer(&verts[0])),
		C.int(r.StencilDepth))
	b.invalidatePipelineState()
	b.setPipeline(pipeSolid)
}

// --- Rotation ---

func (b *Backend) beginRotation(r *gui.RenderCmd) {
	b.mvpStack = append(b.mvpStack, b.mvp)
	s := b.dpiScale
	cx := r.RotCX * s
	cy := r.RotCY * s
	applyRotation(&b.mvp, r.RotAngle, cx, cy)
	b.mvpDirty = true
	b.setPipeline(pipeSolid)
}

func (b *Backend) endRotation() {
	n := len(b.mvpStack)
	if n == 0 {
		return
	}
	b.mvp = b.mvpStack[n-1]
	b.mvpStack = b.mvpStack[:n-1]
	b.mvpDirty = true
	b.setPipeline(pipeSolid)
}

// --- Filter (glow) ---

func (b *Backend) beginFilter(r *gui.RenderCmd) {
	b.filterBlur = r.BlurRadius * b.dpiScale
	b.filterLayer = r.Layers
	b.filterColorMatrix = r.ColorMatrix

	b.setPipeline(pipeSolid)

	rc := C.glesBeginFilter(C.int(b.physW), C.int(b.physH))
	if rc != 0 {
		return
	}
	b.invalidatePipelineState()
	b.setPipeline(pipeSolid)
}

func (b *Backend) endFilter() {
	var cmPtr *C.float
	if b.filterColorMatrix != nil {
		cmPtr = (*C.float)(&b.filterColorMatrix[0])
	}
	C.glesEndFilter(C.float(b.filterBlur),
		C.int(b.filterLayer), cmPtr)
	b.invalidatePipelineState()
	b.setPipeline(pipeSolid)
}
