//go:build js && wasm

package web

import (
	"fmt"
	"strings"
	"sync"
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// fakeGL stands in for goGuiSvgGL and records what the backend asks of
// it, including the 2D state in force at each composite.
type fakeGL struct {
	api        js.Value
	uploads    []int
	removes    []int
	draws      []string
	composites []string
	restores   int
	// ops lists draw, stage, composite and direct (compositeDirect)
	// calls in order, with their rects.
	ops []string
	// live is the JS side's mesh table; drawIDs the ids each draw used,
	// and missing the records whose id was not in the table (gogui.js
	// skips those silently).
	live    map[int]bool
	drawIDs []int
	missing int
	// resizes lists resize calls as "wxh"; maxH, when set, is the
	// tallest drawing buffer resize reports it got.
	resizes []string
	maxH    int
}

func newFakeGL(t *testing.T, ctx *stateCtx) *fakeGL {
	t.Helper()
	g := &fakeGL{api: js.Global().Get("Object").New(), live: map[int]bool{}}
	var funcs []js.Func
	on := func(name string, fn func(a []js.Value)) {
		f := js.FuncOf(func(_ js.Value, a []js.Value) any { fn(a); return nil })
		funcs = append(funcs, f)
		g.api.Set(name, f)
	}
	t.Cleanup(func() {
		for _, f := range funcs {
			f.Release()
		}
	})
	// nullContext: a Proxy whose every method is a no-op.
	noop := js.FuncOf(func(js.Value, []js.Value) any { return nil })
	get := js.FuncOf(func(js.Value, []js.Value) any { return noop })
	set := js.FuncOf(func(js.Value, []js.Value) any { return true })
	funcs = append(funcs, noop, get, set)
	handler := js.Global().Get("Object").New()
	handler.Set("get", get)
	handler.Set("set", set)
	g.api.Set("nullContext",
		js.Global().Get("Proxy").New(js.Global().Get("Object").New(), handler))
	ints := func(a []js.Value) []int {
		v := make([]int, len(a))
		for i := range a {
			v[i] = a[i].Int()
		}
		return v
	}
	on("upload", func(a []js.Value) {
		g.uploads = append(g.uploads, a[1].Int())
		g.live[a[1].Int()] = true
	})
	on("remove", func(a []js.Value) {
		g.removes = append(g.removes, a[1].Int())
		delete(g.live, a[1].Int())
	})
	resize := js.FuncOf(func(_ js.Value, a []js.Value) any {
		g.resizes = append(g.resizes, fmt.Sprintf("%dx%d", a[1].Int(), a[2].Int()))
		return g.maxH == 0 || a[2].Int() <= g.maxH
	})
	funcs = append(funcs, resize)
	g.api.Set("resize", resize)
	on("restore", func([]js.Value) { g.restores++ })
	on("draw", func(a []js.Value) {
		recs, n := a[1], a[2].Int()
		var sc []string
		for i := range n {
			o := i * glRecStride
			id := recs.Index(o).Int()
			g.drawIDs = append(g.drawIDs, id)
			if !g.live[id] {
				g.missing++
			}
			if w := recs.Index(o + 15).Float(); w >= 0 {
				sc = append(sc, fmt.Sprintf("%g,%g %gx%g",
					recs.Index(o+13).Float(), recs.Index(o+14).Float(),
					w, recs.Index(o+16).Float()))
			} else {
				sc = append(sc, "none")
			}
		}
		g.draws = append(g.draws, fmt.Sprintf("n=%d scissors=%v", n, sc))
		v := ints(a[4:])
		g.ops = append(g.ops, fmt.Sprintf("draw %d,%d %dx%d +%d,%d", v[0], v[1], v[2], v[3], v[4], v[5]))
	})
	on("stage", func(a []js.Value) {
		v := ints(a[1:])
		g.ops = append(g.ops, fmt.Sprintf("stage %d,%d %dx%d", v[0], v[1], v[2], v[3]))
	})
	composite := func() {
		s := ctx.cur
		s.filter = ctx.filterNow()
		g.composites = append(g.composites, s.String())
	}
	on("composite", func(a []js.Value) {
		composite()
		v := ints(a[3:])
		g.ops = append(g.ops, fmt.Sprintf("composite %d,%d %dx%d -> %d,%d", v[0], v[1], v[2], v[3], v[4], v[5]))
	})
	on("compositeDirect", func(a []js.Value) {
		composite()
		v := ints(a[3:])
		g.ops = append(g.ops, fmt.Sprintf("direct %d,%d %dx%d -> %d,%d", v[0], v[1], v[2], v[3], v[4], v[5]))
	})
	return g
}

// newGLTestBackend is a backend in svg=webgl mode over the fakes, at
// dpi 1 on a 1000×1000 canvas.
func newGLTestBackend(t *testing.T) (*Backend, *stateCtx, *fakeGL) {
	t.Helper()
	ctx := newStateCtx(t)
	gl := newFakeGL(t, ctx)
	b := &Backend{ctx2d: ctx.obj, dpiScale: 1, svgMode: svgModeWebGL}
	b.resetTransforms()
	b.gl = glSvg{
		api:     gl.api,
		state:   js.Global().Get("Object").New(),
		canvas:  js.Global().Get("Object").New(),
		w:       1000,
		h:       1000,
		nullCtx: gl.api.Get("nullContext"),
	}
	b.gl.cache = newMeshCache(b.releaseGLMesh)
	return b, ctx, gl
}

// Rect clips between SVGs do not split the batch: one composite, each
// draw scissored to its own clip, composited under no rect clip.
func TestSvgGLBatchesAcrossRectClips(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		clipCmd(100, 100, 50, 50), svgCmd(110, 110),
	})
	if want := []string{"n=2 scissors=[0,0 500x500 100,100 50x50]"}; fmt.Sprint(gl.draws) != fmt.Sprint(want) {
		t.Errorf("draws = %v, want %v", gl.draws, want)
	}
	if len(gl.composites) != 1 || !strings.Contains(gl.composites[0], "clips=[]") {
		t.Errorf("composites = %v, want one under no rect clip", gl.composites)
	}
	checkNoErrs(t, ctx)
}

