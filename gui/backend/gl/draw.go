//go:build !js && !darwin && !android

package gl

import (
	"log"
	"math"
	"unsafe"

	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
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

		// Not emitted by the GL backend render path.
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
	// GL scissor Y is bottom-up.
	gogl.Enable(gogl.SCISSOR_TEST)
	gogl.Scissor(x, b.physH-y-h, w, h)
}

func (b *Backend) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	s := b.dpiScale
	b.usePipeline(&b.pipelines.solid)
	b.drawQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
}

func (b *Backend) drawStrokeRect(r *gui.RenderCmd) {
	s := b.dpiScale
	b.usePipeline(&b.pipelines.solid)
	b.drawQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, r.Thickness*s)
}

func (b *Backend) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	s := b.dpiScale
	rad := r.Radius * s
	b.usePipeline(&b.pipelines.solid)
	b.drawQuad(
		(r.X-r.Radius)*s,
		(r.Y-r.Radius)*s,
		2*rad, 2*rad,
		r.Color, rad, 0)
}

func (b *Backend) drawLine(r *gui.RenderCmd) {
	s := b.dpiScale
	x0 := r.X * s
	y0 := r.Y * s
	x1 := r.OffsetX * s
	y1 := r.OffsetY * s

	// Compute line quad from two endpoints.
	dx := x1 - x0
	dy := y1 - y0
	length := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if length < 0.001 {
		return
	}
	thick := max(r.Thickness*s, 1.0)
	// Normal perpendicular to line direction.
	nx := -dy / length * thick * 0.5
	ny := dx / length * thick * 0.5

	cr, cg, cb, ca := gpu.NormColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)

	verts := [4]vertex{
		{X: x0 + nx, Y: y0 + ny, Z: 0, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 + nx, Y: y1 + ny, Z: 0, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 - nx, Y: y1 - ny, Z: 0, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x0 - nx, Y: y0 - ny, Z: 0, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}

	b.usePipeline(&b.pipelines.solid)
	gogl.BindVertexArray(b.quadVAO)
	gogl.BindBuffer(gogl.ARRAY_BUFFER, b.quadVBO)
	gogl.BufferSubData(gogl.ARRAY_BUFFER, 0,
		4*vertexStride, vertPtr(&verts[0]))
	gogl.DrawElements(gogl.TRIANGLES, 6, gogl.UNSIGNED_SHORT, nil)
	gogl.BindVertexArray(0)
}

func (b *Backend) drawShadow(r *gui.RenderCmd) {
	s := b.dpiScale
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

	b.usePipeline(&b.pipelines.shadow)

	// Pack caster offset into tm matrix.
	tm := gpu.IdentityTM()
	// Offset from shadow center to caster center in shadow-local
	// pixel coordinates. Positive values move the caster clip in
	// the same direction as the configured shadow offset.
	tm[12] = r.OffsetX * s // tm[3].x
	tm[13] = r.OffsetY * s // tm[3].y
	tm[14] = spread        // tm[3].z: shadow growth beyond the caster
	gogl.UniformMatrix4fv(b.pipelines.shadow.uTM, 1, false,
		&tm[0])

	b.drawQuad(qx, qy, qw, qh, r.Color, rad+spread, blur)
}

func (b *Backend) drawBlur(r *gui.RenderCmd) {
	s := b.dpiScale
	blur := r.BlurRadius * s
	rad := r.Radius * s
	expand := blur * 1.5

	b.usePipeline(&b.pipelines.blur)
	tm := gpu.IdentityTM()
	gogl.UniformMatrix4fv(b.pipelines.blur.uTM, 1, false,
		&tm[0])

	b.drawQuad(
		r.X*s-expand, r.Y*s-expand,
		r.W*s+2*expand, r.H*s+2*expand,
		r.Color, rad+expand, blur)
}

func (b *Backend) drawGradient(w *gui.Window, r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 ||
		r.W <= 0 || r.H <= 0 {
		return
	}
	s := b.dpiScale
	x := r.X * s
	y := r.Y * s
	width := r.W * s
	h := r.H * s
	rad := r.Radius * s

	stops := gui.NormalizeGradientStopsInto(
		r.Gradient.Stops, &b.normBuf, &b.sampledBuf)
	if len(stops) == 0 {
		return
	}
	if len(stops) < len(r.Gradient.Stops) {
		w.DebugGradientResampled(r.X, r.Y, len(stops), len(r.Gradient.Stops))
	}

	tm, tm2 := gpu.PackGradientUniforms(r.Gradient, stops, width, h)

	b.usePipeline(&b.pipelines.gradient)
	gogl.UniformMatrix4fv(b.pipelines.gradient.uTM, 1, false,
		&tm[0])
	// Stops 4-7 go straight to the fragment stage; uniforms are
	// program-scoped in GL, so this is the same program object.
	gogl.UniformMatrix4fv(b.pipelines.gradient.uTM2, 1, false,
		&tm2[0])

	b.drawQuad(x, y, width, h, gui.White, rad, 0)
}

