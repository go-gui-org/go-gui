package gui

import "fmt"

const boundedOrderCompactMin = 64

// BoundedMap is a map with maximum size.
//
// When Set adds a key to a full map, one entry goes:
//
//   - A map from StateMap, or a hot state map (ScrollX, ScrollY and the
//     overflow and hover maps), removes the oldest inserted entry that was
//     not used in the current view pass. Get, GetOr and Set stamp an entry
//     with the window's view pass, so state that a widget on screen reads
//     every frame is not the target while an unused entry exists. When
//     every entry was used in this pass, the oldest inserted goes (#1263).
//   - A map from NewBoundedMap has no view pass and removes the oldest
//     inserted entry (FIFO).
//
// Keys, Range and time-travel snapshots use insertion order in both cases.
// evictToBudget always removes the oldest inserted entries first.
type BoundedMap[K comparable, V any] struct {
	data  map[K]boundedEntry[V]
	order []K
	head  int
	// pass points at the window's view-pass counter (Window.viewPass).
	// Nil selects FIFO eviction and no stamps are read.
	pass *uint64
	// usedTo caches the scan of evictUnused within one view pass: every
	// live entry in order[head:usedTo] was used in pass usedPass. Stamps
	// only rise, so that stays true for the rest of the pass, and the next
	// eviction starts at usedTo. Without the cache, a pass that inserts
	// many keys into a full map rescans the same used entries on every
	// insert. usedOK is false after any change that moves order slots.
	usedTo   int
	usedPass uint64
	usedOK   bool
	maxSize  int
}

// boundedEntry keeps the stamp beside the value, so a Get that stamps
// costs one map write on an existing key and allocates nothing. A second
// map for the stamps would add a lookup to every Get.
type boundedEntry[V any] struct {
	val V
	// seen is the view pass of the last Get, GetOr or Set. Unused when
	// BoundedMap.pass is nil.
	seen uint64
}

