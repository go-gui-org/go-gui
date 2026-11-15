//go:build !wasm

package soft

import (
	"bytes"
	"fmt"
	"image"
	"math"
	"runtime"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Tests for Renderer, the live-window entry point (#1265). The contract they
// pin: one Draw call produces the same pixels as RenderToImage for the same
// settled frame, on a cold glyph atlas too, while reusing one buffer.

// settle lays a window out the way a live backend's FrameFn would before a
// draw, without RenderToImage's preparation. The headless flag is set only so
// wall-clock-driven visuals match RenderToImage, which sets it.
func settle(w *gui.Window) {
	w.SetHeadlessRender(true)
	w.TestRender(nil)
}

// TestRendererMatchesPixelGoldens: for every pixel-golden case and theme, a
// Renderer's Draw is byte-identical to RenderToImage under the same pinned
// metrics. This is the issue's "draws the same pixels as the soft golden
// tests" acceptance at the package level; the X11 test repeats it on screen.
func TestRendererMatchesPixelGoldens(t *testing.T) {
	for _, c := range pixelCases() {
		themes := c.themes
		if themes == nil {
			themes = pixelThemes()
		}
		for _, th := range themes {
			t.Run(c.name+"."+th.name, func(t *testing.T) {
				want, err := renderPixelGolden(t, th.theme, c)
				if err != nil {
					t.Fatalf("RenderToImage: %v", err)
				}

				w := gui.NewWindow(gui.WindowCfg{
					Width:  pixelGoldenW,
					Height: pixelGoldenH,
					OnInit: func(w *gui.Window) { w.SetView(c.build) },
				})
				w.SetTheme(th.theme)
				// A live backend runs OnInit itself; NewRenderer does not.
				w.Config.OnInit(w)
				lr, err := NewRenderer(w, 1)
				if err != nil {
					t.Fatalf("NewRenderer: %v", err)
				}
				t.Cleanup(lr.Release)
				w.SetTextMeasurer(goldenMetrics{})
				if c.focusID != "" {
					w.SetFocus(c.focusID)
				}
				settle(w)
				got, err := lr.Draw(w, pixelGoldenW, pixelGoldenH)
				if err != nil {
					t.Fatalf("Draw: %v", err)
				}
				assertSamePixels(t, got, want)
			})
		}
	}
}

// textView is a frame with text at several sizes plus an icon, so a cold
// atlas has to rasterize many glyphs on the first frame.
func textView(_ *gui.Window) gui.View {
	content := make([]gui.View, 0, 12)
	for i, size := range []float32{11, 14, 18, 24, 32} {
		style := gui.CurrentTheme().TextStyleDef
		style.Size = size
		content = append(content, gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("Line %d: the quick brown fox 0123456789", i),
			TextStyle: style,
		}))
	}
	content = append(content, gui.Button(gui.ButtonCfg{
		ID:      "ok",
		Content: []gui.View{gui.Text(gui.TextCfg{Text: "OK " + gui.IconCheck})},
	}))
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingMedium,
		Content: content,
	})
}

// TestRendererColdAtlasSinglePass: the first Draw on a fresh glyph atlas
// already shows every glyph, with no warm pass. That holds only because the
// soft glyph backend is a glyph.RectTextureUpdater, which makes go-glyph
// upload new glyphs before it emits the quads that sample them. Without it
// this frame draws its text blank and differs from RenderToImage, whose warm
// pass hides the problem.
func TestRendererColdAtlasSinglePass(t *testing.T) {
	const ww, wh = 480, 320
	ref := gui.NewWindow(gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) { w.SetView(textView) }})
	want, err := RenderToImage(ref, 1)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	t.Cleanup(func() { Release(ref) })

	w := gui.NewWindow(gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) { w.SetView(textView) }})
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)
	settle(w)
	got, err := lr.Draw(w, ww, wh)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	if bytes.Equal(got.Pix, blankFrame(got, w.FrameBackground()).Pix) {
		t.Fatal("frame is blank: the text view drew nothing")
	}
	assertSamePixels(t, got, want)
}

