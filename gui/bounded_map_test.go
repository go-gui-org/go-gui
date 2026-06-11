package gui

import "testing"

func TestBoundedMapBasicOperations(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("a: got %d, %v", v, ok)
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if v, ok := m.Get("c"); !ok || v != 3 {
		t.Errorf("c: got %d, %v", v, ok)
	}
	if m.Len() != 3 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestBoundedMapEviction(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	m.Set("d", 4) // evicts "a"

	if _, ok := m.Get("a"); ok {
		t.Error("a should be evicted")
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if v, ok := m.Get("c"); !ok || v != 3 {
		t.Errorf("c: got %d, %v", v, ok)
	}
	if v, ok := m.Get("d"); !ok || v != 4 {
		t.Errorf("d: got %d, %v", v, ok)
	}
	if m.Len() != 3 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestBoundedMapUpdateExisting(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 10)
	m.Set("c", 3)

	if v, ok := m.Get("a"); !ok || v != 10 {
		t.Errorf("a: got %d, %v", v, ok)
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if v, ok := m.Get("c"); !ok || v != 3 {
		t.Errorf("c: got %d, %v", v, ok)
	}
	if m.Len() != 3 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestBoundedMapContains(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	if !m.Contains("a") {
		t.Error("should contain a")
	}
	if m.Contains("b") {
		t.Error("should not contain b")
	}
}

func TestBoundedMapDelete(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Delete("a")

	if _, ok := m.Get("a"); ok {
		t.Error("a should be deleted")
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if m.Len() != 1 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestBoundedMapClear(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Clear()

	if m.Len() != 0 {
		t.Errorf("len: got %d", m.Len())
	}
	if _, ok := m.Get("a"); ok {
		t.Error("a should be gone")
	}
}

func TestBoundedMapKeys(t *testing.T) {
	m := NewBoundedMap[string, int](5)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	keys := m.Keys()
	if len(keys) != 3 {
		t.Fatalf("keys len: got %d", len(keys))
	}
	if keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("keys: got %v", keys)
	}
}

func TestBoundedMapMaxSizeOne(t *testing.T) {
	m := NewBoundedMap[string, int](1)
	m.Set("a", 1)
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("a: got %d, %v", v, ok)
	}
	if m.Len() != 1 {
		t.Errorf("len: got %d", m.Len())
	}

	m.Set("b", 2) // evicts "a"
	if _, ok := m.Get("a"); ok {
		t.Error("a should be evicted")
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if m.Len() != 1 {
		t.Errorf("len: got %d", m.Len())
	}
}

func TestBoundedMapMaxSizeZero(t *testing.T) {
	// Unbounded map: Set stores, no eviction.
	m := NewBoundedMap[string, int](0)
	m.Set("a", 1)
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a): got (%d, %v), want (1, true)", v, ok)
	}
	if m.Len() != 1 {
		t.Errorf("len: got %d, want 1", m.Len())
	}
}

func TestBoundedMapDeleteNonexistent(t *testing.T) {
	m := NewBoundedMap[string, int](3)
	m.Set("a", 1)
	m.Delete("nonexistent") // should not panic
	if m.Len() != 1 {
		t.Errorf("len: got %d", m.Len())
	}
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("a: got %d, %v", v, ok)
	}
}

func TestBoundedMapKeysStableAfterDeleteAndInsert(t *testing.T) {
	m := NewBoundedMap[string, int](4)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)
	m.Delete("b")
	m.Set("d", 4)
	keys := m.Keys()
	expected := []string{"a", "c", "d"}
	if len(keys) != len(expected) {
		t.Fatalf("keys len: got %d, want %d", len(keys), len(expected))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("key[%d]: got %s, want %s", i, k, expected[i])
		}
	}
}

func TestBoundedMapMaxSizeNegative(t *testing.T) {
	// Negative maxSize treated as unbounded: Set stores, no eviction.
	m := NewBoundedMap[string, int](-1)
	m.Set("a", 1)
	if v, ok := m.Get("a"); !ok || v != 1 {
		t.Errorf("Get(a): got (%d, %v), want (1, true)", v, ok)
	}
	if m.Len() != 1 {
		t.Errorf("len: got %d, want 1", m.Len())
	}
}

func TestBoundedMapSetUpdateDoesNotDuplicateOrderEntry(t *testing.T) {
	m := NewBoundedMap[string, int](4)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 10)
	keys := m.Keys()
	expected := []string{"a", "b"}
	if len(keys) != len(expected) {
		t.Fatalf("keys len: got %d, want %d", len(keys), len(expected))
	}
	for i := range expected {
		if keys[i] != expected[i] {
			t.Fatalf("key[%d]: got %s, want %s", i, keys[i], expected[i])
		}
	}
	if len(m.order) != 2 {
		t.Fatalf("order len: got %d, want 2", len(m.order))
	}
}

