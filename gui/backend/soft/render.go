package soft

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// maxDimension caps the device-pixel size of a render, so a bad scale
// cannot ask for a multi-gigabyte allocation.
const maxDimension = 16384

// maxRenderPixels caps the total device pixels of a render, so the
// 16384x16384 corner the per-side cap still allows (1 GiB of RGBA)
// cannot be asked for. Both inputs are the app's own window size
// and scale, not untrusted data, so the cap sits well above any
// real display render rather than at imgload's decoded-file
// budget: a 5K window at scale 2 is 59M pixels and must render.
const maxRenderPixels = int64(1) << 27

// RenderToImage settles one frame of w and software-renders it at the
// given device pixel ratio. A non-positive or non-finite scale means 1.
//
// The returned image is premultiplied RGBA at
// (window width x scale, window height x scale) pixels.
//
// A window that has not rendered yet has its WindowCfg.OnInit run first,
// as a backend would, and is prepared for headless rendering: a software
// text system becomes its gui.TextMeasurer and an SVG parser is
// installed. Preparation is idempotent and keeps the warmed glyph atlas,
// so driving state between captures — TestClick, SetFocus, then render
// again — costs only the frame.
//
// exportaudit:keep — primary API; RenderToPNG is the convenience wrapper
func RenderToImage(w *gui.Window, scale float32) (*image.RGBA, error) {
	if w == nil {
		return nil, errors.New("soft: nil window")
	}
	scale = sanitizeScale(scale)
	tm, err := prepare(w, scale)
	if err != nil {
		return nil, err
	}
	return renderFrame(w, tm, scale)
}

// renderFrame generates one frame with the window's current text
// measurer and draws it at scale. tm is the text system the renderer
// uses for text kinds; layout-time metrics come from the window's
// measurer, which a caller may have swapped for a deterministic stub
// (the pixel-golden harness does, so a recorded frame does not depend
// on the host font catalog).
func renderFrame(
	w *gui.Window, tm *textMeasurer, scale float32,
) (*image.RGBA, error) {
	logicalW, logicalH := w.WindowSize()
	pw := int(math.Round(float64(float32(logicalW) * scale)))
	ph := int(math.Round(float64(float32(logicalH) * scale)))
	if pw <= 0 || ph <= 0 {
		return nil, fmt.Errorf("soft: window has no size (%dx%d)",
			logicalW, logicalH)
	}
	if err := checkRenderSize(pw, ph); err != nil {
		return nil, err
	}

	w.BackingScale = scale
	w.SetHeadlessRender(true)
	w.TestRender(nil)
	cmds := w.Renderers()

	root := newBuffer(pw, ph)
	r := newFrameRenderer(w, tm, scale)
	r.bind(root)

	// Pass 1 warms the glyph atlas. go-glyph rasterizes a glyph into a
	// staging buffer when it is first drawn but only hands the page to
	// the backend at Commit, so a single-pass render would sample texels
	// that have not been uploaded yet. On screen the next frame fixes
	// it; a one-shot capture has no next frame. Only the text kinds
	// draw here — their quads land on the nil glyph target and are
	// discarded — because nothing else needs warming: shapes, gradients
	// and images are drawn once, in pass 2.
	//
	// Since #1265 the glyph backend is a glyph.RectTextureUpdater, so
	// go-glyph uploads mid-frame and pass 2 alone would already be
	// correct (Renderer.Draw relies on that). The warm pass is kept so
	// RenderToImage's output and cost stay exactly what they were.
	r.glyphBack.buf = nil
	r.warm = true
	r.drawAll(cmds)
	r.textSys.Commit()

	// Pass 2 draws for real.
	r.drawPass(cmds, backgroundColor(w))
	return root.img, nil
}

// newFrameRenderer builds a renderer for w's image limits over tm's text
// system. It has no target yet: bind gives it one per frame. Shared by
// RenderToImage, which builds one per capture, and Renderer, which keeps
// one for the life of a live window so its decoded-image and surface
// caches carry across frames.
func newFrameRenderer(w *gui.Window, tm *textMeasurer, scale float32) *renderer {
	return &renderer{
		scale:             scale,
		textSys:           tm.textSys,
		glyphBack:         tm.back,
		allowedImageRoots: w.Config.AllowedImageRoots,
		maxImageBytes: imageLimit(w.Config.MaxImageBytes,
			imgload.DefaultMaxImageBytes),
		maxImagePixels: imageLimit(w.Config.MaxImagePixels,
			imgload.DefaultMaxImagePixels),
		imageCache: newImageCache(w.Config.MaxTextureBytes),
	}
}

// bind makes root the window buffer for the next frame and resets the
// state a previous frame may have left: an open bracket (finishBrackets
// closes them, but a panic mid-frame would not), the window clip, and the
// root's own clip rect. Pooled layers are full-size copies of the root, so
// a root of another size drops them.
func (r *renderer) bind(root *buffer) {
	if r.rootBuf != nil && r.rootBuf.img.Rect != root.img.Rect {
		r.layerPool = nil
	}
	r.rootBuf = root
	r.buf = root
	r.brackets = r.brackets[:0]
	r.winClipSet = false
	root.clip = root.img.Bounds()
}

