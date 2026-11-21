package gui

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

// testGPUFrame counts its releases; a release past the first is a bug.
type testGPUFrame struct {
	releases  atomic.Int32
	onRelease func()
}

func (f *testGPUFrame) Release() {
	f.releases.Add(1)
	if f.onRelease != nil {
		f.onRelease()
	}
}

// testBufs are DMA-BUF planes for testPlaneSurface: R8 4x2, and RG8 2x1
// in a layout with a metadata plane.
var testBufs = []DMABufPlane{{FD: 3, Pitch: 64},
	{FD: 4, Offset: 128, Pitch: 64, Modifier: 7, Metadata: []DMABufMemory{{FD: 4, Offset: 4096, Pitch: 16}}}}

// samePlane compares two planes, metadata included.
func samePlane(a, b DMABufPlane) bool {
	return a.FD == b.FD && a.Offset == b.Offset && a.Pitch == b.Pitch && a.Modifier == b.Modifier &&
		slices.Equal(a.Metadata, b.Metadata)
}

// readGPU reads the surface's current GPU frame, as an importing renderer
// would, and returns it held, or nil when there is none or it is unchanged.
func readGPU(s *PixelSurface, after uint64) (frame GPUFrame, version uint64) {
	s.ReadDMABuf(after, func(_ []SurfacePlane, _ []DMABufPlane, f GPUFrame, v uint64) {
		frame, version = f, v
	})
	return frame, version
}

func TestPixelSurfaceDMABufFrames(t *testing.T) {
	surface := testPlaneSurface(t)
	first := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, first); err != nil {
		t.Fatal(err)
	}
	if surface.ReadPlanes(0, nil) || surface.ReadPixels(0, nil) {
		t.Error("a GPU frame was offered as main-memory planes")
	}
	var version uint64
	if !surface.ReadDMABuf(0, func(planes []SurfacePlane, bufs []DMABufPlane, frame GPUFrame, v uint64) {
		version = v
		if len(planes) != 2 || planes[0].Format != PixelFormatR8 || planes[0].Width != 4 || planes[0].Height != 2 ||
			planes[1].Format != PixelFormatRG8 || planes[1].Width != 2 || planes[1].Height != 1 ||
			planes[0].Pix != nil || planes[1].Pix != nil {
			t.Errorf("planes = %+v", planes)
		}
		if len(bufs) != 2 || !samePlane(bufs[0], testBufs[0]) || !samePlane(bufs[1], testBufs[1]) || frame != first {
			t.Errorf("bufs = %+v, frame %p", bufs, frame)
		}
	}) {
		t.Fatal("GPU frame unavailable")
	}
	surface.ReadDMABuf(version, func([]SurfacePlane, []DMABufPlane, GPUFrame, uint64) {
		t.Error("unchanged frame visited")
	})

	// A held frame outlives its replacement until the renderer drops it.
	second := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, second); err != nil {
		t.Fatal(err)
	}
	if first.releases.Load() != 0 {
		t.Fatal("a held frame was released when replaced")
	}
	surface.DropGPUFrame(first)
	surface.DropGPUFrame(first)
	if first.releases.Load() != 1 {
		t.Fatalf("first frame released %d times, want 1", first.releases.Load())
	}

	// A frame no renderer read is released as soon as it is replaced,
	// here by main-memory planes, which draw again.
	if err := surface.UpdatePlanes(PixelPlane{Pix: make([]byte, 8), Stride: 4}, PixelPlane{Pix: make([]byte, 4), Stride: 4}); err != nil {
		t.Fatal(err)
	}
	if second.releases.Load() != 1 {
		t.Fatalf("unread frame released %d times, want 1", second.releases.Load())
	}
	if !surface.ReadPlanes(0, nil) || surface.ReadDMABuf(0, nil) {
		t.Error("planes did not become the current frame again")
	}

	// Close releases the current frame, or leaves a held one to its
	// renderer, whose drop releases it after Close.
	held := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, held); err != nil {
		t.Fatal(err)
	}
	if frame, _ := readGPU(surface, 0); frame != held {
		t.Fatal("GPU frame unavailable")
	}
	surface.Close()
	if held.releases.Load() != 0 {
		t.Fatal("Close released a held frame")
	}
	surface.DropGPUFrame(held)
	if held.releases.Load() != 1 {
		t.Fatalf("held frame released %d times after Close, want 1", held.releases.Load())
	}
	if surface.UpdateDMABuf(testBufs, &testGPUFrame{}) == nil || surface.ReadDMABuf(0, nil) {
		t.Error("closed surface took a GPU frame")
	}
}

