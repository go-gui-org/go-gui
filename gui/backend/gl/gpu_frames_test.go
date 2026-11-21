//go:build !js && !darwin && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// fakeFence stands in for GL's fences: each is signaled once the test says
// the GPU got that far. noFence makes fence creation fail, failed makes
// polls fail, and stuck makes waits run out. finishes counts glFinish.
type fakeFence struct {
	next, reached          uintptr
	deleted                map[uintptr]bool
	waited                 []uintptr
	noFence, failed, stuck bool
	finishes               int
	// events, when set, records finish and releases in order.
	events *[]string
}

func newRetirer(f *fakeFence) *frameRetirer {
	f.deleted = map[uintptr]bool{}
	return &frameRetirer{
		fence: func() uintptr {
			if f.noFence {
				return 0
			}
			f.next++
			return f.next
		},
		check: func(s uintptr) fenceState {
			switch {
			case f.failed:
				return fenceFailed
			case s <= f.reached:
				return fenceSignaled
			}
			return fencePending
		},
		wait: func(s uintptr) bool {
			f.waited = append(f.waited, s)
			if f.stuck {
				return false
			}
			f.reached = max(f.reached, s)
			return true
		},
		finish: func() {
			f.finishes++
			f.reached = f.next
			if f.events != nil {
				*f.events = append(*f.events, "finish")
			}
		},
		deleteSync: func(s uintptr) { f.deleted[s] = true },
	}
}

type releasedFrame struct {
	releases int
	events   *[]string // records the release when set
}

func (r *releasedFrame) Release() {
	r.releases++
	if r.events != nil {
		*r.events = append(*r.events, "release")
	}
}

// heldFrame publishes a frame on a fresh surface, holds it as an importing
// renderer would, and replaces it, so that only the renderer's drop is
// left to release it.
func heldFrame(t *testing.T) (*gui.PixelSurface, *releasedFrame) {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: []gui.PixelPlaneCfg{{Format: gui.PixelFormatR8, Width: 2, Height: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	frame := &releasedFrame{}
	if err = surface.UpdateDMABuf([]gui.DMABufPlane{{FD: 3, Pitch: 64}}, frame); err != nil {
		t.Fatal(err)
	}
	surface.ReadDMABuf(0, func([]gui.SurfacePlane, []gui.DMABufPlane, gui.GPUFrame, uint64) {})
	if err = surface.UpdatePlanes(gui.PixelPlane{Pix: make([]byte, 4), Stride: 2}); err != nil {
		t.Fatal(err)
	}
	return surface, frame
}

func TestFrameRetirerReleasesAfterTheFence(t *testing.T) {
	f := &fakeFence{}
	q := newRetirer(f)
	s1, first := heldFrame(t)
	s2, second := heldFrame(t)
	s3, failed := heldFrame(t)
	q.retire(s1, first, true)
	q.retire(s2, second, true)
	q.retire(s3, failed, false) // a failed import: nothing read it
	q.poll()
	if first.releases != 0 || !q.pending() {
		t.Fatal("released before the GPU finished")
	}
	f.reached = 1
	q.poll()
	if first.releases != 1 || second.releases != 0 || !f.deleted[1] || f.deleted[2] {
		t.Fatalf("after fence 1: releases %d %d, deleted %v", first.releases, second.releases, f.deleted)
	}
	// The unfenced frame waits behind the fenced one before it, which
	// keeps the queue in order; it costs at most a frame.
	if failed.releases != 0 {
		t.Fatal("released out of order")
	}
	f.reached = 2
	q.poll()
	if second.releases != 1 || failed.releases != 1 || q.pending() {
		t.Fatalf("after fence 2: releases %d %d, pending %v", second.releases, failed.releases, q.pending())
	}
	q.poll()
	if first.releases != 1 || second.releases != 1 || failed.releases != 1 {
		t.Fatal("a frame was released twice")
	}
}

func TestFrameRetirerDrainWaitsForEveryFence(t *testing.T) {
	f := &fakeFence{}
	q := newRetirer(f)
	s1, first := heldFrame(t)
	s2, second := heldFrame(t)
	q.retire(s1, first, true)
	q.retire(s2, second, true)
	q.drain()
	if first.releases != 1 || second.releases != 1 || q.pending() || len(f.waited) != 2 || !f.deleted[1] || !f.deleted[2] {
		t.Fatalf("releases %d %d, waited %v, deleted %v", first.releases, second.releases, f.waited, f.deleted)
	}
}

// A fence that says nothing about the GPU never releases a frame on its
// own: glFinish confirms the GPU is done first.
func TestFrameRetirerFinishesWhenFencesFail(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*fakeFence)
		end   func(*frameRetirer)
	}{
		{"no fence", func(f *fakeFence) { f.noFence = true }, (*frameRetirer).poll},
		{"failed wait", func(f *fakeFence) { f.failed = true }, (*frameRetirer).poll},
		{"stuck at teardown", func(f *fakeFence) { f.stuck = true }, (*frameRetirer).drain},
	}
	for _, c := range cases {
		var events []string
		f := &fakeFence{events: &events}
		q := newRetirer(f)
		surface, frame := heldFrame(t)
		frame.events = &events
		c.setup(f)
		q.retire(surface, frame, true)
		c.end(q)
		if frame.releases != 1 || f.finishes != 1 || q.pending() {
			t.Errorf("%s: released %d times after %d finishes, pending %v; want 1, 1, false",
				c.name, frame.releases, f.finishes, q.pending())
		}
		// The GPU must be done before the producer gets its buffer back.
		if len(events) != 2 || events[0] != "finish" || events[1] != "release" {
			t.Errorf("%s: events %v, want [finish release]", c.name, events)
		}
	}
}

func TestFrameRetirerIgnoresNoFrame(t *testing.T) {
	q := newRetirer(&fakeFence{})
	q.retire(nil, nil, true)
	if q.pending() {
		t.Fatal("an empty retire queued a frame")
	}
}

func TestFrameRetirerWarmAllocations(t *testing.T) {
	f := &fakeFence{}
	q := newRetirer(f)
	surface, frame := heldFrame(t)
	q.retire(surface, frame, true)
	f.reached = 1
	q.poll()
	allocs := testing.AllocsPerRun(100, func() {
		q.poll()
	})
	if allocs != 0 {
		t.Errorf("poll allocated %g objects", allocs)
	}
}
