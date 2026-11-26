//go:build darwin && arm64

package mtldraw

import (
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// offscreen is a renderer that draws one frame into an n x n texture
// that the CPU can read.
type offscreen struct {
	c   *metalCtx
	tex objc.ID
	n   int
	// quad is the vertex scratch. On the heap, because drawQuad reads it
	// through a pointer.
	quad *[4]gpu.Vertex
}

// newOffscreen makes the renderer and starts the main pass, cleared to
// black, with the pixel MVP and the solid pipeline bound. It skips the
// test when the host has no Metal device.
func newOffscreen(t testing.TB, n int) *offscreen {
	t.Helper()
	pool := darwinffi.PoolPush()
	t.Cleanup(func() { darwinffi.PoolPop(pool) })
	loadRenderer()
	if dev := objc.ID(msgCall(mtlCreateDevice)); dev == 0 {
		t.Skip("no Metal device")
	} else {
		release(dev)
	}
	layer := newObj(rcls.metalLayer)
	t.Cleanup(func() { release(layer) })
	c := newMetalCtx(layer, msl.Source)
	if c == nil {
		t.Fatal("newMetalCtx failed")
	}
	t.Cleanup(c.destroy)
	o := &offscreen{c: c, n: n, quad: new([4]gpu.Vertex)}
	o.tex = c.newTexture(n, n, mtlPixelFormatBGRA8Unorm, mtlTextureUsageRenderTarget, mtlStorageModeShared, 1)
	if o.tex == 0 {
		t.Fatal("no target texture")
	}
	t.Cleanup(func() { release(o.tex) })
	o.begin()
	return o
}

// begin starts a frame: a command buffer and the main pass.
func (o *offscreen) begin() {
	c := o.c
	c.viewW, c.viewH = o.n, o.n
	c.target = o.tex
	c.cmdBuf = objc.ID(msg(c.queue, rsel.commandBuffer))
	c.beginMainEncoder(0, 0, 0, 1, true)
	var mvp [16]float32
	orthoInto(&mvp, o.n, o.n)
	c.setPipeline(pipeSolid)
	c.setMVP(&mvp)
}

// rect batches one solid rect.
func (o *offscreen) rect(x, y, w, h float32, col gui.Color) {
	*o.quad = gpu.BuildQuad(x, y, w, h, col, 0, 0)
	o.c.drawMainQuad(unsafe.Pointer(o.quad))
}

// finish ends the pass, waits for the GPU and returns the BGRA pixels.
func (o *offscreen) finish(t testing.TB) []byte {
	t.Helper()
	c := o.c
	c.endMainEncoder(false)
	msg(c.cmdBuf, rsel.commit)
	msg(c.cmdBuf, rsel.waitUntilCompleted)
	if st := msg(c.cmdBuf, rsel.status); st != mtlCommandBufferStatusCompleted {
		t.Fatalf("command buffer status %d: %s", st, nsErrorText(objc.ID(msg(c.cmdBuf, rsel.errorSel))))
	}
	c.target, c.cmdBuf = 0, 0
	px := make([]byte, o.n*o.n*4)
	c.region = [6]uint{0, 0, 0, uint(o.n), uint(o.n), 1}
	msg(o.tex, rsel.getBytes, uintptr(unsafe.Pointer(&px[0])), uintptr(o.n*4), ptr(&c.region), 0)
	return px
}

// rgbAt returns the color of pixel x, y.
func rgbAt(px []byte, n, x, y int) (r, g, b byte) {
	i := (y*n + x) * 4
	return px[i+2], px[i+1], px[i]
}

// TestBatchFlushesOnStateChange: quads with the same state go in one
// batch, and a state change draws the open batch before it applies. If
// the scissor were set before the batch was drawn, the red and green
// rects would be clipped away.
func TestBatchFlushesOnStateChange(t *testing.T) {
	o := newOffscreen(t, 32)
	o.rect(0, 0, 16, 16, gui.RGB(255, 0, 0))
	o.rect(16, 0, 16, 16, gui.RGB(0, 255, 0))
	if o.c.batchN != 2 {
		t.Fatalf("batch holds %d quads after two rects, want 2", o.c.batchN)
	}
	// A pipeline and MVP that are already bound keep the batch open.
	var mvp [16]float32
	orthoInto(&mvp, 32, 32)
	o.c.setPipeline(pipeSolid)
	o.c.setMVP(&mvp)
	if o.c.batchN != 2 {
		t.Fatalf("bound state flushed the batch: %d quads left", o.c.batchN)
	}
	o.c.setScissor(16, 16, 16, 16)
	if o.c.batchN != 0 {
		t.Fatal("scissor change left the batch open")
	}
	o.rect(0, 0, 32, 32, gui.RGB(0, 0, 255)) // only the bottom right shows
	px := o.finish(t)

	for _, tc := range []struct {
		x, y    int
		r, g, b byte
	}{
		{8, 8, 255, 0, 0},
		{24, 8, 0, 255, 0},
		{8, 24, 0, 0, 0},
		{24, 24, 0, 0, 255},
	} {
		r, g, b := rgbAt(px, 32, tc.x, tc.y)
		if r != tc.r || g != tc.g || b != tc.b {
			t.Errorf("pixel (%d, %d) = %d,%d,%d, want %d,%d,%d", tc.x, tc.y, r, g, b, tc.r, tc.g, tc.b)
		}
	}
}

// TestEmptyClipSuppressesDraws: an empty clip paints nothing. The
// setter binds no scissor (an out-of-bounds 1x1 sentinel fails Metal
// validation), and every draw below is a no-op until the next
// nonempty clip.
func TestEmptyClipSuppressesDraws(t *testing.T) {
	o := newOffscreen(t, 32)
	o.c.setScissor(0, 0, 0, 0)
	if !o.c.clipEmpty {
		t.Fatal("empty clip did not arm suppression")
	}
	o.rect(0, 0, 32, 32, gui.RGB(0, 255, 0))
	if o.c.batchN != 0 {
		t.Fatalf("suppressed rect batched %d quads, want 0", o.c.batchN)
	}
	o.c.setScissor(0, 0, 32, 32)
	if o.c.clipEmpty {
		t.Fatal("nonempty clip left suppression armed")
	}
	o.rect(0, 0, 32, 32, gui.RGB(255, 0, 0))
	px := o.finish(t)
	if r, g, b := rgbAt(px, 32, 8, 8); r != 255 || g != 0 || b != 0 {
		t.Fatalf("recovered pixel = (%d,%d,%d), want (255,0,0)", r, g, b)
	}
}

// TestBatchSpansLimits draws more quads than one batch holds
// (maxBatchQuads) and than one vertex buffer holds (vchunkBytes). The
// quads overdraw 32 cells in turn with three colors, so a quad that is
// lost or drawn from the wrong vertices changes the color of a cell.
func TestBatchSpansLimits(t *testing.T) {
	const total = 40000
	if total*4*mainStride <= vchunkBytes || total <= maxBatchQuads {
		t.Fatal("the quad count no longer crosses the limits")
	}
	o := newOffscreen(t, 32)
	colors := [3]gui.Color{gui.RGB(255, 0, 0), gui.RGB(0, 255, 0), gui.RGB(0, 0, 255)}
	cell := func(k int) (x, y float32) { return float32(k%8) * 4, float32(k%32/8) * 4 }
	for k := range total {
		x, y := cell(k)
		o.rect(x, y, 4, 4, colors[k/32%3])
	}
	if o.c.chunk == 0 {
		t.Fatal("the quads fit in one vertex buffer")
	}
	px := o.finish(t)
	for j := range 32 {
		last := (total-1-j)/32*32 + j // the last k with k%32 == j
		want := colors[last/32%3]
		x, y := cell(j)
		r, g, b := rgbAt(px, 32, int(x)+2, int(y)+2)
		if r != want.R || g != want.G || b != want.B {
			t.Errorf("cell %d = %d,%d,%d, want %d,%d,%d", j, r, g, b, want.R, want.G, want.B)
		}
	}
}

// TestBatchedDrawDoesNotAllocate holds a batched quad and a flush to 0
// heap allocations with a real encoder.
func TestBatchedDrawDoesNotAllocate(t *testing.T) {
	o := newOffscreen(t, 32)
	col := gui.RGB(255, 255, 255)
	if n := testing.AllocsPerRun(1000, func() { o.rect(1, 1, 2, 2, col) }); n != 0 {
		t.Errorf("drawMainQuad allocates %v times, want 0", n)
	}
	if n := testing.AllocsPerRun(100, func() {
		o.rect(1, 1, 2, 2, col)
		o.c.flush()
	}); n != 0 {
		t.Errorf("drawMainQuad + flush allocates %v times, want 0", n)
	}
	o.finish(t)
}

// BenchmarkBatchedQuads encodes a frame of 20,000 solid quads in one
// run, as a frame of glyphs does. It measures the CPU cost of encoding,
// not the GPU time.
func BenchmarkBatchedQuads(b *testing.B) {
	o := newOffscreen(b, 32)
	o.finish(b)
	col := gui.RGB(255, 255, 255)
	b.ReportAllocs()
	for b.Loop() {
		o.begin()
		for k := range 20000 {
			o.rect(float32(k%32), 0, 1, 1, col)
		}
		o.c.endMainEncoder(false)
		// The GPU time is not part of the cost being measured.
		b.StopTimer()
		msg(o.c.cmdBuf, rsel.commit)
		msg(o.c.cmdBuf, rsel.waitUntilCompleted)
		o.c.target, o.c.cmdBuf = 0, 0
		o.c.chunk, o.c.chunkOff = 0, 0
		b.StartTimer()
	}
}

// TestMSLPinFollowsDeploymentFloor checks the built-in library's MSL
// pin per target. iOS deploys to iOS 15, where MSL 3.0 does not exist,
// so a 3.0 pin there fails the library compile and no renderer starts
// (#1342 review). TestBuiltinShadersCompile, run on the simulator,
// checks that the shaders compile at 2.4.
func TestMSLPinFollowsDeploymentFloor(t *testing.T) {
	if got := mslPin("ios"); got != mtlLanguageVersion2_4 {
		t.Errorf("mslPin(ios) = %#x, want MSL 2.4 (%#x)", got, mtlLanguageVersion2_4)
	}
	if got := mslPin("darwin"); got != mtlLanguageVersion3_0 {
		t.Errorf("mslPin(darwin) = %#x, want MSL 3.0 (%#x)", got, mtlLanguageVersion3_0)
	}
}
