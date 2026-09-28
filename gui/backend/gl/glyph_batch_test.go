//go:build !js && !darwin && !android && !cgo

package gl

import (
	"bytes"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
	gogl "github.com/go-gui-org/go-gui/gui/backend/internal/glbind"
)

// Glyph quad batching (#816). A batch collects the quads glyph emits during
// one text command and draws them with one DrawElements per run of quads
// from the same atlas page. These tests need a real GL context: they run in
// CI through scripts/gl-render-test.sh and skip elsewhere without a display.

// newBatchProbeBackend opens a real backend with MSAA off, so frames draw
// straight into the readback framebuffer and pixel values are exact.
func newBatchProbeBackend(t *testing.T) *Backend {
	t.Helper()
	b := newProbeBackend(t)
	b.msaaMain.destroy()
	b.msaaFilter.destroy()
	b.msaaSamples = 1
	return b
}

// readbackTarget is an offscreen RGBA framebuffer the size of the window.
type readbackTarget struct {
	tex glTexture
	fbo uint32
}

func newReadbackTarget(t *testing.T, b *Backend) *readbackTarget {
	t.Helper()
	rt := &readbackTarget{tex: createEmptyTexture(b.physW, b.physH)}
	gogl.GenFramebuffers(1, &rt.fbo)
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, rt.fbo)
	gogl.FramebufferTexture2D(gogl.FRAMEBUFFER, gogl.COLOR_ATTACHMENT0,
		gogl.TEXTURE_2D, rt.tex.id, 0)
	if s := gogl.CheckFramebufferStatus(gogl.FRAMEBUFFER); s != gogl.FRAMEBUFFER_COMPLETE {
		t.Fatalf("readback framebuffer incomplete: 0x%x", s)
	}
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, 0)
	t.Cleanup(func() {
		gogl.DeleteFramebuffers(1, &rt.fbo)
		gogl.DeleteTextures(1, &rt.tex.id)
	})
	return rt
}

// render clears the target to opaque black, runs draw into it and returns
// every pixel. GL rows come back bottom-up; the tests only compare frames
// or sample pixels through pixelAt, so the order does not matter.
func (rt *readbackTarget) render(b *Backend, draw func()) []byte {
	b.plat.makeCurrent()
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, rt.fbo)
	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	gogl.Viewport(0, 0, b.physW, b.physH)
	gogl.ClearColor(0, 0, 0, 1)
	gogl.Clear(gogl.COLOR_BUFFER_BIT | gogl.STENCIL_BUFFER_BIT)

	draw()

	gogl.Disable(gogl.SCISSOR_TEST)
	b.scissorOn = false
	px := make([]byte, int(b.physW)*int(b.physH)*4)
	gogl.BindFramebuffer(gogl.READ_FRAMEBUFFER, rt.fbo)
	gogl.ReadPixels(0, 0, b.physW, b.physH,
		gogl.RGBA, gogl.UNSIGNED_BYTE, unsafe.Pointer(&px[0]))
	gogl.BindFramebuffer(gogl.FRAMEBUFFER, 0)
	return px
}

// pixelAt returns the RGBA of physical pixel (x, y), y counted from the top.
func pixelAt(b *Backend, px []byte, x, y int) [4]byte {
	row := int(b.physH) - 1 - y
	i := (row*int(b.physW) + x) * 4
	return [4]byte{px[i], px[i+1], px[i+2], px[i+3]}
}

// solidPage makes a glyph texture filled with one opaque color.
func solidPage(gb *glyphBackend, size int, c [4]byte) (glyph.TextureID, []byte) {
	id := gb.NewTexture(size, size)
	data := make([]byte, size*size*4)
	fillPixels(data, c)
	gb.UpdateTexture(id, data)
	return id, data
}

func fillPixels(data []byte, c [4]byte) {
	for i := 0; i < len(data); i += 4 {
		copy(data[i:i+4], c[:])
	}
}

var (
	red   = [4]byte{255, 0, 0, 255}
	green = [4]byte{0, 255, 0, 255}
	blue  = [4]byte{0, 0, 255, 255}
)

