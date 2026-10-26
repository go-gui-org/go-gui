// Package surfacecache holds a renderer's native resources for
// gui.PixelSurface sources: one texture (or canvas, or CPU image) per surface
// it draws, uploaded again only when the surface publishes a new frame.
package surfacecache

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// maxSurfaces bounds the cache like every other texcache. It is far above
// any real frame: Prune already limits the cache to the surfaces the
// current frame draws, so LRU eviction only guards against a pathological
// frame.
const maxSurfaces = 1024

// entry is one surface's resource and the frame version it holds.
type entry[T any] struct {
	surface  *gui.PixelSurface
	resource T
	version  uint64
	frame    uint64 // last frame whose commands drew the surface
}

// Cache is confined to the renderer's thread. Its zero value is ready to
// use. Do not copy a Cache after first use.
type Cache[T any] struct {
	entries texcache.Cache[*gui.PixelSurface, *entry[T]]
	ready   bool // entries was built with maxSurfaces and the evict hook
	frame   uint64
	// release frees a resource; nil for resources the garbage collector
	// owns (CPU images, browser canvases). Bound once by SetRelease and
	// never passed per call: a method value stored from a per-frame call
	// would be allocated on every frame.
	release func(T)
}

// SetRelease binds the function that frees a native resource. Call it once,
// when the renderer is set up. A cache without one only forgets resources.
func (c *Cache[T]) SetRelease(release func(T)) {
	c.release = release
}

// init builds the texcache on first use. A zero-value texcache holds one
// entry, so two surfaces would evict each other every frame.
func (c *Cache[T]) init() {
	if !c.ready {
		c.entries = texcache.New[*gui.PixelSurface, *entry[T]](maxSurfaces, c.evicted)
		c.ready = true
	}
}

// evicted releases the resource of an entry texcache evicts.
func (c *Cache[T]) evicted(e *entry[T]) {
	c.free(e)
}

// free releases one entry's resource.
func (c *Cache[T]) free(e *entry[T]) {
	if c.release != nil {
		c.release(e.resource)
	}
}

// Prune releases the resources of surfaces this frame does not draw or
// whose producer closed them. Call it before drawing, on the thread that
// owns the native resources.
func (c *Cache[T]) Prune(cmds []gui.RenderCmd) {
	if c.entries.Len() == 0 {
		return
	}
	c.frame++
	for i := range cmds {
		if cmds[i].Kind != gui.RenderImage || cmds[i].Surface == nil {
			continue
		}
		if e, ok := c.entries.Get(cmds[i].Surface); ok {
			e.frame = c.frame
		}
	}
	c.entries.DiscardAll(c.keepDrawn)
}

// keepDrawn keeps an entry drawn this frame by an open surface and
// releases any other.
func (c *Cache[T]) keepDrawn(e *entry[T]) bool {
	if e.frame == c.frame && !e.surface.Closed() {
		return true
	}
	c.free(e)
	return false
}

// Resolve returns the surface's resource, uploading the current frame first
// when the surface published one since the last upload. upload receives
// the previous resource (the zero value on first use) and must reuse it
// when the dimensions match and release it on resize. ok is false for a
// closed surface, which draws nothing.
func (c *Cache[T]) Resolve(surface *gui.PixelSurface, upload func(T, int, int, []byte) T) (T, bool) {
	e, cached := c.entries.Get(surface)
	if !cached {
		e = &entry[T]{surface: surface, frame: c.frame}
	}
	ok := surface.ReadPixels(e.version, func(w, h int, pix []byte, version uint64) {
		e.resource = upload(e.resource, w, h, pix)
		e.version = version
	})
	if ok && !cached {
		c.init()
		c.entries.Set(surface, e)
	}
	return e.resource, ok
}

// Clear releases every resource, at renderer teardown.
func (c *Cache[T]) Clear() {
	c.entries.DiscardAll(c.releaseAll)
}

// Forget drops every resource without releasing it, after a context loss:
// the old names are invalid and must not be deleted in the new context,
// where they may belong to live objects. The next draw uploads again from
// each surface's CPU copy.
func (c *Cache[T]) Forget() {
	c.entries.DiscardAll(nil)
}

// releaseAll releases an entry's resource and drops it.
func (c *Cache[T]) releaseAll(e *entry[T]) bool {
	c.free(e)
	return false
}

// Len reports the number of cached surfaces.
func (c *Cache[T]) Len() int {
	return c.entries.Len()
}