// The frame's last clip must be in force when the frame ends: the next
// frame clears under it. A stale rect clip left the rest of the canvas
// to accumulate, frame over frame.
func TestSvgGLFrameEndsUnderLastClip(t *testing.T) {
	b, ctx, _ := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		clipCmd(100, 100, 50, 50), svgCmd(110, 110),
		clipCmd(0, 0, 1000, 1000),
	})
	if b.clipDirty {
		t.Error("clipDirty still set after the frame")
	}
	if got, want := fmt.Sprint(ctx.cur.clips), "[0,0 1000x1000]"; got != want {
		t.Errorf("clip in force after the frame = %s, want %s", got, want)
	}
	checkNoErrs(t, ctx)
}

// A non-SVG command after a batch draws under the full clip stack
// again, not the rect-less one the composite ran under.
func TestSvgGLRestoresClipBeforeOtherCommands(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(100, 100, 50, 50), svgCmd(110, 110),
		{Kind: gui.RenderRect, Fill: true, X: 100, Y: 100, W: 5, H: 5, Color: gui.RGB(0, 0, 255)},
	})
	if len(gl.composites) != 1 {
		t.Fatalf("composites = %v, want 1", gl.composites)
	}
	if got, want := fmt.Sprint(ctx.cur.clips), "[100,100 50x50]"; got != want {
		t.Errorf("clip after the rect = %s, want %s", got, want)
	}
	checkNoErrs(t, ctx)
}

