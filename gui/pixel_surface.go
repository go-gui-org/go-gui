package gui

import (
	"errors"
	"sync"
)

// PixelFormat is the layout of one plane of a PixelSurface.
type PixelFormat uint8

// Plane formats. Each is 8 bits per channel, rows tightly packed.
const (
	// PixelFormatRGBA8 is straight-alpha RGBA, 4 bytes per pixel
	// (image.NRGBA). It is the zero value and the only format an Image
	// can draw.
	PixelFormatRGBA8 PixelFormat = iota
	// PixelFormatR8 is one channel, 1 byte per pixel. A shader reads it
	// as .r.
	PixelFormatR8
	// PixelFormatRG8 is two channels, 2 bytes per pixel. A shader reads
	// them as .rg.
	PixelFormatRG8
)

// BytesPerPixel reports the bytes one pixel of the format takes, or 0 for
// an unknown format.
func (f PixelFormat) BytesPerPixel() int {
	switch f {
	case PixelFormatRGBA8:
		return 4
	case PixelFormatR8:
		return 1
	case PixelFormatRG8:
		return 2
	}
	return 0
}

// PixelPlaneCfg configures one plane of a multi-plane surface.
type PixelPlaneCfg struct {
	Format        PixelFormat
	Width, Height int
}

// PixelSurfaceCfg configures an owned, initially zeroed surface: a
// transparent image for the default single RGBA8 plane.
type PixelSurfaceCfg struct {
	Width, Height int
	// Planes, when set, replaces Width and Height: up to four planes, each
	// with its own format and size, published together by UpdatePlanes.
	// A shader reads plane i as iChannel i (Shader.Surface). NV12 video,
	// for example, is an R8 plane and an RG8 plane of half the size.
	Planes []PixelPlaneCfg
}

// PixelPlane is the source of one plane for UpdatePlanes. Stride is the
// bytes between source rows: at least width times the format's bytes per
// pixel. Padding after the last row is optional.
type PixelPlane struct {
	Pix    []byte
	Stride int
}

// SurfacePlane is one plane of a surface's current frame, lent to a
// renderer by ReadPlanes. Pix holds tightly packed rows and is read-only.
type SurfacePlane struct {
	Format        PixelFormat
	Width, Height int
	Pix           []byte
}

// PixelSurface is a mutable image for ImageCfg.Surface: a video frame, a
// camera feed, an emulator screen. Bytes are RGBA8 with straight alpha
// (image.NRGBA), not premultiplied image.RGBA. Update copies the caller's
// pixels before returning, replacing the previous frame, and a new frame
// redraws without regenerating layout. A surface keeps at most 64 MiB of
// pixel storage and has no process-global registry.
//
// A surface configured with Planes holds up to four planes for a shader
// to read (Shader.Surface), such as the luma and chroma planes of a video
// frame. UpdatePlanes publishes all of them as one frame, so a renderer
// never draws one plane's new pixels with another's old ones.
//
// All methods are safe for concurrent use. Do not copy a PixelSurface. Retain
// it across view rebuilds and Close it when its producer stops.
type PixelSurface struct {
	mu      sync.Mutex
	planes  [maxSurfacePlanes]SurfacePlane // Pix is owned storage
	n       int                            // planes in use
	version uint64
	closed  bool
	windows map[*Window]struct{}
}

const (
	maxPixelSurfaceBytes = 64 << 20
	maxPixelSurfaceEdge  = 8192
	maxSurfacePlanes     = 4
)

var (
	errPixelSurfaceClosed = errors.New("gui: pixel surface is closed or uninitialized")
	errPixelSurfaceSource = errors.New("gui: pixel surface source is short or has invalid stride")
)

// NewPixelSurface allocates a zeroed surface. Each edge must be positive
// and at most 8192 pixels, and all planes together must fit in 64 MiB.
func NewPixelSurface(cfg PixelSurfaceCfg) (*PixelSurface, error) {
	planes := cfg.Planes
	if len(planes) == 0 {
		planes = []PixelPlaneCfg{{Format: PixelFormatRGBA8, Width: cfg.Width, Height: cfg.Height}}
	} else if cfg.Width != 0 || cfg.Height != 0 {
		return nil, errors.New("gui: set pixel surface Width and Height or Planes, not both")
	}
	if len(planes) > maxSurfacePlanes || !validPixelPlanes(planes) {
		return nil, errors.New("gui: invalid pixel surface size (at most 4 planes, 8192 per edge and 64 MiB)")
	}
	s := &PixelSurface{n: len(planes), version: 1}
	for i, p := range planes {
		s.planes[i] = SurfacePlane{Format: p.Format, Width: p.Width, Height: p.Height,
			Pix: make([]byte, p.Width*p.Height*p.Format.BytesPerPixel())}
	}
	return s, nil
}

