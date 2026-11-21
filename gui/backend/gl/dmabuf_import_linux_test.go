//go:build linux && !js && !android && !cgo

package gl

import (
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/internal/gbm"
)

// The real import needs a GPU: a display whose EGL imports DMA-BUFs and a
// DRM render node to allocate them on. CI has neither (llvmpipe under Xvfb
// lists no EGL_EXT_image_dma_buf_import), so this test skips there, and is
// not in tools/glrendertest's list. On a machine with a GPU, run
//
//	GOGUI_REQUIRE_DMABUF=1 CGO_ENABLED=0 go test -run TestDMABufImport ./gui/backend/gl/
//
// in a desktop session, on Wayland and on X11; the variable turns the
// skips for a missing import, render node or libgbm into failures.

func requireDMABuf(t *testing.T, format string, args ...any) {
	t.Helper()
	if os.Getenv("GOGUI_REQUIRE_DMABUF") != "" {
		t.Fatalf("GOGUI_REQUIRE_DMABUF is set but "+format, args...)
	}
	t.Skipf(format, args...)
}

// producerThread runs GBM on an OS thread of its own, as a decoder runs on
// its own, apart from the render thread.
type producerThread struct{ work chan func() }

func newProducerThread(t *testing.T) *producerThread {
	p := &producerThread{work: make(chan func())}
	go func() {
		runtime.LockOSThread()
		for f := range p.work {
			f()
		}
	}()
	t.Cleanup(func() { close(p.work) })
	return p
}

// run calls f on the producer thread and waits for it.
func (p *producerThread) run(f func()) {
	done := make(chan struct{})
	p.work <- func() { f(); close(done) }
	<-done
}

// gbmFrame is a two-plane frame in GBM buffers: a uniform R8 plane and a
// uniform GR88 plane. Release only counts: it runs on the render thread,
// and the buffers are freed on the producer thread at cleanup.
type gbmFrame struct {
	bufs     [2]*gbm.Buffer
	planes   [2]gui.DMABufPlane
	releases int
}

func (f *gbmFrame) Release() { f.releases++ }

// allocator makes one plane's buffer on the producer thread.
type allocator func(dev *gbm.Device, format uint32, width, height int) (*gbm.Buffer, error)