// Inside a rotation, rect clips are window rects while the geometry
// is in the rotated frame, so a scissor in that frame would cut in the
// wrong place. Clips there are eager: a clip change flushes the batch,
// and each batch, drawn without scissors, is composited under its own
// canvas clip and the rotation.
func TestSvgGLRotationBracketKeepsTransform(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 1000, 1000),
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 50, RotCY: 50},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		clipCmd(100, 100, 50, 50), svgCmd(110, 110),
		{Kind: gui.RenderRotateEnd},
		clipCmd(0, 0, 1000, 1000), svgCmd(20, 20),
	})
	if want := []string{
		"n=1 scissors=[none]", "n=1 scissors=[none]", "n=1 scissors=[0,0 1000x1000]",
	}; fmt.Sprint(gl.draws) != fmt.Sprint(want) {
		t.Errorf("draws = %v, want %v", gl.draws, want)
	}
	if len(gl.composites) != 3 {
		t.Fatalf("composites = %v, want 3", gl.composites)
	}
	rot := xfOp(rot90about5050())
	for i, wantClip := range []string{"clips=[0,0 500x500]", "clips=[100,100 50x50]"} {
		c := gl.composites[i]
		if !strings.Contains(c, wantClip) || lastXf(c) != rot {
			t.Errorf("composite %d = %s, want %s under %s", i, c, wantClip, rot)
		}
	}
	if c := gl.composites[2]; lastXf(c) != xfIdentity {
		t.Errorf("composite after the bracket = %s, want %s", c, xfIdentity)
	}
	checkNoErrs(t, ctx)
}

// A view transform works the same way. A batch whose bounds in the
// view's frame leave the canvas is rendered elsewhere in the WebGL
// canvas and composited at its own place, so gradients stay smooth;
// only one larger than the canvas is filled with Canvas2D.
func TestSvgGLViewTransform(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	shift := &[9]float32{1, 0, 100, 0, 1, 50, 0, 0, 1}
	b.drawCmds([]gui.RenderCmd{
		{Kind: gui.RenderTransformBegin, Homography: shift, W: 200, H: 200},
		clipCmd(100, 50, 200, 200), svgCmd(10, 10),
		{Kind: gui.RenderTransformEnd},
	})
	if want := []string{"n=1 scissors=[none]"}; fmt.Sprint(gl.draws) != fmt.Sprint(want) {
		t.Errorf("draws = %v, want %v", gl.draws, want)
	}
	if len(gl.composites) != 1 || !strings.Contains(gl.composites[0], "clips=[100,50 200x200]") ||
		lastXf(gl.composites[0]) != "setTransform(1,0,0,1,100,50)" {
		t.Fatalf("composites = %v, want one under the clip and the shift", gl.composites)
	}

	away := []gui.RenderCmd{
		{Kind: gui.RenderTransformBegin, Homography: shift, W: 200, H: 200},
		svgCmd(-50, 10), // left of the canvas in the view's frame
		{Kind: gui.RenderTransformEnd},
	}
	gl.ops = nil
	b.drawCmds(away)
	want := []string{"draw -51,9 12x12 +51,-9", "direct 0,0 12x12 -> -51,9"}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) || !b.gl.wantOverflow {
		t.Errorf("without overflow: ops = %v, wantOverflow %v; want %v, true", gl.ops, b.gl.wantOverflow, want)
	}
	gl.ops = nil
	b.gl.overflow = true
	b.drawCmds(away)
	want = []string{"draw -51,9 12x12 +51,991", "stage 0,1000 12x12", "composite 0,1000 12x12 -> -51,9"}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) {
		t.Errorf("with overflow: ops = %v, want %v", gl.ops, want)
	}
	if len(ctx.fills) != 0 {
		t.Errorf("fills = %v, want none", ctx.fills)
	}

	huge := svgCmd(-50, 10)
	huge.Triangles = []float32{0, 0, 2000, 0, 0, 10} // wider than the canvas
	b.drawCmds([]gui.RenderCmd{
		{Kind: gui.RenderTransformBegin, Homography: shift, W: 200, H: 200},
		huge,
		{Kind: gui.RenderTransformEnd},
	})
	if len(ctx.fills) != 1 {
		t.Errorf("fills = %v, want the batch wider than the canvas filled with Canvas2D", ctx.fills)
	}
	checkNoErrs(t, ctx)
}

