package gui

import (
	"errors"
	"sync"
)

// PixelSurfaceCfg configures an owned, initially transparent RGBA8 image.
type PixelSurfaceCfg struct {
	Width, Height int
}

// PixelSurface is a mutable image for ImageCfg.Surface: a video frame, a
// camera feed, an emulator screen. Bytes are RGBA8 with straight alpha
// (image.NRGBA), not premultiplied image.RGBA. Update copies the caller's
// pixels before returning, replacing the previous frame, and a new frame
// redraws without regenerating layout. A surface keeps at most 64 MiB of
// pixel storage and has no process-global registry.
//
// All methods are safe for concurrent use. Do not copy a PixelSurface. Retain
// it across view rebuilds and Close it when its producer stops.
type PixelSurface struct {
	mu      sync.Mutex
	pix     []byte
	w, h    int
	version uint64
	closed  bool
	windows map[*Window]struct{}
}

const (
	maxPixelSurfaceBytes = 64 << 20
	maxPixelSurfaceEdge  = 8192
)

var errPixelSurfaceClosed = errors.New("gui: pixel surface is closed or uninitialized")

// NewPixelSurface allocates a transparent surface. Each edge must be positive
// and at most 8192 pixels, and width*height*4 must fit in 64 MiB.
func NewPixelSurface(cfg PixelSurfaceCfg) (*PixelSurface, error) {
	if !validPixelSurfaceSize(cfg.Width, cfg.Height) {
		return nil, errors.New("gui: invalid pixel surface size (maximum 8192 per edge and 64 MiB)")
	}
	return &PixelSurface{w: cfg.Width, h: cfg.Height,
		pix: make([]byte, cfg.Width*cfg.Height*4), version: 1}, nil
}

func validPixelSurfaceSize(w, h int) bool {
	return w > 0 && h > 0 && w <= maxPixelSurfaceEdge && h <= maxPixelSurfaceEdge &&
		int64(w)*int64(h)*4 <= maxPixelSurfaceBytes
}

// Update copies the surface's current dimensions from pix. Stride is bytes
// between source rows and must be at least width*4; final-row padding is
// optional. Invalid input leaves the current frame unchanged. At a fixed
// resolution Update reuses storage and allocates nothing, so the caller can
// reuse pix at once.
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
	row := s.w * 4
	// Division avoids overflowing stride*(height-1), even on 32-bit hosts.
	if stride < row || len(pix) < row || (s.h > 1 && stride > (len(pix)-row)/(s.h-1)) {
		return errors.New("gui: pixel surface source is short or has invalid stride")
	}
	for y := range s.h {
		copy(s.pix[y*row:(y+1)*row], pix[y*stride:y*stride+row])
	}
	s.version++
	s.wakeWindows()
	return nil
}

// Resize replaces the image with transparent pixels and wakes its windows.
// The same size is a no-op. Backend storage is recreated on the next draw.
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
	if !validPixelSurfaceSize(width, height) {
		return errors.New("gui: invalid pixel surface resize")
	}
	if s.w == width && s.h == height {
		return nil
	}
	s.w, s.h = width, height
	s.pix = make([]byte, width*height*4)
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
	s.pix = nil
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

// ReadPixels is the renderer integration point. If the surface is open, it
// returns true and synchronously visits tightly packed pixels when version
// differs from after. Each renderer keeps its own version, starting at 0.
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
	if s.closed || s.version == 0 {
		return false
	}
	if s.version != after && visit != nil {
		visit(s.w, s.h, s.pix, s.version)
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
// surfaces this window's image commands draw, each subscribed to wake the
// window on a new frame, and whether the render pass in progress drew one.
type surfaceSubscriptions struct {
	drawn map[*PixelSurface]bool
	drew  bool
}

// syncPixelSurfaces subscribes the window to the surfaces this frame's image
// commands draw and drops the ones it no longer draws. Frame thread only; the
// surface lock serializes subscriptions with producers.
func (w *Window) syncPixelSurfaces() {
	if len(w.pixelSurfaces.drawn) == 0 && !w.pixelSurfaces.drew {
		return
	}
	w.pixelSurfaces.drew = false
	for surface := range w.pixelSurfaces.drawn {
		w.pixelSurfaces.drawn[surface] = false
	}
	for i := range w.renderers {
		surface := w.renderers[i].Surface
		if surface == nil || w.renderers[i].Kind != RenderImage {
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
