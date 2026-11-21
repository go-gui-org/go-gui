package gui

import "errors"

// DMABufPlane is one plane of a frame in a Linux DMA-BUF, as a hardware
// video decoder exports it (#1326).
type DMABufPlane struct {
	// FD is the DMA-BUF's file descriptor. It stays open, and the buffer
	// unchanged, until the frame's Release runs.
	FD int
	// Offset is the bytes from the buffer's start to the plane's first
	// pixel, and Pitch the bytes between its rows.
	Offset, Pitch uint32
	// Modifier is the DRM format modifier, the buffer's layout: 0 is
	// DRM_FORMAT_MOD_LINEAR, rows one after another.
	Modifier uint64
	// Metadata lists the further memory planes the modifier's layout takes
	// for this image plane, in the modifier's order: up to three, such as
	// the compression metadata of AMD's DCC or Intel's CCS. Most layouts
	// take none. The surface copies the entries, so the caller can reuse
	// the slice's storage at once.
	Metadata []DMABufMemory
}

// DMABufMemory is one further memory plane of a DMABufPlane.
type DMABufMemory struct {
	// FD stays open, and the buffer unchanged, until the frame's Release
	// runs. It may name the same buffer as the plane's own FD.
	FD            int
	Offset, Pitch uint32
}

// maxDMABufMetadata is the most further memory planes an image plane can
// take: EGL imports four memory planes per image.
const maxDMABufMetadata = 3

// GPUFrame is a frame that lives on the GPU, lent to a PixelSurface by
// UpdateDMABuf. Release runs once, when no renderer draws the frame any
// more: after a later frame or Close replaced it, and after the GPU has
// finished reading it. It runs on the goroutine that replaced the frame or
// on a render thread, never under the surface's lock, and must not block
// or call the surface.
//
// The value must be comparable, such as a pointer: the surface tells one
// lent frame from another by it. A pooled pointer per frame publishes a
// frame without allocating.
type GPUFrame interface {
	Release()
}

// maxGPUFrames bounds the frames a surface lends at once: its current one
// and those renderers still hold. A renderer holds a replaced frame for
// about a frame, until its fence signals.
const maxGPUFrames = 16

// gpuSlot is a frame lent by UpdateDMABuf. A free slot has no frame. bufs
// hold no Metadata slice: their further memory planes are copied into
// meta, so the slot never points at the producer's storage.
type gpuSlot struct {
	frame GPUFrame
	bufs  [maxSurfacePlanes]DMABufPlane
	meta  [maxSurfacePlanes][maxDMABufMetadata]DMABufMemory
	metaN [maxSurfacePlanes]int
	// holds counts the renderers that imported the frame and have not yet
	// dropped it; current marks the surface's current frame.
	holds   int
	current bool
}

// UpdateDMABuf publishes a frame that lives in DMA-BUFs as the surface's
// next frame, one buffer plane per configured plane, in plane order. The
// planes keep their configured formats and sizes: an R8 plane is a
// DRM_FORMAT_R8 buffer, RG8 is DRM_FORMAT_GR88 and RGBA8 is
// DRM_FORMAT_ABGR8888. The frame must be complete when published.
//
// The surface keeps the frame until it is replaced by a later Update,
// UpdatePlanes, UpdateDMABuf, Resize or Close, and until every renderer
// that imported it is done; then it calls frame.Release. On error the
// surface did not take the frame: the caller still owns it.
//
// Only a window whose backend imports DMA-BUFs draws such a frame
// (Window.ImportsDMABuf); any other draws the surface transparent, and
// an Image never draws it. A producer checks ImportsDMABuf for every frame
// and publishes with UpdatePlanes while it is false: it also turns false
// once the window's driver refused a frame's buffers.
func (s *PixelSurface) UpdateDMABuf(planes []DMABufPlane, frame GPUFrame) error {
	if s == nil {
		return errPixelSurfaceClosed
	}
	if frame == nil {
		return errors.New("gui: pixel surface UpdateDMABuf needs a GPUFrame")
	}
	var done GPUFrame
	defer releaseGPUFrame(&done)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 {
		return errPixelSurfaceClosed
	}
	if len(planes) != s.n {
		return errors.New("gui: pixel surface UpdateDMABuf needs one buffer per plane")
	}
	for i, p := range planes {
		if p.FD < 0 || int64(p.Pitch) < int64(s.planes[i].Width)*int64(s.planes[i].Format.BytesPerPixel()) {
			return errors.New("gui: pixel surface DMA-BUF plane has no file descriptor or a short pitch")
		}
		if len(p.Metadata) > maxDMABufMetadata {
			return errors.New("gui: pixel surface DMA-BUF plane has more than three metadata planes")
		}
		for _, m := range p.Metadata {
			if m.FD < 0 {
				return errors.New("gui: pixel surface DMA-BUF metadata plane has no file descriptor")
			}
		}
	}
	slot := -1
	for i := range s.gpu {
		g := &s.gpu[i]
		if g.frame == frame {
			return errors.New("gui: pixel surface already holds this GPU frame")
		}
		reusable := g.frame == nil || (g.current && g.holds == 0)
		if slot < 0 && reusable {
			slot = i
		}
	}
	if slot < 0 && len(s.gpu) >= maxGPUFrames {
		return errors.New("gui: pixel surface renderers still hold too many GPU frames")
	}
	done = s.retireGPUFrame()
	if slot < 0 {
		s.gpu = append(s.gpu, gpuSlot{})
		slot = len(s.gpu) - 1
	}
	g := &s.gpu[slot]
	*g = gpuSlot{frame: frame, current: true}
	for i, p := range planes {
		g.metaN[i] = copy(g.meta[i][:], p.Metadata)
		p.Metadata = nil
		g.bufs[i] = p
	}
	s.gpuCurrent = slot + 1
	s.version++
	s.wakeWindows()
	return nil
}

