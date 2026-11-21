package surfacecache

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// countedFrame counts its releases.
type countedFrame struct{ releases int }

func (f *countedFrame) Release() { f.releases++ }

// frameBufs are DMA-BUF planes for newPlaneSurface: R8 2x2, RG8 1x1.
var frameBufs = []gui.DMABufPlane{{FD: 3, Pitch: 64}, {FD: 4, Pitch: 64}}

// importer mirrors an importing backend: its resource is the frame it
// holds, which it drops when it imports another, uploads planes, or is
// released. A real backend drops only after a fence; that is the GL
// backend's to test.
type importer struct {
	surfaces         Cache[held]
	imports, uploads int
	// pending collects the resources replaced during a resolve.
	pending []held
}

type held struct {
	surface *gui.PixelSurface
	frame   gui.GPUFrame
}

func newImporter() *importer {
	p := &importer{}
	p.surfaces.SetRelease(p.drop)
	return p
}

func (p *importer) drop(h held) {
	if h.frame != nil {
		h.surface.DropGPUFrame(h.frame)
	}
}

func (p *importer) importFrame(previous held, surface *gui.PixelSurface, planes []gui.SurfacePlane, bufs []gui.DMABufPlane, frame gui.GPUFrame) held {
	p.imports++
	if len(planes) != 2 || len(bufs) != 2 {
		panic("frame without its planes")
	}
	// Dropping inside the visit would deadlock on the surface lock; a
	// backend queues it. The fake keeps it until resolve returns.
	p.pending = append(p.pending, previous)
	return held{surface: surface, frame: frame}
}

func (p *importer) upload(previous held, _ []gui.SurfacePlane) held {
	p.uploads++
	p.pending = append(p.pending, previous)
	return held{}
}

func (p *importer) resolve(surface *gui.PixelSurface) bool {
	_, ok := p.surfaces.ResolveFrame(surface, p.upload, p.importFrame)
	for _, h := range p.pending {
		p.drop(h)
	}
	p.pending = p.pending[:0]
	return ok
}

func TestResolveFrameImportsAndUploads(t *testing.T) {
	surface := newPlaneSurface(t)
	p := newImporter()
	first := &countedFrame{}
	if err := surface.UpdateDMABuf(frameBufs, first); err != nil {
		t.Fatal(err)
	}
	// Not cached yet: the GPU frame alone makes an entry.
	if !p.resolve(surface) || p.imports != 1 || p.uploads != 0 || p.surfaces.Len() != 1 {
		t.Fatalf("imports %d, uploads %d, entries %d", p.imports, p.uploads, p.surfaces.Len())
	}
	p.resolve(surface)
	if p.imports != 1 {
		t.Fatal("an unchanged GPU frame was imported again")
	}
	second := &countedFrame{}
	if err := surface.UpdateDMABuf(frameBufs, second); err != nil {
		t.Fatal(err)
	}
	p.resolve(surface)
	if p.imports != 2 || first.releases != 1 {
		t.Fatalf("imports %d, first released %d times", p.imports, first.releases)
	}
	// Planes in main memory go to upload, and the import is dropped.
	if err := surface.UpdatePlanes(gui.PixelPlane{Pix: make([]byte, 4), Stride: 2}, gui.PixelPlane{Pix: make([]byte, 2), Stride: 2}); err != nil {
		t.Fatal(err)
	}
	p.resolve(surface)
	if p.uploads != 1 || second.releases != 1 {
		t.Fatalf("uploads %d, second released %d times", p.uploads, second.releases)
	}
	// Pruning an entry that holds a frame releases it through the cache.
	third := &countedFrame{}
	if err := surface.UpdateDMABuf(frameBufs, third); err != nil {
		t.Fatal(err)
	}
	p.resolve(surface)
	p.surfaces.PruneShaders(nil)
	if third.releases != 0 {
		t.Fatal("the surface's current frame was released")
	}
	surface.Close()
	if third.releases != 1 || p.surfaces.Len() != 0 {
		t.Fatalf("closed: released %d times, %d entries", third.releases, p.surfaces.Len())
	}
	if p.resolve(surface) {
		t.Fatal("closed surface resolved")
	}
}

// A renderer that does not import resolves a GPU frame as nothing to
// draw, and keeps no hold on it.
func TestResolvePlanesSkipsGPUFrames(t *testing.T) {
	surface := newPlaneSurface(t)
	p := &planeUploads{}
	frame := &countedFrame{}
	if err := surface.UpdateDMABuf(frameBufs, frame); err != nil {
		t.Fatal(err)
	}
	if _, ok := p.surfaces.ResolvePlanes(surface, p.upload); ok || p.uploads != 0 {
		t.Fatalf("GPU frame resolved as planes: ok %v, uploads %d", ok, p.uploads)
	}
	surface.Close()
	if frame.releases != 1 {
		t.Fatalf("released %d times, want 1", frame.releases)
	}
}

func TestWarmImportFrameAllocations(t *testing.T) {
	surface := newPlaneSurface(t)
	p := newImporter()
	commands := []gui.RenderCmd{{Kind: gui.RenderCustomShader, Shader: &gui.Shader{Surface: surface}}}
	frames := [2]*countedFrame{{}, {}}
	n := 0
	cycle := func() {
		if err := surface.UpdateDMABuf(frameBufs, frames[n%2]); err != nil {
			panic(err)
		}
		n++
		p.surfaces.PruneShaders(commands)
		p.resolve(surface)
	}
	cycle()
	cycle()
	if allocs := testing.AllocsPerRun(100, cycle); allocs != 0 {
		t.Errorf("warm import frame allocated %g objects", allocs)
	}
}