// A filter bracket's filter must still be set when its batch is
// composited.
func TestSvgGLFilterBracketKeepsFilter(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 1000, 1000),
		{Kind: gui.RenderFilterBegin, BlurRadius: 4},
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		{Kind: gui.RenderFilterEnd},
	})
	if len(gl.composites) != 1 || !strings.Contains(gl.composites[0], `filter="blur(4px)"`) {
		t.Errorf("composites = %v, want one under blur(4px)", gl.composites)
	}
	checkNoErrs(t, ctx)
}

// After a context loss SVGs draw with Canvas2D and a repaint is asked
// for; after the restore the program is rebuilt and meshes upload
// again into the new context.
func TestSvgGLContextLossAndRestore(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	cmds := []gui.RenderCmd{clipCmd(0, 0, 500, 500), svgCmd(10, 10)}
	b.drawCmds(cmds)
	if len(gl.composites) != 1 || len(gl.uploads) != 1 {
		t.Fatalf("before the loss: composites %v uploads %v, want 1 each", gl.composites, gl.uploads)
	}

	b.redrawPending = false
	b.onSvgGLLost()
	if !b.redrawPending {
		t.Error("loss did not ask for a repaint")
	}
	// A different clip than the frame before: the fill must not run
	// under the clip still in force from that frame.
	b.drawCmds([]gui.RenderCmd{clipCmd(100, 100, 50, 50), svgCmd(110, 110)})
	if len(gl.composites) != 1 || len(gl.draws) != 1 {
		t.Errorf("while lost: draws %v composites %v, want no new WebGL work", gl.draws, gl.composites)
	}
	if len(ctx.fills) != 1 || !strings.Contains(ctx.fills[0], "clips=[100,100 50x50]") {
		t.Errorf("while lost: fills %v, want one Canvas2D fill under its own clip", ctx.fills)
	}

	b.redrawPending = false
	b.onSvgGLRestored()
	if gl.restores != 1 || b.gl.lost || !b.redrawPending {
		t.Fatalf("restore: restores=%d lost=%v redraw=%v, want 1, false, true",
			gl.restores, b.gl.lost, b.redrawPending)
	}
	b.drawCmds(cmds)
	if len(gl.composites) != 2 || len(gl.uploads) != 2 {
		t.Errorf("after the restore: composites %v uploads %v, want WebGL again with a fresh upload",
			gl.composites, gl.uploads)
	}
	checkNoErrs(t, ctx)
}

// A batch already queued when the context goes is filled with Canvas2D
// rather than composited from the dead context.
func TestSvgGLLossMidBatchFills(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawSvg(func() *gui.RenderCmd { c := svgCmd(10, 10); return &c }())
	b.onSvgGLLost()
	b.flushSvgGL()
	if len(gl.composites) != 0 || len(ctx.fills) != 1 {
		t.Errorf("composites %v fills %v, want the queued SVG filled with Canvas2D",
			gl.composites, ctx.fills)
	}
	checkNoErrs(t, ctx)
}

// A mesh is cached on its second frame; on its first it is drawn from
// a transient slot. A buffer rewritten in place is a new mesh, cached
// on its own second frame beside the old variant until that idles out.
func TestSvgGLMeshCache(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	cmd := svgCmd(10, 10)
	frame := func() {
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFrameSvgGL()
	}
	frame()
	if len(b.gl.cache.entries) != 0 || len(gl.uploads) != 1 {
		t.Fatalf("first frame: %d cached, uploads %v; want a transient upload", len(b.gl.cache.entries), gl.uploads)
	}
	frame()
	frame()
	if len(b.gl.cache.entries) != 1 || len(gl.uploads) != 2 {
		t.Fatalf("after three frames: %d cached, uploads %v; want one mesh, cached on the second frame",
			len(b.gl.cache.entries), gl.uploads)
	}
	cmd.Triangles[2] = 20 // same backing array, new content
	frame()
	frame()
	if len(b.gl.cache.entries) != 2 || gl.missing != 0 {
		t.Fatalf("after an in-place rewrite: %d cached, missing %d; want a second variant",
			len(b.gl.cache.entries), gl.missing)
	}
	for range meshCacheIdleFrames + 60 {
		b.endFrameSvgGL()
	}
	// Two transient slots released at the end of the frames that did
	// not use them, and two cached meshes that idled out.
	if len(b.gl.cache.entries) != 0 || len(b.gl.cache.vars) != 0 || len(gl.removes) != 4 {
		t.Errorf("after idling: %d meshes, %d variant lists, removes %v; want all released",
			len(b.gl.cache.entries), len(b.gl.cache.vars), gl.removes)
	}
	checkNoErrs(t, ctx)
}

