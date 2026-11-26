//go:build darwin && arm64

package mtldraw

import (
	"log"
	"math"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/glyphconv"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
)

// renderersDraw iterates render commands and draws them.
func (b *Renderer) renderersDraw(w *gui.Window) {
	b.surfaces.prune(w.Renderers())
	b.sweepMemTextures()
	// Textures drawn last frame stay safe from the byte budget (#1239).
	b.textures.BeginFrame()
	// A truncated command list must not leak a rotated MVP into the
	// next frame. Filter/stencil targets are per-frame encoder state
	// owned by metalCtx.beginFrame/endFrame; only the MVP stack
	// persists. A balanced stream ends restored, so no-op.
	savedMVP := b.mvp
	savedStackLen := len(b.mvpStack)
	defer func() {
		b.mvp = savedMVP
		b.mvpStack = b.mvpStack[:savedStackLen]
		b.ctx.setPipeline(pipeSolid)
		b.ctx.setMVP(&b.mvp)
	}()
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
		case gui.RenderTransformBegin:
			b.beginTransform(r)
		case gui.RenderTransformEnd:
			b.endRotation()

		case gui.RenderNone,
			gui.RenderFilterComposite,
			gui.RenderLayoutPlaced:
		}
	}
}

// --- Individual draw commands ---

// drawQuad batches one SDF quad from b.quad. Its address can reach
// Metal as a uintptr (metalCtx.drawInline), which does not keep a stack
// local in place, so the vertices go through the window's scratch field,
// on the heap. Main thread only, like every draw call: the field is
// reused by the next draw.
func (b *Renderer) drawQuad(v [4]vertex) {
	b.quad = v
	b.ctx.drawMainQuad(unsafe.Pointer(&b.quad))
}

// setTM binds a per-draw uniform matrix from b.tm. The renderer copies
// it, and sends nothing when it is already bound.
func (b *Renderer) setTM(tm [16]float32) {
	b.tm = tm
	b.ctx.setTM(&b.tm)
}

func (b *Renderer) drawClip(r *gui.RenderCmd) {
	// Rounds outward: floor the near edge, ceil the far one, so a
	// fractional DPI scale never shaves the right or bottom pixel.
	x, y, w, h := gpu.ClipRect(r.X, r.Y, r.W, r.H, b.dpiScale)
	b.ctx.setScissor(int(x), int(y), int(w), int(h))
}

func (b *Renderer) drawRect(r *gui.RenderCmd) {
	if !r.Fill {
		return
	}
	s := b.dpiScale
	x, y, w, h := gpu.SnapRect(r.X, r.Y, r.W, r.H, s)
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	verts := gpu.BuildQuad(x, y, w, h,
		r.Color, r.Radius*s, 0)
	b.drawQuad(verts)
}

func (b *Renderer) drawStrokeRect(r *gui.RenderCmd) {
	s := b.dpiScale
	x, y, w, h := gpu.SnapRect(r.X, r.Y, r.W, r.H, s)
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	verts := gpu.BuildQuad(x, y, w, h,
		r.Color, r.Radius*s, gpu.SnapThickness(r.Thickness, s))
	b.drawQuad(verts)
}

func (b *Renderer) drawCircle(r *gui.RenderCmd) {
	if !r.Fill || r.Radius <= 0 {
		return
	}
	s := b.dpiScale
	rad := r.Radius * s
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	verts := gpu.BuildQuad(
		(r.X-r.Radius)*s,
		(r.Y-r.Radius)*s,
		2*rad, 2*rad,
		r.Color, rad, 0)
	b.drawQuad(verts)
}

