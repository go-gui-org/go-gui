package surfacecache

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newSurface(t *testing.T) *gui.PixelSurface {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Width: 1, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	return surface
}

// fakeBackend mirrors how a backend uses the cache: the cache is a field of
// a heap-allocated backend, and upload and release are bound method values.
type fakeBackend struct {
	surfaces       Cache[int]
	uploads, frees int
}

func newFakeBackend() *fakeBackend {
	b := &fakeBackend{}
	b.surfaces.SetRelease(b.release)
	return b
}

func (b *fakeBackend) upload(existing, _, _ int, _ []byte) int {
	b.uploads++
	return existing + 1
}

func (b *fakeBackend) release(int) { b.frees++ }

func TestIndependentConsumersAndResourceLifetime(t *testing.T) {
	surface := newSurface(t)
	first, second := newFakeBackend(), newFakeBackend()
	commands := []gui.RenderCmd{{Kind: gui.RenderImage, Surface: surface}}
	first.surfaces.Resolve(surface, first.upload)
	second.surfaces.Resolve(surface, second.upload)
	first.surfaces.Resolve(surface, first.upload)
	if first.uploads != 1 || second.uploads != 1 {
		t.Fatal("unchanged frame uploaded again")
	}
	for range 3 {
		if err := surface.Update([]byte{255, 0, 0, 255}, 4); err != nil {
			t.Fatal(err)
		}
	}
	first.surfaces.Resolve(surface, first.upload)
	second.surfaces.Resolve(surface, second.upload)
	if first.uploads != 2 || second.uploads != 2 || first.surfaces.Len() != 1 || second.surfaces.Len() != 1 {
		t.Fatal("pending frames accumulated or a consumer missed an update")
	}
	first.surfaces.Prune(commands)
	if first.frees != 0 || first.surfaces.Len() != 1 {
		t.Fatal("drawn resource released")
	}
	first.surfaces.Prune(nil)
	if first.frees != 1 || first.surfaces.Len() != 0 {
		t.Fatal("removed view kept its resource")
	}
	// Forgetting lost-context handles must not release them, and the next
	// resolve must upload again although the producer did not change.
	second.surfaces.Forget()
	second.surfaces.Resolve(surface, second.upload)
	if second.frees != 0 || second.uploads != 3 {
		t.Fatal("context-loss recovery kept a stale upload")
	}
	surface.Close()
	second.surfaces.Prune(commands)
	if second.frees != 1 || second.surfaces.Len() != 0 {
		t.Fatal("closed surface kept its resource")
	}
	if _, ok := second.surfaces.Resolve(surface, second.upload); ok {
		t.Fatal("closed surface resolved")
	}
}

func TestManySurfacesStayCached(t *testing.T) {
	// A zero-value texcache holds one entry; the cache must not let two
	// surfaces drawn in the same frame evict each other.
	b := newFakeBackend()
	surfaces := []*gui.PixelSurface{newSurface(t), newSurface(t), newSurface(t)}
	var commands []gui.RenderCmd
	for _, s := range surfaces {
		commands = append(commands, gui.RenderCmd{Kind: gui.RenderImage, Surface: s})
	}
	for range 3 {
		b.surfaces.Prune(commands)
		for _, s := range surfaces {
			b.surfaces.Resolve(s, b.upload)
		}
	}
	if b.uploads != len(surfaces) || b.surfaces.Len() != len(surfaces) || b.frees != 0 {
		t.Fatalf("uploads = %d, cached = %d, frees = %d; want %d, %d, 0",
			b.uploads, b.surfaces.Len(), b.frees, len(surfaces), len(surfaces))
	}
}

func TestClearReleasesEverything(t *testing.T) {
	b := newFakeBackend()
	b.surfaces.Resolve(newSurface(t), b.upload)
	b.surfaces.Resolve(newSurface(t), b.upload)
	b.surfaces.Clear()
	if b.frees != 2 || b.surfaces.Len() != 0 {
		t.Fatalf("released %d, %d left; want 2 released, none left", b.frees, b.surfaces.Len())
	}
}

func TestWithoutReleaseResourcesAreOnlyForgotten(t *testing.T) {
	// Soft and web resources belong to the garbage collector.
	var cache Cache[int]
	upload := func(existing, _, _ int, _ []byte) int { return existing + 1 }
	cache.Resolve(newSurface(t), upload)
	cache.Prune(nil)
	cache.Resolve(newSurface(t), upload)
	cache.Clear()
	if cache.Len() != 0 {
		t.Fatalf("%d left after prune and clear", cache.Len())
	}
}

func TestWarmFrameAllocations(t *testing.T) {
	// The backend shape matters: a release function stored from a
	// per-frame call would move the bound method value to the heap on
	// every frame (review of #1168).
	surface := newSurface(t)
	b := newFakeBackend()
	commands := []gui.RenderCmd{{Kind: gui.RenderImage, Surface: surface}}
	pix := []byte{0, 0, 0, 255}
	b.surfaces.Resolve(surface, b.upload)
	allocs := testing.AllocsPerRun(100, func() {
		if err := surface.Update(pix, 4); err != nil {
			panic(err)
		}
		b.surfaces.Prune(commands)
		b.surfaces.Resolve(surface, b.upload)
	})
	if allocs != 0 {
		t.Errorf("warm publish, prune and resolve allocated %g objects", allocs)
	}
}