// An animated SVG path is tessellated into a new slice every frame, so
// its meshes never repeat. They are never cached: each frame refills
// one transient slot, and nothing is kept or freed per frame.
func TestSvgGLAnimatedMeshNotCached(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	for i := range 300 {
		cmd := svgCmd(10, 10)
		cmd.Triangles = []float32{0, 0, 10 + float32(i), 0, 0, 10} // a new slice
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFrameSvgGL()
	}
	ids := map[int]bool{}
	for _, id := range gl.uploads {
		ids[id] = true
	}
	if len(b.gl.cache.entries) != 0 || len(b.gl.transient) != 1 || len(ids) != 1 ||
		len(gl.removes) != 0 || gl.missing != 0 {
		t.Errorf("%d cached, %d transient slots, upload ids %v, removes %v, missing %d; "+
			"want one slot refilled every frame", len(b.gl.cache.entries), len(b.gl.transient),
			ids, gl.removes, gl.missing)
	}
	checkNoErrs(t, ctx)
}

// gradSvgCmd is svgCmd with per-vertex colors: a gradient mesh.
func gradSvgCmd(x, y float32, tris []float32, c gui.Color) gui.RenderCmd {
	r := svgCmd(x, y)
	r.Triangles = tris
	r.VertexColors = []gui.Color{c, gui.RGB(0, 0, 255), c}
	return r
}

// Commands that share triangles but differ in colors are two variants:
// both are cached and drawn from live meshes in one batch, neither
// replaces the other, and later frames upload nothing.
func TestSvgGLColorVariantsCoexist(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	tris := []float32{0, 0, 10, 0, 0, 10}
	cmds := []gui.RenderCmd{
		gradSvgCmd(10, 10, tris, gui.RGB(255, 0, 0)),
		gradSvgCmd(40, 10, tris, gui.RGB(0, 255, 0)), // same slice, other tint
	}
	for range 2 {
		b.drawCmds(cmds)
		b.endFrameSvgGL()
	}
	if len(b.gl.cache.entries) != 2 || gl.missing != 0 {
		t.Fatalf("%d cached, missing %d; want two live variants", len(b.gl.cache.entries), gl.missing)
	}
	uploads := len(gl.uploads)
	gl.drawIDs = nil
	b.drawCmds(cmds)
	cached := gl.uploads[uploads-2:]
	if len(gl.uploads) != uploads || gl.missing != 0 || fmt.Sprint(gl.drawIDs) != fmt.Sprint(cached) {
		t.Errorf("third frame: uploads %v, drawn ids %v, missing %d; want both cached variants %v drawn, no upload",
			gl.uploads[uploads:], gl.drawIDs, gl.missing, cached)
	}
	checkNoErrs(t, ctx)
}