// NewBoundedMap creates a BoundedMap with the given max size.
// maxSize <= 0 leaves the map unbounded: Set stores without
// eviction. Pass a positive bound for state maps; unbounded maps
// grow without limit. (BoundedStack differs: Push on a zero-size
// stack drops the element.)
func NewBoundedMap[K comparable, V any](maxSize int) *BoundedMap[K, V] {
	orderCap := max(maxSize, 0)
	// data is deliberately not pre-sized to maxSize: occupancy is
	// typically far below capacity (e.g. a handful of scroll containers
	// against capScroll=200), and a map that stays within one Swiss-table
	// group (<=8 entries) keeps the runtime's small-map lookup fast path
	// (~4x faster Get on integer keys; measured in #77). Pre-sizing also
	// allocates the full table up front per map. Maps that do fill pay
	// only amortized rehash cost on the way up.
	return &BoundedMap[K, V]{
		data:    make(map[K]boundedEntry[V]),
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
// Existing entries are cleared first. A value of another type
// panics naming both types; time-travel restore relies on the
// panic and skips the namespace (see safeRestoreAny).
func (m *BoundedMap[K, V]) restoreAny(src any) {
	typed, typeOK := src.(*BoundedMap[K, V])
	if !typeOK {
		var want *BoundedMap[K, V]
		panic(fmt.Sprintf(
			"gui: BoundedMap.restoreAny holds %T, not %T", src, want))
	}
	m.Clear()
	typed.Range(func(k K, v V) bool {
		m.Set(k, v)
		return true
	})
}

// stamp returns the current view pass, or zero for a map with no pass.
func (m *BoundedMap[K, V]) stamp() uint64 {
	if m.pass == nil {
		return 0
	}
	return *m.pass
}

// Set adds or updates a key-value pair. At capacity it evicts one entry
// first: for a StateMap map the oldest inserted entry not used in this
// view pass, otherwise the oldest inserted (see BoundedMap).
func (m *BoundedMap[K, V]) Set(key K, value V) {
	seen := m.stamp()
	if _, exists := m.data[key]; exists {
		m.data[key] = boundedEntry[V]{val: value, seen: seen}
		return
	}
	if m.maxSize > 0 {
		if m.pass != nil && len(m.data) >= m.maxSize {
			m.evictUnused()
		}
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
	m.data[key] = boundedEntry[V]{val: value, seen: seen}
	if m.maxSize > 0 {
		m.compactOrder()
	}
}

// evictUnused removes the oldest inserted entry not used in the current
// view pass. It does nothing when every entry was used in this pass: Set
// then evicts the oldest inserted entry, as FIFO does.
//
// Exact least-recently-used order was rejected (#1263): it needs a scan of
// the whole map on every insert into a full map. That cost 900 µs per pass
// for 1000 keys through a 256-entry map, against 46 µs for FIFO. This rule
// still keeps state that is read every frame, and with the usedTo cache it
// passes each used entry at most once per pass.
func (m *BoundedMap[K, V]) evictUnused() {
	cur := *m.pass
	i := m.head
	if m.usedOK && m.usedPass == cur && m.usedTo > i {
		i = m.usedTo
	}
	for ; i < len(m.order); i++ {
		k := m.order[i]
		e, exists := m.data[k]
		if !exists || e.seen >= cur {
			continue
		}
		m.removeSlot(i)
		delete(m.data, k)
		// Slots before i did not move, so the cache stays valid.
		m.usedTo, m.usedPass, m.usedOK = i, cur, true
		return
	}
	m.usedTo, m.usedPass, m.usedOK = len(m.order), cur, true
}

// removeSlot drops order[i]. The head slot only advances head; any other
// slot shifts the tail left in place, which allocates nothing. Either way
// the slots before i keep their index.
func (m *BoundedMap[K, V]) removeSlot(i int) {
	var zero K
	if i == m.head {
		m.order[i] = zero
		m.head++
		return
	}
	copy(m.order[i:], m.order[i+1:])
	m.order[len(m.order)-1] = zero
	m.order = m.order[:len(m.order)-1]
}

// Get returns value for key. Second return is false if not found. For a
// StateMap map, a hit stamps the entry with the current view pass. The
// write happens only when the stamp is old, so a key read many times in one
// pass costs one extra map write in that pass.
func (m *BoundedMap[K, V]) Get(key K) (V, bool) {
	e, ok := m.data[key]
	if ok && m.pass != nil && e.seen != *m.pass {
		e.seen = *m.pass
		m.data[key] = e
	}
	return e.val, ok
}

// GetOr returns the value for key, or def if the key is absent.
// Use at call sites that would otherwise ignore Get's ok boolean, so
// the assumed default is explicit rather than an implicit zero value.
func (m *BoundedMap[K, V]) GetOr(key K, def V) V {
	if v, ok := m.Get(key); ok {
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
	// The shift below moves slots under the evictUnused cache.
	m.usedOK = false
	if len(m.data) == 0 {
		// Zero the slots before the reslice: slots past len must stay
		// zero so dead keys are releasable and Clear's clear(m.order)
		// covers every live slot.
		clear(m.order)
		m.order = m.order[:0]
		m.head = 0
		return
	}
	// Drop the key's ordering slot now. compactOrder only rewrites when its
	// thresholds fire, so on a small map the slot would stay. A later
	// Set(key) sees the key as absent and appends a second slot: Keys and
	// Range then yield it twice, and eviction treats it as the oldest entry.
	// Every live key holds exactly one slot in order[head:], so the first
	// match is the only one. removeSlot allocates nothing.
	for i, k := range m.order[m.head:] {
		if k == key {
			m.removeSlot(m.head + i)
			break
		}
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
	// Zero the ordering slots so cleared keys (strings, pointers)
	// are releasable; [:0] alone keeps them alive through the
	// backing array.
	clear(m.order)
	m.order = m.order[:0]
	m.head = 0
	m.usedOK = false
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
// If fn returns false, iteration stops. fn must not call Set,
// Delete, Clear or evictToBudget on m: those rewrite the ordering
// being walked.
func (m *BoundedMap[K, V]) rangeKeys(fn func(K) bool) {
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
// active entry. If fn returns false, iteration stops. fn must not
// call Set, Delete, Clear or evictToBudget on m: those rewrite the
// ordering being walked.
func (m *BoundedMap[K, V]) Range(fn func(K, V) bool) {
	if len(m.data) == 0 || m.head >= len(m.order) {
		return
	}
	for _, k := range m.order[m.head:] {
		e, exists := m.data[k]
		if !exists {
			continue
		}
		if !fn(k, e.val) {
			return
		}
	}
}

// EvictToBudget evicts oldest entries until the sum of sizeFn across
// remaining entries plus newBytes is <= maxBytes. Returns the number
// of entries evicted. If maxBytes <= 0 the budget is unbounded and
// no entries are evicted.
func (m *BoundedMap[K, V]) evictToBudget(maxBytes int, newBytes int, sizeFn func(V) int) int {
	if maxBytes <= 0 || len(m.data) == 0 {
		return 0
	}
	// Sum current entries + pending new bytes.
	total := newBytes
	for _, e := range m.data {
		total += sizeFn(e.val)
	}
	if total <= maxBytes {
		return 0
	}
	evicted := 0
	for total > maxBytes && m.head < len(m.order) {
		oldestKey := m.order[m.head]
		m.head++
		if e, exists := m.data[oldestKey]; exists {
			total -= sizeFn(e.val)
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
	m.usedOK = false
}

// stringKeys returns the map's keys when they are strings, and nil
// otherwise. The signature names no type parameter, so a caller
// holding a BoundedMap boxed in an `any` can ask for its keys through
// an interface assertion without knowing K.
//
// Only the dev-mode state-key audit calls it (see
// debug_state_keys.go), so the per-key type assertion is not on a
// frame path.
func (m *BoundedMap[K, V]) stringKeys() []string {
	if m == nil {
		return nil
	}
	var out []string
	m.rangeKeys(func(k K) bool {
		s, ok := any(k).(string)
		if !ok {
			// A non-string key type: the whole map is uninteresting,
			// and one key is enough to establish that.
			return false
		}
		out = append(out, s)
		return true
	})
	return out
}