// drawPass clears the bound root to bg and replays cmds once, for real:
// shapes, images and text all land. Commit runs before the brackets close
// so any glyph left pending is on its page before a layer composites.
func (r *renderer) drawPass(cmds []gui.RenderCmd, bg gui.Color) {
	r.warm = false
	r.setTarget(r.rootBuf)
	r.rootBuf.clear(bg)
	r.drawAll(cmds)
	r.textSys.Commit()
	// A stream that ended inside a begin/end bracket still has its
	// content sitting in a layer; land it rather than drop it.
	r.finishBrackets()
}

// RenderToPNG renders w and writes the result to path as a PNG,
// creating the parent directory if needed.
func RenderToPNG(w *gui.Window, scale float32, path string) error {
	img, err := RenderToImage(w, scale)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err = os.MkdirAll(dir, 0o750); err != nil {
			return fmt.Errorf("soft: create %s: %w", dir, err)
		}
	}
	f, err := os.Create(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("soft: create %s: %w", path, err)
	}
	if err = png.Encode(f, img); err != nil {
		_ = f.Close()
		return fmt.Errorf("soft: encode %s: %w", path, err)
	}
	if err = f.Close(); err != nil {
		return fmt.Errorf("soft: close %s: %w", path, err)
	}
	return nil
}

// Release frees the text system this package attached to w. Call it when
// a window that was rendered headlessly is discarded but the process
// keeps running — a test that renders many windows, say. Rendering w
// again after Release simply builds a fresh text system.
// exportaudit:keep — lifecycle counterpart to RenderToImage
func Release(w *gui.Window) {
	if w == nil {
		return
	}
	if tm, ok := w.TextMeasurer().(*textMeasurer); ok {
		tm.textSys.Free()
		w.SetTextMeasurer(nil)
	}
}

// prepare wires the headless seams a backend's init would wire, and is a
// no-op on a window already prepared at this scale.
func prepare(w *gui.Window, scale float32) (*textMeasurer, error) {
	if tm, ok := w.TextMeasurer().(*textMeasurer); ok {
		// A changed scale moves the same text system to the new density
		// (#1265): freeing it would leave the window's persisted text
		// blocks holding layouts from a freed system.
		if tm.setScale(scale) {
			return tm, nil
		}
		// go-glyph refused the scale (past its density limit): only a
		// new system built at that scale can render it.
		tm.textSys.Free()
	} else if w.Config.OnInit != nil && len(w.Renderers()) == 0 {
		// First render of this window: do what a backend does before
		// its first frame. A window that has already produced render
		// commands has been initialized, so this cannot run twice.
		w.Config.OnInit(w)
	}

	tm, err := newTextMeasurer(scale)
	if err != nil {
		return nil, err
	}
	w.SetTextMeasurer(tm)
	w.SetSvgParser(svg.New())
	return tm, nil
}

// newTextMeasurer builds a go-glyph text system over a software glyph
// backend at scale, with the icon font and the app's fonts loaded. It is
// what both prepare and NewRenderer install as the window's measurer.
func newTextMeasurer(scale float32) (*textMeasurer, error) {
	back := newGlyphBackend(scale)
	textSys, err := glyph.NewTextSystem(back)
	if err != nil {
		return nil, fmt.Errorf("soft: text system: %w", err)
	}
	if data := gui.IconFontData; len(data) > 0 {
		if aerr := textSys.AddFontBytes(data); aerr != nil {
			textSys.Free()
			return nil, fmt.Errorf("soft: icon font: %w", aerr)
		}
	}
	gui.LoadAppFonts(textSys, "soft")
	return &textMeasurer{textSys: textSys, back: back, scale: scale}, nil
}

// checkRenderSize rejects a device-pixel size past the per-side or total
// pixel cap, so a bad scale or window size cannot ask for a
// multi-gigabyte buffer.
func checkRenderSize(pw, ph int) error {
	if pw > maxDimension || ph > maxDimension {
		return fmt.Errorf(
			"soft: render %dx%d exceeds the %d px limit",
			pw, ph, maxDimension)
	}
	if int64(pw)*int64(ph) > maxRenderPixels {
		return fmt.Errorf(
			"soft: render %dx%d exceeds the %d pixel limit",
			pw, ph, maxRenderPixels)
	}
	return nil
}

// sanitizeScale maps a NaN, infinite or non-positive scale to 1. NaN and
// +Inf both pass a <= 0 check (NaN compares false, +Inf is positive) and
// would poison the pixel math into an implementation-defined int
// conversion.
func sanitizeScale(scale float32) float32 {
	if math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) ||
		scale <= 0 {
		return 1
	}
	return scale
}

// backgroundColor is the window's clear color, matching what the GPU
// backends paint before replaying the command stream.
func backgroundColor(w *gui.Window) gui.Color {
	return w.FrameBackground()
}

// imageLimit resolves a WindowCfg image cap, where zero or negative
// selects the backend default.
func imageLimit(cfg, def int64) int64 {
	if cfg <= 0 {
		return def
	}
	return cfg
}