// The surface copies the metadata planes: the producer may reuse its
// slice at once, and a renderer sees what was published.
func TestPixelSurfaceDMABufCopiesMetadata(t *testing.T) {
	surface := testPlaneSurface(t)
	meta := []DMABufMemory{{FD: 9, Offset: 1, Pitch: 2}, {FD: 9, Offset: 3, Pitch: 4}}
	planes := []DMABufPlane{{FD: 3, Pitch: 64}, {FD: 9, Pitch: 64, Metadata: meta}}
	if err := surface.UpdateDMABuf(planes, &testGPUFrame{}); err != nil {
		t.Fatal(err)
	}
	want := slices.Clone(meta)
	meta[0].FD, meta[1].Offset = 99, 99
	surface.ReadDMABuf(0, func(_ []SurfacePlane, bufs []DMABufPlane, _ GPUFrame, _ uint64) {
		if len(bufs[0].Metadata) != 0 || !slices.Equal(bufs[1].Metadata, want) {
			t.Errorf("metadata = %+v, %+v; want none, %+v", bufs[0].Metadata, bufs[1].Metadata, want)
		}
	})
}

func TestPixelSurfaceDMABufCloseReleasesCurrent(t *testing.T) {
	surface := testPlaneSurface(t)
	frame := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
		t.Fatal(err)
	}
	surface.Close()
	surface.Close()
	if frame.releases.Load() != 1 {
		t.Fatalf("released %d times, want 1", frame.releases.Load())
	}
}

// Update and Resize replace a GPU frame on a single-plane surface too.
// Each case has a surface of its own: Resize changes the width.
func TestPixelSurfaceDMABufSinglePlane(t *testing.T) {
	cases := []struct {
		name    string
		replace func(*PixelSurface) error
	}{
		{"Update", func(s *PixelSurface) error { return s.Update(make([]byte, 8), 8) }},
		{"Resize", func(s *PixelSurface) error { return s.Resize(4, 1) }},
	}
	for _, c := range cases {
		surface := testPixelSurface(t, 2, 1)
		frame := &testGPUFrame{}
		if err := surface.UpdateDMABuf([]DMABufPlane{{FD: 3, Pitch: 8}}, frame); err != nil {
			t.Fatal(c.name, err)
		}
		if surface.ReadPixels(0, nil) {
			t.Errorf("%s: an Image could draw a GPU frame", c.name)
		}
		if err := c.replace(surface); err != nil {
			t.Fatal(c.name, err)
		}
		if frame.releases.Load() != 1 || !surface.ReadPixels(0, nil) {
			t.Errorf("%s: released %d times; Image drawable %v", c.name, frame.releases.Load(), surface.ReadPixels(0, nil))
		}
	}
}

// Each renderer that imported a frame holds it: the last drop releases it.
func TestPixelSurfaceDMABufHeldByEveryRenderer(t *testing.T) {
	surface := testPlaneSurface(t)
	frame := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
		t.Fatal(err)
	}
	readGPU(surface, 0) // one window
	readGPU(surface, 0) // another
	if err := surface.UpdateDMABuf(testBufs, &testGPUFrame{}); err != nil {
		t.Fatal(err)
	}
	surface.DropGPUFrame(frame)
	if frame.releases.Load() != 0 {
		t.Fatal("released while another renderer holds it")
	}
	surface.DropGPUFrame(frame)
	if frame.releases.Load() != 1 {
		t.Fatalf("released %d times, want 1", frame.releases.Load())
	}
}