// quadAt draws the whole of page id into the physical-pixel square at
// (x, y) with side n. Coordinates are divided by the DPI scale because
// glyphBackend multiplies them back.
func quadAt(b *Backend, id glyph.TextureID, size, x, y, n int) {
	s := b.dpiScale
	b.glyphBack.DrawTexturedQuad(id,
		glyph.Rect{Width: float32(size), Height: float32(size)},
		glyph.Rect{X: float32(x) / s, Y: float32(y) / s,
			Width: float32(n) / s, Height: float32(n) / s},
		glyph.Color{R: 255, G: 255, B: 255, A: 255})
}

// TestGlyphBatchFlushCounts pins the success criterion of #816 as draw
// counts: quads from one page share a draw, a page switch starts a new one,
// and a full batch draws at once.
func TestGlyphBatchFlushCounts(t *testing.T) {
	b := newBatchProbeBackend(t)
	gb := b.glyphBack
	const size = 4
	pageA, _ := solidPage(gb, size, red)
	pageB, _ := solidPage(gb, size, green)

	count := func(draw func()) int {
		b.plat.makeCurrent()
		b.useGlyphPipeline()
		before := gb.flushes
		draw()
		b.restoreAfterGlyph()
		return gb.flushes - before
	}

	if got := count(func() {
		for i := range 40 {
			quadAt(b, pageA, size, i, 0, 1)
		}
	}); got != 1 {
		t.Errorf("40 quads from one page took %d draws, want 1", got)
	}
	if got := count(func() {
		quadAt(b, pageA, size, 0, 0, 1)
		quadAt(b, pageA, size, 1, 0, 1)
		quadAt(b, pageB, size, 2, 0, 1)
		quadAt(b, pageA, size, 3, 0, 1)
	}); got != 3 {
		t.Errorf("pages A,A,B,A took %d draws, want 3", got)
	}
	if got := count(func() {}); got != 0 {
		t.Errorf("an empty text command took %d draws, want 0", got)
	}
	if got := count(func() {
		for i := range maxGlyphQuads + 1 {
			quadAt(b, pageA, size, i%8, 0, 1)
		}
	}); got != 2 {
		t.Errorf("%d quads from one page took %d draws, want 2 "+
			"(one full batch, one remainder)", maxGlyphQuads+1, got)
	}
}

// TestGlyphBatchFlushesBeforeUpload guards the mid-frame atlas eviction
// case. go-glyph can reset a page while a text command is still drawing
// (atlas.go resetPage) and re-upload it before emitting the next quads.
// Quads queued before the upload must sample the old texels, so the upload
// has to draw them first. Without that flush the first square renders from
// the new contents.
func TestGlyphBatchFlushesBeforeUpload(t *testing.T) {
	b := newBatchProbeBackend(t)
	gb := b.glyphBack
	rt := newReadbackTarget(t, b)
	const size = 4
	page, data := solidPage(gb, size, red)
	other, otherData := solidPage(gb, size, blue)

	px := rt.render(b, func() {
		b.useGlyphPipeline()
		quadAt(b, page, size, 0, 0, 8)
		// An upload to another page must leave this batch queued: it
		// cannot change what the queued quads sample.
		before := gb.flushes
		gb.UpdateTextureRect(other, otherData, size*4, 0, 0, size, size)
		if gb.flushes != before {
			t.Errorf("upload to an unrelated page flushed the batch")
		}
		fillPixels(data, green)
		gb.UpdateTexture(page, data)
		quadAt(b, page, size, 16, 0, 8)
		b.restoreAfterGlyph()
	})

	if got := pixelAt(b, px, 4, 4); got != red {
		t.Errorf("quad queued before the upload = %v, want red %v", got, red)
	}
	if got := pixelAt(b, px, 20, 4); got != green {
		t.Errorf("quad queued after the upload = %v, want green %v", got, green)
	}
}

// TestGlyphBatchFlushesBeforeDelete: a queued quad whose page is deleted
// must already be drawn, not left to sample a dead texture name.
func TestGlyphBatchFlushesBeforeDelete(t *testing.T) {
	b := newBatchProbeBackend(t)
	gb := b.glyphBack
	rt := newReadbackTarget(t, b)
	const size = 4
	page, _ := solidPage(gb, size, red)

	px := rt.render(b, func() {
		b.useGlyphPipeline()
		quadAt(b, page, size, 0, 0, 8)
		gb.DeleteTexture(page)
		b.restoreAfterGlyph()
	})
	if got := pixelAt(b, px, 4, 4); got != red {
		t.Errorf("quad queued before its page was deleted = %v, want red", got)
	}
}

