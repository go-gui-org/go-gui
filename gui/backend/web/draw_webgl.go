//go:build js && wasm

package web

import (
	"log"
	"math"
	"syscall/js"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// The WebGL2 SVG path (#1045) draws SVG meshes with WebGL2 (goGuiSvgGL
// in gogui.js) into an offscreen canvas and composites each run of
// consecutive RenderSvg commands onto the 2D canvas with one drawImage.
// The 2D context's clip, transform and filter apply to that drawImage,
// so clipping, view transforms and filters behave as they do for the
// Canvas2D path. Canvas2D redraws each mesh as paths every frame; on an
// SVG-heavy page that was a few frames a second.
//
// Meshes are uploaded once, keyed like the Path2D cache. Each frame Go
// writes one record per draw into a shared Float32Array and calls the
// JS draw loop once per batch.
//
// A frame takes two passes (prepassSvgGL). The first renders every
// batch into the WebGL canvas, each at its own place, and copies the
// canvas once into a 2D staging canvas; the second draws the frame and
// composites each batch from the copy. Firefox snapshots a WebGL canvas
// for every drawImage from it, which made a frame of many small batches
// slow there; from a 2D canvas, drawImage is cheap.

const glRecStride = 17 // must match recStride in gogui.js

// glBatchClip is the rect clip a queued command was drawn under.
type glBatchClip struct {
	clip clipRegion
	ok   bool
}

type glMesh struct {
	id       int
	tris     []float32 // pinned, see meshKey
	lastUsed uint64
	// Mesh-space bounds, for the batch rect.
	minX, minY, maxX, maxY float32
}

// glPrepared is a batch as the pre-pass placed it.
type glPrepared struct {
	n int // records
	// Device-pixel rect on the 2D canvas; w == 0: nothing to draw.
	x0, y0, w, h int
	// The batch is rendered at (x0+ox, y0+oy) of the WebGL canvas.
	ox, oy int
	// fill: drawn with Canvas2D (an off-buffer batch under a transform).
	// direct: rendered and composited on its own in the second pass,
	// having found no free place in the first.
	fill, direct bool
}

type glSvg struct {
	api     js.Value // goGuiSvgGL
	state   js.Value
	canvas  js.Value
	nullCtx js.Value // goGuiSvgGL.nullContext
	w, h    int      // 2D canvas size in device pixels
	// The drawing buffer is w × h, or w × 2h with overflow: a batch
	// whose own place another batch took this frame is rendered in the
	// lower half. It grows when a frame first needs it.
	overflow, wantOverflow bool

	meshes map[meshKey]*glMesh
	vars   meshVariants // mesh_cache.go
	// Mesh ids travel to JS in a Float32Array, exact only up to 2^24.
	// Freed ids are reused, so ids stay below the live mesh count.
	nextID  int
	freeIDs []int
	frame   uint64

	// staging carries mesh uploads (float32 positions, then RGBA8
	// colors) into JS.
	staging    []byte
	stagingJS  js.Value
	stagingCap int

	// Queued draws of the current batch and their union bounds in
	// logical pixels.
	recs                       []float32
	recsF32, recsU8            js.Value
	recsCap                    int
	bMinX, bMinY, bMaxX, bMaxY float32

	lost bool

	// composites counts drawImage calls this frame, filled the batches
	// drawn with Canvas2D, and direct those composited straight from
	// the WebGL canvas (bench_hook.go).
	composites, filled, direct int

	// The current batch's commands and the rect clip each was queued
	// under, for the Canvas2D fallback.
	cmds  []*gui.RenderCmd
	clips []glBatchClip
	// batchXf: the batch was queued under a view transform or
	// rotation. Its clips were applied eagerly (svgGLLazyClips), so it
	// has no scissors and composites under the canvas clip.
	batchXf bool

	// preparing: the pre-pass is running. prepared holds its batches in
	// order, and next the one the drawing pass composites next.
	preparing bool
	prepared  []glPrepared
	next      int
	// placed holds the device-pixel rects (x0, y0, x1, y1) batches took
	// at their own places, and shelfX, shelfY, shelfH the shelf packing
	// cursor of the overflow half.
	placed                 [][4]int
	shelfX, shelfY, shelfH int
	// The WebGL canvas area the pre-pass drew, which is staged.
	usedX0, usedY0, usedX1, usedY1 int
}

// initSvgGL sets up the WebGL2 path. It reports false when WebGL2 or
// the helper is unavailable, and the backend keeps Canvas2D.
func (b *Backend) initSvgGL() (ok bool) {
	api := js.Global().Get("goGuiSvgGL")
	if api.Type() != js.TypeObject {
		return false
	}
	defer func() {
		if r := recover(); r != nil {
			log.Printf("web: WebGL2 SVG path unavailable, init failed: %v", r)
			ok = false
		}
	}()
	canvas := js.Global().Get("document").Call("createElement", "canvas")
	state := api.Call("init", canvas)
	if state.IsNull() || state.IsUndefined() {
		return false
	}
	b.gl = glSvg{
		api:     api,
		state:   state,
		canvas:  canvas,
		nullCtx: api.Get("nullContext"),
		meshes:  map[meshKey]*glMesh{},
	}
	lostFn := js.FuncOf(func(_ js.Value, args []js.Value) any {
		// preventDefault asks the browser to restore the context
		// later; without it webglcontextrestored never fires.
		if len(args) > 0 {
			args[0].Call("preventDefault")
		}
		b.onSvgGLLost()
		return nil
	})
	restoredFn := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		b.onSvgGLRestored()
		return nil
	})
	b.callbacks = append(b.callbacks, lostFn, restoredFn)
	canvas.Call("addEventListener", "webglcontextlost", lostFn)
	canvas.Call("addEventListener", "webglcontextrestored", restoredFn)
	return true
}

