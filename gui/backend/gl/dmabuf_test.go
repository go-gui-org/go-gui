//go:build !js && !darwin && !android && !cgo

package gl

import (
	"runtime"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// testBufs describe channelSurface's planes as DMA-BUFs. The descriptors
// are never opened: these tests check what happens around an import.
var testBufs = []gui.DMABufPlane{{FD: 3, Pitch: 64}, {FD: 4, Pitch: 64}}

// countingFrame counts its releases; a GPUFrame must be released once.
type countingFrame struct{ releases int }

func (f *countingFrame) Release() { f.releases++ }

// refusingImporter is a driver that refuses every plane.
type refusingImporter struct{ creates, destroys int }

func (r *refusingImporter) createImage(gui.PixelFormat, int, int, gui.DMABufPlane) uintptr {
	r.creates++
	return 0
}

func (r *refusingImporter) bindImage(uintptr) bool { return true }

func (r *refusingImporter) destroyImage(uintptr) { r.destroys++ }

// unbindableImporter makes images GL then refuses to bind, as for a
// layout only an external texture can sample.
type unbindableImporter struct{ creates, destroys int }

func (u *unbindableImporter) createImage(gui.PixelFormat, int, int, gui.DMABufPlane) uintptr {
	u.creates++
	return uintptr(u.creates)
}

func (u *unbindableImporter) bindImage(uintptr) bool { return false }

func (u *unbindableImporter) destroyImage(uintptr) { u.destroys++ }

// channelReadback draws a shader that shows plane 0 as red and plane 1 as
// green and blue, and returns the centre pixel.
func channelReadback(b *Backend, target *readbackTarget, surface *gui.PixelSurface) [4]byte {
	cmd := gui.RenderCmd{Kind: gui.RenderCustomShader,
		Shader: &gui.Shader{GLSL: `vec2 p = uv * 0.5 + 0.5;
vec4 frag_color = vec4(texture(iChannel0, p).r, texture(iChannel1, p).rg, 1.0);`, Surface: surface},
		W: float32(b.physW) / b.dpiScale, H: float32(b.physH) / b.dpiScale}
	pixels := target.render(b, func() { b.drawCustomShader(&cmd) })
	return pixelAt(b, pixels, int(b.physW)/2, int(b.physH)/2)
}

// The window reports DMA-BUF import exactly when the backend has an
// importer, and only Linux can have one.
func TestBackendReportsDMABufImport(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "gl")
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := New(w)
	if err != nil || b == nil {
		t.Skipf("no GL backend: %v", err)
	}
	t.Cleanup(b.Destroy)
	if w.ImportsDMABuf() != (b.dmabuf != nil) {
		t.Errorf("ImportsDMABuf %v, importer %v", w.ImportsDMABuf(), b.dmabuf != nil)
	}
	if runtime.GOOS != "linux" && w.ImportsDMABuf() {
		t.Error("a window imports DMA-BUFs off Linux")
	}
	t.Logf("ImportsDMABuf: %v", w.ImportsDMABuf())
}

// A window that cannot import draws a GPU frame transparent, holds no
// part of it, and draws main-memory planes again after it.
func TestGPUFrameWithoutImportDrawsTransparent(t *testing.T) {
	b := newBatchProbeBackend(t)
	b.dmabuf = nil
	target := newReadbackTarget(t, b)
	surface := channelSurface(t)
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{128, 64, 192, 255}) {
		t.Fatalf("planes = %v", got)
	}
	frame := &countingFrame{}
	if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
		t.Fatal(err)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{0, 0, 0, 255}) {
		t.Errorf("GPU frame without import = %v, want transparent channels", got)
	}
	if err := surface.UpdatePlanes(
		gui.PixelPlane{Pix: []byte{10, 10, 10, 10}, Stride: 2},
		gui.PixelPlane{Pix: []byte{20, 30}, Stride: 2},
	); err != nil {
		t.Fatal(err)
	}
	if frame.releases != 1 {
		t.Errorf("released %d times, want 1: the window held a frame it never drew", frame.releases)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{10, 20, 30, 255}) {
		t.Errorf("planes after the GPU frame = %v", got)
	}
}

