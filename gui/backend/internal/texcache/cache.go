// Package texcache provides a generic bounded LRU cache used
// by all GPU backends for texture and path caching.
package texcache

import "container/list"

// mapEntry pairs a cached value with its position in the LRU
// list for O(1) promote and eviction.
type mapEntry[V any] struct {
	val  V
	elem *list.Element // list element stores the key (K)
}

// Cache is a bounded LRU cache. On eviction the destroy function
// (if non-nil) is called to release the value's resources.
type Cache[K comparable, V any] struct {
	data    map[K]mapEntry[V]
	destroy func(V)
	order   list.List
	maxSize int
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

// Get returns the cached value and true, or the zero value and
// false. A hit promotes the entry to most-recently-used.
func (c *Cache[K, V]) Get(key K) (V, bool) {
	me, ok := c.data[key]
	if !ok {
		var zero V
		return zero, false
	}
	c.order.MoveToBack(me.elem)
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
	front := c.order.Front()
	evictKey := front.Value.(K)
	c.order.Remove(front)
	if c.destroy != nil {
		c.destroy(c.data[evictKey].val)
	}
	delete(c.data, evictKey)
	return true
}

// Set inserts or updates key. If the cache is full the
// least-recently-used entry is evicted first.
func (c *Cache[K, V]) Set(key K, val V) {
	if c.maxSize <= 0 {
		c.maxSize = 1
	}
	if c.data == nil {
		c.data = make(map[K]mapEntry[V], c.maxSize)
	}
	if me, ok := c.data[key]; ok {
		if c.destroy != nil {
			c.destroy(me.val)
		}
		me.val = val
		c.data[key] = me
		c.order.MoveToBack(me.elem)
		return
	}
	if c.order.Len() >= c.maxSize {
		c.EvictOldest()
	}
	elem := c.order.PushBack(key)
	c.data[key] = mapEntry[V]{val: val, elem: elem}
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
		return
	}
	for e := c.order.Front(); e != nil; {
		// Take next before Remove: Remove clears e's links.
		next := e.Next()
		key := e.Value.(K)
		if !keep(c.data[key].val) {
			c.order.Remove(e)
			delete(c.data, key)
		}
		e = next
	}
}