// onSvgGLLost handles webglcontextlost: the browser took the GPU
// context (a driver reset, a backgrounded mobile tab, too many
// contexts), and every GL object went with it. SVGs draw with Canvas2D
// until the context comes back.
func (b *Backend) onSvgGLLost() {
	g := &b.gl
	g.lost = true
	// The uploaded meshes are gone; forget them so a restore
	// uploads afresh rather than drawing ids the new context lacks.
	b.resetGLMeshes()
	b.requestRedraw()
}

// onSvgGLRestored handles webglcontextrestored: the context is back
// but empty. The program is rebuilt here; meshes upload again on their
// next draw. A failed rebuild leaves SVGs on Canvas2D.
func (b *Backend) onSvgGLRestored() {
	g := &b.gl
	defer func() {
		if r := recover(); r != nil {
			log.Printf("web: WebGL2 SVG restore failed, staying on Canvas2D: %v", r)
		}
	}()
	g.api.Call("restore", g.state)
	b.resetGLMeshes()
	// The canvas keeps its width and height, and the restored drawing
	// buffer takes them, so g.w, g.h and g.overflow still hold.
	g.lost = false
	b.requestRedraw()
}

// beginFrameSvgGL matches the drawing buffer to the 2D canvas, with
// the overflow half once a frame has asked for it. A new canvas size
// drops the overflow half until it is needed again.
func (b *Backend) beginFrameSvgGL() {
	g := &b.gl
	w := b.canvas.Get("width").Int()
	h := b.canvas.Get("height").Int()
	resized := w != g.w || h != g.h
	if resized {
		g.wantOverflow = false
	}
	if !resized && g.overflow == g.wantOverflow {
		return
	}
	g.w, g.h = w, h
	bh := h
	if g.wantOverflow {
		bh = 2 * h
	}
	ok := g.api.Call("resize", g.state, w, bh).Truthy()
	// A browser may give a smaller buffer than asked, which would cut
	// the overflow half; such batches go direct instead.
	g.overflow = g.wantOverflow && ok
}

// endFrameSvgGL evicts meshes not drawn for pathCacheIdleFrames.
func (b *Backend) endFrameSvgGL() {
	g := &b.gl
	g.frame++
	if g.frame%60 != 0 {
		return
	}
	for k, m := range g.meshes {
		if g.frame-m.lastUsed > pathCacheIdleFrames {
			b.dropGLMesh(k, m)
		}
	}
}