func (b *Backend) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	s := b.dpiScale
	rects := gui.GradientBorderRects(r)
	b.usePipeline(&b.pipelines.solid)
	for i := range 4 {
		rc := &rects[i]
		b.drawQuad(rc.X*s, rc.Y*s, rc.W*s, rc.H*s, rc.Color, 0, 0)
	}
}

func (b *Backend) drawImage(r *gui.RenderCmd) {
	tex, ok := b.resolveImageTexture(r.Resource)
	if !ok || tex.id == 0 {
		return
	}

	s := b.dpiScale
	x := r.X * s
	y := r.Y * s
	w := r.W * s
	h := r.H * s

	// Fill background.
	if r.Color.A > 0 {
		b.usePipeline(&b.pipelines.solid)
		b.drawQuad(x, y, w, h, r.Color, 0, 0)
	}

	gogl.ActiveTexture(gogl.TEXTURE0)
	gogl.BindTexture(gogl.TEXTURE_2D, tex.id)

	b.usePipeline(&b.pipelines.imageClip)
	if b.pipelines.imageClip.uTex >= 0 {
		gogl.Uniform1i(b.pipelines.imageClip.uTex, 0)
	}
	b.drawQuadUV(x, y, w, h, gui.White, r.ClipRadius*s)
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
}

// resolveImageTexture returns the uploaded texture for a render
// command's Resource, uploading on first use.
//
// A mem: source names a buffer in gui's in-memory registry: it has no
// path, so it skips path resolution and the AllowedImageRoots sandbox
// (see gui.LookupImage on why that is safe). The texture is cached
// under the Resource string itself, which callers content-key, so a
// changed buffer arrives as a new key and never reuses a stale upload.
func (b *Backend) resolveImageTexture(res string) (glTexture, bool) {
	if iw, ih, pix, ok := gui.LookupImage(res); ok {
		tex, hit := b.textures.Get(res)
		if !hit {
			tex = createTexture(int32(iw), int32(ih), pix)
			b.textures.Set(res, tex)
		}
		return tex, true
	}

	path, ok := b.imagePathCache.Get(res)
	if !ok {
		var err error
		path, err = imgload.ResolveValidatedPath(
			res, b.allowedImageRoots)
		if err != nil {
			log.Printf("gl: drawImage: %v", err)
			path = "-"
		}
		b.imagePathCache.Set(res, path)
	}
	if path == "-" {
		return glTexture{}, false
	}

	tex, ok := b.textures.Get(path)
	if !ok {
		var err error
		tex, err = b.loadImageTexture(path)
		if err != nil {
			log.Printf("gl: drawImage: %v", err)
		}
		b.textures.Set(path, tex)
	}
	return tex, true
}

func (b *Backend) drawSvg(r *gui.RenderCmd) {
	if r.IsClipMask {
		return // clip masks not yet supported in render pipeline
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
		b.svgVerts = make([]gpu.Vertex, numVerts)
	}
	verts := b.svgVerts[:numVerts]
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
			cr, cg, cb, ca := gpu.NormColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)
			v.R = cr
			v.G = cg
			v.B = cb
			v.A = ca
		}
	}

	b.usePipeline(&b.pipelines.solid)
	b.uploadSvgVerts(verts)
}

func (b *Backend) drawText(r *gui.RenderCmd) {
	if b.textSys == nil || len(r.Text) == 0 {
		return
	}
	if gui.DrawTextTransformed(r, b.textSys,
		guiStyleToGlyphConfig,
		func(layout glyph.Layout, grad *glyph.GradientConfig) {
			b.useGlyphPipeline()
			if grad != nil {
				b.textSys.DrawLayoutTransformedWithGradient(
					layout, r.X, r.Y, *r.LayoutTransform, grad)
			} else {
				b.textSys.DrawLayoutTransformed(
					layout, r.X, r.Y, *r.LayoutTransform)
			}
			b.restoreAfterGlyph()
		}) {
		return
	}
	cfg := glyphconv.GuiTextConfigFromRender(r)

	// Glyph renders with its own GL calls through the
	// glyphBackend. Need to use a simple textured-quad
	// pipeline for it.
	b.useGlyphPipeline()
	if err := b.textSys.DrawText(r.X, r.Y, r.Text, cfg); err != nil && !b.textErrLogged {
		// Warn once per backend: a persistent failure (e.g. missing
		// font) would otherwise log every frame.
		b.textErrLogged = true
		log.Printf("gl: DrawText: %v", err)
	}
	b.restoreAfterGlyph()
}

