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

const glRecStride = 17 // must match recStride in gogui.js

// svgGLMinTrisDefault is minTris unless the page sets
// goGuiSvgGLMinTris (the bench does, to tune it).
const svgGLMinTrisDefault = 2000

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

type glSvg struct {
	api    js.Value // goGuiSvgGL
	state  js.Value
	canvas js.Value
	w, h   int // drawing buffer size in device pixels

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

	// composites counts drawImage calls this frame (bench_hook.go).
	composites int

	// minTris is the batch size, in triangles, from which a batch is
	// drawn with WebGL; a lighter one is filled with Canvas2D instead.
	// Firefox pays a large cost per composite outside the measured
	// calls (the WebGL canvas is snapshotted for drawImage), so a
	// light batch costs more composited than filled directly.
	minTris int
	// The current batch's commands, the rect clip each was queued
	// under, and its triangle count, for the Canvas2D path.
	cmds      []*gui.RenderCmd
	clips     []glBatchClip
	batchTris int
	// batchXf: the batch was queued under a view transform or
	// rotation. Its clips were applied eagerly (svgGLLazyClips), so it
	// has no scissors and composites under the canvas clip.
	batchXf bool
	// batchGradient: a mesh in the batch interpolates vertex colors.
	// Canvas2D would draw it flat, so the batch stays on WebGL however
	// light it is.
	batchGradient bool
	// filled counts light batches drawn with Canvas2D (bench_hook.go).
	filled int
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
		api:    api,
		state:  state,
		canvas: canvas,
		meshes: map[meshKey]*glMesh{},
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
	// buffer takes them, so g.w and g.h still hold.
	g.lost = false
	b.requestRedraw()
}

// beginFrameSvgGL matches the drawing buffer to the 2D canvas.
func (b *Backend) beginFrameSvgGL() {
	g := &b.gl
	w := b.canvas.Get("width").Int()
	h := b.canvas.Get("height").Int()
	if w != g.w || h != g.h {
		g.api.Call("resize", g.state, w, h)
		g.w, g.h = w, h
	}
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
	g.batchTris += len(r.Triangles) / 6
	if len(r.VertexColors) == len(r.Triangles)/2 && !r.VertexColorsFlat {
		g.batchGradient = true
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

// fillSvgBatch draws a light batch with Canvas2D, each command under
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

// flushSvgGL draws the queued batch and composites it. Called before
// any non-SVG command and at the end of the frame, so the batch draws
// under the clip and transform it was queued under.
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
		g.batchTris = 0
		g.batchXf = false
		g.batchGradient = false
	}()
	// A light batch, or one queued before the context was lost this
	// frame, is filled with Canvas2D rather than composited.
	if g.lost || (g.batchTris < g.minTris && !g.batchGradient) {
		b.fillSvgBatch()
		return
	}

	s := b.dpiScale
	// Under a transform the batch is drawn in the view's frame, which
	// can lie outside the drawing buffer (the window's size), even
	// wholly, though the transform maps it on screen. Fill such a
	// batch with Canvas2D rather than cut or drop it.
	if g.batchXf && (g.bMinX*s < 0 || g.bMinY*s < 0 ||
		g.bMaxX*s > float32(g.w) || g.bMaxY*s > float32(g.h)) {
		b.fillSvgBatch()
		return
	}

	// Device-pixel rect, padded a pixel for antialiasing and clamped
	// to the drawing buffer.
	x0 := max(0, int(math.Floor(float64(g.bMinX*s)))-1)
	y0 := max(0, int(math.Floor(float64(g.bMinY*s)))-1)
	x1 := min(g.w, int(math.Ceil(float64(g.bMaxX*s)))+1)
	y1 := min(g.h, int(math.Ceil(float64(g.bMaxY*s)))+1)
	if x1 <= x0 || y1 <= y0 {
		return
	}

	if len(g.recs) > g.recsCap {
		g.recsCap = max(len(g.recs), 2*g.recsCap)
		buf := js.Global().Get("ArrayBuffer").New(g.recsCap * 4)
		g.recsF32 = js.Global().Get("Float32Array").New(buf)
		g.recsU8 = js.Global().Get("Uint8Array").New(buf)
	}
	js.CopyBytesToJS(g.recsU8,
		unsafe.Slice((*byte)(unsafe.Pointer(&g.recs[0])), len(g.recs)*4))
	w, h := x1-x0, y1-y0
	g.api.Call("draw", g.state, g.recsF32, n, float64(s), x0, y0, w, h)
	g.composites++
	// Each draw is already scissored to its rect clip, and the batch
	// spans several; only the other clips may apply to the composite.
	// The full stack is restored before the next non-SVG command.
	if _, ok := b.stackRectClip(); ok && !g.batchXf {
		b.rebuildState(nil, true)
	}
	g.api.Call("composite", g.state, b.ctx2d, float64(s), x0, y0, w, h)
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
