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

// meshCacheIdleFrames is how many frames an unused mesh is kept.
const meshCacheIdleFrames = 120

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

// meshCache holds the meshes of one SVG draw path: GPU meshes for
// WebGL (draw_webgl.go), Path2D runs for Canvas2D (draw_path2d.go).
//
// A mesh is cached only once it is drawn in a second frame (admit). An
// animated SVG path is tessellated afresh every frame, into a new
// slice, so its meshes never repeat; cached on first sight, each frame
// would add one per path, uploaded and pinned until it idled out. Such
// meshes are drawn uncached instead, as the native backends draw every
// mesh. A static mesh pays one uncached draw, on its first frame.
type meshCache[E any] struct {
	entries map[meshKey]*cachedMesh[E]
	vars    meshVariants
	// seen holds meshes drawn uncached, by the frame they were last
	// drawn in. Entries older than the previous frame are dropped.
	seen  map[meshKey]uint64
	frame uint64
	// release frees an entry's resources when it leaves the cache.
	release func(E)
}

type cachedMesh[E any] struct {
	val      E
	tris     []float32 // pinned, see meshKey
	lastUsed uint64
}

func newMeshCache[E any](release func(E)) meshCache[E] {
	return meshCache[E]{
		entries: map[meshKey]*cachedMesh[E]{},
		vars:    meshVariants{},
		seen:    map[meshKey]uint64{},
		release: release,
	}
}

// get returns the cached mesh for k and marks it used this frame.
func (c *meshCache[E]) get(k meshKey) (E, bool) {
	if m, ok := c.entries[k]; ok {
		m.lastUsed = c.frame
		return m.val, true
	}
	var zero E
	return zero, false
}

// admit reports whether a mesh missing from the cache is to be cached:
// it was drawn uncached in an earlier frame. Otherwise it is noted as
// seen in this frame, and the caller draws it uncached.
func (c *meshCache[E]) admit(k meshKey) bool {
	if f, ok := c.seen[k]; ok && f < c.frame {
		delete(c.seen, k)
		return true
	}
	c.seen[k] = c.frame
	return false
}

// put caches v for k, pinning tris, and drops the least recently used
// variants of k's array beyond maxMeshVariants. A variant used this
// frame is kept: a queued draw may still refer to it.
func (c *meshCache[E]) put(k meshKey, tris []float32, v E) {
	c.entries[k] = &cachedMesh[E]{val: v, tris: tris, lastUsed: c.frame}
	c.vars.add(k)
	list := c.vars[k.base()]
	if len(list) <= maxMeshVariants {
		return
	}
	cands := slices.Clone(list)
	slices.SortFunc(cands, func(a, b meshKey) int {
		la, lb := c.entries[a].lastUsed, c.entries[b].lastUsed
		switch {
		case la < lb:
			return -1
		case la > lb:
			return 1
		}
		return 0
	})
	excess := len(list) - maxMeshVariants
	for _, ck := range cands {
		if excess == 0 {
			break
		}
		if c.entries[ck].lastUsed < c.frame {
			c.drop(ck)
			excess--
		}
	}
}

// drop removes k from the cache and releases its resources.
func (c *meshCache[E]) drop(k meshKey) {
	m := c.entries[k]
	delete(c.entries, k)
	c.vars.remove(k)
	if c.release != nil {
		c.release(m.val)
	}
}

// endFrame advances the frame, forgets meshes seen uncached before the
// previous frame and, every 60 frames, drops the meshes not drawn for
// meshCacheIdleFrames.
func (c *meshCache[E]) endFrame() {
	c.frame++
	for k, f := range c.seen {
		if f+1 < c.frame {
			delete(c.seen, k)
		}
	}
	if c.frame%60 != 0 {
		return
	}
	for k, m := range c.entries {
		if c.frame-m.lastUsed > meshCacheIdleFrames {
			c.drop(k)
		}
	}
}

// reset forgets every mesh without releasing it, after the resources
// went elsewhere (a lost WebGL context).
func (c *meshCache[E]) reset() {
	clear(c.entries)
	clear(c.vars)
	clear(c.seen)
}