// dropGLMesh frees mesh m and its id. Callers drop only meshes not
// used this frame, so no queued record still refers to the id.
func (b *Backend) dropGLMesh(k meshKey, m *glMesh) {
	g := &b.gl
	g.api.Call("remove", g.state, m.id)
	delete(g.meshes, k)
	g.vars.remove(k)
	g.freeIDs = append(g.freeIDs, m.id)
}

// resetGLMeshes forgets every mesh and id, after a context loss or
// restore: the GL objects went with the context.
func (b *Backend) resetGLMeshes() {
	g := &b.gl
	clear(g.meshes)
	clear(g.vars)
	g.freeIDs = g.freeIDs[:0]
	g.nextID = 0
}

// meshFor returns r's uploaded mesh, uploading it when new or changed.
func (b *Backend) glMeshFor(r *gui.RenderCmd) *glMesh {
	g := &b.gl
	key := meshKeyFor(r)
	if m, ok := g.meshes[key]; ok {
		m.lastUsed = g.frame
		return m
	}
	var id int
	if n := len(g.freeIDs); n > 0 {
		id, g.freeIDs = g.freeIDs[n-1], g.freeIDs[:n-1]
	} else {
		g.nextID++
		id = g.nextID
	}
	m := &glMesh{id: id, tris: r.Triangles, lastUsed: g.frame}
	m.minX, m.minY = float32(math.Inf(1)), float32(math.Inf(1))
	m.maxX, m.maxY = float32(math.Inf(-1)), float32(math.Inf(-1))
	for i := 0; i+1 < len(r.Triangles); i += 2 {
		x, y := r.Triangles[i], r.Triangles[i+1]
		m.minX, m.maxX = min(m.minX, x), max(m.maxX, x)
		m.minY, m.maxY = min(m.minY, y), max(m.maxY, y)
	}

	numVerts := len(r.Triangles) / 2
	posLen := len(r.Triangles) * 4
	colLen := 0
	if len(r.VertexColors) == numVerts {
		colLen = numVerts * 4
	}
	g.staging = append(g.staging[:0],
		unsafe.Slice((*byte)(unsafe.Pointer(&r.Triangles[0])), posLen)...)
	if colLen > 0 {
		for _, c := range r.VertexColors {
			g.staging = append(g.staging, c.R, c.G, c.B, c.A)
		}
	}
	if len(g.staging) > g.stagingCap {
		g.stagingCap = max(len(g.staging), 2*g.stagingCap)
		g.stagingJS = js.Global().Get("Uint8Array").New(g.stagingCap)
	}
	js.CopyBytesToJS(g.stagingJS, g.staging)
	g.api.Call("upload", g.state, m.id, g.stagingJS, posLen, colLen, numVerts)
	g.meshes[key] = m
	if g.vars == nil {
		g.vars = meshVariants{}
	}
	g.vars.add(key)
	trimMeshVariants(g.meshes, g.vars, key.base(), g.frame,
		func(m *glMesh) uint64 { return m.lastUsed },
		func(k meshKey, m *glMesh) { b.dropGLMesh(k, m) })
	return m
}

