// Package texcache provides a generic bounded LRU cache used
// by all GPU backends for texture and path caching.
package texcache

import "container/list"

// Default texture byte budgets per window (#1239). A backend uses one when
// WindowCfg.MaxTextureBytes is zero or negative. Mobile gets less: there the
// GPU shares memory with the CPU, and the OS kills an app that uses too much.
const (
	DesktopTextureBudget int64 = 256 << 20 // 256 MiB
	MobileTextureBudget  int64 = 128 << 20 // 128 MiB
)

// Budget returns cfg when it is positive, else def. It applies the
// "zero or negative selects the default" rule of WindowCfg.MaxTextureBytes.
func Budget(cfg, def int64) int64 {
	if cfg > 0 {
		return cfg
	}
	return def
}

// mapEntry pairs a cached value with its position in the LRU
// list for O(1) promote and eviction.
type mapEntry[V any] struct {
	val  V
	elem *list.Element // list element stores the key (K)
	// size is the value's byte size; 0 in a cache made by New.
	size int64
	// frame is the cache frame of the last Get or Set; only a sized
	// cache reads it.
	frame uint64
}

// Cache is a bounded LRU cache. On eviction the destroy function
// (if non-nil) is called to release the value's resources.
//
// A cache made by New bounds the entry count only. A cache made by
// NewSized also bounds the total byte size, and never evicts an entry
// used in the current or the previous frame (see NewSized).
type Cache[K comparable, V any] struct {
	data    map[K]mapEntry[V]
	destroy func(V)
	// sizeOf is nil in a cache made by New; its presence is what makes
	// the cache sized.
	sizeOf   func(V) int64
	order    list.List
	maxSize  int
	maxBytes int64 // 0 = no byte bound
	bytes    int64 // sum of every entry's size
	frame    uint64
}

// New returns a cache that holds at most maxSize entries.
// destroy is called on evicted or cleared values; nil skips cleanup.
// A non-positive maxSize is clamped to 1 so eviction stays live:
// without this, Set with maxSize<=0 never evicts and grows
// without bound.
func New[K comparable, V any](
	maxSize int, destroy func(V),
) Cache[K, V] {
	if maxSize <= 0 {
		maxSize = 1
	}
	return Cache[K, V]{
		data:    make(map[K]mapEntry[V], maxSize),
		maxSize: maxSize,
		destroy: destroy,
	}
}

// NewSized returns a cache bounded by both maxSize entries and maxBytes
// total bytes, where size gives one value's bytes (#1239). A non-positive
// maxBytes leaves only the count bound. size must not be nil.
//
// The bounds are soft for entries in use. An entry that a Get or Set touched
// in the current frame or the previous one is never evicted, so the cache
// can go over its bounds by the size of what is drawn. This is on purpose:
// when one frame draws more than the budget, a strict LRU evicts each
// texture just before the frame draws it again, and every frame uploads
// every texture. Keeping the in-use set costs no more memory than the frame
// needs anyway. Entries that nothing drew in the last two frames are evicted,
// oldest first, until the cache is within both bounds again.
//
// The caller marks frames with BeginFrame. A pure function, not a closure,
// for size keeps Set free of allocations.
func NewSized[K comparable, V any](
	maxSize int, maxBytes int64, size func(V) int64, destroy func(V),
) Cache[K, V] {
	c := New[K, V](maxSize, destroy)
	c.sizeOf = size
	if maxBytes > 0 {
		c.maxBytes = maxBytes
	}
	return c
}

// BeginFrame starts a new frame. Entries that the frame before last used
// stop being protected from eviction, and a sized cache that is over a
// bound evicts them now. Without this, the textures of an over-budget frame
// stay on the GPU after they leave the screen until a later Set, which a view
// with no new images never makes. Entries used in the previous frame stay
// protected, so this cannot delete what the new frame draws again. It does
// nothing visible on a cache made by New.
func (c *Cache[K, V]) BeginFrame() {
	c.frame++
	if c.sizeOf != nil {
		c.trim()
	}
}

// Bytes returns the total byte size of the cached entries; 0 for a cache
// made by New.
func (c *Cache[K, V]) Bytes() int64 {
	return c.bytes
}

// Get returns the cached value and true, or the zero value and
// false. A hit promotes the entry to most-recently-used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	me, ok := c.data[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToBack(me.elem)
	if c.sizeOf != nil && me.frame != c.frame {
		// Mark the entry in use. Only a sized cache pays for the map
		// write, and only once per entry per frame.
		me.frame = c.frame
		c.data[key] = me
	}
	return me.val, true
}