// A rejected frame stays the caller's: it is not released, and the
// current frame and version stay as they were.
func TestPixelSurfaceDMABufRejects(t *testing.T) {
	surface := testPlaneSurface(t)
	current := &testGPUFrame{}
	if err := surface.UpdateDMABuf(testBufs, current); err != nil {
		t.Fatal(err)
	}
	_, version := readGPU(surface, 0)
	short := []DMABufPlane{{FD: 3, Pitch: 3}, testBufs[1]} // the R8 plane is 4 wide
	noFD := []DMABufPlane{{FD: -1, Pitch: 64}, testBufs[1]}
	metaNoFD := []DMABufPlane{testBufs[0], {FD: 4, Pitch: 64, Metadata: []DMABufMemory{{FD: -1}}}}
	fourMeta := []DMABufPlane{testBufs[0], {FD: 4, Pitch: 64, Metadata: make([]DMABufMemory, 4)}}
	rejected := &testGPUFrame{}
	for name, call := range map[string]func() error{
		"nil frame":      func() error { return surface.UpdateDMABuf(testBufs, nil) },
		"one plane":      func() error { return surface.UpdateDMABuf(testBufs[:1], rejected) },
		"short pitch":    func() error { return surface.UpdateDMABuf(short, rejected) },
		"no fd":          func() error { return surface.UpdateDMABuf(noFD, rejected) },
		"metadata no fd": func() error { return surface.UpdateDMABuf(metaNoFD, rejected) },
		"four metadata":  func() error { return surface.UpdateDMABuf(fourMeta, rejected) },
		"same frame":     func() error { return surface.UpdateDMABuf(testBufs, current) },
		"nil surface":    func() error { return (*PixelSurface)(nil).UpdateDMABuf(testBufs, rejected) },
		"zero surface":   func() error { return new(PixelSurface).UpdateDMABuf(testBufs, rejected) },
	} {
		if call() == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if rejected.releases.Load() != 0 || current.releases.Load() != 0 {
		t.Error("a rejected update released a frame")
	}
	surface.ReadDMABuf(version, func([]SurfacePlane, []DMABufPlane, GPUFrame, uint64) {
		t.Error("a rejected update changed the frame")
	})
	surface.DropGPUFrame(current)
}

// A surface lends at most maxGPUFrames frames: past that, renderers that
// never drop theirs make UpdateDMABuf fail instead of growing.
func TestPixelSurfaceDMABufBound(t *testing.T) {
	surface := testPlaneSurface(t)
	frames := make([]*testGPUFrame, maxGPUFrames)
	for i := range frames {
		frames[i] = &testGPUFrame{}
		if err := surface.UpdateDMABuf(testBufs, frames[i]); err != nil {
			t.Fatal(i, err)
		}
		readGPU(surface, 0)
	}
	extra := &testGPUFrame{}
	if surface.UpdateDMABuf(testBufs, extra) == nil {
		t.Fatal("lent more than maxGPUFrames frames")
	}
	surface.DropGPUFrame(frames[0])
	if err := surface.UpdateDMABuf(testBufs, extra); err != nil {
		t.Fatalf("a dropped frame's slot was not reused: %v", err)
	}
	if len(surface.gpu) != maxGPUFrames {
		t.Errorf("%d slots, want %d", len(surface.gpu), maxGPUFrames)
	}
	for _, f := range frames {
		surface.DropGPUFrame(f)
	}
	for i, f := range frames {
		if f.releases.Load() != 1 {
			t.Errorf("frame %d released %d times", i, f.releases.Load())
		}
	}
}

func TestPixelSurfaceDMABufWakesWindows(t *testing.T) {
	surface := testPlaneSurface(t)
	w := NewTestWindow(t, WindowCfg{Width: 100, Height: 100})
	shader := &Shader{GLSL: "g", Metal: "m", Surface: surface}
	w.SetView(func(*Window) View {
		return Column(ContainerCfg{Shader: shader, Sizing: FixedFixed, Width: 40, Height: 40})
	})
	w.FrameFn()
	if err := surface.UpdateDMABuf(testBufs, &testGPUFrame{}); err != nil {
		t.Fatal(err)
	}
	if !w.FrameFn() {
		t.Error("a GPU frame did not request a frame")
	}
}

func TestWindowImportsDMABuf(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 10, Height: 10})
	if w.ImportsDMABuf() {
		t.Error("a window without a backend imports DMA-BUFs")
	}
	SetWindowImportsDMABuf(w, true)
	if !w.ImportsDMABuf() {
		t.Error("the backend's flag was lost")
	}
	SetWindowImportsDMABuf(nil, true)
	if (*Window)(nil).ImportsDMABuf() {
		t.Error("a nil window imports DMA-BUFs")
	}
}

// Publishing, importing and dropping pooled frames allocates nothing once
// the surface's slots exist.
func TestPixelSurfaceDMABufAllocations(t *testing.T) {
	surface := testPlaneSurface(t)
	frames := [2]*testGPUFrame{{}, {}}
	var held GPUFrame
	var version uint64
	n := 0
	cycle := func() {
		if err := surface.UpdateDMABuf(testBufs, frames[n%2]); err != nil {
			panic(err)
		}
		n++
		if held != nil {
			surface.DropGPUFrame(held)
		}
		held, version = readGPU(surface, version)
	}
	cycle()
	cycle()
	allocs := testing.AllocsPerRun(100, cycle)
	if allocs != 0 {
		t.Errorf("a GPU frame cycle allocated %.1f objects", allocs)
	}
}

// A producer recycling a pool of frames, and a renderer that drops each
// frame one frame late, as after a fence, never lose or double-release
// a frame.
func TestPixelSurfaceDMABufConcurrent(t *testing.T) {
	surface := testPlaneSurface(t)
	pool := make(chan *testGPUFrame, 6)
	frames := make([]*testGPUFrame, cap(pool))
	for i := range frames {
		f := &testGPUFrame{}
		f.onRelease = func() { pool <- f }
		frames[i] = f
		pool <- f
	}
	var published atomic.Int32
	var workers sync.WaitGroup
	done := make(chan struct{})
	workers.Add(2)
	go func() {
		defer workers.Done()
		defer close(done)
		for range 2000 {
			f := <-pool
			if surface.UpdateDMABuf(testBufs, f) != nil {
				pool <- f
				continue
			}
			published.Add(1)
		}
	}()
	go func() {
		defer workers.Done()
		var held GPUFrame
		var version uint64
		for {
			select {
			case <-done:
				if held != nil {
					surface.DropGPUFrame(held)
				}
				return
			default:
			}
			frame, v := readGPU(surface, version)
			if frame == nil {
				continue
			}
			if held != nil {
				surface.DropGPUFrame(held)
			}
			held, version = frame, v
		}
	}()
	workers.Wait()
	surface.Close()
	var released int32
	for _, f := range frames {
		released += f.releases.Load()
	}
	if released != published.Load() || len(pool) != cap(pool) {
		t.Errorf("published %d, released %d, %d of %d frames back", published.Load(), released, len(pool), cap(pool))
	}
}