// queueSvgGL adds r to the current batch. The caller has validated r
// as drawSvg does.
func (b *Backend) queueSvgGL(r *gui.RenderCmd) {
	g := &b.gl
	m := b.glMeshFor(r)
	a, bb, c, d, e, f := svgAffine(r)

	// Draw bounds: the mesh box's corners under the affine, cut to the
	// rect clip, which the JS loop applies as this draw's scissor.
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, p := range [4][2]float32{
		{m.minX, m.minY}, {m.maxX, m.minY}, {m.minX, m.maxY}, {m.maxX, m.maxY},
	} {
		x := float32(a*float64(p[0]) + c*float64(p[1]) + e)
		y := float32(bb*float64(p[0]) + d*float64(p[1]) + f)
		minX, maxX = min(minX, x), max(maxX, x)
		minY, maxY = min(minY, y), max(maxY, y)
	}
	if len(g.recs) == 0 {
		g.batchXf = !b.svgGLLazyClips()
	}
	sx, sy, sw, sh := float32(0), float32(0), float32(-1), float32(-1)
	if clip, ok := b.stackRectClip(); ok && !g.batchXf {
		minX, minY = max(minX, clip.x), max(minY, clip.y)
		maxX, maxY = min(maxX, clip.x+clip.w), min(maxY, clip.y+clip.h)
		s := b.dpiScale
		// Rounded to the nearest device pixel: the Canvas2D clip it
		// replaces covers a pixel by more than half.
		x0 := float32(math.Round(float64(clip.x * s)))
		y0 := float32(math.Round(float64(clip.y * s)))
		sx, sy = x0, y0
		sw = max(0, float32(math.Round(float64((clip.x+clip.w)*s)))-x0)
		sh = max(0, float32(math.Round(float64((clip.y+clip.h)*s)))-y0)
	}
	if maxX <= minX || maxY <= minY {
		return // clipped away
	}
	rc, rcOK := b.stackRectClip()
	g.cmds = append(g.cmds, r)
	g.clips = append(g.clips, glBatchClip{rc, rcOK})
	if len(g.recs) == 0 {
		g.bMinX, g.bMinY, g.bMaxX, g.bMaxY = minX, minY, maxX, maxY
	} else {
		g.bMinX, g.bMinY = min(g.bMinX, minX), min(g.bMinY, minY)
		g.bMaxX, g.bMaxY = max(g.bMaxX, maxX), max(g.bMaxY, maxY)
	}

	useV, alpha := float32(0), float32(1)
	if len(r.VertexColors) == len(r.Triangles)/2 {
		useV = 1
		if r.HasVertexAlpha {
			alpha = max(0, min(r.VertexAlphaScale, 1))
		}
	}
	col := r.Color
	g.recs = append(g.recs, float32(m.id),
		float32(a), float32(bb), float32(c), float32(d), float32(e), float32(f),
		float32(col.R)/255, float32(col.G)/255, float32(col.B)/255, float32(col.A)/255,
		alpha, useV, sx, sy, sw, sh)
}

// fillSvgBatch draws the batch with Canvas2D, each command under
// the rect clip it was queued under (rect clips are applied lazily, see
// drawClip). A clip change replays the 2D state, so consecutive
// commands under the same clip share one replay.
func (b *Backend) fillSvgBatch() {
	g := &b.gl
	g.filled++
	var last glBatchClip
	for i, r := range g.cmds {
		if g.batchXf {
			// Clips are eager under a transform: the 2D clip is exact.
		} else if c := g.clips[i]; i == 0 || c != last {
			if c.ok {
				b.rebuildState(&c.clip, false)
			} else {
				b.rebuildState(nil, true)
			}
			last = c
		}
		if b.paths.entries != nil {
			b.drawSvgPath2D(r)
		} else {
			b.drawSvgCanvas(r)
		}
	}
}

// svgGLLazyClips reports whether rect clips may be applied lazily, as
// per-mesh scissors. Only without a view transform or rotation: a rect
// clip is a window rect (draw_transform.go rectClip), and a batch is
// drawn in the frame its geometry comes in, which a transform maps to
// the window only at composite time.
func (b *Backend) svgGLLazyClips() bool {
	return b.svgMode == svgModeWebGL && !b.gl.lost &&
		len(b.xfStack) == 0 && b.xfCur == affineIdentity
}

// stackRectClip returns the clip stack's rect entry, if any.
func (b *Backend) stackRectClip() (clipRegion, bool) {
	for i := range b.clipStack {
		if b.clipStack[i].kind == clipKindRect {
			return b.clipStack[i], true
		}
	}
	return clipRegion{}, false
}

