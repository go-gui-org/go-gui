//go:build js && wasm

package web

import (
	"fmt"
	"strings"
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
	// live is the JS side's mesh table; drawIDs the ids each draw used,
	// and missing the records whose id was not in the table (gogui.js
	// skips those silently).
	live    map[int]bool
	drawIDs []int
	missing int
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
	on("upload", func(a []js.Value) {
		g.uploads = append(g.uploads, a[1].Int())
		g.live[a[1].Int()] = true
	})
	on("remove", func(a []js.Value) {
		g.removes = append(g.removes, a[1].Int())
		delete(g.live, a[1].Int())
	})
	on("resize", func([]js.Value) {})
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
	})
	on("composite", func([]js.Value) {
		s := ctx.cur
		s.filter = ctx.filterNow()
		g.composites = append(g.composites, s.String())
	})
	return g
}

// newGLTestBackend is a backend in svg=webgl mode over the fakes, at
// dpi 1 on a 1000×1000 canvas. minTris sets the light-batch threshold.
func newGLTestBackend(t *testing.T, minTris int) (*Backend, *stateCtx, *fakeGL) {
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
		meshes:  map[meshKey]*glMesh{},
		minTris: minTris,
	}
	return b, ctx, gl
}

// Rect clips between SVGs do not split the batch: one composite, each
// draw scissored to its own clip, composited under no rect clip.
func TestSvgGLBatchesAcrossRectClips(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
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
	b, ctx, _ := newGLTestBackend(t, 1)
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
	b, ctx, gl := newGLTestBackend(t, 1)
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
	b, ctx, gl := newGLTestBackend(t, 1)
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

// A view transform works the same way, and a batch whose bounds in the
// view's frame leave the drawing buffer is filled with Canvas2D rather
// than cut.
func TestSvgGLViewTransform(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
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

	b.drawCmds([]gui.RenderCmd{
		{Kind: gui.RenderTransformBegin, Homography: shift, W: 200, H: 200},
		svgCmd(-50, 10), // left of the buffer in the view's frame
		{Kind: gui.RenderTransformEnd},
	})
	if len(gl.composites) != 1 || len(ctx.fills) != 1 {
		t.Errorf("composites %v fills %v, want the off-buffer batch filled with Canvas2D",
			gl.composites, ctx.fills)
	}
	checkNoErrs(t, ctx)
}

// A filter bracket's filter must still be set when its batch is
// composited.
func TestSvgGLFilterBracketKeepsFilter(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
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

// A batch below minTris is filled with Canvas2D, each command under the
// rect clip it was queued under, and costs no WebGL draw or composite.
func TestSvgGLLightBatchFillsWithClips(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1000)
	b.drawCmds([]gui.RenderCmd{
		clipCmd(0, 0, 500, 500), svgCmd(10, 10),
		clipCmd(100, 100, 50, 50), svgCmd(110, 110),
	})
	if len(gl.draws)+len(gl.composites) != 0 {
		t.Errorf("light batch used WebGL: draws %v composites %v", gl.draws, gl.composites)
	}
	if len(ctx.fills) != 2 {
		t.Fatalf("fills = %v, want 2", ctx.fills)
	}
	for i, wantClip := range []string{"[0,0 500x500]", "[100,100 50x50]"} {
		if !strings.Contains(ctx.fills[i], "clips="+wantClip) {
			t.Errorf("fill %d = %s, want clips=%s", i, ctx.fills[i], wantClip)
		}
	}
	checkNoErrs(t, ctx)
}

// After a context loss SVGs draw with Canvas2D and a repaint is asked
// for; after the restore the program is rebuilt and meshes upload
// again into the new context.
func TestSvgGLContextLossAndRestore(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
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
	b, ctx, gl := newGLTestBackend(t, 1)
	b.drawSvg(func() *gui.RenderCmd { c := svgCmd(10, 10); return &c }())
	b.onSvgGLLost()
	b.flushSvgGL()
	if len(gl.composites) != 0 || len(ctx.fills) != 1 {
		t.Errorf("composites %v fills %v, want the queued SVG filled with Canvas2D",
			gl.composites, ctx.fills)
	}
	checkNoErrs(t, ctx)
}

// The mesh cache uploads a mesh once and uploads again when its buffer
// is rewritten in place; the old variant stays until it idles out.
func TestSvgGLMeshCache(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
	cmd := svgCmd(10, 10)
	frame := func() {
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFrameSvgGL()
	}
	frame()
	frame()
	if len(gl.uploads) != 1 {
		t.Fatalf("uploads after two frames = %v, want 1", gl.uploads)
	}
	cmd.Triangles[2] = 20 // same backing array, new content
	frame()
	if len(gl.uploads) != 2 || len(gl.removes) != 0 || gl.missing != 0 {
		t.Fatalf("after an in-place rewrite: uploads %v removes %v missing %d, want a second variant",
			gl.uploads, gl.removes, gl.missing)
	}
	for range pathCacheIdleFrames + 60 {
		b.endFrameSvgGL()
	}
	if len(b.gl.meshes) != 0 || len(gl.removes) != 2 || len(b.gl.vars) != 0 {
		t.Errorf("after idling: %d meshes, %d variant lists, removes %v; want all evicted",
			len(b.gl.meshes), len(b.gl.vars), gl.removes)
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
// both are drawn from live meshes in one batch, neither replaces the
// other, and the next frame uploads nothing.
func TestSvgGLColorVariantsCoexist(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
	tris := []float32{0, 0, 10, 0, 0, 10}
	cmds := []gui.RenderCmd{
		gradSvgCmd(10, 10, tris, gui.RGB(255, 0, 0)),
		gradSvgCmd(40, 10, tris, gui.RGB(0, 255, 0)), // same slice, other tint
	}
	b.drawCmds(cmds)
	b.endFrameSvgGL()
	if len(gl.uploads) != 2 || len(gl.removes) != 0 || gl.missing != 0 {
		t.Fatalf("uploads %v removes %v missing %d, want two live variants",
			gl.uploads, gl.removes, gl.missing)
	}
	if fmt.Sprint(gl.drawIDs) != fmt.Sprint(gl.uploads) {
		t.Errorf("drawn ids %v, want both variants %v", gl.drawIDs, gl.uploads)
	}
	b.drawCmds(cmds)
	if len(gl.uploads) != 2 || gl.missing != 0 {
		t.Errorf("second frame: uploads %v missing %d, want no re-upload", gl.uploads, gl.missing)
	}
	checkNoErrs(t, ctx)
}

// A buffer rewritten every frame keeps at most maxMeshVariants meshes,
// and a variant drawn in the current frame is never the one dropped.
func TestSvgGLVariantCap(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
	cmd := svgCmd(10, 10)
	for i := range 3 * maxMeshVariants {
		cmd.Triangles[2] = float32(20 + i)
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFrameSvgGL()
		if n := len(b.gl.vars[meshBase{ptr: meshKeyFor(&cmd).ptr, n: len(cmd.Triangles)}]); n > maxMeshVariants {
			t.Fatalf("frame %d: %d variants, want at most %d", i, n, maxMeshVariants)
		}
	}
	if gl.missing != 0 {
		t.Errorf("%d draws used a dropped mesh", gl.missing)
	}
	if len(b.gl.meshes) != maxMeshVariants {
		t.Errorf("%d meshes cached, want %d", len(b.gl.meshes), maxMeshVariants)
	}
	checkNoErrs(t, ctx)
}

// Freed mesh ids are reused, so the ids sent through the Float32Array
// stay small however many meshes come and go.
func TestSvgGLIDsRecycled(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1)
	cmd := svgCmd(10, 10)
	for i := range 1000 {
		cmd.Triangles[2] = float32(20 + i)
		b.drawCmds([]gui.RenderCmd{cmd})
		b.endFrameSvgGL()
	}
	maxID := 0
	for _, id := range gl.uploads {
		maxID = max(maxID, id)
	}
	if maxID > maxMeshVariants+1 {
		t.Errorf("highest mesh id after 1000 rewrites = %d, want ids recycled (<= %d)",
			maxID, maxMeshVariants+1)
	}
	if gl.missing != 0 {
		t.Errorf("%d draws used a dropped mesh", gl.missing)
	}
	checkNoErrs(t, ctx)
}

// A light batch with an interpolated gradient stays on WebGL: Canvas2D
// would draw it flat, so how it looks would depend on its neighbors.
// A light batch of flat per-triangle colors is still filled.
func TestSvgGLGradientStaysOnWebGL(t *testing.T) {
	b, ctx, gl := newGLTestBackend(t, 1000)
	tris := []float32{0, 0, 10, 0, 0, 10}
	b.drawCmds([]gui.RenderCmd{gradSvgCmd(10, 10, tris, gui.RGB(255, 0, 0))})
	if len(gl.draws) != 1 || len(ctx.fills) != 0 {
		t.Errorf("gradient: draws %v fills %v, want WebGL", gl.draws, ctx.fills)
	}
	flat := gradSvgCmd(40, 10, []float32{0, 0, 10, 0, 0, 10}, gui.RGB(255, 0, 0))
	flat.VertexColors = []gui.Color{gui.RGB(1, 2, 3), gui.RGB(1, 2, 3), gui.RGB(1, 2, 3)}
	flat.VertexColorsFlat = true
	b.drawCmds([]gui.RenderCmd{flat})
	if len(gl.draws) != 1 || len(ctx.fills) != 1 {
		t.Errorf("flat colors: draws %v fills %v, want Canvas2D", gl.draws, ctx.fills)
	}
	checkNoErrs(t, ctx)
}
