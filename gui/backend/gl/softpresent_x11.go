//go:build linux && !js && !android

package gl

// GOGUI_RENDERER=soft present path for X11 (#1265, step 2).
//
// A window on this path has no EGL display, surface or context and no GL
// function table. Each frame is drawn on the CPU by soft.Renderer into one
// reused premultiplied-RGBA buffer, swizzled to BGRA band by band, and sent
// with core PutImage. Everything else — events, IME, clipboard, DPI, cursors,
// window state — is the ordinary X11 platform code, shared with GL.
//
// Full frame on every redraw, 1x first: step 1 measured 7–15 ms per frame at
// 1x against llvmpipe's 2–3x CPU. Zero allocations per frame (MIT-SHM, or a
// PutImage writer of our own) is step 5; xgb's PutImage copies each band into
// a fresh request buffer.

import (
	"fmt"
	"log"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// putImageHeader is the fixed part of a core PutImage request, in bytes.
const putImageHeader = 24

// defaultFrameInterval paces soft frames when the monitor's refresh rate is
// unknown: no RandR, no CRTC, or a mode without timing.
const defaultFrameInterval = time.Second / 60

// softPresent is one window's CPU render-and-upload state. Non-nil
// Backend.soft selects this path.
type softPresent struct {
	conn  *xgb.Conn
	win   xproto.Window
	gc    xproto.Gcontext
	depth byte
	r     *soft.Renderer

	// band is the reused BGRA staging buffer for one PutImage request;
	// last is the frame most recently drawn, kept to repaint on Expose. It
	// is the renderer's own buffer, so it is valid until the next Draw.
	band         []byte
	last         []byte
	lastW, lastH int
	// maxReq is the server's maximum request size in bytes.
	maxReq int

	// Frame pacing stands in for eglSwapInterval(1): without it, a view that
	// invalidates every frame would redraw as fast as the CPU allows.
	vsyncOff bool
	interval time.Duration
	pacedFor randr.Crtc
	// paced is set once setMonitor has read an interval. pacedFor alone
	// cannot say so (CRTC 0 is a valid "unknown" answer), and next is zero
	// under VSyncOff and before the first frame.
	paced bool
	next  time.Time

	errLogged bool
}

// newSoftPresent creates the GC and the renderer for win. The window's
// visual must already have passed pickSoftVisual and softImageFormatOK.
func newSoftPresent(conn *xgb.Conn, win xproto.Window, depth byte,
	w *gui.Window, scale float32) (*softPresent, error) {

	gcid, err := conn.NewId()
	if err != nil {
		return nil, fmt.Errorf("gl: soft: new gc id: %w", err)
	}
	gc := xproto.Gcontext(gcid)
	// GraphicsExposures off: PutImage never generates NoExpose events then.
	xproto.CreateGC(conn, gc, xproto.Drawable(win), xproto.GcGraphicsExposures,
		[]uint32{0})
	r, err := soft.NewRenderer(w, scale)
	if err != nil {
		xproto.FreeGC(conn, gc)
		return nil, fmt.Errorf("gl: soft: %w", err)
	}
	return &softPresent{
		conn:     conn,
		win:      win,
		gc:       gc,
		depth:    depth,
		r:        r,
		maxReq:   int(xproto.Setup(conn).MaximumRequestLength) * 4,
		vsyncOff: w.Config.VSyncOff,
		interval: defaultFrameInterval,
	}, nil
}

// present draws w at physW x physH, uploads it, and paces to the monitor.
// It reports whether a frame reached the window: false when Draw refused
// the size, in which case nothing was put. A refused frame still paces: an
// animating view asks for a frame every loop, and a size that fails every
// time would otherwise spin a core at 100% doing nothing.
// Draw reads the window's render commands, so it runs under the window lock,
// as the GL drawFrame does; the upload runs after the lock is released so
// a slow server does not hold up the animation goroutine.
func (sp *softPresent) present(w *gui.Window, physW, physH int32) bool {
	w.Lock()
	img, err := sp.r.Draw(w, int(physW), int(physH))
	w.Unlock()
	if err != nil {
		// A size past the soft limits fails every frame until the window
		// shrinks: say it once, not 60 times a second.
		if !sp.errLogged {
			log.Printf("gl: soft: %v", err)
			sp.errLogged = true
		}
		sp.pace()
		return false
	}
	sp.errLogged = false
	sp.last, sp.lastW, sp.lastH = img.Pix, img.Rect.Dx(), img.Rect.Dy()
	sp.put()
	sp.pace()
	return true
}

// repaint puts the last frame back without drawing, for an Expose on a
// server that keeps no backing store (Xvfb, an uncomposited session). It is
// nil before the first frame, and the first frame covers the window anyway.
func (sp *softPresent) repaint() {
	if sp.last != nil {
		sp.put()
	}
}

// put uploads sp.last in row bands that each fit one request.
func (sp *softPresent) put() {
	rows := bandRows(sp.lastW, sp.maxReq)
	rowBytes := sp.lastW * 4
	if need := rows * rowBytes; cap(sp.band) < need {
		sp.band = make([]byte, need)
	}
	for y := 0; y < sp.lastH; y += rows {
		n := min(rows, sp.lastH-y)
		band := sp.band[:n*rowBytes]
		swizzleBGRA(band, sp.last[y*rowBytes:(y+n)*rowBytes])
		xproto.PutImage(sp.conn, xproto.ImageFormatZPixmap,
			xproto.Drawable(sp.win), sp.gc, uint16(sp.lastW), uint16(n),
			0, int16(y), 0, sp.depth, band)
	}
}

// pace waits for the server to take the frame, then sleeps out the rest of
// the monitor's frame interval. The round trip is the backpressure GL gets
// from SwapBuffers: without it a fast loop queues frames in the socket
// faster than the server can draw them. The interval is the monitor's, set
// by setMonitor.
func (sp *softPresent) pace() {
	_, _ = xproto.GetInputFocus(sp.conn).Reply()
	if sp.vsyncOff {
		return
	}
	now := time.Now()
	if sp.next.After(now) {
		time.Sleep(sp.next.Sub(now))
		now = sp.next
	}
	sp.next = now.Add(sp.interval)
}

// setMonitor re-reads the frame interval when the window has moved to
// another CRTC. Called from the DPI tracking path, which already knows when
// that happens.
func (sp *softPresent) setMonitor(root xproto.Window, haveRandr bool, crtc randr.Crtc) {
	if sp.paced && crtc == sp.pacedFor {
		return
	}
	sp.paced = true
	sp.pacedFor = crtc
	sp.interval = crtcFrameInterval(sp.conn, root, haveRandr, crtc)
}

// destroy frees the renderer and the GC. The window itself is the
// platform's to destroy.
func (sp *softPresent) destroy() {
	if sp == nil {
		return
	}
	sp.r.Release()
	if sp.conn != nil {
		xproto.FreeGC(sp.conn, sp.gc)
	}
	sp.last = nil
}

// --- pure helpers ---

// pickSoftVisual chooses the X visual for a soft window. soft writes 8-bit
// B, G, R (and alpha) bytes, so only a TrueColor visual with masks 0xff0000,
// 0xff00, 0xff fits. An opaque window keeps the root visual when it fits, as
// the GL path keeps the driver's first choice; a transparent one takes the
// first fitting depth-32 visual and, with none, degrades to opaque with
// argb=false (the caller warns, as for GL). ok=false means the screen offers
// nothing soft can draw into.
func pickSoftVisual(depths []xproto.DepthInfo, root xproto.Visualid, transparent bool) (
	vis xproto.Visualid, depth byte, argb, ok bool,
) {
	fits := func(v xproto.VisualInfo) bool {
		return v.Class == xproto.VisualClassTrueColor &&
			v.RedMask == 0xff0000 && v.GreenMask == 0xff00 && v.BlueMask == 0xff
	}
	if transparent {
		for _, d := range depths {
			if d.Depth != argbDepth {
				continue
			}
			for _, v := range d.Visuals {
				if fits(v) {
					return v.VisualId, argbDepth, true, true
				}
			}
		}
	}
	// Opaque, or a transparent request with no ARGB visual: the root
	// visual first, then any fitting depth-24 visual.
	var firstVis xproto.Visualid
	for _, d := range depths {
		if d.Depth != 24 {
			continue
		}
		for _, v := range d.Visuals {
			if !fits(v) {
				continue
			}
			if v.VisualId == root {
				return root, 24, false, true
			}
			if firstVis == 0 {
				firstVis = v.VisualId
			}
		}
	}
	if firstVis != 0 {
		return firstVis, 24, false, true
	}
	return 0, 0, false, false
}

// softImageFormatOK reports whether a ZPixmap of depth on this server is 4
// bytes per pixel in little-endian order, the layout swizzleBGRA writes.
func softImageFormatOK(formats []xproto.Format, imageByteOrder, depth byte) bool {
	if imageByteOrder != xproto.ImageOrderLSBFirst {
		return false
	}
	for _, f := range formats {
		if f.Depth == depth {
			return f.BitsPerPixel == 32
		}
	}
	return false
}

// bandRows is how many rows of a width-pixel frame fit one PutImage request
// of at most maxReq bytes. At least 1: a row over the limit still goes, and
// the server's error says why, which beats a window that silently stays
// blank.
func bandRows(width, maxReq int) int {
	if width <= 0 {
		return 1
	}
	return max(1, (maxReq-putImageHeader)/(width*4))
}

// swizzleBGRA converts premultiplied RGBA to the B, G, R, A byte order of a
// little-endian 32 bpp TrueColor ZPixmap. Alpha is copied: a depth-24 visual
// ignores the byte, and a depth-32 one wants it premultiplied, which it is.
// Only whole pixels that fit both slices are written.
func swizzleBGRA(dst, src []byte) {
	n := min(len(dst), len(src)) &^ 3
	for i := 0; i < n; i += 4 {
		// One bounds check per pixel instead of four.
		d, s := dst[i:i+4:i+4], src[i:i+4:i+4]
		d[0], d[1], d[2], d[3] = s[2], s[1], s[0], s[3]
	}
}

// modeFrameInterval is one frame of a RandR mode: htotal * vtotal / dot
// clock. A mode without timing, or with a rate outside 20–1000 Hz (a
// misreported clock), falls back to 60 Hz.
func modeFrameInterval(m randr.ModeInfo) time.Duration {
	if m.DotClock == 0 || m.Htotal == 0 || m.Vtotal == 0 {
		return defaultFrameInterval
	}
	hz := float64(m.DotClock) / (float64(m.Htotal) * float64(m.Vtotal))
	if hz < 20 || hz > 1000 {
		return defaultFrameInterval
	}
	return time.Duration(float64(time.Second) / hz)
}

// crtcFrameInterval reads the frame interval of the mode crtc shows. With
// crtc 0 (not tracked, e.g. under XWayland) it takes the first active CRTC.
// Any failure gives the 60 Hz default.
func crtcFrameInterval(conn *xgb.Conn, root xproto.Window, haveRandr bool,
	crtc randr.Crtc) time.Duration {

	if !haveRandr {
		return defaultFrameInterval
	}
	res, err := randr.GetScreenResourcesCurrent(conn, root).Reply()
	if err != nil || res == nil {
		return defaultFrameInterval
	}
	crtcs := res.Crtcs
	if crtc != 0 {
		crtcs = []randr.Crtc{crtc}
	}
	for _, c := range crtcs {
		info, ierr := randr.GetCrtcInfo(conn, c, res.ConfigTimestamp).Reply()
		if ierr != nil || info == nil || info.Mode == 0 {
			continue
		}
		for _, m := range res.Modes {
			if randr.Mode(m.Id) == info.Mode {
				return modeFrameInterval(m)
			}
		}
	}
	return defaultFrameInterval
}

// --- Backend glue: the sites where the X11 path branches on soft ---

// initSoftX11 stands in for initGLResources on a soft window: the renderer
// installs the text measurer and SVG parser, and the window is told it cannot
// run custom shaders (soft has no GLSL; a shader container draws its children
// and no fill, as in soft screenshots).
func (b *Backend) initSoftX11(w *gui.Window, depth byte, crtc randr.Crtc) error {
	sp, err := newSoftPresent(b.plat.conn, b.plat.window, depth, w, b.dpiScale)
	if err != nil {
		return err
	}
	sp.setMonitor(b.plat.root, b.plat.haveRandr, crtc)
	b.plat.soft = sp
	w.SetNativePlatform(&nativePlatform{b: b})
	gui.SetWindowRendersShaders(w, false)
	return nil
}

// presentX11 is the X11 loops' frame call: GL draws and swaps, soft draws on
// the CPU and puts the image.
func (b *Backend) presentX11(w *gui.Window) {
	if sp := b.plat.soft; sp != nil {
		// presented means a buffer reached the window, as after a
		// successful swap; a frame Draw refused put nothing.
		if sp.present(w, b.physW, b.physH) {
			b.plat.presented = true
		}
		return
	}
	b.renderFrame(w)
}

// resizeX11 is handleResize for the X11 ConfigureNotify path. handleResize
// sets the GL viewport and projection, which a soft window has no context
// for; soft only needs the new size and scale.
func (b *Backend) resizeX11() {
	if b.plat.softMode {
		b.physW, b.physH = b.plat.drawableSize()
		b.applyScaleX11(b.plat.dpiScale())
		return
	}
	b.handleResize()
}

// applyScaleX11 adopts a new device scale. applyDPIScale validates it and is
// nil-safe for a soft window, which has no GL glyph backend; the soft
// renderer then moves its text system to the scale applyDPIScale kept.
func (b *Backend) applyScaleX11(s float32) {
	b.applyDPIScale(s)
	if sp := b.plat.soft; sp != nil {
		if err := sp.r.SetScale(b.dpiScale); err != nil {
			log.Printf("gl: soft: rescale: %v", err)
		}
	}
}

// exposeX11 repaints a soft window's exposed area from its last frame. GL
// needs nothing: its next swap repaints, and Expose comes with a frame.
// Expose arrives once per damaged rect with count the number still to come;
// only the last of a series (count 0) repaints, and it repaints the whole
// window, which is cheaper than tracking the rects for a full-frame upload.
func (b *Backend) exposeX11(count uint16) {
	if sp := b.plat.soft; sp != nil && count == 0 {
		sp.repaint()
	}
}

// monitorChangedX11 tells the soft pacer the window may be on another CRTC.
func (b *Backend) monitorChangedX11(crtc randr.Crtc) {
	if sp := b.plat.soft; sp != nil {
		sp.setMonitor(b.plat.root, b.plat.haveRandr, crtc)
	}
}