func TestPruneOnEmptyCacheAllocatesNothing(t *testing.T) {
	// Every frame of every app runs Prune; without surfaces it must cost
	// nothing.
	b := newFakeBackend()
	commands := make([]gui.RenderCmd, 64)
	allocs := testing.AllocsPerRun(100, func() { b.surfaces.Prune(commands) })
	if allocs != 0 {
		t.Errorf("empty prune allocated %g objects", allocs)
	}
}

func newPlaneSurface(t *testing.T) *gui.PixelSurface {
	t.Helper()
	surface, err := gui.NewPixelSurface(gui.PixelSurfaceCfg{Planes: []gui.PixelPlaneCfg{
		{Format: gui.PixelFormatR8, Width: 2, Height: 2},
		{Format: gui.PixelFormatRG8, Width: 1, Height: 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(surface.Close)
	return surface
}

// planeUploads counts plane uploads the way a backend's shader path would.
type planeUploads struct {
	surfaces Cache[int]
	uploads  int
	planes   int
}

func (p *planeUploads) upload(existing int, planes []gui.SurfacePlane) int {
	p.uploads++
	p.planes = len(planes)
	return existing + 1
}

func TestResolvePlanesUploadsEachFrameOnce(t *testing.T) {
	surface := newPlaneSurface(t)
	p := &planeUploads{}
	p.surfaces.ResolvePlanes(surface, p.upload)
	p.surfaces.ResolvePlanes(surface, p.upload)
	if p.uploads != 1 || p.planes != 2 {
		t.Fatalf("uploads = %d with %d planes; want 1 with 2", p.uploads, p.planes)
	}
	if err := surface.UpdatePlanes(gui.PixelPlane{Pix: make([]byte, 4), Stride: 2}, gui.PixelPlane{Pix: make([]byte, 2), Stride: 2}); err != nil {
		t.Fatal(err)
	}
	p.surfaces.ResolvePlanes(surface, p.upload)
	if p.uploads != 2 {
		t.Fatalf("new frame: uploads = %d, want 2", p.uploads)
	}
}

func TestPruneCountsOnlyItsKindOfCommand(t *testing.T) {
	// One RGBA8 surface in both caches: an image draws it through
	// Resolve, a shader through ResolvePlanes. Each cache must count only
	// its own kind of command for that same surface.
	surface := newSurface(t)
	image, shader := newFakeBackend(), newFakeBackend()
	upload := func(existing int, _ []gui.SurfacePlane) int { return existing + 1 }
	image.surfaces.Resolve(surface, image.upload)
	shader.surfaces.ResolvePlanes(surface, upload)
	if image.surfaces.Len() != 1 || shader.surfaces.Len() != 1 {
		t.Fatal("setup: each cache should hold the surface")
	}
	// A frame that draws the surface only through a shader keeps the
	// shader resource and releases the image one, and the reverse.
	shaderFrame := []gui.RenderCmd{{Kind: gui.RenderCustomShader, Shader: &gui.Shader{Surface: surface}}}
	shader.surfaces.PruneShaders(shaderFrame)
	image.surfaces.Prune(shaderFrame)
	if shader.surfaces.Len() != 1 || shader.frees != 0 {
		t.Error("shader cache released a surface its shader still reads")
	}
	if image.surfaces.Len() != 0 || image.frees != 1 {
		t.Error("image cache kept a surface no image draws")
	}
	shader.surfaces.PruneShaders([]gui.RenderCmd{{Kind: gui.RenderImage, Surface: surface}})
	if shader.surfaces.Len() != 0 || shader.frees != 1 {
		t.Error("shader cache kept a surface only an image draws")
	}
}

func TestWarmShaderFrameAllocations(t *testing.T) {
	surface := newPlaneSurface(t)
	p := &planeUploads{}
	p.surfaces.SetRelease(func(int) {})
	commands := []gui.RenderCmd{{Kind: gui.RenderCustomShader, Shader: &gui.Shader{Surface: surface}}}
	luma, chroma := make([]byte, 4), make([]byte, 2)
	p.surfaces.ResolvePlanes(surface, p.upload)
	allocs := testing.AllocsPerRun(100, func() {
		if err := surface.UpdatePlanes(gui.PixelPlane{Pix: luma, Stride: 2}, gui.PixelPlane{Pix: chroma, Stride: 2}); err != nil {
			panic(err)
		}
		p.surfaces.PruneShaders(commands)
		p.surfaces.ResolvePlanes(surface, p.upload)
	})
	if allocs != 0 {
		t.Errorf("warm shader frame allocated %g objects", allocs)
	}
}

func TestRefusedSurfacesAllocateNothing(t *testing.T) {
	// An Image given a surface it cannot draw (here two planes), or a
	// shader whose surface is closed, stays on screen frame after frame:
	// resolving it must not allocate each time.
	planes := newPlaneSurface(t)
	closed := newSurface(t)
	closed.Close()
	b := newFakeBackend()
	p := &planeUploads{}
	allocs := testing.AllocsPerRun(100, func() {
		if _, ok := b.surfaces.Resolve(planes, b.upload); ok {
			panic("a two-plane surface resolved on the image path")
		}
		if _, ok := p.surfaces.ResolvePlanes(closed, p.upload); ok {
			panic("a closed surface resolved")
		}
	})
	if allocs != 0 {
		t.Errorf("refused surfaces allocated %g objects per frame", allocs)
	}
	if b.surfaces.Len() != 0 || p.surfaces.Len() != 0 {
		t.Error("a refused surface was cached")
	}
}