// retireGPUFrame makes the CPU planes the current frame again. It runs with
// s.mu held and returns the frame to release, if no renderer holds it.
func (s *PixelSurface) retireGPUFrame() GPUFrame {
	if s.gpuCurrent == 0 {
		return nil
	}
	g := &s.gpu[s.gpuCurrent-1]
	s.gpuCurrent = 0
	g.current = false
	if g.holds > 0 {
		return nil
	}
	frame := g.frame
	*g = gpuSlot{}
	return frame
}

// releaseGPUFrame releases *frame, if set. Deferred before the surface
// lock, it runs after the unlock.
func releaseGPUFrame(frame *GPUFrame) {
	if *frame != nil {
		(*frame).Release()
	}
}

// ReadDMABuf is the integration point of a renderer that imports DMA-BUFs.
// If the surface is open and its current frame is one lent by UpdateDMABuf,
// it returns true and, when version differs from after, holds the frame
// for the renderer and visits it. planes gives each plane's format and
// size (Pix is nil), bufs its DMA-BUF. The slices are valid only during
// visit, and visit must not call surface methods.
//
// Each visit's hold ends with one DropGPUFrame, once the renderer will not
// draw the frame again and the GPU has finished reading it, also when the
// import failed. Otherwise the frame is never released.
func (s *PixelSurface) ReadDMABuf(after uint64, visit func(planes []SurfacePlane, bufs []DMABufPlane, frame GPUFrame, version uint64)) bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.version == 0 || s.gpuCurrent == 0 {
		return false
	}
	if s.version != after && visit != nil {
		g := &s.gpu[s.gpuCurrent-1]
		g.holds++
		for i := range s.n {
			s.gpuPlanes[i] = s.planes[i]
			s.gpuPlanes[i].Pix = nil
			s.gpuBufs[i] = g.bufs[i]
			s.gpuBufs[i].Metadata = g.meta[i][:g.metaN[i]]
		}
		visit(s.gpuPlanes[:s.n], s.gpuBufs[:s.n], g.frame, s.version)
	}
	return true
}

// DropGPUFrame ends a renderer's hold on a frame ReadDMABuf visited. When it
// was the last hold on a frame the surface has replaced, the frame is
// released. It works after Close, so a renderer can finish its frames.
func (s *PixelSurface) DropGPUFrame(frame GPUFrame) {
	if s == nil || frame == nil {
		return
	}
	var done GPUFrame
	defer releaseGPUFrame(&done)
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.gpu {
		g := &s.gpu[i]
		if g.frame != frame || g.holds == 0 {
			continue
		}
		g.holds--
		if g.holds == 0 && !g.current {
			done = g.frame
			*g = gpuSlot{}
		}
		return
	}
}

// ImportsDMABuf reports whether the backend drawing w draws frames lent by
// PixelSurface.UpdateDMABuf: the OpenGL backend on Linux, when its EGL
// display imports DMA-BUFs. It turns false for good once the driver has
// refused a frame's buffers, for example a layout it cannot import. Where
// it is false such a frame draws transparent, so a producer publishes with
// UpdatePlanes instead. A lock-free read, safe from any goroutine.
func (w *Window) ImportsDMABuf() bool {
	return w != nil && w.importsDMABuf.Load()
}

// SetWindowImportsDMABuf records whether the backend drawing w draws
// frames lent by UpdateDMABuf; see Window.ImportsDMABuf. Intended for
// backend use. Safe for a nil window (no-op).
func SetWindowImportsDMABuf(w *Window, ok bool) {
	if w != nil {
		w.importsDMABuf.Store(ok)
	}
}
