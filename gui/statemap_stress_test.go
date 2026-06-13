package gui

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestStateMapCreateAndAccess stress-tests StateMap creation and access
// patterns at high call rates.
func TestStateMapCreateAndAccess(t *testing.T) {
	w := newTestWindow()

	const ns = "stress.test.basic"
	const iterations = 1000

	// Verify lazy creation works.
	smNil := StateMapRead[string, int](w, ns)
	if smNil != nil {
		t.Error("expected nil for uninitialized namespace")
	}

	sm := StateMap[string, int](w, ns, capModerate)
	if sm == nil {
		t.Fatal("StateMap returned nil")
	}

	// Stress Set/Get/Delete.
	for i := range iterations {
		key := "key-" + string(rune('a'+i%26))
		sm.Set(key, i)
		v, ok := sm.Get(key)
		if !ok {
			t.Errorf("iteration %d: key %q not found", i, key)
		}
		if v != i {
			t.Errorf("iteration %d: got %d, want %d", i, v, i)
		}
	}

	if sm.Len() != capModerate { // highest-n by insert order
		// Actually, BoundedMap evicts oldest when over capacity.
		// After 1000 inserts with cap=50, len should be 50.
		if sm.Len() > capModerate {
			t.Errorf("bounded map grew beyond capacity: %d", sm.Len())
		}
	}

	// Delete all remaining.
	keys := sm.Keys()
	for _, k := range keys {
		sm.Delete(k)
	}
	if sm.Len() != 0 {
		t.Errorf("expected empty map, got %d entries", sm.Len())
	}
}

// TestStateMapEviction verifies bounded eviction under pressure.
func TestStateMapEvictionUnderPressure(t *testing.T) {
	cap := 10
	w := newTestWindow()
	sm := StateMap[int, int](w, "stress.test.evict", cap)

	// Insert 3x capacity.
	for i := range cap * 3 {
		sm.Set(i, i)
	}

	if sm.Len() != cap {
		t.Errorf("after 3x capacity inserts: len=%d, want=%d", sm.Len(), cap)
	}

	// Oldest keys should be evicted.
	if sm.Contains(0) {
		t.Error("oldest key should be evicted")
	}
	// Newest keys should survive.
	if !sm.Contains(cap*3 - 1) {
		t.Error("newest key should still exist")
	}
}

// TestStateMapMultipleNamespaces verifies namespaces are independent.
func TestStateMapMultipleNamespaces(t *testing.T) {
	w := newTestWindow()

	sm1 := StateMap[string, int](w, "stress.test.ns1", 5)
	sm2 := StateMap[string, float32](w, "stress.test.ns2", 5)

	sm1.Set("a", 1)
	sm2.Set("a", 3.14)

	v1, ok1 := sm1.Get("a")
	if !ok1 || v1 != 1 {
		t.Error("ns1 lookup failed")
	}

	v2, ok2 := sm2.Get("a")
	if !ok2 || !f32AreClose(v2, 3.14) {
		t.Error("ns2 lookup failed")
	}

	// Clear one namespace (removes from registry only).
	w.viewState.registry.ClearNamespace("stress.test.ns1")

	// After ClearNamespace, StateMapRead returns nil for that ns.
	got := StateMapRead[string, int](w, "stress.test.ns1")
	if got != nil {
		t.Error("StateMapRead should return nil after ClearNamespace")
	}

	// Other namespace unaffected.
	if sm2.Len() != 1 {
		t.Error("ns2 should still have its entry after ns1 clear")
	}

	// Re-create the namespace (lazy init) and verify it has no old data.
	sm1b := StateMap[string, int](w, "stress.test.ns1", 5)
	if sm1b.Len() != 0 {
		t.Error("re-created ns1 should be empty")
	}
}

// TestStateMapClearAll verifies Clear drops all namespaces.
func TestStateMapClearAll(t *testing.T) {
	w := newTestWindow()

	StateMap[string, int](w, "stress.test.ca1", 5).Set("x", 1)
	StateMap[string, int](w, "stress.test.ca2", 5).Set("y", 2)

	if w.viewState.registry.entryCount("stress.test.ca1") != 1 {
		t.Error("expected 1 entry in ca1")
	}

	w.viewState.registry.Clear()
	if w.viewState.registry.entryCount("stress.test.ca1") != 0 {
		t.Error("expected 0 after Clear")
	}
	if w.viewState.registry.entryCount("stress.test.ca2") != 0 {
		t.Error("expected 0 after Clear")
	}
}

// TestStateMapRangeAndKeys verifies iteration correctness under load.
func TestStateMapRangeAndKeys(t *testing.T) {
	w := newTestWindow()
	sm := StateMap[int, int](w, "stress.test.range", 20)

	const n = 20
	for i := range n {
		sm.Set(i, i*10)
	}

	// Keys.
	keys := sm.Keys()
	if len(keys) != n {
		t.Errorf("Keys: got %d, want %d", len(keys), n)
	}

	// Range.
	count := 0
	sum := 0
	sm.Range(func(k, v int) bool {
		count++
		sum += v
		return true
	})
	if count != n {
		t.Errorf("Range count: got %d, want %d", count, n)
	}

	// RangeKeys.
	count = 0
	sm.RangeKeys(func(k int) bool {
		count++
		return true
	})
	if count != n {
		t.Errorf("RangeKeys count: got %d, want %d", count, n)
	}
}

// TestBoundedMapConcurrentReads verifies concurrent reads don't panic
// under the race detector. BoundedMap is not thread-safe for writes,
// but concurrent reads (when no writes are happening) should be fine.
func TestBoundedMapConcurrentReads(t *testing.T) {
	m := NewBoundedMap[int, int](100)
	for i := range 100 {
		m.Set(i, i)
	}

	var wg sync.WaitGroup
	const readers = 8
	const iterations = 1000

	var errCount atomic.Int32
	for range readers {
		wg.Go(func() {
			for range iterations {
				v, ok := m.Get(42)
				if ok && v != 42 {
					errCount.Add(1)
				}
				m.Len()
				m.Contains(99)
			}
		})
	}
	wg.Wait()
	if errCount.Load() > 0 {
		t.Errorf("concurrent reads returned wrong value %d times", errCount.Load())
	}
	wg.Wait()
}

// TestStateReadOr verifies the convenience accessor.
func TestStateReadOr(t *testing.T) {
	w := newTestWindow()

	// Returns default when namespace doesn't exist.
	got := StateReadOr(w, "stress.test.nonexistent", "key", "default")
	if got != "default" {
		t.Errorf("got %q, want default", got)
	}

	// Returns value when set.
	sm := StateMap[string, string](w, "stress.test.reador", capFew)
	sm.Set("key", "value")
	got = StateReadOr(w, "stress.test.reador", "key", "default")
	if got != "value" {
		t.Errorf("got %q, want value", got)
	}

	// Returns default when key not found.
	got = StateReadOr(w, "stress.test.reador", "nope", "default")
	if got != "default" {
		t.Errorf("got %q, want default", got)
	}
}