func (b *Renderer) drawLine(r *gui.RenderCmd) {
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

	cr, cg, cb, ca := gpu.NormColor(r.Color.R, r.Color.G, r.Color.B, r.Color.A)

	verts := [4]vertex{
		{X: x0 + nx, Y: y0 + ny, Z: 0, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 + nx, Y: y1 + ny, Z: 0, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x1 - nx, Y: y1 - ny, Z: 0, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x0 - nx, Y: y0 - ny, Z: 0, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}

	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	b.drawQuad(verts)
}

func (b *Renderer) drawShadow(r *gui.RenderCmd) {
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

	b.ctx.setPipeline(pipeShadow)
	b.ctx.setMVP(&b.mvp)

	tm := gpu.IdentityTM()
	tm[12] = r.OffsetX * s
	tm[13] = r.OffsetY * s
	tm[14] = spread
	b.setTM(tm)

	verts := gpu.BuildQuad(qx, qy, qw, qh, r.Color, rad+spread, blur)
	b.drawQuad(verts)
}

func (b *Renderer) drawBlur(r *gui.RenderCmd) {
	s := b.dpiScale
	blur := r.BlurRadius * s
	rad := r.Radius * s
	expand := blur * 1.5

	b.ctx.setPipeline(pipeBlur)
	b.ctx.setMVP(&b.mvp)
	tm := gpu.IdentityTM()
	b.setTM(tm)

	verts := gpu.BuildQuad(
		r.X*s-expand, r.Y*s-expand,
		r.W*s+2*expand, r.H*s+2*expand,
		r.Color, rad+expand, blur)
	b.drawQuad(verts)
}

func (b *Renderer) drawGradient(w *gui.Window, r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 ||
		r.W <= 0 || r.H <= 0 {
		return
	}
	s := b.dpiScale
	x, y, width, h := gpu.SnapRect(r.X, r.Y, r.W, r.H, s)
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

	b.ctx.setPipeline(pipeGradient)
	b.ctx.setMVP(&b.mvp)
	b.setTM(tm)
	b.tm2 = tm2
	b.ctx.setFragment0(&b.tm2)

	verts := gpu.BuildQuad(x, y, width, h, gui.White, rad, 0)
	b.drawQuad(verts)
}

func (b *Renderer) drawGradientBorder(r *gui.RenderCmd) {
	if r.Gradient == nil || len(r.Gradient.Stops) == 0 {
		return
	}
	s := b.dpiScale
	rects := gui.GradientBorderRects(r)
	strips := gpu.SnapBorderStrips(r.X, r.Y, r.W, r.H, r.Thickness, s)
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	for i := range 4 {
		rc := &rects[i]
		st := &strips[i]
		bx, by, bw, bh := st.X, st.Y, st.W, st.H
		verts := gpu.BuildQuad(bx, by, bw, bh,
			rc.Color, 0, 0)
		b.drawQuad(verts)
	}
}

func (b *Renderer) drawImage(r *gui.RenderCmd) {
	var tex metalTexture
	var ok bool
	if r.Surface != nil {
		tex, ok = b.surfaces.images.Resolve(r.Surface, b.uploadSurfaceTexture)
	} else {
		tex, ok = b.resolveImageTexture(r.Resource)
	}
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
		b.ctx.setPipeline(pipeSolid)
		b.ctx.setMVP(&b.mvp)
		verts := gpu.BuildQuad(x, y, w, h, r.Color, 0, 0)
		b.drawQuad(verts)
	}

	b.ctx.setPipeline(pipeImageClip)
	b.ctx.setMVP(&b.mvp)
	b.ctx.bindTexture(int(tex.id))

	// Opacity rides the vertex color: fs_image_clip multiplies
	// texel alpha by it.
	z := gpu.PackParams(r.ClipRadius*s, 0)
	tint := gui.White.WithOpacity(r.Opacity)
	cr, cg, cb, ca := gpu.NormColor(tint.R, tint.G, tint.B, tint.A)
	verts := [4]vertex{
		{X: x, Y: y, Z: z, U: -1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y, Z: z, U: 1, V: -1, R: cr, G: cg, B: cb, A: ca},
		{X: x + w, Y: y + h, Z: z, U: 1, V: 1, R: cr, G: cg, B: cb, A: ca},
		{X: x, Y: y + h, Z: z, U: -1, V: 1, R: cr, G: cg, B: cb, A: ca},
	}
	// The quad is batched. Setting the sampler back draws the batch
	// first, so the nearest sampler is bound for this one quad only.
	nearest := r.ImageFilter == gui.ImageFilterNearest
	if nearest {
		b.ctx.setImageFilter(true)
	}
	b.drawQuad(verts)
	if nearest {
		b.ctx.setImageFilter(false)
	}
}

// sweepMemTextures deletes the textures of mem: images that left gui's
// registry — dropped, evicted or registered again — since the last sweep.
// A frame with no removal costs one atomic load (#1230).
func (b *Renderer) sweepMemTextures() {
	if b.memSweep.Due() {
		b.textures.Sweep(keepTexture)
	}
}

