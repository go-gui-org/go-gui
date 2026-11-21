//go:build !js && !darwin && !android

package gl

import "github.com/go-gui-org/go-gui/gui"

// A frame imported from a DMA-BUF (gui.PixelSurface.UpdateDMABuf, #1326)
// stays in its producer's buffer: a video decoder writes a later picture
// into it once the frame is released. So a frame the renderer stops
// drawing goes back only after the GPU has finished the commands that
// read it. GL keeps the buffer itself alive for commands in flight, so
// the EGLImage and texture can go at once; only the release waits, on a
// fence inserted after those commands. A frame goes back only once a fence
// confirms that, or after glFinish: a fence that could not be made, a
// failed wait, or a wait that runs out says nothing about the GPU.

// dmabufImporter makes EGLImages from DMA-BUF planes (dmabuf_linux.go).
// A Backend has none where its window cannot import them.
type dmabufImporter interface {
	// createImage returns 0 when the driver refuses the plane.
	createImage(format gui.PixelFormat, width, height int, buf gui.DMABufPlane) uintptr
	// bindImage gives the texture bound to GL_TEXTURE_2D the image's
	// storage, and reports false when GL refuses it.
	bindImage(image uintptr) bool
	destroyImage(image uintptr)
}

// fenceState is what polling a fence says.
type fenceState uint8

const (
	fencePending fenceState = iota
	fenceSignaled
	// fenceFailed: the wait itself failed, which says nothing about the
	// GPU's progress.
	fenceFailed
)

// retiredFrame is a frame the renderer no longer draws. fence is zero
// when no command read it, after a failed import.
type retiredFrame struct {
	fence   uintptr
	surface *gui.PixelSurface
	frame   gui.GPUFrame
}

// frameRetirer hands retired frames back, in order, as their fences
// signal. Fences in one context signal in the order they were inserted,
// so the queue stops at the first that has not. The GL calls are fields,
// bound once, so the ordering is tested without a context.
type frameRetirer struct {
	queue []retiredFrame
	// fence inserts a fence after the commands issued so far, or returns
	// 0 when it cannot; check polls one without blocking; wait blocks
	// until it signals, at teardown, and reports false when it could not
	// confirm that in time; finish blocks until the GPU has completed
	// every command (glFinish); deleteSync frees a fence.
	fence      func() uintptr
	check      func(uintptr) fenceState
	wait       func(uintptr) bool
	finish     func()
	deleteSync func(uintptr)
}

// retire queues a frame. fenced is false when no command read it. A fence
// that cannot be made is replaced by finish, so the frame is safe to
// release at the next poll.
func (q *frameRetirer) retire(surface *gui.PixelSurface, frame gui.GPUFrame, fenced bool) {
	if frame == nil {
		return
	}
	r := retiredFrame{surface: surface, frame: frame}
	if fenced {
		if r.fence = q.fence(); r.fence == 0 {
			q.finish()
		}
	}
	q.queue = append(q.queue, r)
}

// poll releases the frames whose fences have signaled. A failed wait
// releases only after finish. Call it with the window's context current,
// before drawing.
func (q *frameRetirer) poll() {
	done := 0
	for _, r := range q.queue {
		if r.fence != 0 {
			state := q.check(r.fence)
			if state == fencePending {
				break
			}
			if state == fenceFailed {
				q.finish()
			}
		}
		q.release(r)
		done++
	}
	q.shift(done)
}

// drain waits for every fence and releases every frame, at teardown,
// with the window's context current. A wait that cannot confirm the fence
// falls back to finish.
func (q *frameRetirer) drain() {
	for _, r := range q.queue {
		if r.fence != 0 && !q.wait(r.fence) {
			q.finish()
		}
		q.release(r)
	}
	q.shift(len(q.queue))
}

// pending reports whether a frame still waits for its fence: the window
// must draw again so that poll runs.
func (q *frameRetirer) pending() bool { return len(q.queue) != 0 }

func (q *frameRetirer) release(r retiredFrame) {
	if r.fence != 0 {
		q.deleteSync(r.fence)
	}
	r.surface.DropGPUFrame(r.frame)
}

// shift drops the first n frames, keeping the queue's storage.
func (q *frameRetirer) shift(n int) {
	if n == 0 {
		return
	}
	rest := copy(q.queue, q.queue[n:])
	clear(q.queue[rest:])
	q.queue = q.queue[:rest]
}