// TestRendererReusesBuffer: steady-state frames return the same image and
// allocate far less than one frame buffer each. RenderToImage allocates a
// new w*h*4 buffer per call; a live window at 60 fps cannot.
func TestRendererReusesBuffer(t *testing.T) {
	const ww, wh = 640, 480
	w := gui.NewWindow(gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) { w.SetView(textView) }})
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)
	settle(w)

	first, err := lr.Draw(w, ww, wh)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	// Second frame: warm caches, same buffer.
	if again, _ := lr.Draw(w, ww, wh); again != first {
		t.Fatal("Draw returned a new image for an unchanged size")
	}

	const frames = 20
	var m0, m1 runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m0)
	for range frames {
		if _, err = lr.Draw(w, ww, wh); err != nil {
			t.Fatalf("Draw: %v", err)
		}
	}
	runtime.ReadMemStats(&m1)
	perFrame := (m1.TotalAlloc - m0.TotalAlloc) / frames
	// A tenth of a frame buffer: a buffer allocation per frame (1.2 MB
	// here) fails it by an order of magnitude, while the renderer's
	// remaining per-frame garbage (step 5 of #1265 removes it) passes.
	const limit = ww * wh * 4 / 10
	if perFrame > limit {
		t.Errorf("Draw allocates %d B/frame, want <= %d (buffer not reused?)",
			perFrame, limit)
	}
}

// TestRendererResize: a new size gets a new buffer once, and the frame after
// it reuses that buffer. Stale full-size layers from the old size must not
// survive into the new one (bind drops the pool).
func TestRendererResize(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{Width: 200, Height: 100,
		OnInit: func(w *gui.Window) { w.SetView(textView) }})
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)
	settle(w)

	a, err := lr.Draw(w, 200, 100)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	b, err := lr.Draw(w, 260, 140)
	if err != nil {
		t.Fatalf("Draw resized: %v", err)
	}
	if b == a || b.Rect.Dx() != 260 || b.Rect.Dy() != 140 {
		t.Fatalf("resized frame: same=%v size=%v, want a new 260x140 image",
			b == a, b.Rect.Size())
	}
	if c, _ := lr.Draw(w, 260, 140); c != b {
		t.Fatal("frame after a resize did not reuse the new buffer")
	}
}

// TestRendererSetScale: a scale change keeps the window's text measurer and
// moves its text system to the new density (glyph.TextSystem.SetDPIScale, as
// the GL backend does), and a same-scale call is a no-op. Text laid out and
// drawn at 1x before the change must come out at 2x exactly as a fresh 2x
// RenderToImage draws it. Rebuilding and freeing the text system instead
// would leave the window's persisted text blocks holding layouts shaped by
// the freed system.
func TestRendererSetScale(t *testing.T) {
	const ww, wh = 240, 160
	cfg := gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) { w.SetView(textView) }}
	w := gui.NewWindow(cfg)
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)

	// A frame at 1x first, so text blocks and the atlas hold 1x state.
	settle(w)
	if _, err = lr.Draw(w, ww, wh); err != nil {
		t.Fatalf("Draw at 1x: %v", err)
	}

	before := w.TextMeasurer()
	if err = lr.SetScale(1); err != nil {
		t.Fatalf("SetScale same: %v", err)
	}
	if err = lr.SetScale(2); err != nil {
		t.Fatalf("SetScale(2): %v", err)
	}
	tm, ok := w.TextMeasurer().(*textMeasurer)
	if !ok || tm != before || tm.scale != 2 || tm.textSys.DPIScale() != 2 ||
		tm.back.scale != 2 {
		t.Fatalf("after SetScale(2): same measurer=%v, want the same measurer at scale 2",
			w.TextMeasurer() == before)
	}

	settle(w)
	got, err := lr.Draw(w, ww*2, wh*2)
	if err != nil {
		t.Fatalf("Draw at 2x: %v", err)
	}
	ref := gui.NewWindow(cfg)
	want, err := RenderToImage(ref, 2)
	if err != nil {
		t.Fatalf("RenderToImage at 2x: %v", err)
	}
	t.Cleanup(func() { Release(ref) })
	assertSamePixels(t, got, want)
}