// prepassSvgGL is the first pass of a frame in WebGL mode. It runs the
// frame's clip, transform, filter and SVG commands through the same
// batching as the drawing pass, with the 2D context swapped for a
// stand-in that ignores every call, so the batches come out the same.
// Each batch is rendered into the WebGL canvas where placeSvgGL puts
// it, and the drawn area is copied once into the staging canvas. The
// drawing pass then composites each batch from the copy.
func (b *Backend) prepassSvgGL(cmds []gui.RenderCmd) {
	g := &b.gl
	g.prepared = g.prepared[:0]
	g.next = 0
	g.placed = g.placed[:0]
	g.shelfX, g.shelfY, g.shelfH = 0, 0, 0
	g.usedX0, g.usedY0, g.usedX1, g.usedY1 = math.MaxInt, math.MaxInt, 0, 0
	if g.lost {
		return
	}
	ctx := b.ctx2d
	b.ctx2d = g.nullCtx
	g.preparing = true
	for i := range cmds {
		r := &cmds[i]
		b.beforeCmdSvgGL(r)
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderSvg:
			b.drawSvg(r)
		case gui.RenderFilterBegin:
			b.beginFilter(r)
		case gui.RenderFilterEnd:
			b.endFilter()
		case gui.RenderStencilBegin:
			b.beginStencilClip(r)
		case gui.RenderStencilEnd:
			b.endStencilClip()
		case gui.RenderRotateBegin:
			b.beginRotation(r)
		case gui.RenderRotateEnd:
			b.endRotation()
		case gui.RenderTransformBegin:
			b.beginTransform(r)
		case gui.RenderTransformEnd:
			b.endTransform()
		default:
		}
	}
	b.settleSvgGL()
	g.preparing = false
	b.ctx2d = ctx
	// Back to the frame's start, as renderFrame left it. The saves were
	// made on the stand-in, so none is restored.
	b.resetTransforms()
	b.stateDepth = 0
	b.clipStack = b.clipStack[:0]
	b.brackets = b.brackets[:0]
	b.clipDirty = false
	if g.usedX1 > g.usedX0 {
		g.api.Call("stage", g.state, g.usedX0, g.usedY0,
			g.usedX1-g.usedX0, g.usedY1-g.usedY0)
	}
}

// placeSvgGL picks where in the WebGL canvas the pre-pass renders p: at
// its own place unless a batch took part of it, else on the next shelf
// of the overflow half, else nowhere (direct).
func (g *glSvg) placeSvgGL(p *glPrepared) {
	r := [4]int{p.x0, p.y0, p.x0 + p.w, p.y0 + p.h}
	free := true
	for _, q := range g.placed {
		if r[0] < q[2] && q[0] < r[2] && r[1] < q[3] && q[1] < r[3] {
			free = false
			break
		}
	}
	switch {
	case free:
		g.placed = append(g.placed, r)
	case !g.overflow:
		g.wantOverflow = true
		p.direct = true
		return
	default:
		if g.shelfX+p.w > g.w {
			g.shelfX, g.shelfY, g.shelfH = 0, g.shelfY+g.shelfH, 0
		}
		if g.shelfY+p.h > g.h {
			p.direct = true
			return
		}
		p.ox, p.oy = g.shelfX-p.x0, g.h+g.shelfY-p.y0
		g.shelfX += p.w
		g.shelfH = max(g.shelfH, p.h)
	}
	g.usedX0, g.usedY0 = min(g.usedX0, p.x0+p.ox), min(g.usedY0, p.y0+p.oy)
	g.usedX1 = max(g.usedX1, p.x0+p.ox+p.w)
	g.usedY1 = max(g.usedY1, p.y0+p.oy+p.h)
}

// flushSvgGL ends the queued batch. Called before any non-SVG command
// and at the end of the frame, so the batch draws under the clip and
// transform it was queued under. The pre-pass places and renders it;
// the drawing pass composites it.
func (b *Backend) flushSvgGL() {
	g := &b.gl
	n := len(g.recs) / glRecStride
	if n == 0 {
		return
	}
	defer func() {
		g.recs = g.recs[:0]
		clear(g.cmds) // drop the frame's command pointers
		g.cmds = g.cmds[:0]
		g.clips = g.clips[:0]
		g.batchXf = false
	}()
	// A batch queued before the context was lost this frame is filled
	// with Canvas2D.
	if g.lost {
		if !g.preparing {
			b.fillSvgBatch()
		}
		return
	}
	p := b.svgGLBatch(n)
	if g.preparing {
		if p.w > 0 && !p.fill {
			g.placeSvgGL(&p)
			if !p.direct {
				b.renderSvgGL(&p)
			}
		}
		g.prepared = append(g.prepared, p)
		return
	}
	b.compositeSvgGL(&p)
}

