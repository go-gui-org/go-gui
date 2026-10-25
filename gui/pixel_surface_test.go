package gui

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func testPixelSurface(t testing.TB, w, h int) *PixelSurface {
	t.Helper()
	surface, err := NewPixelSurface(PixelSurfaceCfg{Width: w, Height: h})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	return surface
}

func TestPixelSurfaceOwnershipAndVersions(t *testing.T) {
	surface := testPixelSurface(t, 2, 2)
	input := []byte{1, 2, 3, 255, 4, 5, 6, 255, 99, 99, 99, 99,
		7, 8, 9, 255, 10, 11, 12, 255}
	if err := surface.Update(input, 12); err != nil {
		t.Fatal(err)
	}
	want := append(bytes.Clone(input[:8]), input[12:]...)
	clear(input) // the caller may reuse its buffer at once
	var version uint64
	if !surface.ReadPixels(0, func(w, h int, pix []byte, generation uint64) {
		if w != 2 || h != 2 || !bytes.Equal(pix, want) {
			t.Errorf("snapshot = %dx%d %v, want %v", w, h, pix, want)
		}
		version = generation
	}) {
		t.Fatal("open surface unavailable")
	}
	surface.ReadPixels(version, func(int, int, []byte, uint64) {
		t.Error("unchanged frame visited")
	})
	for _, stride := range []int{-1, 0, 7, 100, int(^uint(0) >> 1)} {
		if err := surface.Update(input, stride); err == nil {
			t.Errorf("accepted stride %d", stride)
		}
	}
	surface.ReadPixels(version, func(int, int, []byte, uint64) {
		t.Error("invalid update changed the version")
	})
	if err := surface.Resize(3, 1); err != nil {
		t.Fatal(err)
	}
	surface.ReadPixels(version, func(w, h int, pix []byte, generation uint64) {
		if w != 3 || h != 1 || !bytes.Equal(pix, make([]byte, 12)) || generation == version {
			t.Error("resize did not publish a transparent replacement")
		}
	})
	surface.Close()
	surface.Close()
	if !surface.Closed() || surface.ReadPixels(0, nil) || surface.Update(input, 12) == nil || surface.Resize(1, 1) == nil {
		t.Error("closed surface remained usable")
	}
}

func TestPixelSurfaceInvalidSizes(t *testing.T) {
	for _, size := range [][2]int{{0, 1}, {-1, 1}, {1, 0}, {8193, 1}, {8192, 8192}, {int(^uint(0) >> 1), 2}} {
		if _, err := NewPixelSurface(PixelSurfaceCfg{Width: size[0], Height: size[1]}); err == nil {
			t.Errorf("accepted %v", size)
		}
	}
	var zero PixelSurface
	if !zero.Closed() || zero.ReadPixels(0, nil) || zero.Update(nil, 0) == nil {
		t.Error("zero surface usable")
	}
	var absent *PixelSurface
	absent.Close()
	if !absent.Closed() || absent.ReadPixels(0, nil) || absent.Resize(1, 1) == nil || absent.Update(nil, 0) == nil {
		t.Error("nil surface usable")
	}
}

func TestPixelSurfaceConcurrentPublication(t *testing.T) {
	surface := testPixelSurface(t, 16, 16)
	var workers sync.WaitGroup
	workers.Add(2)
	go func() {
		defer workers.Done()
		pix := make([]byte, 16*16*4)
		for i := range 300 {
			for j := range pix {
				pix[j] = byte(i)
			}
			if err := surface.Update(pix, 16*4); err != nil {
				t.Error(err)
			}
		}
	}()
	go func() {
		defer workers.Done()
		for range 300 {
			surface.ReadPixels(0, func(_, _ int, pix []byte, _ uint64) {
				for _, value := range pix {
					if value != pix[0] {
						t.Error("torn frame")
						return
					}
				}
			})
		}
	}()
	workers.Wait()
}

