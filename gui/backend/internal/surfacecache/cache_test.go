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
