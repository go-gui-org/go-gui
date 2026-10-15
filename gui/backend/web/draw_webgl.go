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
	id int
	// Mesh-space bounds, for the batch rect.
	minX, minY, maxX, maxY float32
}

// glPrepared is a batch as the pre-pass placed it.
type glPrepared struct {
	n    int  // records
	seen int  // SVG commands queued, clipped away or not
	xf   bool // batchXf
	// Device-pixel rect on the 2D canvas; w == 0: nothing to draw.
	x0, y0, w, h int
	// The batch is rendered at (x0+ox, y0+oy) of the WebGL canvas.
	ox, oy int
	// fill: drawn with Canvas2D (under a transform, larger than the
	// canvas). direct: rendered and composited on its own in the second
	// pass, having found no free place in the first. away: under a
	// transform, it reaches past the canvas, so its own place is not in
	// the WebGL canvas and it is rendered elsewhere.
	fill, direct, away bool
}

// overflowIdleFrames is how many animation frames, painted or not, the
// overflow half is kept without a frame needing it.
const overflowIdleFrames = 300

type glSvg struct {
	api     js.Value // goGuiSvgGL
	state   js.Value
	canvas  js.Value
	nullCtx js.Value // goGuiSvgGL.nullContext
	w, h    int      // 2D canvas size in device pixels
	// The drawing buffer is w × h, or w × 2h with overflow: a batch
	// whose own place another batch took this frame, or that has none
	// (away), is rendered in the lower half. It grows when a frame first
	// needs it, and goes after overflowIdleFrames frames that did not
	// (overflowUsed, overflowIdle). overflowDenied: the browser could
	// not give the doubled buffer, so it is not asked for again until
	// the canvas size changes.
	overflow, wantOverflow       bool
	overflowUsed, overflowDenied bool
	overflowIdle                 int

	cache meshCache[*glMesh] // mesh_cache.go
	// transient holds the slots of meshes the cache did not admit; a
	// frame uses transientN of them, and the next frame refills their
	// buffers.
	transient  []*glMesh
	transientN int
	// Mesh ids travel to JS in a Float32Array, exact only up to 2^24.
	// Freed ids are reused, so ids stay below the live mesh count.
	nextID  int
	freeIDs []int

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
	// seen counts the batch's SVG commands. fast: the drawing pass
	// composites this batch from the copy, so it only counts them.
	seen int
	fast bool

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
	}
	b.gl.cache = newMeshCache(b.releaseGLMesh)
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
		g.wantOverflow, g.overflowDenied = false, false
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
	if g.wantOverflow && !ok {
		// A browser that cannot give the doubled buffer gives a smaller
		// one and scales it, which would misplace every draw. Go back to
		// w × h and do not ask again until the canvas size changes;
		// batches that need the overflow half go direct.
		g.api.Call("resize", g.state, w, h)
		g.wantOverflow, g.overflowDenied = false, true
	}
	g.overflow = g.wantOverflow && ok
}

// endFrameSvgGL ends the frame for the mesh cache, frees the transient
// slots the frame did not use, and ages an unused overflow half.
func (b *Backend) endFrameSvgGL() {
	g := &b.gl
	g.cache.endFrame()
	for _, m := range g.transient[g.transientN:] {
		b.releaseGLMesh(m)
	}
	clear(g.transient[g.transientN:])
	g.transient = g.transient[:g.transientN]
	switch {
	case g.overflowUsed:
		g.overflowIdle = 0
	case g.wantOverflow:
		b.ageOverflowSvgGL()
	}
	g.overflowUsed = false
}

// idleFrameSvgGL ages the overflow half on an animation frame that did
// not repaint. The web loop skips renderFrame while a page is idle, so
// doing this only from endFrameSvgGL would retain the doubled WebGL and
// staging canvases indefinitely on a static page.
func (b *Backend) idleFrameSvgGL() {
	if b.gl.wantOverflow {
		b.ageOverflowSvgGL()
	}
}