// TestRenderToImageRescaleKeepsMeasurer: RenderToImage at a new scale on a
// window it already prepared moves the same text system to the new density,
// as Renderer.SetScale does, rather than freeing it under the window's text
// blocks; the 2x capture matches a fresh window's.
func TestRenderToImageRescaleKeepsMeasurer(t *testing.T) {
	cfg := gui.WindowCfg{Width: 200, Height: 120,
		OnInit: func(w *gui.Window) { w.SetView(textView) }}
	w := gui.NewWindow(cfg)
	t.Cleanup(func() { Release(w) })
	if _, err := RenderToImage(w, 1); err != nil {
		t.Fatalf("RenderToImage 1x: %v", err)
	}
	before := w.TextMeasurer()
	got, err := RenderToImage(w, 2)
	if err != nil {
		t.Fatalf("RenderToImage 2x: %v", err)
	}
	if w.TextMeasurer() != before {
		t.Error("a rescale replaced the window's text measurer")
	}
	ref := gui.NewWindow(cfg)
	t.Cleanup(func() { Release(ref) })
	want, err := RenderToImage(ref, 2)
	if err != nil {
		t.Fatalf("RenderToImage fresh 2x: %v", err)
	}
	assertSamePixels(t, got, want)
}

// TestRendererRejectsBadInput: a nil window and out-of-range sizes fail with
// an error instead of a panic or a giant allocation; a non-finite or
// non-positive scale means 1, as in RenderToImage.
func TestRendererRejectsBadInput(t *testing.T) {
	if _, err := NewRenderer(nil, 1); err == nil {
		t.Error("NewRenderer(nil) did not fail")
	}
	w := gui.NewWindow(gui.WindowCfg{Width: 50, Height: 50})
	nan := float32(math.NaN())
	lr, err := NewRenderer(w, nan)
	if err != nil {
		t.Fatalf("NewRenderer(NaN): %v", err)
	}
	t.Cleanup(lr.Release)
	if tm := w.TextMeasurer().(*textMeasurer); tm.scale != 1 {
		t.Errorf("NaN scale became %v, want 1", tm.scale)
	}
	if err = lr.SetScale(-3); err != nil || w.TextMeasurer().(*textMeasurer).scale != 1 {
		t.Errorf("SetScale(-3): err=%v, want scale kept at 1", err)
	}
	if err = lr.SetScale(11); err == nil {
		t.Error("SetScale(11) past go-glyph's density limit did not fail")
	}
	for _, sz := range [][2]int{{0, 10}, {10, -1}, {maxDimension + 1, 1}, {16384, 16384}} {
		if _, err = lr.Draw(w, sz[0], sz[1]); err == nil {
			t.Errorf("Draw(%d, %d) did not fail", sz[0], sz[1])
		}
	}
	if _, err = lr.Draw(nil, 10, 10); err == nil {
		t.Error("Draw(nil window) did not fail")
	}
}

// TestRendererAfterRelease: Release frees the text system, so a late Draw or
// SetScale (a backend's destroy racing a queued frame) must fail or no-op
// instead of shaping text through a freed system or dereferencing nil.
func TestRendererAfterRelease(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{Width: 20, Height: 20})
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	lr.Release()
	lr.Release() // idempotent
	if _, err = lr.Draw(w, 20, 20); err == nil {
		t.Error("Draw after Release did not fail")
	}
	if err = lr.SetScale(2); err == nil {
		t.Error("SetScale after Release did not fail")
	}
	var nilR *Renderer
	nilR.Release()
}

// blankFrame is a frame of the given size filled with bg only.
func blankFrame(like *image.RGBA, bg gui.Color) *image.RGBA {
	b := newBuffer(like.Rect.Dx(), like.Rect.Dy())
	b.clear(bg)
	return b.img
}

// assertSamePixels fails on any byte difference, naming the first one. Both
// images come from the same renderer on the same machine, so the golden
// suite's cross-GOARCH tolerance does not apply.
func assertSamePixels(t *testing.T, got, want *image.RGBA) {
	t.Helper()
	if got.Rect != want.Rect {
		t.Fatalf("size %v, want %v", got.Rect, want.Rect)
	}
	if bytes.Equal(got.Pix, want.Pix) {
		return
	}
	n := 0
	first := -1
	for i := 0; i < len(got.Pix); i += 4 {
		if !bytes.Equal(got.Pix[i:i+4], want.Pix[i:i+4]) {
			if first < 0 {
				first = i / 4
			}
			n++
		}
	}
	x, y := first%got.Rect.Dx(), first/got.Rect.Dx()
	t.Fatalf("%d pixels differ; first at (%d,%d): got %v want %v",
		n, x, y, got.Pix[first*4:first*4+4], want.Pix[first*4:first*4+4])
}
