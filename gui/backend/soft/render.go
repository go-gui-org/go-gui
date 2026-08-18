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

// RenderToImage settles one frame of w and software-renders it at the
// given device pixel ratio. scale <= 0 means 1.
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
	if scale <= 0 {
		scale = 1
	}
	tm, err := prepare(w, scale)
	if err != nil {
		return nil, err
	}

	logicalW, logicalH := w.WindowSize()
	pw := int(math.Round(float64(float32(logicalW) * scale)))
	ph := int(math.Round(float64(float32(logicalH) * scale)))
	if pw <= 0 || ph <= 0 {
		return nil, fmt.Errorf("soft: window has no size (%dx%d)",
			logicalW, logicalH)
	}
	if pw > maxDimension || ph > maxDimension {
		return nil, fmt.Errorf(
			"soft: render %dx%d exceeds the %d px limit",
			pw, ph, maxDimension)
	}

	w.BackingScale = scale
	w.SetHeadlessRender(true)
	w.TestRender(nil)
	cmds := w.Renderers()

	root := newBuffer(pw, ph)
	r := &renderer{
		buf:               root,
		rootBuf:           root,
		scale:             scale,
		textSys:           tm.textSys,
		glyphBack:         tm.back,
		allowedImageRoots: w.Config.AllowedImageRoots,
		maxImageBytes: imageLimit(w.Config.MaxImageBytes,
			imgload.DefaultMaxImageBytes),
		maxImagePixels: imageLimit(w.Config.MaxImagePixels,
			imgload.DefaultMaxImagePixels),
	}

	// Pass 1 warms the glyph atlas. go-glyph rasterizes a glyph into a
	// staging buffer when it is first drawn but only hands the page to
	// the backend at Commit, so a single-pass render would sample texels
	// that have not been uploaded yet. On screen the next frame fixes
	// it; a one-shot capture has no next frame. Only the text kinds
	// draw here — their quads land on the nil glyph target and are
	// discarded — because nothing else needs warming: shapes, gradients
	// and images are drawn once, in pass 2.
	r.glyphBack.buf = nil
	r.warm = true
	r.drawAll(cmds)
	r.textSys.Commit()

	// Pass 2 draws for real.
	r.warm = false
	r.setTarget(root)
	root.clear(backgroundColor(w))
	r.drawAll(cmds)
	r.textSys.Commit()
	// A stream that ended inside a begin/end bracket still has its
	// content sitting in a layer; land it rather than drop it.
	r.finishBrackets()

	return root.img, nil
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
		if tm.scale == scale {
			return tm, nil
		}
		// The glyph context is built around one DPI scale; a changed
		// scale needs a new one, and the old atlas is dead weight.
		tm.textSys.Free()
	} else if w.Config.OnInit != nil && len(w.Renderers()) == 0 {
		// First render of this window: do what a backend does before
		// its first frame. A window that has already produced render
		// commands has been initialized, so this cannot run twice.
		w.Config.OnInit(w)
	}

	back := newGlyphBackend(scale)
	textSys, err := glyph.NewTextSystem(back)
	if err != nil {
		return nil, fmt.Errorf("soft: text system: %w", err)
	}
	if data := gui.IconFontData; len(data) > 0 {
		if aerr := textSys.AddFontBytes(data); aerr != nil {
			return nil, fmt.Errorf("soft: icon font: %w", aerr)
		}
	}
	gui.LoadAppFonts(textSys, "soft")

	tm := &textMeasurer{textSys: textSys, back: back, scale: scale}
	w.SetTextMeasurer(tm)
	w.SetSvgParser(svg.New())
	return tm, nil
}

// backgroundColor is the window's clear color, matching what the GPU
// backends paint before replaying the command stream.
func backgroundColor(w *gui.Window) gui.Color {
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		bg = w.Theme().ColorBackground
	}
	return bg
}

// imageLimit resolves a WindowCfg image cap, where zero or negative
// selects the backend default.
func imageLimit(cfg, def int64) int64 {
	if cfg <= 0 {
		return def
	}
	return cfg
}