// validPixelPlanes checks each plane's format and size and the total.
func validPixelPlanes(planes []PixelPlaneCfg) bool {
	var total int64
	for _, p := range planes {
		bpp := p.Format.BytesPerPixel()
		if bpp == 0 || p.Width <= 0 || p.Height <= 0 ||
			p.Width > maxPixelSurfaceEdge || p.Height > maxPixelSurfaceEdge {
			return false
		}
		total += int64(p.Width) * int64(p.Height) * int64(bpp)
	}
	return total <= maxPixelSurfaceBytes
}

// copyPlane copies src into dst's owned storage. The caller has checked
// src with validSource.
func copyPlane(dst SurfacePlane, src PixelPlane) {
	row := dst.Width * dst.Format.BytesPerPixel()
	for y := range dst.Height {
		copy(dst.Pix[y*row:(y+1)*row], src.Pix[y*src.Stride:y*src.Stride+row])
	}
}

// validSource reports whether src holds every row of dst. Division avoids
// overflowing stride*(height-1), even on 32-bit hosts.
func validSource(dst SurfacePlane, src PixelPlane) bool {
	row := dst.Width * dst.Format.BytesPerPixel()
	return src.Stride >= row && len(src.Pix) >= row &&
		(dst.Height <= 1 || src.Stride <= (len(src.Pix)-row)/(dst.Height-1))
}