// ageOverflowSvgGL drops the overflow half after it has gone unused.
// Resize immediately: clearing wantOverflow alone would leave both
// doubled canvases allocated until some later painted frame.
func (b *Backend) ageOverflowSvgGL() {
	g := &b.gl
	g.overflowIdle++
	if g.overflowIdle < overflowIdleFrames {
		return
	}
	if g.overflow {
		g.api.Call("resize", g.state, g.w, g.h)
	}
	g.overflow, g.wantOverflow = false, false
	g.overflowIdle = 0
}

// releaseGLMesh frees mesh m on the GPU and its id. Callers release
// only meshes not used this frame, so no queued record still refers to
// the id.
func (b *Backend) releaseGLMesh(m *glMesh) {
	g := &b.gl
	g.api.Call("remove", g.state, m.id)
	g.freeIDs = append(g.freeIDs, m.id)
}

// resetGLMeshes forgets every mesh and id, after a context loss or
// restore: the GL objects went with the context.
func (b *Backend) resetGLMeshes() {
	g := &b.gl
	g.cache.reset()
	clear(g.transient)
	g.transient = g.transient[:0]
	g.transientN = 0
	g.freeIDs = g.freeIDs[:0]
	g.nextID = 0
}

// newGLMeshID returns a free mesh id.
func (g *glSvg) newGLMeshID() int {
	if n := len(g.freeIDs); n > 0 {
		id := g.freeIDs[n-1]
		g.freeIDs = g.freeIDs[:n-1]
		return id
	}
	g.nextID++
	return g.nextID
}

// glMeshFor returns r's mesh on the GPU: cached, or uploaded, and kept
// when the cache admits it (meshCache.admit). A mesh not admitted goes
// into a transient slot, whose buffers a later upload refills.
func (b *Backend) glMeshFor(r *gui.RenderCmd) *glMesh {
	g := &b.gl
	key := meshKeyFor(r)
	if m, ok := g.cache.get(key); ok {
		return m
	}
	keep := g.cache.admit(key)
	var m *glMesh
	if keep {
		m = &glMesh{id: g.newGLMeshID()}
	} else {
		if g.transientN == len(g.transient) {
			g.transient = append(g.transient, &glMesh{id: g.newGLMeshID()})
		}
		m = g.transient[g.transientN]
		g.transientN++
	}
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
	if keep {
		g.cache.put(key, r.Triangles, m)
	}
	return m
}