func TestPixelSurfaceWindowLifecycle(t *testing.T) {
	surface := testPixelSurface(t, 1, 1)
	builds := 0
	first := NewTestWindow(t, WindowCfg{Width: 100, Height: 100})
	second := NewTestWindow(t, WindowCfg{Width: 100, Height: 100})
	for _, window := range []*Window{first, second} {
		window.SetView(func(*Window) View {
			builds++
			return Image(ImageCfg{Surface: surface, Src: "missing-file-must-not-load", Width: 40, Height: 40})
		})
		window.FrameFn()
	}
	before := builds
	if err := surface.Update([]byte{255, 0, 0, 255}, 4); err != nil {
		t.Fatal(err)
	}
	for _, window := range []*Window{first, second} {
		if !window.FrameFn() {
			t.Error("update did not request a frame")
		}
		found := false
		for _, cmd := range window.Renderers() {
			if cmd.Kind == RenderImage && cmd.Surface == surface {
				found = true
			}
		}
		if !found {
			t.Error("surface lost during the render-only refresh")
		}
	}
	if builds != before {
		t.Error("pixel update regenerated layout")
	}
	first.SetView(func(*Window) View { return Image(ImageCfg{Invisible: true}) })
	first.FrameFn()
	if _, subscribed := surface.windows[first]; subscribed {
		t.Error("removed view kept its subscription")
	}
	second.WindowCleanup()
	if len(surface.windows) != 0 {
		t.Error("destroyed window kept its subscription")
	}
}

func TestPixelSurfaceImageKeepsFileBehaviour(t *testing.T) {
	// A surface image must not stat or load Src, and an image without a
	// surface must not gain effects or a surface on its command.
	surface := testPixelSurface(t, 1, 1)
	w := NewTestWindow(t, WindowCfg{Width: 100, Height: 100})
	w.SetView(func(*Window) View {
		return Row(ContainerCfg{Content: []View{
			Image(ImageCfg{Surface: surface, Src: "missing-file-must-not-load", Width: 20, Height: 20}),
			Image(ImageCfg{Src: UseImage("surface-test-plain", 1, 1, []byte{0, 0, 0, 255}), Width: 20, Height: 20}),
		}})
	})
	w.FrameFn()
	var withSurface, plain int
	for _, cmd := range w.Renderers() {
		switch {
		case cmd.Kind == RenderImage && cmd.Surface == surface:
			withSurface++
		case cmd.Kind == RenderImage && cmd.Surface == nil:
			plain++
		case cmd.Kind == RenderText:
			t.Errorf("surface image fell back to error text %q", cmd.Text)
		}
	}
	if withSurface != 1 || plain != 1 {
		t.Errorf("image commands: %d with surface, %d plain; want 1 each", withSurface, plain)
	}
}

func TestPixelSurfaceWindowWithoutSurfacesSkipsSync(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 100, Height: 100})
	w.SetView(func(*Window) View {
		return Image(ImageCfg{Src: UseImage("surface-test-skip", 1, 1, []byte{0, 0, 0, 255}), Width: 20, Height: 20})
	})
	w.FrameFn()
	if w.pixelSurfaces.drawn != nil || w.pixelSurfaces.drew {
		t.Error("a window that draws no surface built subscription state")
	}
}

func TestPixelSurfaceUpdateAllocations(t *testing.T) {
	surface := testPixelSurface(t, 32, 32)
	pix := make([]byte, 32*32*4)
	allocs := testing.AllocsPerRun(100, func() {
		if err := surface.Update(pix, 32*4); err != nil {
			panic(err)
		}
	})
	if allocs != 0 {
		t.Errorf("update allocated %.1f objects", allocs)
	}
}

func TestPixelSurfacePDF(t *testing.T) {
	surface := testPixelSurface(t, 2, 1)
	if err := surface.Update([]byte{255, 0, 0, 255, 0, 0, 255, 255}, 8); err != nil {
		t.Fatal(err)
	}
	job := testPrintJob(t)
	commands := []RenderCmd{
		{Kind: RenderImage, Surface: surface, W: 50, H: 30, Opacity: 0.5, ImageFilter: ImageFilterNearest},
		{Kind: RenderImage, Surface: surface, X: 60, W: 50, H: 30, Opacity: 1},
	}
	if err := renderToPDF(commands, job, 120, 80); err != nil {
		t.Fatal(err)
	}
	assertPDFExists(t, job.OutputPath)
}

func BenchmarkPixelSurfaceUpdate(b *testing.B) {
	for _, size := range [][2]int{{1280, 720}, {1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(b *testing.B) {
			surface := testPixelSurface(b, size[0], size[1])
			pix := make([]byte, size[0]*size[1]*4)
			b.SetBytes(int64(len(pix)))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if err := surface.Update(pix, size[0]*4); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
