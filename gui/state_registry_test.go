package gui

import "testing"

func TestStateMapRoundTrip(t *testing.T) {
	w := &Window{}
	om := StateMap[string, int](w, nsOverflow, capModerate)

	om.Set("panel_a", 3)
	om.Set("panel_b", 5)

	if v, ok := om.Get("panel_a"); !ok || v != 3 {
		t.Errorf("panel_a: got %d, %v", v, ok)
	}
	if v, ok := om.Get("panel_b"); !ok || v != 5 {
		t.Errorf("panel_b: got %d, %v", v, ok)
	}
	if _, ok := om.Get("panel_c"); ok {
		t.Error("panel_c should not exist")
	}
	if om.Len() != 2 {
		t.Errorf("len: got %d", om.Len())
	}
}

func TestStateMapReturnsSameInstance(t *testing.T) {
	w := &Window{}
	m1 := StateMap[string, int](w, "test.ns", 10)
	m1.Set("x", 42)

	m2 := StateMap[string, int](w, "test.ns", 10)
	if v, ok := m2.Get("x"); !ok || v != 42 {
		t.Errorf("x: got %d, %v", v, ok)
	}
}

func TestStateMapEviction(t *testing.T) {
	w := &Window{}
	m := StateMap[string, int](w, "test.evict", 2)

	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("c", 3)

	if _, ok := m.Get("a"); ok {
		t.Error("a should be evicted")
	}
	if v, ok := m.Get("b"); !ok || v != 2 {
		t.Errorf("b: got %d, %v", v, ok)
	}
	if v, ok := m.Get("c"); !ok || v != 3 {
		t.Errorf("c: got %d, %v", v, ok)
	}
}

func TestClearViewStateDropsRegistry(t *testing.T) {
	w := &Window{}
	m := StateMap[string, int](w, "test.clear", 10)
	m.Set("k", 99)

	w.clearViewState()

	m2 := StateMap[string, int](w, "test.clear", 10)
	if _, ok := m2.Get("k"); ok {
		t.Error("k should be gone after clear")
	}
	if m2.Len() != 0 {
		t.Errorf("len: got %d", m2.Len())
	}
}

func TestStateMapReadReturnsNilForMissing(t *testing.T) {
	w := &Window{}
	if sm := StateMapRead[string, int](w, "test.read.none"); sm != nil {
		t.Error("should be nil for missing namespace")
	}
}

func TestRegistryEntryCount(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, int](w, "test.count", 10)
	if w.viewState.registry.entryCount("test.count") != 0 {
		t.Error("should be 0 initially")
	}

	sm.Set("a", 1)
	sm.Set("b", 2)
	if w.viewState.registry.entryCount("test.count") != 2 {
		t.Errorf("should be 2, got %d", w.viewState.registry.entryCount("test.count"))
	}

	if w.viewState.registry.entryCount("no.such.ns") != 0 {
		t.Error("missing ns should be 0")
	}
}
