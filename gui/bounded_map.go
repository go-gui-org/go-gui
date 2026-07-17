package gui

const boundedOrderCompactMin = 64

// BoundedMap is a map with maximum size. When full, oldest entries
// are evicted (FIFO).
type BoundedMap[K comparable, V any] struct {
	data    map[K]V
	order   []K
	head    int
	maxSize int
}

// NewBoundedMap creates a BoundedMap with the given max size.
func NewBoundedMap[K comparable, V any](maxSize int) *BoundedMap[K, V] {
	dataCap := 0
	orderCap := 0
	if maxSize > 0 {
		dataCap = maxSize
		orderCap = maxSize
	}
	return &BoundedMap[K, V]{
		data:    make(map[K]V, dataCap),
		order:   make([]K, 0, orderCap),
		maxSize: maxSize,
	}
}

// cloneAny returns a deep copy of the map as an *BoundedMap[K, V]
// boxed in any. Used by time-travel snapshot to capture
// whitelisted StateMap namespaces without erasing the element
// types.
func (m *BoundedMap[K, V]) cloneAny() any {
	c := NewBoundedMap[K, V](m.maxSize)
	m.Range(func(k K, v V) bool {
		c.Set(k, v)
		return true
	})
	return c
}

// restoreAny overwrites the receiver with entries from src, which
// must be the *BoundedMap[K, V] previously returned by cloneAny.
// Existing entries are cleared first.
func (m *BoundedMap[K, V]) restoreAny(src any) {
	m.Clear()
	src.(*BoundedMap[K, V]).Range(func(k K, v V) bool {
		m.Set(k, v)
		return true
	})
}

// Set adds or updates a key-value pair. Evicts oldest if at capacity.
func (m *BoundedMap[K, V]) Set(key K, value V) {
	if _, exists := m.data[key]; exists {
		m.data[key] = value
		return
	}
	if m.maxSize > 0 {
		for len(m.data) >= m.maxSize && m.head < len(m.order) {
			oldestKey := m.order[m.head]
			m.head++
			if _, exists := m.data[oldestKey]; exists {
				delete(m.data, oldestKey)
				break
			}
		}
	}
	m.order = append(m.order, key)
	m.data[key] = value
	if m.maxSize > 0 {
		m.compactOrder()
	}
}

// Get returns value for key. Second return is false if not found.
func (m *BoundedMap[K, V]) Get(key K) (V, bool) {
	v, ok := m.data[key]
	return v, ok
}

// GetOr returns the value for key, or def if the key is absent.
// Use at call sites that would otherwise ignore Get's ok boolean, so
// the assumed default is explicit rather than an implicit zero value.
func (m *BoundedMap[K, V]) GetOr(key K, def V) V {
	if v, ok := m.data[key]; ok {
		return v
	}
	return def
}

// Delete removes key from map.
func (m *BoundedMap[K, V]) Delete(key K) {
	if _, exists := m.data[key]; !exists {
		return
	}
	delete(m.data, key)
	if len(m.data) == 0 {
		m.order = m.order[:0]
		m.head = 0
		return
	}
	m.compactOrder()
}

// Contains returns true if key exists.
func (m *BoundedMap[K, V]) Contains(key K) bool {
	_, ok := m.data[key]
	return ok
}

// Len returns number of entries.
func (m *BoundedMap[K, V]) Len() int {
	return len(m.data)
}

// Clear removes all entries.
func (m *BoundedMap[K, V]) Clear() {
	clear(m.data)
	m.order = m.order[:0]
	m.head = 0
}

// Keys returns all keys in insertion order.
func (m *BoundedMap[K, V]) Keys() []K {
	if len(m.data) == 0 || m.head >= len(m.order) {
		return nil
	}
	out := make([]K, 0, len(m.data))
	for _, k := range m.order[m.head:] {
		if _, exists := m.data[k]; exists {
			out = append(out, k)
		}
	}
	return out
}

// RangeKeys iterates active keys in insertion order.
// If fn returns false, iteration stops.
func (m *BoundedMap[K, V]) RangeKeys(fn func(K) bool) {
	if len(m.data) == 0 || m.head >= len(m.order) {
		return
	}
	for _, k := range m.order[m.head:] {
		if _, exists := m.data[k]; !exists {
			continue
		}
		if !fn(k) {
			return
		}
	}
}

// Range iterates keys in insertion order and calls fn for each
// active entry. If fn returns false, iteration stops.
func (m *BoundedMap[K, V]) Range(fn func(K, V) bool) {
	if len(m.data) == 0 || m.head >= len(m.order) {
		return
	}
	for _, k := range m.order[m.head:] {
		v, exists := m.data[k]
		if !exists {
			continue
		}
		if !fn(k, v) {
			return
		}
	}
}

// EvictToBudget evicts oldest entries until the sum of sizeFn across
// remaining entries plus newBytes is <= maxBytes. Returns the number
// of entries evicted. If maxBytes <= 0 the budget is unbounded and
// no entries are evicted.
func (m *BoundedMap[K, V]) EvictToBudget(maxBytes int, newBytes int, sizeFn func(V) int) int {
	if maxBytes <= 0 || len(m.data) == 0 {
		return 0
	}
	// Sum current entries + pending new bytes.
	total := newBytes
	for _, v := range m.data {
		total += sizeFn(v)
	}
	if total <= maxBytes {
		return 0
	}
	evicted := 0
	for total > maxBytes && m.head < len(m.order) {
		oldestKey := m.order[m.head]
		m.head++
		if v, exists := m.data[oldestKey]; exists {
			total -= sizeFn(v)
			delete(m.data, oldestKey)
			evicted++
		}
	}
	if evicted > 0 {
		m.compactOrder()
	}
	return evicted
}

func (m *BoundedMap[K, V]) compactOrder() {
	switch {
	case m.head >= boundedOrderCompactMin:
	case m.head > 0 && m.head*2 >= len(m.order):
	case len(m.order) >= boundedOrderCompactMin && len(m.order) > len(m.data)*2:
	default:
		return
	}
	dst := 0
	for _, k := range m.order[m.head:] {
		if _, exists := m.data[k]; exists {
			m.order[dst] = k
			dst++
		}
	}
	var zero K
	for i := dst; i < len(m.order); i++ {
		m.order[i] = zero
	}
	m.order = m.order[:dst]
	m.head = 0
}
