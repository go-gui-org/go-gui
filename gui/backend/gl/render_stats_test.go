//go:build !js && !darwin && !android && !cgo

package gl

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
	"github.com/go-gui-org/go-gui/gui/backend/internal/rstats"
)

// TestRenderStatsCounts pins what GOGUI_RENDER_STATS counts on GL (#1303).
// A rect is one draw of its own, since SDF quads are not batched here. A
// glyph run is one draw per page, and the end of the text command is its
// own cause. A run of SVG meshes is one draw.
func TestRenderStatsCounts(t *testing.T) {
	b := newBatchProbeBackend(t)
	b.plat.makeCurrent()
	b.stats = rstats.New("test", io.Discard, time.Now, time.Hour)
	b.glyphBack.stats = b.stats
	gb := b.glyphBack

	// Two rects: two draws, two pipeline binds, no batch causes.
	drawStream(b, []gui.RenderCmd{
		{Kind: gui.RenderRect, Fill: true, X: 0, Y: 0, W: 4, H: 4, Color: gui.RGB(255, 0, 0)},
		{Kind: gui.RenderRect, Fill: true, X: 4, Y: 0, W: 4, H: 4, Color: gui.RGB(0, 255, 0)},
	}, false)
	c := b.stats.Counts()
	if c.Draws != 2 || c.Quads != 2 {
		t.Errorf("two rects: draws, quads = %d, %d; want 2, 2", c.Draws, c.Quads)
	}
	if c.PipeBinds < 2 {
		t.Errorf("two rects: %d pipeline binds, want at least 2", c.PipeBinds)
	}

	// Pages A,A,B,A: a texture change twice, then the end of the command.
	const size = 4
	pageA, _ := solidPage(gb, size, red)
	pageB, _ := solidPage(gb, size, green)
	b.stats = rstats.New("test", io.Discard, time.Now, time.Hour)
	gb.stats = b.stats
	b.useGlyphPipeline()
	quadAt(b, pageA, size, 0, 0, 1)
	quadAt(b, pageA, size, 1, 0, 1)
	quadAt(b, pageB, size, 2, 0, 1)
	quadAt(b, pageA, size, 3, 0, 1)
	b.restoreAfterGlyph()
	c = b.stats.Counts()
	if c.Flush[rstats.CauseTexture] != 2 || c.Flush[rstats.CauseText] != 1 {
		t.Errorf("A,A,B,A: texture, text flushes = %d, %d; want 2, 1",
			c.Flush[rstats.CauseTexture], c.Flush[rstats.CauseText])
	}
	if c.Quads != 4 || c.Upload != 4*glyphQuadBytes {
		t.Errorf("A,A,B,A: quads, upload = %d, %d; want 4, %d",
			c.Quads, c.Upload, 4*glyphQuadBytes)
	}

	// Three SVG meshes in a row: one draw.
	b.stats = rstats.New("test", io.Discard, time.Now, time.Hour)
	gb.stats = b.stats
	drawStream(b, []gui.RenderCmd{
		svgSquare(b, 0, 0, 4, gui.RGB(255, 0, 0)),
		svgSquare(b, 4, 0, 4, gui.RGB(0, 255, 0)),
		svgSquare(b, 8, 0, 4, gui.RGB(0, 0, 255)),
	}, false)
	c = b.stats.Counts()
	if c.Draws != 1 || c.Verts == 0 {
		t.Errorf("three SVG meshes: draws, verts = %d, %d; want 1, >0", c.Draws, c.Verts)
	}
}

// TestGPUTimerReportsFrames runs frames with the timer on and expects at
// least one GPU sample once the ring has wrapped. llvmpipe implements
// GL_TIME_ELAPSED, so a missing sample means the ring never reads back.
func TestGPUTimerReportsFrames(t *testing.T) {
	b := newBatchProbeBackend(t)
	b.plat.makeCurrent()
	defer b.gpuTimer.destroy()
	var gpuSamples int
	rec := rstats.New("test", gpuCounter{&gpuSamples}, time.Now, 0)
	for range gpuTimerRing + 2 {
		rec.BeginFrame()
		b.gpuTimer.begin(rec)
		drawStream(b, []gui.RenderCmd{
			{Kind: gui.RenderRect, Fill: true, X: 0, Y: 0, W: 8, H: 8, Color: gui.RGB(255, 0, 0)},
		}, false)
		b.gpuTimer.end()
		// A real frame swaps, which submits the query. The probe never
		// swaps, so Finish stands in: without it llvmpipe holds the
		// commands and no result ever becomes available.
		gogl.Finish()
		rec.EndFrame() // period 0: reports, and clears, every frame
	}
	if gpuSamples == 0 {
		t.Fatal("no frame reported a GPU time")
	}
}

// gpuCounter counts summary lines whose gpu_us_p50 is not -1.
type gpuCounter struct{ n *int }

func (g gpuCounter) Write(p []byte) (int, error) {
	if !bytes.Contains(p, []byte(" gpu_us_p50=-1 ")) {
		*g.n++
	}
	return len(p), nil
}