func newGBMFrame(t *testing.T, p *producerThread, dev *gbm.Device, alloc allocator, y, u, v byte) *gbmFrame {
	t.Helper()
	f := &gbmFrame{}
	var err error
	p.run(func() {
		if f.bufs[0], err = alloc(dev, gbm.FormatR8, 64, 64); err != nil {
			return
		}
		if f.bufs[1], err = alloc(dev, gbm.FormatGR88, 32, 32); err != nil {
			return
		}
		if err = f.bufs[0].Write(1, func(_ int, row []byte) {
			for i := range row {
				row[i] = y
			}
		}); err != nil {
			return
		}
		err = f.bufs[1].Write(2, func(_ int, row []byte) {
			for i := 0; i < len(row); i += 2 {
				row[i], row[i+1] = u, v
			}
		})
		f.planes = [2]gui.DMABufPlane{f.bufs[0].Plane(), f.bufs[1].Plane()}
	})
	t.Cleanup(func() {
		p.run(func() {
			for _, b := range f.bufs {
				if b != nil {
					b.Close()
				}
			}
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestDMABufImport publishes GBM buffers with known pixels through
// UpdateDMABuf, draws them with a channel shader and reads them back, then
// checks that a replaced frame goes back only after the GPU is done with
// it, and exactly once. It runs with the implicit layout GBM picks, with
// an explicit linear layout, with a tiled modifier the display imports,
// as a hardware decoder exports, and with a compressed one, whose
// metadata planes travel as DMABufPlane.Metadata.
//
// The layouts run in a loop, not as subtests: a subtest runs on another
// goroutine, and so on another OS thread than the one New made the
// window's GL context current on.
func TestDMABufImport(t *testing.T) {
	// A required import needs a working GL backend too: without this, a
	// backend that fails to start would skip the test, not fail it.
	if os.Getenv("GOGUI_REQUIRE_DMABUF") != "" {
		t.Setenv("GOGUI_REQUIRE_GL", "1")
	}
	b := newBatchProbeBackend(t)
	if b.dmabuf == nil {
		requireDMABuf(t, "the display does not import DMA-BUFs")
	}
	p := newProducerThread(t)
	var dev *gbm.Device
	var err error
	p.run(func() { dev, err = gbm.Open() })
	if err != nil {
		requireDMABuf(t, "%v", err)
	}
	t.Cleanup(func() { p.run(dev.Close) })
	target := newReadbackTarget(t, b)

	// The tiled layouts the display imports, split by whether they keep
	// an image plane in one memory plane or add metadata planes, as a
	// compressed layout (AMD's DCC, Intel's CCS) does.
	tiled, compressed := map[uint32][]uint64{}, map[uint32][]uint64{}
	for _, format := range []uint32{gbm.FormatR8, gbm.FormatGR88} {
		mods, external := queryModifiers(b.plat.eglDpy, format)
		for i, m := range mods {
			if m == 0 || m == drmFormatModInvalid || external[i] {
				continue
			}
			var planes int
			p.run(func() { planes = dev.ModifierPlanes(format, m) })
			t.Logf("format %#x: modifier %#x, %d memory planes", format, m, planes)
			if planes == 1 {
				tiled[format] = append(tiled[format], m)
			} else if planes > 1 {
				compressed[format] = append(compressed[format], m)
			}
		}
	}
	for _, c := range []struct {
		name  string
		alloc allocator
	}{
		{"implicit layout", func(dev *gbm.Device, format uint32, w, h int) (*gbm.Buffer, error) {
			return dev.NewBuffer(format, w, h, false)
		}},
		{"linear", func(dev *gbm.Device, format uint32, w, h int) (*gbm.Buffer, error) {
			return dev.NewBufferModifiers(format, w, h, []uint64{0})
		}},
		{"tiled", func(dev *gbm.Device, format uint32, w, h int) (*gbm.Buffer, error) {
			return dev.NewBufferModifiers(format, w, h, tiled[format])
		}},
		{"compressed", func(dev *gbm.Device, format uint32, w, h int) (*gbm.Buffer, error) {
			return dev.NewBufferModifiers(format, w, h, compressed[format])
		}},
	} {
		if mods := map[string]map[uint32][]uint64{"tiled": tiled, "compressed": compressed}[c.name]; mods != nil &&
			(len(mods[gbm.FormatR8]) == 0 || len(mods[gbm.FormatGR88]) == 0) {
			t.Logf("%s: skipped, the display imports R8 and GR88 in no such layout", c.name)
			continue
		}
		checkImport(t, b, target, p, dev, c.name, c.alloc)
	}
}

// checkImport runs TestDMABufImport for one layout, on the test's thread.
func checkImport(t *testing.T, b *Backend, target *readbackTarget, p *producerThread, dev *gbm.Device,
	name string, alloc allocator) {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: []gui.PixelPlaneCfg{
		{Format: gui.PixelFormatR8, Width: 64, Height: 64},
		{Format: gui.PixelFormatRG8, Width: 32, Height: 32},
	}})
	if err != nil {
		t.Fatal(err)
	}
	first := newGBMFrame(t, p, dev, alloc, 128, 64, 192)
	t.Logf("%s: modifiers %#x, %#x; metadata planes %d, %d", name, first.planes[0].Modifier, first.planes[1].Modifier,
		len(first.planes[0].Metadata), len(first.planes[1].Metadata))
	if err = surface.UpdateDMABuf(first.planes[:], first); err != nil {
		t.Fatal(err)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{128, 64, 192, 255}) {
		t.Errorf("%s: first frame = %v, want [128 64 192 255]", name, got)
	}

	second := newGBMFrame(t, p, dev, alloc, 32, 200, 16)
	if err = surface.UpdateDMABuf(second.planes[:], second); err != nil {
		t.Fatal(err)
	}
	if got := channelReadback(b, target, surface); !nearPixel(got, [4]byte{32, 200, 16, 255}) {
		t.Errorf("%s: second frame = %v, want [32 200 16 255]", name, got)
	}
	// The draw retired the first frame behind a fence; the poll that
	// releases it comes with a later frame.
	if first.releases != 0 {
		t.Fatalf("%s: the first frame went back before its fence was polled", name)
	}
	deadline := time.Now().Add(5 * time.Second)
	for first.releases == 0 && time.Now().Before(deadline) {
		b.retired.poll()
		time.Sleep(time.Millisecond)
	}
	if first.releases != 1 {
		t.Fatalf("%s: first frame released %d times, want 1", name, first.releases)
	}

	// Closing the surface and pruning its channels hands back the frame
	// on screen at teardown, once.
	surface.Close()
	b.channels.PruneShaders(nil)
	b.retired.drain()
	if second.releases != 1 {
		t.Fatalf("%s: second frame released %d times, want 1", name, second.releases)
	}
}
