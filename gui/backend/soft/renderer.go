package soft

import (
	"errors"
	"fmt"
	"image"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Renderer draws a live window's frames on the CPU (#1265). A backend that
// presents pixels itself — the Linux backend's GOGUI_RENDERER=soft path,
// which uploads with X11 PutImage or commits a wl_shm buffer on Wayland —
// keeps one per window.
//
// It differs from RenderToImage in what it leaves to the caller:
//
//   - It draws the render commands the window already holds. The backend's
//     own FrameFn produced them; Draw runs no layout and no TestRender, and
//     never sets the headless flag, so animations stay live.
//   - It draws one pass. The glyph backend is a glyph.RectTextureUpdater, so
//     go-glyph uploads new glyphs before it emits the quads that sample them
//     and a cold atlas needs no warm pass.
//   - It reuses one frame buffer, the decoded-image cache and the layer pool
//     across frames. The image Draw returns is that buffer: it is valid
//     until the next Draw, SetScale or Release. The image cache is bounded
//     like the GL texture cache, by WindowCfg.MaxTextureBytes (#1239), and
//     drops a mem: image once its registration leaves gui's registry.
//
// Draw reads w.Renderers(), so call it under w.Lock(), as a GPU backend
// holds the lock around its frame. SetScale purges, and Release frees, the
// window's text system; call them from the thread that runs FrameFn,
// between frames.
//
// exportaudit:keep — the Linux backend's soft present path (gui/backend/gl)
type Renderer struct {
	// w is the window NewRenderer installed tm on; Release detaches tm from
	// it so the window never keeps a freed text system as its measurer.
	w     *gui.Window
	tm    *textMeasurer
	r     *renderer
	root  *buffer
	scale float32
}

// NewRenderer prepares w for live software rendering at the given device
// pixel ratio and returns the renderer that draws its frames. A
// non-positive or non-finite scale means 1.
//
// It installs a software text system as w's gui.TextMeasurer and an SVG
// parser, as a backend does at window creation. It does not run
// WindowCfg.OnInit: the backend's event loop does that.
func NewRenderer(w *gui.Window, scale float32) (*Renderer, error) {
	if w == nil {
		return nil, errors.New("soft: nil window")
	}
	scale = sanitizeScale(scale)
	tm, err := newTextMeasurer(scale)
	if err != nil {
		return nil, err
	}
	w.SetTextMeasurer(tm)
	w.SetSvgParser(svg.New())
	return &Renderer{
		w:     w,
		tm:    tm,
		r:     newFrameRenderer(w, tm, scale),
		scale: scale,
	}, nil
}

// Draw renders w's current render commands into a width x height device
// pixel buffer and returns it. The buffer is reused while the size stays
// the same; a new size allocates a new one. The image is premultiplied
// RGBA and stays valid until the next Draw, SetScale or Release.
func (lr *Renderer) Draw(w *gui.Window, width, height int) (*image.RGBA, error) {
	if w == nil {
		return nil, errors.New("soft: nil window")
	}
	// After Release the renderer still points at the freed text system;
	// shaping through it would be a use-after-free in go-glyph.
	if lr.tm == nil {
		return nil, errRendererReleased
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("soft: frame has no size (%dx%d)", width, height)
	}
	if err := checkRenderSize(width, height); err != nil {
		return nil, err
	}
	if lr.root == nil || lr.root.img.Rect.Dx() != width ||
		lr.root.img.Rect.Dy() != height {
		lr.root = newBuffer(width, height)
	}
	// The GPU backends set this under the same lock before replaying the
	// stream; render kinds that size by device pixels read it.
	w.BackingScale = lr.scale
	lr.r.bind(lr.root)
	lr.r.drawPass(w.Renderers(), w.FrameBackground())
	return lr.root.img, nil
}

// SetScale moves the renderer to a new device pixel ratio, as when a window
// crosses to a monitor with another scale. The window keeps its text
// measurer: the text system moves to the new density in place
// (glyph.TextSystem.SetDPIScale, as the GL backend does), which purges its
// layout and glyph caches. The same scale, or a non-positive or non-finite
// one, is a no-op: an implausible value means a failed DPI query, and keeping
// the current scale beats adopting it. A scale go-glyph refuses (above 10)
// is an error and changes nothing.
func (lr *Renderer) SetScale(scale float32) error {
	if lr.tm == nil {
		return errRendererReleased
	}
	if sanitizeScale(scale) != scale || scale == lr.scale {
		return nil
	}
	if !lr.tm.setScale(scale) {
		return fmt.Errorf("soft: scale %v is past the text system's limit", scale)
	}
	lr.scale = scale
	lr.r.scale = scale
	return nil
}

// errRendererReleased is what Draw and SetScale return after Release.
var errRendererReleased = errors.New("soft: renderer released")

// Release frees the text system and drops the frame buffer, the image cache
// and the layer pool. Draw and SetScale fail afterwards; a backend calls it
// when the window is destroyed. A second Release, or one on a nil Renderer,
// does nothing.
//
// The window's measurer is cleared when it is still this renderer's text
// system, as the package-level Release does. Left in place, a later
// RenderToImage of the window at the same scale would take the window as
// already prepared and shape through the freed system. A measurer the app
// installed since NewRenderer is not touched.
func (lr *Renderer) Release() {
	if lr == nil || lr.tm == nil {
		return
	}
	if lr.w.TextMeasurer() == lr.tm {
		lr.w.SetTextMeasurer(nil)
	}
	lr.tm.textSys.Free()
	lr.tm = nil
	// Drop every per-frame allocation, so a caller that keeps the released
	// Renderer does not keep the buffers alive. Draw and SetScale check tm
	// first and never reach r.
	lr.r = nil
	lr.root = nil
}