// One backing array keeps at most maxMeshVariants cached meshes, and a
// variant drawn in the current frame is never the one dropped.
func TestSvgGLVariantCap(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	cmd := svgCmd(10, 10)
	base := meshBase{ptr: meshKeyFor(&cmd).ptr, n: len(cmd.Triangles)}
	for i := range 3 * maxMeshVariants {
		cmd.Triangles[2] = float32(20 + i)
		for range 2 { // cached on its second frame
			b.drawCmds([]gui.RenderCmd{cmd})
			b.endFrameSvgGL()
		}
		if n := len(b.gl.cache.vars[base]); n > maxMeshVariants {
			t.Fatalf("variant %d: %d variants, want at most %d", i, n, maxMeshVariants)
		}
	}
	if gl.missing != 0 {
		t.Errorf("%d draws used a dropped mesh", gl.missing)
	}
	if len(b.gl.cache.entries) != maxMeshVariants {
		t.Errorf("%d meshes cached, want %d", len(b.gl.cache.entries), maxMeshVariants)
	}
	checkNoErrs(t, ctx)
}

// Freed mesh ids are reused, so the ids sent through the Float32Array
// stay small however many meshes come and go.
func TestSvgGLIDsRecycled(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	cmd := svgCmd(10, 10)
	for i := range 1000 {
		cmd.Triangles[2] = float32(20 + i)
		for range 2 {
			b.drawCmds([]gui.RenderCmd{cmd})
			b.endFrameSvgGL()
		}
	}
	maxID := 0
	for _, id := range gl.uploads {
		maxID = max(maxID, id)
	}
	if maxID > maxMeshVariants+2 {
		t.Errorf("highest mesh id after 1000 rewrites = %d, want ids recycled (<= %d)",
			maxID, maxMeshVariants+2)
	}
	if gl.missing != 0 {
		t.Errorf("%d draws used a dropped mesh", gl.missing)
	}
	checkNoErrs(t, ctx)
}

// rectCmd is a filled rect, a non-SVG command that ends a batch.
func rectCmd(x, y float32) gui.RenderCmd {
	return gui.RenderCmd{Kind: gui.RenderRect, Fill: true, X: x, Y: y, W: 5, H: 5,
		Color: gui.RGB(0, 0, 255)}
}

// Every batch is rendered before the first composite, the WebGL canvas
// is staged once, and each batch is composited from its place in the
// copy, under its own 2D state.
func TestSvgGLStagesOncePerFrame(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawCmds([]gui.RenderCmd{
		svgCmd(10, 10), rectCmd(0, 0),
		clipCmd(0, 0, 500, 500),
		{Kind: gui.RenderFilterBegin, BlurRadius: 4},
		svgCmd(100, 10),
		{Kind: gui.RenderFilterEnd},
	})
	want := []string{
		"draw 9,9 12x12 +0,0",
		"draw 99,9 12x12 +0,0",
		"stage 9,9 102x12",
		"composite 9,9 12x12 -> 9,9",
		"composite 99,9 12x12 -> 99,9",
	}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) {
		t.Errorf("ops = %v, want %v", gl.ops, want)
	}
	if len(gl.composites) != 2 || strings.Contains(gl.composites[0], "blur") ||
		!strings.Contains(gl.composites[1], `filter="blur(4px)"`) {
		t.Errorf("composites = %v, want the second alone under blur(4px)", gl.composites)
	}
	checkNoErrs(t, ctx)
}

// A batch whose place another batch took this frame goes direct and
// asks for the overflow half; with it, the batch is rendered on a
// shelf there and composited from the copy.
func TestSvgGLOverlapUsesOverflow(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	cmds := []gui.RenderCmd{svgCmd(10, 10), rectCmd(0, 0), svgCmd(15, 15)}
	b.drawCmds(cmds)
	want := []string{
		"draw 9,9 12x12 +0,0",
		"stage 9,9 12x12",
		"composite 9,9 12x12 -> 9,9",
		"draw 14,14 12x12 +0,0",
		"direct 14,14 12x12 -> 14,14",
	}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) {
		t.Errorf("without overflow: ops = %v, want %v", gl.ops, want)
	}
	if !b.gl.wantOverflow || b.gl.direct != 1 {
		t.Errorf("wantOverflow %v direct %d, want the overflow half asked for and 1 direct",
			b.gl.wantOverflow, b.gl.direct)
	}

	gl.ops = nil
	b.gl.overflow = true
	b.drawCmds(cmds)
	want = []string{
		"draw 9,9 12x12 +0,0",
		"draw 14,14 12x12 +-14,986", // to (0, 1000), the overflow half
		"stage 0,9 21x1003",
		"composite 9,9 12x12 -> 9,9",
		"composite 0,1000 12x12 -> 14,14",
	}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) {
		t.Errorf("with overflow: ops = %v, want %v", gl.ops, want)
	}
	checkNoErrs(t, ctx)
}