// Len returns the number of cached entries.
func (c *Cache[K, V]) Len() int {
	return len(c.data)
}

// EvictOldest removes the least-recently-used entry.
// It returns false if the cache is empty.
func (c *Cache[K, V]) EvictOldest() bool {
	if c.order.Len() == 0 {
		return false
	}
	c.remove(c.order.Front(), true)
	return true
}

// Remove destroys and removes key. It returns false if key is not cached.
func (c *Cache[K, V]) Remove(key K) bool {
	me, ok := c.data[key]
	if !ok {
		return false
	}
	c.remove(me.elem, true)
	return true
}

// remove unlinks the entry e names, subtracts its size and, when destroy
// is true, destroys its value.
func (c *Cache[K, V]) remove(e *list.Element, destroy bool) {
	key := e.Value.(K)
	me := c.data[key]
	c.order.Remove(e)
	delete(c.data, key)
	c.bytes -= me.size
	if destroy && c.destroy != nil {
		c.destroy(me.val)
	}
}

// Set inserts or updates key. If the cache is full the
// least-recently-used entry is evicted first. In a sized cache the
// eviction runs after the insert and skips entries in use (see NewSized).
func (c *Cache[K, V]) Set(key K, val V) {
	if c.maxSize <= 0 {
		c.maxSize = 1
	}
	if c.data == nil {
		c.data = make(map[K]mapEntry[V], c.maxSize)
	}
	var size int64
	if c.sizeOf != nil {
		size = c.sizeOf(val)
	}
	if me, ok := c.data[key]; ok {
		if c.destroy != nil {
			c.destroy(me.val)
		}
		// A replaced value can change size: a web image is cached at 0
		// while it loads and again at its real size once it has.
		c.bytes += size - me.size
		me.val = val
		me.size = size
		me.frame = c.frame
		c.data[key] = me
		c.order.MoveToBack(me.elem)
		if c.sizeOf != nil {
			c.trim()
		}
		return
	}
	if c.sizeOf == nil && c.order.Len() >= c.maxSize {
		c.EvictOldest()
	}
	elem := c.order.PushBack(key)
	c.data[key] = mapEntry[V]{val: val, elem: elem, size: size, frame: c.frame}
	c.bytes += size
	if c.sizeOf != nil {
		c.trim()
	}
}

// trim evicts the oldest entries of a sized cache until it is within both
// bounds, stopping at the first entry in use. The LRU order is also the
// order of last use, so every entry after an in-use one is in use too.
func (c *Cache[K, V]) trim() {
	for c.order.Len() > c.maxSize ||
		(c.maxBytes > 0 && c.bytes > c.maxBytes) {
		front := c.order.Front()
		// frame+1 >= c.frame: used this frame or the one before. Written
		// as an addition so frame 0 entries do not underflow.
		if c.data[front.Value.(K)].frame+1 >= c.frame {
			return
		}
		c.remove(front, true)
	}
}

// DestroyAll calls destroy on every cached value and resets
// the cache to empty.
func (c *Cache[K, V]) DestroyAll() {
	if c.destroy != nil {
		for _, me := range c.data {
			c.destroy(me.val)
		}
	}
	c.DiscardAll(nil)
}

// Sweep destroys and removes every entry keep rejects, leaving the rest in
// their LRU order. A backend calls it to release textures whose source is
// gone, such as a dropped in-memory image (#1230). Pass a plain function
// rather than a closure when it runs per frame: a closure would allocate.
func (c *Cache[K, V]) Sweep(keep func(K, V) bool) {
	for e := c.order.Front(); e != nil; {
		// Take next before Remove: Remove clears e's links.
		next := e.Next()
		key := e.Value.(K)
		if !keep(key, c.data[key].val) {
			c.remove(e, true)
		}
		e = next
	}
}

// DiscardAll removes entries without calling destroy. Use it when the
// values are already gone with the context that held them (a lost GL
// context): destroying them would delete names that may now belong to
// live objects.
//
// keep, when non-nil, names the entries to leave in place, in their LRU
// order. A backend keeps the entries that hold no GPU object, such as a
// cached load failure, so a context loss does not retry them. A nil keep
// empties the cache.
func (c *Cache[K, V]) DiscardAll(keep func(V) bool) {
	if keep == nil {
		c.data = make(map[K]mapEntry[V], c.maxSize)
		c.order.Init()
		c.bytes = 0
		return
	}
	for e := c.order.Front(); e != nil; {
		// Take next before Remove: Remove clears e's links.
		next := e.Next()
		if !keep(c.data[e.Value.(K)].val) {
			c.remove(e, false)
		}
		e = next
	}
}
