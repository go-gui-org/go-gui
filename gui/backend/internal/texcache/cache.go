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
func New[K comparable, V any](
	maxSize int, destroy func(V),
) Cache[K, V] {
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
	c.data = make(map[K]mapEntry[V], c.maxSize)
	c.order.Init()
}