// The overflow half packs batches in shelves and sends a batch that
// fits nowhere direct.
func TestSvgGLOverflowShelves(t *testing.T) {
	g := &glSvg{w: 100, h: 100, overflow: true,
		placed: [][4]int{{0, 0, 100, 100}}} // the whole canvas taken
	var got []string
	for _, wh := range [][2]int{{60, 30}, {30, 40}, {50, 20}, {90, 60}} {
		p := glPrepared{x0: 0, y0: 0, w: wh[0], h: wh[1]}
		g.placeSvgGL(&p)
		if p.direct {
			got = append(got, "direct")
		} else {
			got = append(got, fmt.Sprintf("%d,%d", p.ox, p.oy))
		}
	}
	want := []string{"0,100", "60,100", "0,140", "direct"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("placements = %v, want %v", got, want)
	}
}

// A batch flushed outside a frame has no pre-pass place: it is
// rendered and composited straight from the WebGL canvas.
func TestSvgGLUnpreparedBatchGoesDirect(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.drawSvg(func() *gui.RenderCmd { c := svgCmd(10, 10); return &c }())
	b.flushSvgGL()
	want := []string{"draw 9,9 12x12 +0,0", "direct 9,9 12x12 -> 9,9"}
	if fmt.Sprint(gl.ops) != fmt.Sprint(want) {
		t.Errorf("ops = %v, want %v", gl.ops, want)
	}
	checkNoErrs(t, ctx)
}

// The drawing pass only counts a staged batch's commands. A batch that
// counts differently from the pre-pass's has no records to render, so
// it is filled with Canvas2D rather than composited from a stale place.
func TestSvgGLStagedCountMismatchFills(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	one, two := svgCmd(10, 10), svgCmd(30, 10)
	b.prepassSvgGL([]gui.RenderCmd{one})
	b.drawSvg(&one)
	b.drawSvg(&two) // not in the pre-pass's batch
	b.flushSvgGL()
	if len(gl.composites) != 0 || len(ctx.fills) != 2 {
		t.Errorf("composites %v fills %v, want both SVGs filled with Canvas2D",
			gl.composites, ctx.fills)
	}
	checkNoErrs(t, ctx)
}

// A browser that cannot give the doubled drawing buffer is not asked
// again: the buffer goes back to the canvas size, overlapping batches
// go direct, and later frames neither resize nor ask.
func TestSvgGLOverflowResizeFailure(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	canvas := js.Global().Get("Object").New()
	canvas.Set("width", 1000)
	canvas.Set("height", 1000)
	b.canvas = canvas
	gl.maxH = 1000
	b.gl.wantOverflow = true
	b.beginFrameSvgGL()
	if b.gl.overflow || b.gl.wantOverflow || !b.gl.overflowDenied ||
		fmt.Sprint(gl.resizes) != "[1000x2000 1000x1000]" {
		t.Fatalf("overflow %v want %v denied %v resizes %v; want back to 1000x1000 and denied",
			b.gl.overflow, b.gl.wantOverflow, b.gl.overflowDenied, gl.resizes)
	}
	cmds := []gui.RenderCmd{svgCmd(10, 10), rectCmd(0, 0), svgCmd(15, 15)}
	for range 3 {
		b.beginFrameSvgGL()
		b.drawCmds(cmds)
		b.endFrameSvgGL()
	}
	if len(gl.resizes) != 2 || b.gl.wantOverflow {
		t.Errorf("resizes %v, wantOverflow %v; want no further resize and no ask", gl.resizes, b.gl.wantOverflow)
	}
	checkNoErrs(t, ctx)
}