// svgGLBatch returns the queued batch's device-pixel rect, or marks it
// for Canvas2D.
func (b *Backend) svgGLBatch(n int) glPrepared {
	g := &b.gl
	p := glPrepared{n: n}
	s := b.dpiScale
	// Under a transform the batch is drawn in the view's frame, which
	// can lie outside the 2D canvas's area, even wholly, though the
	// transform maps it on screen. Fill such a batch with Canvas2D
	// rather than cut or drop it.
	if g.batchXf && (g.bMinX*s < 0 || g.bMinY*s < 0 ||
		g.bMaxX*s > float32(g.w) || g.bMaxY*s > float32(g.h)) {
		p.fill = true
		return p
	}
	// Device-pixel rect, padded a pixel for antialiasing and clamped
	// to the 2D canvas.
	x0 := max(0, int(math.Floor(float64(g.bMinX*s)))-1)
	y0 := max(0, int(math.Floor(float64(g.bMinY*s)))-1)
	x1 := min(g.w, int(math.Ceil(float64(g.bMaxX*s)))+1)
	y1 := min(g.h, int(math.Ceil(float64(g.bMaxY*s)))+1)
	if x1 > x0 && y1 > y0 {
		p.x0, p.y0, p.w, p.h = x0, y0, x1-x0, y1-y0
	}
	return p
}

// renderSvgGL renders the queued records into p's place in the WebGL
// canvas.
func (b *Backend) renderSvgGL(p *glPrepared) {
	g := &b.gl
	if len(g.recs) > g.recsCap {
		g.recsCap = max(len(g.recs), 2*g.recsCap)
		buf := js.Global().Get("ArrayBuffer").New(g.recsCap * 4)
		g.recsF32 = js.Global().Get("Float32Array").New(buf)
		g.recsU8 = js.Global().Get("Uint8Array").New(buf)
	}
	js.CopyBytesToJS(g.recsU8,
		unsafe.Slice((*byte)(unsafe.Pointer(&g.recs[0])), len(g.recs)*4))
	g.api.Call("draw", g.state, g.recsF32, p.n, float64(b.dpiScale),
		p.x0, p.y0, p.w, p.h, p.ox, p.oy)
}

// compositeSvgGL draws batch p onto the 2D canvas in the drawing pass:
// from the staging canvas where the pre-pass rendered it, or, for a
// batch the pre-pass did not render or rendered differently, rendered
// now and composited straight from the WebGL canvas.
func (b *Backend) compositeSvgGL(p *glPrepared) {
	g := &b.gl
	var q glPrepared
	if g.next < len(g.prepared) {
		q = g.prepared[g.next]
		g.next++
	} else {
		q.direct = true
	}
	if p.fill {
		b.fillSvgBatch()
		return
	}
	if p.w == 0 {
		return
	}
	direct := q.direct || q.fill || q.n != p.n || q.x0 != p.x0 ||
		q.y0 != p.y0 || q.w != p.w || q.h != p.h
	if direct {
		b.renderSvgGL(p)
		g.direct++
	}
	// Each draw is already scissored to its rect clip, and the batch
	// spans several; only the other clips may apply to the composite.
	// The full stack is restored before the next non-SVG command.
	if _, ok := b.stackRectClip(); ok && !g.batchXf {
		b.rebuildState(nil, true)
	}
	s := float64(b.dpiScale)
	if direct {
		g.api.Call("compositeDirect", g.state, b.ctx2d, s, p.x0, p.y0, p.w, p.h)
	} else {
		g.api.Call("composite", g.state, b.ctx2d, s,
			q.x0+q.ox, q.y0+q.oy, p.w, p.h, p.x0, p.y0)
	}
	g.composites++
}

// beforeCmdSvgGL runs before each render command. A batch draws under
// the 2D state it was queued under, so it is flushed, and a lazy rect
// clip replayed, before any command other than an SVG or a clip can
// change that state.
func (b *Backend) beforeCmdSvgGL(r *gui.RenderCmd) {
	if b.svgMode != svgModeWebGL || r.Kind == gui.RenderSvg || r.Kind == gui.RenderClip {
		return
	}
	b.settleSvgGL()
}

// settleSvgGL flushes the batch and replays a lazy rect clip. drawCmds
// also runs it at the end of the frame: the next frame's clear runs
// under the clip in force, and a stale rect clip would leave the rest
// of the canvas to accumulate.
func (b *Backend) settleSvgGL() {
	b.flushSvgGL()
	if b.clipDirty {
		b.rebuildClipStack()
	}
}
