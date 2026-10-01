//go:build !js && !darwin && !android && !cgo

package gl

import (
	"os"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// probeSize is the side of the square the probe triangle fills, in physical
// pixels. Its diagonal edge crosses probeSize pixels.
const probeSize = 16

// newProbeBackend opens a real GL backend. Without a display or a GL 3.3
// driver it skips, unless GOGUI_REQUIRE_GL is set: the CI jobs that provide
// Mesa set it, so a broken context there fails instead of passing silently.
func newProbeBackend(t *testing.T) *Backend {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := New(w)
	if err == nil && b == nil {
		t.Skip("no GL backend on this platform")
	}
	if err != nil {
		if os.Getenv("GOGUI_REQUIRE_GL") != "" {
			t.Fatalf("GOGUI_REQUIRE_GL is set but the backend failed: %v", err)
		}
		t.Skipf("backend init failed (no display?): %v", err)
	}
	t.Cleanup(b.Destroy)
	return b
}

// probeTriangle is a white right triangle filling the top-left probeSize
// square. Scale undoes the backend's DPI multiply, so its vertices land on
// exact physical pixels whatever the display scale.
func probeTriangle(b *Backend) *gui.RenderCmd {
	return &gui.RenderCmd{
		Kind:      gui.RenderSvg,
		Triangles: []float32{0, 0, probeSize, 0, 0, probeSize},
		Color:     gui.White,
		Scale:     1 / b.dpiScale,
	}
}

// edgeCoverage draws a white right triangle through the real SVG path and
// frame target, resolves the frame into an offscreen single-sample
// framebuffer, and counts the pixels in the triangle's square that are only
// partly covered. Without antialiasing every pixel is fully in or fully out
// and the count is 0; with 4x MSAA the pixels on the diagonal land between.
func edgeCoverage(t *testing.T, b *Backend) int {
	t.Helper()
	b.plat.makeCurrent()

	// Readback target: the window's back buffer cannot be read reliably
	// after a resolve on every platform, an offscreen texture can.
	readTex := createEmptyTexture(b.physW, b.physH)
	defer gogl.DeleteTextures(1, &readTex.id)
	var readFBO uint32
	gogl.GenFramebuffers(1, &readFBO)
	defer gogl.DeleteFramebuffers(1, &readFBO)
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, readFBO)
	gogl.FramebufferTexture2D(gogl.FRAMEBUFFER, gogl.COLOR_ATTACHMENT0,
		gogl.TEXTURE_2D, readTex.id, 0)
	if s := gogl.CheckFramebufferStatus(gogl.FRAMEBUFFER); s != gogl.FRAMEBUFFER_COMPLETE {
		t.Fatalf("readback framebuffer incomplete: 0x%x", s)
	}

	// The same steps renderFrame takes, with the window swapped for readFBO.
	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	b.beginMainPass()
	if b.msaaMain.fbo == 0 {
		// No MSAA: renderFrame draws straight into the window. Draw
		// straight into the readback target instead.
		gogl.BindFramebuffer(gogl.FRAMEBUFFER, readFBO)
	}
	gogl.Viewport(0, 0, b.physW, b.physH)
	gogl.ClearColor(0, 0, 0, 1)
	gogl.Clear(gogl.COLOR_BUFFER_BIT | gogl.STENCIL_BUFFER_BIT)

	b.drawSvg(probeTriangle(b))
	b.flushSvg()
	b.endMainPass(readFBO)

	return countPartial(b, readFBO)
}

// countPartial reads the top-left probeSize square of fbo's color attachment
// and counts the pixels whose green channel is neither 0 nor 255.
func countPartial(b *Backend, fbo uint32) int {
	var px [probeSize * probeSize * 4]byte
	gogl.BindFramebuffer(gogl.READ_FRAMEBUFFER, fbo)
	// GL rows run bottom-up; the triangle sits at the top of the frame.
	gogl.ReadPixels(0, b.physH-probeSize, probeSize, probeSize,
		gogl.RGBA, gogl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, 0)

	partial := 0
	for i := 0; i < len(px); i += 4 {
		if g := px[i+1]; g > 0 && g < 255 {
			partial++
		}
	}
	return partial
}

func TestTriangleEdgesAntialiased(t *testing.T) {
	b := newProbeBackend(t)
	if b.msaaSamples < 2 {
		t.Skipf("driver offers no multisampling (samples=%d)", b.msaaSamples)
	}
	// Every pixel on the diagonal is partly covered; 8 leaves room for
	// sample patterns that put a few of them fully in or out.
	if got := edgeCoverage(t, b); got < probeSize/2 {
		t.Fatalf("diagonal triangle edge has %d partly covered pixels, "+
			"want >= %d: the main pass is not multisampled", got, probeSize/2)
	}
}