// The overflow half goes after overflowIdleFrames animation frames that
// did not need a repaint, including on an otherwise idle page, and is
// resized immediately instead of waiting for a later painted frame.
func TestSvgGLOverflowReleased(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	b.gl.wantOverflow, b.gl.overflow = true, true
	overlap := []gui.RenderCmd{svgCmd(10, 10), rectCmd(0, 0), svgCmd(15, 15)}
	for range 2 * overflowIdleFrames {
		b.drawCmds(overlap)
		b.endFrameSvgGL()
	}
	if !b.gl.wantOverflow {
		t.Fatal("overflow half dropped while frames needed it")
	}
	for range overflowIdleFrames {
		b.idleFrameSvgGL()
	}
	if b.gl.wantOverflow || b.gl.overflow || b.gl.overflowIdle != 0 ||
		fmt.Sprint(gl.resizes) != "[1000x1000]" {
		t.Errorf("wantOverflow %v overflow %v idle %d resizes %v; want released and resized now",
			b.gl.wantOverflow, b.gl.overflow, b.gl.overflowIdle, gl.resizes)
	}
	checkNoErrs(t, ctx)
}

// A panic in the pre-pass (a JS exception through syscall/js) leaves
// the real 2D context, frame state and window lock in place, so the
// next frame, after the render loop recovers, draws again.
func TestSvgGLPrepassPanicRecovers(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	var w sync.Mutex // stands in for the *gui.Window
	cmds := []gui.RenderCmd{clipCmd(0, 0, 500, 500), svgCmd(10, 10)}
	// A draw that throws in JS, which syscall/js raises as a panic in
	// the calling goroutine. A Go panic inside a js.FuncOf fake would
	// instead unwind the runtime's callback handling and deadlock the
	// test binary.
	fake := gl.api.Get("draw")
	gl.api.Set("draw", js.Global().Get("Function").New(`throw new Error("draw failed")`))
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("pre-pass did not panic")
			}
		}()
		withWindowRenderLock(&w, func() { b.drawCmds(cmds) })
	}()
	if !b.ctx2d.Equal(ctx.obj) || b.gl.preparing || len(b.gl.recs) != 0 || len(b.clipStack) != 0 {
		t.Fatalf("after the panic: real ctx %v, preparing %v, %d queued records, %d clips; want all reset",
			b.ctx2d.Equal(ctx.obj), b.gl.preparing, len(b.gl.recs)/glRecStride, len(b.clipStack))
	}
	gl.api.Set("draw", fake)
	if !w.TryLock() {
		t.Fatal("window remained locked after render panic")
	}
	w.Unlock()
	withWindowRenderLock(&w, func() { b.drawCmds(cmds) })
	if len(gl.composites) != 1 {
		t.Errorf("composites after recovery = %v, want 1", gl.composites)
	}
	checkNoErrs(t, ctx)
}

// While the WebGL context is lost, SVGs draw from the Path2D cache when
// it is set up, not as paths rebuilt every frame.
func TestSvgGLLostUsesPath2D(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t)
	builds := 0
	build := js.FuncOf(func(js.Value, []js.Value) any {
		builds++
		return js.Global().Get("Object").New()
	})
	t.Cleanup(build.Release)
	b.paths.build = build.Value
	b.paths.cache = newMeshCache[*pathEntry](nil)
	b.onSvgGLLost()
	cmd := svgCmd(10, 10)
	for range 3 {
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFramePaths()
	}
	if len(gl.draws) != 0 || len(ctx.fills) != 3 || len(b.paths.cache.entries) != 1 || builds != 2 {
		t.Errorf("draws %v, fills %d, %d cached paths, %d builds; want Path2D fills, cached on the second frame",
			gl.draws, len(ctx.fills), len(b.paths.cache.entries), builds)
	}
	checkNoErrs(t, ctx)
}
