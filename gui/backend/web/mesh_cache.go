//go:build js && wasm

package web

import (
	"slices"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// meshKey identifies one variant of a mesh in the WebGL and Path2D
// caches: its backing array and its content hash (triangles and vertex
// colors). Commands that share triangles but differ in colors (a tint,
// an opacity, a disabled state) are separate variants and coexist, so
// one never replaces another that is still queued for drawing. A
// buffer rewritten in place (DrawCanvas reuses its buffers) becomes a
// new variant; the old one goes when its array holds too many variants
// or when it has idled.
//
// The entry pins the triangle slice, so the address cannot be reused
// by another allocation while the entry lives.
type meshKey struct {
	ptr  uintptr
	n    int
	hash uint64
}

// meshBase is a meshKey without its hash: the backing array.
type meshBase struct {
	ptr uintptr
	n   int
}

func (k meshKey) base() meshBase { return meshBase{k.ptr, k.n} }

// meshKeyFor returns r's cache key. It hashes r every call: the hash is
// what notices a buffer rewritten in place.
func meshKeyFor(r *gui.RenderCmd) meshKey {
	return meshKey{
		ptr:  uintptr(unsafe.Pointer(&r.Triangles[0])),
		n:    len(r.Triangles),
		hash: meshHash(r), // mesh_hash.go
	}
}

// maxMeshVariants caps the variants one backing array keeps. A mesh
// rewritten every frame would otherwise hold a GPU buffer per frame
// until it idles out.
const maxMeshVariants = 8

// meshVariants lists the cache keys per backing array.
type meshVariants map[meshBase][]meshKey

func (v meshVariants) add(k meshKey) {
	v[k.base()] = append(v[k.base()], k)
}

func (v meshVariants) remove(k meshKey) {
	b := k.base()
	list := slices.DeleteFunc(v[b], func(x meshKey) bool { return x == k })
	if len(list) == 0 {
		delete(v, b)
		return
	}
	v[b] = list
}

// trimMeshVariants drops the least recently used variants of base
// beyond maxMeshVariants. A variant used in frame is kept: a queued
// draw may still refer to it.
func trimMeshVariants[E any](
	entries map[meshKey]E, vars meshVariants, base meshBase, frame uint64,
	lastUsed func(E) uint64, drop func(meshKey, E),
) {
	list := vars[base]
	if len(list) <= maxMeshVariants {
		return
	}
	cands := slices.Clone(list)
	slices.SortFunc(cands, func(a, b meshKey) int {
		la, lb := lastUsed(entries[a]), lastUsed(entries[b])
		switch {
		case la < lb:
			return -1
		case la > lb:
			return 1
		}
		return 0
	})
	excess := len(list) - maxMeshVariants
	for _, k := range cands {
		if excess == 0 {
			break
		}
		if e := entries[k]; lastUsed(e) < frame {
			drop(k, e)
			excess--
		}
	}
}