// TestProbeSeesAliasedEdges is the control for the test above: with MSAA
// turned off the same probe must count no partial pixels. Without it, a probe
// that always reports soft edges would pass the real test for any backend.
func TestProbeSeesAliasedEdges(t *testing.T) {
	b := newProbeBackend(t)
	b.msaaMain.destroy()
	b.msaaSamples = 1
	if got := edgeCoverage(t, b); got != 0 {
		t.Fatalf("without MSAA the probe counted %d partly covered pixels, want 0", got)
	}
}

// TestRefusedResolveFallsBack checks the fallback for a window whose format
// the multisample resolve cannot write. Mesa accepts the RGBA8 texture the
// other tests use, so the refusal is forced another way: the destination is a
// framebuffer name that was never generated, which a core-profile context
// refuses with the same INVALID_OPERATION a format mismatch gives. (A
// multisampled destination does not work for this: newer GL, and Mesa, accept
// a blit between two targets with equal sample counts.) After the refusal the
// main target must be gone for good, so the next frame draws straight into
// the window.
func TestRefusedResolveFallsBack(t *testing.T) {
	b := newProbeBackend(t)
	if b.msaaSamples < 2 {
		t.Skipf("driver offers no multisampling (samples=%d)", b.msaaSamples)
	}
	b.plat.makeCurrent()
	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	b.beginMainPass()
	if b.msaaMain.fbo == 0 {
		t.Fatal("main pass has no multisampled target")
	}
	const neverGenerated = 0x7fff_ffff
	b.endMainPass(neverGenerated)

	if b.msaaMain.fbo != 0 || !b.msaaMain.failed {
		t.Fatalf("refused resolve kept the target (fbo=%d failed=%v); "+
			"the window would stay blank", b.msaaMain.fbo, b.msaaMain.failed)
	}
	b.beginMainPass()
	if b.msaaMain.fbo != 0 {
		t.Fatal("beginMainPass re-created the target after a refused resolve")
	}
}

// filterCoverage draws the probe triangle inside a filter container (no blur,
// no color matrix) and counts the partly covered pixels in filterTexA, the
// texture the blur and color passes read.
//
// Before endFilter the clip moves to a small rect away from the triangle, as
// a later child's clip would. The multisampled filter content must still be
// resolved in full: glBlitFramebuffer obeys the scissor, so a resolve that
// did not lift it would copy only that small rect, and the triangle would be
// missing from filterTexA.
func filterCoverage(t *testing.T, b *Backend) int {
	t.Helper()
	b.plat.makeCurrent()
	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	b.beginMainPass()
	gogl.Viewport(0, 0, b.physW, b.physH)
	gogl.ClearColor(0, 0, 0, 1)
	gogl.Clear(gogl.COLOR_BUFFER_BIT | gogl.STENCIL_BUFFER_BIT)

	s := b.dpiScale
	b.beginFilter(&gui.RenderCmd{Kind: gui.RenderFilterBegin, Layers: 1})
	b.drawClip(&gui.RenderCmd{Kind: gui.RenderClip,
		W: float32(b.physW) / s, H: float32(b.physH) / s})
	b.drawSvg(probeTriangle(b))
	b.flushSvg()
	far := float32(probeSize+8) / s
	b.drawClip(&gui.RenderCmd{Kind: gui.RenderClip,
		X: far, Y: far, W: 4 / s, H: 4 / s})
	b.endFilter()
	if !b.scissorOn {
		t.Fatal("endFilter left scissorOn false; the clip must survive the resolve")
	}

	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	b.bindFBO(b.filterTexA)
	return countPartial(b, b.filterFBO)
}

func TestFilterEdgesAntialiased(t *testing.T) {
	b := newProbeBackend(t)
	if b.msaaSamples < 2 {
		t.Skipf("driver offers no multisampling (samples=%d)", b.msaaSamples)
	}
	if got := filterCoverage(t, b); got < probeSize/2 {
		t.Fatalf("filter content has %d partly covered pixels on the triangle "+
			"edge, want >= %d: the filter pass is not multisampled, or its "+
			"resolve was clipped", got, probeSize/2)
	}
}

// TestFilterProbeSeesAliasedEdges is the control for the filter test: with
// MSAA off the filter content goes straight into filterTexA with hard edges.
func TestFilterProbeSeesAliasedEdges(t *testing.T) {
	b := newProbeBackend(t)
	b.msaaMain.destroy()
	b.msaaFilter.destroy()
	b.msaaSamples = 1
	if got := filterCoverage(t, b); got != 0 {
		t.Fatalf("without MSAA the filter probe counted %d partly covered pixels, want 0", got)
	}
}
