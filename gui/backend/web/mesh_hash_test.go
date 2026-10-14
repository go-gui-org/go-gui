//go:build js && wasm

package web

import (
	"hash/maphash"
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

func testMesh(nTris int) *gui.RenderCmd {
	r := &gui.RenderCmd{Triangles: make([]float32, nTris*6)}
	for i := range r.Triangles {
		r.Triangles[i] = float32(i%97) * 0.5
	}
	r.VertexColors = make([]gui.Color, nTris*3)
	for i := range r.VertexColors {
		r.VertexColors[i] = gui.RGBA(uint8(i), uint8(i>>8), 7, 255)
	}
	return r
}

// The caches rely on meshHash to notice a buffer rewritten in place:
// any change to one vertex coordinate or one color must move it.
func TestMeshHashDetectsSingleChanges(t *testing.T) {
	t.Parallel()
	for _, nTris := range []int{1, 2, 5, 37} { // 37: lanes plus tails
		r := testMesh(nTris)
		base := meshHash(r)
		if again := meshHash(r); again != base {
			t.Fatalf("%d tris: hash not deterministic", nTris)
		}
		for i := range r.Triangles {
			old := r.Triangles[i]
			r.Triangles[i] = old + 0.25
			if meshHash(r) == base {
				t.Errorf("%d tris: change at float %d not detected", nTris, i)
			}
			r.Triangles[i] = old
		}
		for i := range r.VertexColors {
			old := r.VertexColors[i]
			r.VertexColors[i].A ^= 1
			if meshHash(r) == base {
				t.Errorf("%d tris: change at color %d not detected", nTris, i)
			}
			r.VertexColors[i] = old
		}
		noCols := *r
		noCols.VertexColors = nil
		if meshHash(&noCols) == base {
			t.Errorf("%d tris: dropping vertex colors not detected", nTris)
		}
	}
}

// maphashMesh is the hash meshHash replaced, kept for the benchmark.
func maphashMesh(seed maphash.Seed, r *gui.RenderCmd) uint64 {
	var h maphash.Hash
	h.SetSeed(seed)
	_, _ = h.Write(unsafe.Slice((*byte)(unsafe.Pointer(&r.Triangles[0])),
		len(r.Triangles)*4))
	_, _ = h.Write(unsafe.Slice((*byte)(unsafe.Pointer(&r.VertexColors[0])),
		len(r.VertexColors)*int(unsafe.Sizeof(r.VertexColors[0]))))
	return h.Sum64()
}

// About the tiger: ~600 triangles per path.
func BenchmarkMeshHash(b *testing.B) {
	r := testMesh(600)
	bytes := int64(len(r.Triangles)*4 + len(r.VertexColors)*5)
	b.Run("maphash", func(b *testing.B) {
		seed := maphash.MakeSeed()
		b.SetBytes(bytes)
		for range b.N {
			_ = maphashMesh(seed, r)
		}
	})
	b.Run("words", func(b *testing.B) {
		b.SetBytes(bytes)
		for range b.N {
			_ = meshHash(r)
		}
	})
}