// TestGlyphBatchFlushesBeforeFilledRect: DrawFilledRect (text backgrounds,
// underlines, strikethrough) draws at once, not queued, so it must draw the
// queued glyphs first or it lands underneath glyphs emitted before it.
//
// The page is white, so the quad renders its vertex color (red). The fill
// samples texel (0, 0) of the bound texture (#835); with the flush in place
// that is the white page, so the fill renders its own color (blue) on top.
// Without the flush the fill draws first and the red quad covers it.
func TestGlyphBatchFlushesBeforeFilledRect(t *testing.T) {
	b := newBatchProbeBackend(t)
	gb := b.glyphBack
	rt := newReadbackTarget(t, b)
	const size = 4
	page, _ := solidPage(gb, size, [4]byte{255, 255, 255, 255})
	s := b.dpiScale
	sq := glyph.Rect{Width: 8 / s, Height: 8 / s}

	px := rt.render(b, func() {
		b.useGlyphPipeline()
		gb.DrawTexturedQuad(page,
			glyph.Rect{Width: size, Height: size}, sq,
			glyph.Color{R: 255, A: 255})
		gb.DrawFilledRect(sq, glyph.Color{B: 255, A: 255})
		b.restoreAfterGlyph()
	})
	if got := pixelAt(b, px, 4, 4); got != blue {
		t.Errorf("fill drawn after a queued glyph = %v, want blue %v on top",
			got, blue)
	}
}

// batchTextCmds is a text-heavy frame: two overlapping runs of different
// colors, a clip that cuts through the second, and a third run after the
// clip. Overlap makes draw order visible; the clip makes a batch that
// leaked across a scissor change visible.
func batchTextCmds(b *Backend) []gui.RenderCmd {
	w := float32(b.physW) / b.dpiScale
	h := float32(b.physH) / b.dpiScale
	return []gui.RenderCmd{
		{Kind: gui.RenderText, X: 2, Y: 2, FontSize: 18,
			Color: gui.RGB(255, 80, 80), Text: "Batch glyphs 0123 ÄÖÜ"},
		{Kind: gui.RenderText, X: 6, Y: 6, FontSize: 18,
			Color: gui.RGB(80, 255, 80), Text: "Overlap WWW mmm ___"},
		{Kind: gui.RenderClip, X: 0, Y: 0, W: w / 2, H: h},
		{Kind: gui.RenderText, X: 2, Y: 30, FontSize: 14,
			Color: gui.RGB(80, 80, 255), Text: "Clipped half of this line"},
		{Kind: gui.RenderClip, X: 0, Y: 0, W: w, H: h},
		{Kind: gui.RenderText, X: 2, Y: 50, FontSize: 12,
			Color: gui.White, Text: "Tail run after the clip"},
	}
}

func drawCmds(b *Backend, cmds []gui.RenderCmd) {
	for i := range cmds {
		r := &cmds[i]
		switch r.Kind {
		case gui.RenderClip:
			b.drawClip(r)
		case gui.RenderText:
			b.drawText(r)
		}
	}
	b.textSys.Commit()
}

// TestGlyphBatchMatchesUnbatched renders the same text frame batched and
// with batching turned off (a batch of one quad draws each quad at once,
// which is the pre-#816 call sequence) and requires identical pixels.
//
// The batched frame goes first, on a cold atlas: glyph rasterizes and
// uploads mid-frame while quads are queued, the path most likely to draw
// from a stale page. The unbatched frame then reuses the warm atlas.
func TestGlyphBatchMatchesUnbatched(t *testing.T) {
	b := newBatchProbeBackend(t)
	rt := newReadbackTarget(t, b)
	cmds := batchTextCmds(b)

	batched := rt.render(b, func() { drawCmds(b, cmds) })
	b.glyphBack.batchCap = 1
	unbatched := rt.render(b, func() { drawCmds(b, cmds) })

	lit := 0
	for i := 0; i < len(unbatched); i += 4 {
		if unbatched[i]|unbatched[i+1]|unbatched[i+2] != 0 {
			lit++
		}
	}
	if lit < 100 {
		t.Fatalf("unbatched frame lit %d pixels; the text did not render, "+
			"so the comparison would prove nothing", lit)
	}
	if !bytes.Equal(batched, unbatched) {
		diff := 0
		for i := range batched {
			if batched[i] != unbatched[i] {
				diff++
			}
		}
		t.Fatalf("batched frame differs from unbatched in %d bytes", diff)
	}
}