// resolveImageTexture returns the uploaded texture for a render
// command's Resource, uploading on first use.
//
// A mem: source names a buffer in gui's in-memory registry: it has no
// path, so it skips path resolution and the AllowedImageRoots sandbox
// (see gui.LookupImage on why that is safe). A dropped one draws
// nothing. The texture is cached under the Resource string with the
// registration's serial; a draw that finds another serial — the key was
// registered again — uploads anew, and Set deletes the old texture.
func (b *Renderer) resolveImageTexture(
	res string,
) (metalTexture, bool) {
	if gui.IsMemImage(res) {
		mi, ok := gui.LookupImage(res)
		if !ok {
			return metalTexture{}, false
		}
		tex, hit := b.textures.Get(res)
		if !hit || tex.serial != mi.Serial {
			tex = createMetalTexture(
				b.ctx, int32(mi.W), int32(mi.H), mi.Pix)
			tex.serial = mi.Serial
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
			log.Printf("metal: drawImage: %v", err)
			path = "-"
		}
		b.imagePathCache.Set(res, path)
	}
	if path == "-" {
		return metalTexture{}, false
	}

	tex, ok := b.textures.Get(path)
	if !ok {
		var err error
		tex, err = b.loadImageTexture(path)
		if err != nil {
			log.Printf("metal: drawImage: %v", err)
		}
		// Negative caching: failures store the zero texture
		// (id 0, skipped by drawImage), so a missing file logs
		// once, not every frame. No retry until restart.
		b.textures.Set(path, tex)
	}
	return tex, true
}

// maxSvgTriangleFloats caps a RenderSvg triangle list in floats,
// mirroring the gui package's emit-side cap. It bounds the
// per-frame vertex allocation an oversized command would force.
const maxSvgTriangleFloats = 1_200_000

func (b *Renderer) drawSvg(r *gui.RenderCmd) {
	if r.IsClipMask {
		return // clip masks not yet supported in render pipeline
	}
	if len(r.Triangles) == 0 || len(r.Triangles)%6 != 0 ||
		len(r.Triangles) > maxSvgTriangleFloats {
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
	var sx, sy, tx, ty, sxy, syx float32
	if hasXform {
		sx, sy, tx, ty = r.ScaleX, r.ScaleY, r.TransX, r.TransY
		sxy, syx = r.XformXY, r.XformYX
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
	var flatR, flatG, flatB, flatA float32
	if !hasVCols {
		flatR, flatG, flatB, flatA = gpu.NormColor(r.Color.R, r.Color.G,
			r.Color.B, r.Color.A)
	}
	for i := range numVerts {
		vx := r.Triangles[i*2]
		vy := r.Triangles[i*2+1]
		if hasXform {
			ox, oy := vx, vy
			vx = ox*sx + oy*sxy + tx
			vy = ox*syx + oy*sy + ty
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

	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	b.ctx.drawTriangles(unsafe.Pointer(&verts[0]), numVerts)
}

func (b *Renderer) drawText(r *gui.RenderCmd) {
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
		}) {
		return
	}
	cfg := glyphconv.GuiTextConfigFromRender(r)

	b.useGlyphPipeline()
	if err := b.textSys.DrawText(r.X, r.Y, r.Text, cfg); err != nil && !b.textErrLogged {
		// Warn once per backend: a persistent failure (e.g. missing
		// font) would otherwise log every frame.
		b.textErrLogged = true
		log.Printf("metal: DrawText: %v", err)
	}
}

func (b *Renderer) drawTextPath(r *gui.RenderCmd) {
	layout, placements, err := gui.ComputeTextPathPlacements(
		r, b.textSys, &b.textPathPlacements,
		guiStyleToGlyphConfig)
	if err != nil {
		log.Printf("metal: drawTextPath: %v", err)
		return
	}
	if len(placements) == 0 {
		return
	}
	b.textPathPlacements = placements
	b.useGlyphPipeline()
	b.textSys.DrawLayoutPlaced(layout, placements)
}

func (b *Renderer) drawLayout(r *gui.RenderCmd) {
	if b.textSys == nil || r.LayoutPtr == nil {
		return
	}
	b.useGlyphPipeline()
	if r.TextGradient != nil {
		b.textSys.DrawLayoutWithGradient(
			*r.LayoutPtr, r.X, r.Y, r.TextGradient,
		)
		return
	}
	b.textSys.DrawLayout(*r.LayoutPtr, r.X, r.Y)
}

func (b *Renderer) drawLayoutTransformed(r *gui.RenderCmd) {
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
		return
	}
	b.textSys.DrawLayoutTransformed(
		*r.LayoutPtr, r.X, r.Y, *r.LayoutTransform,
	)
}

func (b *Renderer) drawRtf(r *gui.RenderCmd) {
	b.drawLayout(r)
}

func (b *Renderer) drawCustomShader(r *gui.RenderCmd) {
	// Trust boundary: Shader.Metal is app-developer source, compiled
	// as-is with no sandbox. Never feed network-supplied MSL here;
	// a malicious shader hangs the GPU (app DoS).
	if r.Shader == nil || r.Shader.Metal == "" {
		return
	}

	h := b.shaderHashes.Hash(r.Shader)
	idx, ok := b.customCache.Get(h)
	if ok && idx < 0 {
		return // cached compile failure: do not recompile every frame
	}
	if !ok {
		if b.customCache.Len() >= maxCustomPipelines {
			b.customCache.EvictOldest()
		}
		src := msl.BuildCustomDecls(r.Shader.MetalDecls, r.Shader.Metal)
		if r.Shader.Surface != nil {
			src = msl.BuildCustomChannels(r.Shader.MetalDecls, r.Shader.Metal)
		}
		idx = b.ctx.buildCustomPipeline(src)
		// Cache failures too (negative idx), so a broken body
		// compiles once, not once per frame.
		b.customCache.Set(h, idx)
		if idx < 0 {
			return
		}
	}

	s := b.dpiScale
	b.ctx.setCustomPipeline(idx)
	b.ctx.setMVP(&b.mvp)
	// Keyed off Shader.Surface, which also chose the channel program, so
	// that program is never drawn with its channel textures unbound.
	if r.Shader.Surface != nil {
		ch, ok := b.surfaces.channels.ResolvePlanes(r.Shader.Surface, b.uploadChannels)
		b.bindChannels(ch, ok)
	}

	var tm [16]float32
	for i := range min(len(r.Shader.Params), 16) {
		tm[i] = r.Shader.Params[i]
	}
	b.setTM(tm)
	builtins := r.ShaderBuiltinsAt(s)
	b.ctx.setFragment0(&builtins)

	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		r.Color, r.Radius*s, 0)
	b.drawQuad(verts)
}