func TestBoundedMapDeleteChurnDoesNotGrowOrderUnbounded(t *testing.T) {
	m := NewBoundedMap[int, int](64)
	m.Set(0, 0) // keep one live key so len(data) never reaches zero

	for i := 1; i <= 5000; i++ {
		m.Set(i, i)
		m.Delete(i)
	}

	if m.Len() != 1 {
		t.Fatalf("len: got %d, want 1", m.Len())
	}
	if len(m.order) > boundedOrderCompactMin*4 {
		t.Fatalf("order grew too large: got %d", len(m.order))
	}
	keys := m.Keys()
	if len(keys) != 1 || keys[0] != 0 {
		t.Fatalf("keys: got %v, want [0]", keys)
	}
}

func TestBoundedMapRangeAfterHeavyDeleteInsertChurn(t *testing.T) {
	m := NewBoundedMap[int, int](64)
	m.Set(0, 100)
	for i := 1; i <= 2000; i++ {
		m.Set(i, i)
		m.Delete(i)
	}

	seen := 0
	m.Range(func(k, v int) bool {
		seen++
		if k != 0 || v != 100 {
			t.Fatalf("unexpected pair: %d=%d", k, v)
		}
		return true
	})
	if seen != 1 {
		t.Fatalf("range count: got %d, want 1", seen)
	}
}

func TestBoundedMapRangeKeysOrder(t *testing.T) {
	m := NewBoundedMap[string, int](5)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	var got []string
	m.RangeKeys(func(k string) bool {
		got = append(got, k)
		return true
	})

	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]: %s, want %s", i, got[i], want[i])
		}
	}
}

func TestBoundedMapRangeKeysEarlyStop(t *testing.T) {
	m := NewBoundedMap[int, int](5)
	for i := range 5 {
		m.Set(i, i)
	}
	seen := 0
	m.RangeKeys(func(k int) bool {
		seen++
		return k != 2
	})
	if seen != 3 {
		t.Fatalf("seen: got %d, want 3", seen)
	}
}

func TestBoundedMapRangeKeysAfterChurn(t *testing.T) {
	m := NewBoundedMap[int, int](64)
	m.Set(0, 100)
	for i := 1; i <= 2000; i++ {
		m.Set(i, i)
		m.Delete(i)
	}

	var got []int
	m.RangeKeys(func(k int) bool {
		got = append(got, k)
		return true
	})
	if len(got) != 1 || got[0] != 0 {
		t.Fatalf("keys: got %v, want [0]", got)
	}
}

func TestBoundedMapCloneAnyUnbounded(t *testing.T) {
	m := NewBoundedMap[string, int](0)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	clone := m.cloneAny().(*BoundedMap[string, int])
	if clone.Len() != 3 {
		t.Fatalf("clone len: got %d, want 3", clone.Len())
	}
	for _, k := range []string{"a", "b", "c"} {
		want, _ := m.Get(k)
		if v, ok := clone.Get(k); !ok || v != want {
			t.Errorf("clone[%s]: got (%d, %v), want (%d, true)", k, v, ok, want)
		}
	}
	// Verify clone is iterable via Range (order populated).
	seen := 0
	clone.Range(func(k string, v int) bool {
		seen++
		return true
	})
	if seen != 3 {
		t.Errorf("Range on clone saw %d entries, want 3", seen)
	}
	// Verify independence: mutate clone, original unchanged.
	clone.Set("a", 99)
	if v, _ := m.Get("a"); v != 1 {
		t.Errorf("original mutated: got %d, want 1", v)
	}
}

func TestBoundedMapRestoreAnyUnbounded(t *testing.T) {
	m := NewBoundedMap[string, int](0)
	m.Set("a", 1)

	src := NewBoundedMap[string, int](0)
	src.Set("x", 10)
	src.Set("y", 20)

	m.restoreAny(src)
	if m.Len() != 2 {
		t.Fatalf("len: got %d, want 2", m.Len())
	}
	if v, ok := m.Get("x"); !ok || v != 10 {
		t.Errorf("x: got (%d, %v), want (10, true)", v, ok)
	}
	if v, ok := m.Get("y"); !ok || v != 20 {
		t.Errorf("y: got (%d, %v), want (20, true)", v, ok)
	}
	if _, ok := m.Get("a"); ok {
		t.Error("a should be cleared")
	}
	// Verify restored map is iterable via Range.
	seen := 0
	m.Range(func(k string, v int) bool {
		seen++
		return true
	})
	if seen != 2 {
		t.Errorf("Range after restore saw %d entries, want 2", seen)
	}
}