// queueSvgGL adds r to the current batch. The caller has validated r
// as drawSvg does.
func (b *Backend) queueSvgGL(r *gui.RenderCmd) {
	g := &b.gl
	if g.seen == 0 {
		g.batchXf = !b.svgGLLazyClips()
		// The pre-pass rendered this batch already: the drawing pass
		// needs no mesh, bounds or record, only the count to check
		// the batch against the pre-pass's.
		g.fast = false
		if !g.preparing && g.next < len(g.prepared) {
			q := &g.prepared[g.next]
			g.fast = !q.direct && !q.fill
		}
	}
	g.seen++
	rc, rcOK := b.stackRectClip()
	g.cmds = append(g.cmds, r)
	g.clips = append(g.clips, glBatchClip{rc, rcOK})
	if g.fast {
		return
	}
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
	sx, sy, sw, sh := float32(0), float32(0), float32(-1), float32(-1)
	if clip := rc; rcOK && !g.batchXf {
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
	if len(g.recs) == 0 {
		g.bMinX, g.bMinY, g.bMaxX, g.bMaxY = minX, minY, maxX, maxY
	} else {
		g.bMinX, g.bMinY = min(g.bMinX, minX), min(g.bMinY, minY)
		g.bMaxX, g.bMaxY = max(g.bMaxX, maxX), max(g.bMaxY, maxY)
	}

	useV, alpha := float32(0), float32(1)
	if len(r.VertexColors) == len(r.Triangles)/2 {
		useV = 1
		alpha = svgVertexAlpha(r)
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
		if b.paths.ok() {
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

// flushSvgGL ends the queued batch. Called before any non-SVG command
// and at the end of the frame, so the batch draws under the clip and
// transform it was queued under. The pre-pass places and renders it;
// the drawing pass composites it.
func (b *Backend) flushSvgGL() {
	g := &b.gl
	if g.seen == 0 {
		return
	}
	n := len(g.recs) / glRecStride
	defer g.resetBatch()
	// A batch queued before the context was lost this frame is filled
	// with Canvas2D.
	if g.lost {
		if !g.preparing {
			b.fillSvgBatch()
		}
		return
	}
	if g.fast {
		b.compositeStagedSvgGL()
		return
	}
	p := b.svgGLBatch(n)
	p.seen, p.xf = g.seen, g.batchXf
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

// resetBatch empties the queued batch.
func (g *glSvg) resetBatch() {
	g.recs = g.recs[:0]
	clear(g.cmds) // drop the frame's command pointers
	g.cmds = g.cmds[:0]
	g.clips = g.clips[:0]
	g.batchXf = false
	g.seen = 0
	g.fast = false
}

// svgGLBatch returns the queued batch's device-pixel rect, or marks it
// for Canvas2D.
func (b *Backend) svgGLBatch(n int) glPrepared {
	g := &b.gl
	p := glPrepared{n: n}
	if n == 0 {
		return p // every command clipped away
	}
	s := b.dpiScale
	// Device-pixel rect, padded a pixel for antialiasing.
	x0 := int(math.Floor(float64(g.bMinX*s))) - 1
	y0 := int(math.Floor(float64(g.bMinY*s))) - 1
	x1 := int(math.Ceil(float64(g.bMaxX*s))) + 1
	y1 := int(math.Ceil(float64(g.bMaxY*s))) + 1
	// Under a transform the batch is drawn in the view's frame, which
	// can lie outside the 2D canvas's area, even wholly, though the
	// transform maps it on screen. It is rendered elsewhere in the
	// WebGL canvas (away); one larger than the canvas is filled with
	// Canvas2D, which draws gradients flat.
	if g.batchXf && (g.bMinX*s < 0 || g.bMinY*s < 0 ||
		g.bMaxX*s > float32(g.w) || g.bMaxY*s > float32(g.h)) {
		if x1-x0 > g.w || y1-y0 > g.h {
			p.fill = true
			return p
		}
		p.x0, p.y0, p.w, p.h = x0, y0, x1-x0, y1-y0
		// Rendered at the canvas origin unless placed on a shelf.
		p.away, p.ox, p.oy = true, -x0, -y0
		return p
	}
	// Clamped to the 2D canvas.
	x0, y0 = max(0, x0), max(0, y0)
	x1, y1 = min(g.w, x1), min(g.h, y1)
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
	b.compositeAtSvgGL(&q, p, direct)
}

// compositeStagedSvgGL composites a fast batch (queueSvgGL) from the
// copy, at the place and rect the pre-pass gave it. A batch that does
// not match the pre-pass's has no records to render, so it is filled
// with Canvas2D; the passes see the same commands, so that is a guard.
func (b *Backend) compositeStagedSvgGL() {
	g := &b.gl
	q := g.prepared[g.next]
	g.next++
	if q.seen != g.seen || q.xf != g.batchXf {
		b.fillSvgBatch()
		return
	}
	if q.w > 0 {
		b.compositeAtSvgGL(&q, &q, false)
	}
}

// compositeAtSvgGL draws batch p onto the 2D canvas: from the copy at
// q's place, or with direct from the WebGL canvas at p's own.
func (b *Backend) compositeAtSvgGL(q, p *glPrepared, direct bool) {
	g := &b.gl
	// Each draw is already scissored to its rect clip, and the batch
	// spans several; only the other clips may apply to the composite.
	// The full stack is restored before the next non-SVG command.
	if _, ok := b.stackRectClip(); ok && !g.batchXf {
		b.rebuildState(nil, true)
	}
	s := float64(b.dpiScale)
	if direct {
		g.api.Call("compositeDirect", g.state, b.ctx2d, s,
			p.x0+p.ox, p.y0+p.oy, p.w, p.h, p.x0, p.y0)
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