func (b *Backend) drawTextPath(r *gui.RenderCmd) {
	layout, placements, err := gui.ComputeTextPathPlacements(
		r, b.textSys, &b.textPathPlacements,
		guiStyleToGlyphConfig)
	if err != nil || len(placements) == 0 {
		return
	}
	b.textPathPlacements = placements
	b.useGlyphPipeline()
	b.textSys.DrawLayoutPlaced(layout, placements)
	b.restoreAfterGlyph()
}

func (b *Backend) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	b.useGlyphPipeline()
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient,
		)
		b.restoreAfterGlyph()
		return
	}
	b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
	b.restoreAfterGlyph()
}

func (b *Backend) drawLayoutTransformed(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil ||
		r.LayoutTransform == nil {
		return
	}
	b.useGlyphPipeline()
	if r.TextGradient != nil {
		b.textSys.DrawLayoutTransformedWithGradient(
			*r.LayoutPtr, r.X, r.Y,
			*r.LayoutTransform, r.TextGradient,
		)
		b.restoreAfterGlyph()
		return
	}
	b.textSys.DrawLayoutTransformed(
		*r.LayoutPtr, r.X, r.Y, *r.LayoutTransform,
	)
	b.restoreAfterGlyph()
}

func (b *Backend) drawRtf(r *gui.RenderCmd) {
	b.drawLayout(r)
}

func (b *Backend) drawCustomShader(r *gui.RenderCmd) {
	if r.Shader == nil || r.Shader.GLSL == "" {
		return
	}
	p, err := b.getOrBuildCustomPipeline(r.Shader)
	if err != nil {
		b.customOnce.Do(func() {
			log.Printf("gl: custom shader compile: %v", err)
		})
		return
	}

	s := b.dpiScale
	b.usePipeline(&p)

	// Pack params into tm matrix (up to 16 floats → 4 columns).
	var tm [16]float32
	for i := range min(len(r.Shader.Params), 16) {
		tm[i] = r.Shader.Params[i]
	}
	gogl.UniformMatrix4fv(p.uTM, 1, false, &tm[0])

	b.drawQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
}

// --- Filter (glow) ---

func (b *Backend) beginFilter(r *gui.RenderCmd) {
	if !b.ensureFilterFBO(b.physW, b.physH) {
		return
	}
	b.filterBlur = r.BlurRadius * b.dpiScale
	b.filterLayer = r.Layers
	b.filterColorMatrix = r.ColorMatrix

	b.bindFBO(b.filterTexA)
	gogl.Viewport(0, 0, b.physW, b.physH)
	gogl.ClearColor(0, 0, 0, 0)
	gogl.Clear(gogl.COLOR_BUFFER_BIT)
}

func (b *Backend) endFilter() {
	b.unbindFBO()
	gogl.Viewport(0, 0, b.physW, b.physH)

	layers := b.filterLayer
	layers = max(layers, 1)

	// compositeSrc tracks which texture holds the final result.
	compositeSrc := b.filterTexA

	// Blur passes (skip when blur < 1).
	if b.filterBlur >= 1 {
		stdDev := b.filterBlur

		// Horizontal pass A→B.
		b.bindFBO(b.filterTexB)
		gogl.ClearColor(0, 0, 0, 0)
		gogl.Clear(gogl.COLOR_BUFFER_BIT)
		b.usePipeline(&b.pipelines.filterBlurH)
		var tm [16]float32
		tm[0] = stdDev
		gogl.UniformMatrix4fv(b.pipelines.filterBlurH.uTM, 1,
			false, &tm[0])
		gogl.ActiveTexture(gogl.TEXTURE0)
		gogl.BindTexture(gogl.TEXTURE_2D, b.filterTexA)
		if b.pipelines.filterBlurH.uTex >= 0 {
			gogl.Uniform1i(b.pipelines.filterBlurH.uTex, 0)
		}
		b.drawQuadTex(0, 0, float32(b.physW), float32(b.physH),
			gui.White)
		gogl.BindTexture(gogl.TEXTURE_2D, 0)

		// Vertical pass B→A.
		b.bindFBO(b.filterTexA)
		gogl.ClearColor(0, 0, 0, 0)
		gogl.Clear(gogl.COLOR_BUFFER_BIT)
		b.usePipeline(&b.pipelines.filterBlurV)
		gogl.UniformMatrix4fv(b.pipelines.filterBlurV.uTM, 1,
			false, &tm[0])
		gogl.ActiveTexture(gogl.TEXTURE0)
		gogl.BindTexture(gogl.TEXTURE_2D, b.filterTexB)
		if b.pipelines.filterBlurV.uTex >= 0 {
			gogl.Uniform1i(b.pipelines.filterBlurV.uTex, 0)
		}
		b.drawQuadTex(0, 0, float32(b.physW), float32(b.physH),
			gui.White)
		gogl.BindTexture(gogl.TEXTURE_2D, 0)
		// After blur, result is in filterTexA.
	}

	// Color matrix pass A→B (if color matrix is set).
	if b.filterColorMatrix != nil {
		b.bindFBO(b.filterTexB)
		gogl.ClearColor(0, 0, 0, 0)
		gogl.Clear(gogl.COLOR_BUFFER_BIT)
		b.usePipeline(&b.pipelines.filterColor)
		gogl.UniformMatrix4fv(b.pipelines.filterColor.uTM, 1,
			false, &b.filterColorMatrix[0])
		gogl.ActiveTexture(gogl.TEXTURE0)
		gogl.BindTexture(gogl.TEXTURE_2D, b.filterTexA)
		if b.pipelines.filterColor.uTex >= 0 {
			gogl.Uniform1i(b.pipelines.filterColor.uTex, 0)
		}
		b.drawQuadTex(0, 0, float32(b.physW), float32(b.physH),
			gui.White)
		gogl.BindTexture(gogl.TEXTURE_2D, 0)
		compositeSrc = b.filterTexB
	}

	b.unbindFBO()
	gogl.Viewport(0, 0, b.physW, b.physH)

	// Composite: draw result texture once per layer at full alpha.
	b.usePipeline(&b.pipelines.filterTex)
	gogl.ActiveTexture(gogl.TEXTURE0)
	gogl.BindTexture(gogl.TEXTURE_2D, compositeSrc)
	if b.pipelines.filterTex.uTex >= 0 {
		gogl.Uniform1i(b.pipelines.filterTex.uTex, 0)
	}
	for range layers {
		b.drawQuadTex(0, 0, float32(b.physW), float32(b.physH),
			gui.White)
	}
	gogl.BindTexture(gogl.TEXTURE_2D, 0)
}