// Update copies the surface's current dimensions from pix. Stride is bytes
// between source rows and must be at least width*4 (width times the bytes
// per pixel of a single non-RGBA plane); final-row padding is optional.
// Invalid input leaves the current frame unchanged. At a fixed resolution
// Update reuses storage and allocates nothing, so the caller can reuse pix
// at once. A surface with several planes takes UpdatePlanes instead.
//
// Publishing wakes every window currently drawing this surface, without
// regenerating layout. Renderers upload the newest frame on their own thread.
func (s *PixelSurface) Update(pix []byte, stride int) error {
	if s == nil {
		return errPixelSurfaceClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 {
		return errPixelSurfaceClosed
	}
	if s.n != 1 {
		return errors.New("gui: pixel surface has several planes; use UpdatePlanes")
	}
	src := PixelPlane{Pix: pix, Stride: stride}
	if !validSource(s.planes[0], src) {
		return errPixelSurfaceSource
	}
	copyPlane(s.planes[0], src)
	s.version++
	s.wakeWindows()
	return nil
}

// UpdatePlanes copies one source per plane, in plane order, and publishes
// them together as one frame. Every source is checked before any is
// copied, so invalid input leaves the current frame unchanged. Like
// Update, it allocates nothing at a fixed resolution and the caller can
// reuse the sources at once.
func (s *PixelSurface) UpdatePlanes(planes ...PixelPlane) error {
	if s == nil {
		return errPixelSurfaceClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 {
		return errPixelSurfaceClosed
	}
	if len(planes) != s.n {
		return errors.New("gui: pixel surface UpdatePlanes needs one source per plane")
	}
	for i := range planes {
		if !validSource(s.planes[i], planes[i]) {
			return errPixelSurfaceSource
		}
	}
	for i := range planes {
		copyPlane(s.planes[i], planes[i])
	}
	s.version++
	s.wakeWindows()
	return nil
}

// Resize replaces the image with zeroed pixels and wakes its windows. The
// same size is a no-op. Backend storage is recreated on the next draw. A
// surface with several planes cannot be resized: create a new one.
//
// exportaudit:keep — applications resize when their frame producer changes resolution.
func (s *PixelSurface) Resize(width, height int) error {
	if s == nil {
		return errPixelSurfaceClosed
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 {
		return errPixelSurfaceClosed
	}
	if s.n != 1 {
		return errors.New("gui: a pixel surface with several planes cannot be resized")
	}
	p := &s.planes[0]
	if !validPixelPlanes([]PixelPlaneCfg{{Format: p.Format, Width: width, Height: height}}) {
		return errors.New("gui: invalid pixel surface resize")
	}
	if p.Width == width && p.Height == height {
		return nil
	}
	p.Width, p.Height = width, height
	p.Pix = make([]byte, width*height*p.Format.BytesPerPixel())
	s.version++
	s.wakeWindows()
	return nil
}

// Close releases the pixels, wakes attached windows and stops further
// updates. It is idempotent. Each renderer releases its native resources on
// its next frame or teardown, including when the image view is already gone.
func (s *PixelSurface) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	for i := range s.planes {
		s.planes[i].Pix = nil
	}
	s.wakeWindows()
	s.windows = nil
}

// Closed reports whether the surface is closed or uninitialized. Nil is closed.
func (s *PixelSurface) Closed() bool {
	if s == nil {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed || s.version == 0
}

// ReadPixels is the image renderer's integration point. If the surface is
// open and is a single RGBA8 plane, it returns true and synchronously
// visits tightly packed pixels when version differs from after. Each
// renderer keeps its own version, starting at 0. Any other surface returns
// false: an Image draws only RGBA8.
// The slice is read-only and valid only during visit; copy or upload it before
// returning. visit must not call surface methods or mutate window state.
// The surface lock is held during visit, so a concurrent Update cannot change
// the pixels mid-upload.
func (s *PixelSurface) ReadPixels(after uint64, visit func(width, height int, pix []byte, version uint64)) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 || s.n != 1 || s.planes[0].Format != PixelFormatRGBA8 {
		return false
	}
	if s.version != after && visit != nil {
		p := s.planes[0]
		visit(p.Width, p.Height, p.Pix, s.version)
	}
	return true
}

// ReadPlanes is the shader renderer's integration point: ReadPixels for
// every plane at once. If the surface is open, it returns true and visits
// the planes, in order, when version differs from after. The slice and
// the pixels are read-only and valid only during visit, under the same
// rules as ReadPixels.
func (s *PixelSurface) ReadPlanes(after uint64, visit func(planes []SurfacePlane, version uint64)) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 {
		return false
	}
	if s.version != after && visit != nil {
		visit(s.planes[:s.n], s.version)
	}
	return true
}

// wakeWindows runs with s.mu held. A backend wake only posts a wake; it must
// not enter the frame pipeline synchronously (the same rule as animation
// wake).
func (s *PixelSurface) wakeWindows() {
	for w := range s.windows {
		if !w.destroyed.Load() {
			w.InvalidateRender()
		}
	}
}

// surfaceSubscriptions is the window side of PixelSurface wake-ups: the
// surfaces this window's image and shader commands draw, each subscribed
// to wake the window on a new frame, and whether the render pass in
// progress drew one.
type surfaceSubscriptions struct {
	drawn map[*PixelSurface]bool
	drew  bool
}

// syncPixelSurfaces subscribes the window to the surfaces this frame's
// commands draw and drops the ones it no longer draws. Frame thread only;
// the surface lock serializes subscriptions with producers.
func (w *Window) syncPixelSurfaces() {
	if len(w.pixelSurfaces.drawn) == 0 && !w.pixelSurfaces.drew {
		return
	}
	w.pixelSurfaces.drew = false
	for surface := range w.pixelSurfaces.drawn {
		w.pixelSurfaces.drawn[surface] = false
	}
	for i := range w.renderers {
		surface := w.renderers[i].DrawnSurface()
		if surface == nil {
			continue
		}
		if w.pixelSurfaces.drawn == nil {
			w.pixelSurfaces.drawn = make(map[*PixelSurface]bool)
		}
		if _, exists := w.pixelSurfaces.drawn[surface]; !exists {
			surface.mu.Lock()
			if !surface.closed && surface.version != 0 && !w.destroyed.Load() {
				if surface.windows == nil {
					surface.windows = make(map[*Window]struct{})
				}
				surface.windows[w] = struct{}{}
			}
			surface.mu.Unlock()
		}
		w.pixelSurfaces.drawn[surface] = true
	}
	for surface, used := range w.pixelSurfaces.drawn {
		if !used {
			surface.mu.Lock()
			delete(surface.windows, w)
			surface.mu.Unlock()
			delete(w.pixelSurfaces.drawn, surface)
		}
	}
}

// detachPixelSurfaces drops every subscription when the window is destroyed.
func (w *Window) detachPixelSurfaces() {
	for surface := range w.pixelSurfaces.drawn {
		surface.mu.Lock()
		delete(surface.windows, w)
		surface.mu.Unlock()
	}
	w.pixelSurfaces.drawn = nil
}

// DrawnSurface reports the PixelSurface a command draws, or nil: Surface
// for a RenderImage, Shader.Surface for a RenderCustomShader. Renderers and
// window subscriptions read it so the two kinds have one source each.
func (r *RenderCmd) DrawnSurface() *PixelSurface {
	switch r.Kind {
	case RenderImage:
		return r.Surface
	case RenderCustomShader:
		if r.Shader != nil {
			return r.Shader.Surface
		}
	}
	return nil
}