// A plane the driver refuses leaves the surface transparent until its
// next frame, and the frame goes back unread, exactly once.
func TestRefusedImportReleasesFrame(t *testing.T) {
	b := newBatchProbeBackend(t)
	importer := &refusingImporter{}
	b.dmabuf = importer
	target := newReadbackTarget(t, b)
	surface := channelSurface(t)
	frame := &countingFrame{}
	if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
		t.Fatal(err)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{0, 0, 0, 255}) {
		t.Errorf("refused import = %v, want transparent channels", got)
	}
	channelReadback(b, target, surface) // the same frame is not tried again
	if importer.creates != 1 {
		t.Errorf("%d imports of one refused frame, want 1", importer.creates)
	}
	if err := surface.UpdatePlanes(
		gui.PixelPlane{Pix: []byte{1, 1, 1, 1}, Stride: 2},
		gui.PixelPlane{Pix: []byte{1, 1}, Stride: 2},
	); err != nil {
		t.Fatal(err)
	}
	b.retired.poll()
	if frame.releases != 1 || b.retired.pending() {
		t.Errorf("released %d times, pending %v; want once, nothing pending", frame.releases, b.retired.pending())
	}
	// The window stops reporting that it imports, so the producer sends
	// its next frames with UpdatePlanes, and views that read the flag
	// rebuild.
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 8, Height: 8})
	builds := 0
	w.SetView(func(*gui.Window) gui.View { builds++; return gui.Column(gui.ContainerCfg{}) })
	w.FrameFn()
	gui.SetWindowImportsDMABuf(w, true)
	b.reportRefusedImport(w)
	if w.ImportsDMABuf() {
		t.Error("a refused import left the window reporting that it imports")
	}
	before := builds
	w.FrameFn()
	if builds == before {
		t.Error("the capability changed without rebuilding the view")
	}
	gui.SetWindowImportsDMABuf(w, true)
	b.reportRefusedImport(w)
	if !w.ImportsDMABuf() {
		t.Error("one refused import was reported twice")
	}
}

// An image GL refuses to bind is a refused import too: it is destroyed,
// the surface draws transparent, and the frame goes back unread, once.
func TestUnbindableImportReleasesFrame(t *testing.T) {
	b := newBatchProbeBackend(t)
	importer := &unbindableImporter{}
	b.dmabuf = importer
	target := newReadbackTarget(t, b)
	surface := channelSurface(t)
	frame := &countingFrame{}
	if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
		t.Fatal(err)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{0, 0, 0, 255}) {
		t.Errorf("refused bind = %v, want transparent channels", got)
	}
	if importer.creates != 1 || importer.destroys != 1 || !b.importRefused {
		t.Errorf("creates %d, destroys %d, refused %v; want 1, 1, true", importer.creates, importer.destroys, b.importRefused)
	}
	surface.Close()
	b.retired.poll()
	if frame.releases != 1 || b.retired.pending() {
		t.Errorf("released %d times, pending %v; want once, nothing pending", frame.releases, b.retired.pending())
	}
}

// The GL fences release a retired frame once the GPU has finished, and
// teardown releases what is left.
func TestRealFencesReleaseRetiredFrames(t *testing.T) {
	b := newBatchProbeBackend(t)
	held := func() (*gui.PixelSurface, *countingFrame) {
		surface := channelSurface(t)
		frame := &countingFrame{}
		if err := surface.UpdateDMABuf(testBufs, frame); err != nil {
			t.Fatal(err)
		}
		surface.ReadDMABuf(0, func([]gui.SurfacePlane, []gui.DMABufPlane, gui.GPUFrame, uint64) {})
		surface.Close() // only the hold keeps it now
		return surface, frame
	}
	s1, first := held()
	b.retired.retire(s1, first, true)
	deadline := time.Now().Add(5 * time.Second)
	for first.releases == 0 && time.Now().Before(deadline) {
		b.retired.poll()
		time.Sleep(time.Millisecond)
	}
	if first.releases != 1 {
		t.Fatalf("released %d times after the fence, want 1", first.releases)
	}
	s2, second := held()
	b.retired.retire(s2, second, true)
	b.retired.drain()
	if second.releases != 1 || b.retired.pending() {
		t.Fatalf("drain: released %d times, pending %v", second.releases, b.retired.pending())
	}
}
