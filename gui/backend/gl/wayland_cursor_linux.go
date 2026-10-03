//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"fmt"
	"os"
	"strconv"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Wayland pointer cursors (#919 phase 6).
//
// A Wayland client sets its own cursor every time the pointer enters one
// of its surfaces; until then the cursor is undefined. Two ways exist:
//
//   - cursor-shape-v1 (sway, mutter 46+, KWin 6): the client names a shape
//     and the compositor draws it from the user's theme at the right scale.
//     Nothing to load.
//   - Otherwise the client draws the cursor: the image comes from the
//     Xcursor theme (xcursor.go, shared with X11), is copied into a wl_shm
//     buffer and attached to a cursor surface with wl_pointer.set_cursor.

// wlCursorShapes maps gui cursors to cursor-shape-v1 shapes. CSS names, as
// the protocol uses, so resize-all is "move" and the I-beam is "text".
var wlCursorShapes = [...]uint32{
	gui.CursorDefault:      wl.WpCursorShapeDeviceV1ShapeDefault,
	gui.CursorArrow:        wl.WpCursorShapeDeviceV1ShapeDefault,
	gui.CursorIBeam:        wl.WpCursorShapeDeviceV1ShapeText,
	gui.CursorCrosshair:    wl.WpCursorShapeDeviceV1ShapeCrosshair,
	gui.CursorPointingHand: wl.WpCursorShapeDeviceV1ShapePointer,
	gui.CursorResizeEW:     wl.WpCursorShapeDeviceV1ShapeEwResize,
	gui.CursorResizeNS:     wl.WpCursorShapeDeviceV1ShapeNsResize,
	gui.CursorResizeNWSE:   wl.WpCursorShapeDeviceV1ShapeNwseResize,
	gui.CursorResizeNESW:   wl.WpCursorShapeDeviceV1ShapeNeswResize,
	gui.CursorResizeAll:    wl.WpCursorShapeDeviceV1ShapeMove,
	gui.CursorNotAllowed:   wl.WpCursorShapeDeviceV1ShapeNotAllowed,
	gui.CursorGrab:         wl.WpCursorShapeDeviceV1ShapeGrab,
	gui.CursorGrabbing:     wl.WpCursorShapeDeviceV1ShapeGrabbing,
}

// wlCursorNames maps gui cursors to Xcursor file names, for the fallback.
// The same lists the X11 backend loads.
var wlCursorNames = [...][]string{
	gui.CursorDefault:      leftPtrCursorNames,
	gui.CursorArrow:        leftPtrCursorNames,
	gui.CursorIBeam:        xtermCursorNames,
	gui.CursorCrosshair:    crosshairCursorNames,
	gui.CursorPointingHand: handCursorNames,
	gui.CursorResizeEW:     hResizeCursorNames,
	gui.CursorResizeNS:     vResizeCursorNames,
	gui.CursorResizeNWSE:   nwseCursorNames,
	gui.CursorResizeNESW:   neswCursorNames,
	gui.CursorResizeAll:    moveCursorNames,
	gui.CursorNotAllowed:   notAllowedCursorNames,
	gui.CursorGrab:         grabCursorNames,
	gui.CursorGrabbing:     grabbingCursorNames,
}

// wlCursorShape is the cursor-shape-v1 shape for mc. An unknown value (a
// cursor added to gui later) is the default arrow.
func wlCursorShape(mc gui.MouseCursor) uint32 {
	if int(mc) < len(wlCursorShapes) && wlCursorShapes[mc] != 0 {
		return wlCursorShapes[mc]
	}
	return wl.WpCursorShapeDeviceV1ShapeDefault
}

// wlCursorImage is one fallback cursor, uploaded once per scale.
type wlCursorImage struct {
	buf        wl.Buffer
	hotX, hotY int32 // surface-local, logical pixels
	failed     bool  // the theme has no image; do not try again
}

// wlCursor is the cursor state of the seat's pointer. Main thread only.
type wlCursor struct {
	// device is the cursor-shape-v1 device for the pointer, made on the
	// first set when the compositor has the protocol.
	device wl.WpCursorShapeDeviceV1

	// The fallback: surface carries the image; images caches one buffer
	// per gui cursor at imgScale.
	surface  wl.Surface
	images   [len(wlCursorNames)]wlCursorImage
	imgScale int32
	theme    string
	size     int

	// cur, curScale and valid are what was last set, so an unchanged
	// cursor costs no request. invalidate clears valid.
	cur      gui.MouseCursor
	curScale int32
	valid    bool
}

// invalidate makes the next set send its request even when the cursor did
// not change. A pointer enter needs that: the cursor is undefined over a
// surface until the client sets it.
func (c *wlCursor) invalidate() { c.valid = false }

// dropDevice destroys the cursor-shape device. It belongs to a wl_pointer,
// so it goes when the pointer does.
func (c *wlCursor) dropDevice() {
	if c.device.Valid() {
		c.device.Destroy()
	}
	c.device = wl.WpCursorShapeDeviceV1{}
	c.valid = false
}

// destroy frees every cursor object.
func (c *wlCursor) destroy() {
	c.dropDevice()
	c.dropImages()
	if c.surface.Valid() {
		c.surface.Destroy()
	}
	c.surface = wl.Surface{}
}

func (c *wlCursor) dropImages() {
	for i := range c.images {
		if c.images[i].buf.Valid() {
			c.images[i].buf.Destroy()
		}
		c.images[i] = wlCursorImage{}
	}
}

// setCursor shows mc while the pointer is over the focused window. The run
// loop calls it every pass; it sends nothing when nothing changed.
func (s *wlSeat) setCursor(mc gui.MouseCursor) {
	b := s.ptrFocus
	if b == nil || !s.pointer.Valid() {
		return
	}
	c := &s.cursor
	scale := b.plat.wl.cursorScale()
	if c.valid && c.cur == mc && c.curScale == scale {
		return
	}
	c.cur, c.curScale, c.valid = mc, scale, true
	if mgr := s.d.cursorShape; mgr.Valid() {
		if !c.device.Valid() {
			c.device = mgr.GetPointer(s.pointer)
		}
		c.device.SetShape(s.enterSerial, wlCursorShape(mc))
		return
	}
	s.setCursorImage(mc, scale)
}

// setCursorImage is the fallback: attach the theme image for mc to the
// cursor surface. With no image (no theme installed), the cursor stays
// whatever the compositor shows, which is better than hiding it.
func (s *wlSeat) setCursorImage(mc gui.MouseCursor, scale int32) {
	c := &s.cursor
	if !s.d.shm.Valid() || int(mc) >= len(c.images) {
		return
	}
	if c.theme == "" {
		c.theme, c.size = wlCursorTheme()
	}
	if scale != c.imgScale {
		// Buffers are made for one scale; a window on another output
		// needs new ones.
		c.dropImages()
		c.imgScale = scale
	}
	img := &c.images[mc]
	if !img.buf.Valid() && !img.failed {
		*img = s.d.loadCursorImage(wlCursorNames[mc], c.theme, c.size, scale)
	}
	if !img.buf.Valid() {
		return
	}
	if !c.surface.Valid() {
		c.surface = s.d.compositor.CreateSurface()
	}
	if c.surface.Version() >= 3 {
		c.surface.SetBufferScale(scale)
	}
	c.surface.Attach(img.buf, 0, 0)
	c.surface.Damage(0, 0, 1<<30, 1<<30)
	c.surface.Commit()
	s.pointer.SetCursor(s.enterSerial, c.surface, img.hotX, img.hotY)
}

// cursorScale is the integer scale cursor images are drawn at: the
// buffer scale, rounded up from a fractional one so the cursor is never
// blurred by upscaling.
func (ww *wlWindow) cursorScale() int32 {
	return max((ww.scale120+119)/120, 1)
}

// wlCursorTheme resolves the cursor theme and size for the fallback. A
// Wayland session has no XSETTINGS or X resources, so it is the
// environment (sway and KWin export XCURSOR_THEME and XCURSOR_SIZE), the
// GTK settings files, then the Xcursor defaults.
func wlCursorTheme() (string, int) {
	theme, size := os.Getenv("XCURSOR_THEME"), 0
	if n, err := strconv.Atoi(os.Getenv("XCURSOR_SIZE")); err == nil && n > 0 && n <= xcursorMaxImageDim {
		size = n
	}
	dir, err := os.UserConfigDir()
	if err == nil {
		if theme == "" {
			theme = settingsIniValue(dir, "gtk-cursor-theme-name")
		}
		if size == 0 {
			if n, err := strconv.Atoi(settingsIniValue(dir, "gtk-cursor-theme-size")); err == nil &&
				n > 0 && n <= xcursorMaxImageDim {
				size = n
			}
		}
	}
	if theme == "" {
		theme = "default"
	}
	if size == 0 {
		size = 24
	}
	return theme, size
}

// loadCursorImage finds a theme image and copies it into a wl_shm buffer.
// failed is set when there is none, so the theme is searched once.
func (d *wlDisplay) loadCursorImage(names []string, theme string, size int, scale int32) wlCursorImage {
	img, err := findThemeCursor(names, size*int(scale), xcursorSearchDirs(), theme)
	if err != nil {
		return wlCursorImage{failed: true}
	}
	// A buffer whose size is not a multiple of the buffer scale is a
	// protocol error. Themes ship sizes like 24/48, but a theme can hold
	// only odd sizes.
	if img.Width%int(scale) != 0 || img.Height%int(scale) != 0 {
		return wlCursorImage{failed: true}
	}
	buf, err := d.shmBuffer(img.Width, img.Height, img.Pixels)
	if err != nil {
		return wlCursorImage{failed: true}
	}
	return wlCursorImage{
		buf:  buf,
		hotX: int32(img.Xhot) / scale,
		hotY: int32(img.Yhot) / scale,
	}
}

// shmBuffer makes an ARGB8888 wl_shm buffer holding pixels (premultiplied
// ARGB, the Xcursor and wl_shm format alike). The pool and the memory
// are released at once: the buffer keeps the compositor's mapping alive.
func (d *wlDisplay) shmBuffer(w, h int, pixels []uint32) (wl.Buffer, error) {
	if w <= 0 || h <= 0 || w > xcursorMaxImageDim || h > xcursorMaxImageDim || len(pixels) < w*h {
		return wl.Buffer{}, fmt.Errorf("wayland: bad cursor image %dx%d", w, h)
	}
	n := w * h * 4
	fd, err := unix.MemfdCreate("go-gui-cursor", unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return wl.Buffer{}, err
	}
	defer func() { _ = unix.Close(fd) }()
	if err := unix.Ftruncate(fd, int64(n)); err != nil {
		return wl.Buffer{}, err
	}
	mem, err := unix.Mmap(fd, 0, n, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_SHARED)
	if err != nil {
		return wl.Buffer{}, err
	}
	copy(mem, unsafe.Slice((*byte)(unsafe.Pointer(&pixels[0])), n))
	_ = unix.Munmap(mem)
	pool := d.shm.CreatePool(fd, int32(n))
	buf := pool.CreateBuffer(0, int32(w), int32(h), int32(w*4), wl.ShmFormatArgb8888)
	pool.Destroy()
	return buf, nil
}