// --- Stencil clip ---

func (b *Renderer) beginStencilClip(r *gui.RenderCmd) {
	s := b.dpiScale
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	b.quad = verts
	b.ctx.beginStencilClip(unsafe.Pointer(&b.quad), int(r.StencilDepth))
	// Restore solid pipeline for children.
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}

func (b *Renderer) endStencilClip(r *gui.RenderCmd) {
	s := b.dpiScale
	verts := gpu.BuildQuad(r.X*s, r.Y*s, r.W*s, r.H*s,
		gui.White, r.Radius*s, 0)
	b.quad = verts
	b.ctx.endStencilClip(unsafe.Pointer(&b.quad), int(r.StencilDepth))
	// Restore solid pipeline.
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}

// --- Filter (glow) ---

// maxFilterLayers caps the composite repeat count. Layers is the
// count of feMergeNode elements in an SVG filter, so an untrusted
// document can name an arbitrary number of them; past a handful the
// glow is already saturated and each extra pass is a full-layer
// blend. Mirrors the soft backend's maxFilterLayers.
const maxFilterLayers = 32

func (b *Renderer) beginFilter(r *gui.RenderCmd) {
	b.filterBlur = r.BlurRadius * b.dpiScale
	b.filterLayer = min(max(r.Layers, 1), maxFilterLayers)
	b.filterColorMatrix = r.ColorMatrix

	// Set pipelines and MVP before switching to filter target.
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)

	rc := b.ctx.beginFilter(int(b.physW), int(b.physH))
	if rc != 0 {
		return
	}
	// Reset pipeline state on the new encoder.
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}

func (b *Renderer) endFilter() {
	b.ctx.endFilter(b.filterBlur, b.filterLayer, b.filterColorMatrix)
	// Restore pipeline state on the resumed main encoder.
	b.ctx.setPipeline(pipeSolid)
	b.ctx.setMVP(&b.mvp)
}