// --- Stencil clip ---

func (b *Backend) beginStencilClip(r *gui.RenderCmd) {
	s := b.dpiScale
	depth := r.StencilDepth

	// Enable stencil test.
	gogl.Enable(gogl.STENCIL_TEST)

	// Write to stencil: increment where SDF passes.
	gogl.StencilFunc(gogl.ALWAYS, 0, 0xFF)
	gogl.StencilOp(gogl.KEEP, gogl.KEEP, gogl.INCR)
	gogl.ColorMask(false, false, false, false)

	// Draw rounded-rect mask using stencil pipeline.
	b.usePipeline(&b.pipelines.stencil)
	b.drawQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)

	// Restore color writes; set stencil test for children.
	gogl.ColorMask(true, true, true, true)
	gogl.StencilFunc(gogl.LEQUAL, int32(depth), 0xFF)
	gogl.StencilOp(gogl.KEEP, gogl.KEEP, gogl.KEEP)
}

func (b *Backend) endStencilClip(r *gui.RenderCmd) {
	s := b.dpiScale
	depth := r.StencilDepth

	// Decrement stencil where SDF passes.
	gogl.StencilFunc(gogl.ALWAYS, 0, 0xFF)
	gogl.StencilOp(gogl.KEEP, gogl.KEEP, gogl.DECR)
	gogl.ColorMask(false, false, false, false)

	b.usePipeline(&b.pipelines.stencil)
	b.drawQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)

	gogl.ColorMask(true, true, true, true)

	if depth <= 1 {
		gogl.Disable(gogl.STENCIL_TEST)
	} else {
		gogl.StencilFunc(gogl.LEQUAL, int32(depth-1), 0xFF)
		gogl.StencilOp(gogl.KEEP, gogl.KEEP, gogl.KEEP)
	}
}

// --- Glyph pipeline helpers ---

// useGlyphPipeline sets up minimal GL state for glyph's
// DrawBackend to render textured quads. Glyph uses its own
// VAO/VBO but needs blend and our projection active.
func (b *Backend) useGlyphPipeline() {
	// Glyph draws through its own backend which issues raw GL
	// calls. We need a simple textured-quad shader active.
	b.usePipeline(&b.pipelines.filterTex)
	if b.pipelines.filterTex.uTex >= 0 {
		gogl.Uniform1i(b.pipelines.filterTex.uTex, 0)
	}
	tm := gpu.IdentityTM()
	gogl.UniformMatrix4fv(b.pipelines.filterTex.uTM, 1, false,
		&tm[0])
}

func (b *Backend) restoreAfterGlyph() {
	// Re-bind the quad VAO in case glyph changed it.
	gogl.BindVertexArray(0)
}

// --- Helpers ---

func vertPtr(v *vertex) unsafe.Pointer {
	return unsafe.Pointer(v)
}
