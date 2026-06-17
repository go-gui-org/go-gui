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

func TestClearViewStateClearsHotMaps(t *testing.T) {
	w := &Window{}
	// Populate hot maps.
	sy := w.scrollY()
	sy.Set(1, float32(-50))
	sx := w.scrollX()
	sx.Set(2, float32(-30))
	w.hoverInside().Set("h", true)
	w.overflow().Set("o", 5)

	// Verify values are present.
	if v, ok := sy.Get(1); !ok || v != -50 {
		t.Errorf("scrollY before clear: got %v, %v", v, ok)
	}

	w.clearViewState()

	// After clear, the old maps should be gone and new
	// lazy-init maps should be empty.
	sy2 := w.scrollY()
	if sy2 == sy {
		t.Error("scrollY should be a new instance after clear")
	}
	if _, ok := sy2.Get(1); ok {
		t.Error("scrollY value should be cleared")
	}
	if _, ok := w.scrollX().Get(2); ok {
		t.Error("scrollX value should be cleared")
	}
	if _, ok := w.hoverInside().Get("h"); ok {
		t.Error("hoverInside value should be cleared")
	}
	if _, ok := w.overflow().Get("o"); ok {
		t.Error("overflow value should be cleared")
	}
}

func TestHotMapAccessorSameInstance(t *testing.T) {
	w := &Window{}

	sy1 := w.scrollY()
	sy1.Set(1, float32(-42))

	sy2 := w.scrollY()
	if sy1 != sy2 {
		t.Error("scrollY should return same instance across calls")
	}
	if v, ok := sy2.Get(1); !ok || v != -42 {
		t.Errorf("value not preserved across accessor calls: %v, %v", v, ok)
	}

	// Same for other accessors.
	sx1 := w.scrollX()
	sx2 := w.scrollX()
	if sx1 != sx2 {
		t.Error("scrollX should return same instance")
	}

	h1 := w.hoverInside()
	h2 := w.hoverInside()
	if h1 != h2 {
		t.Error("hoverInside should return same instance")
	}

	o1 := w.overflow()
	o2 := w.overflow()
	if o1 != o2 {
		t.Error("overflow should return same instance")
	}
}

func TestScrollReadReturnsNilWhenCold(t *testing.T) {
	w := &Window{}

	if sx := w.scrollXRead(); sx != nil {
		t.Error("scrollXRead should return nil before first access")
	}
	if sy := w.scrollYRead(); sy != nil {
		t.Error("scrollYRead should return nil before first access")
	}

	// After warming via the lazy accessor, Read should return
	// the same instance.
	syWarm := w.scrollY()
	syRead := w.scrollYRead()
	if syRead != syWarm {
		t.Error("scrollYRead should return warm instance after lazy init")
	}
	if syRead == nil {
		t.Error("scrollYRead should be non-nil after warm")
	}

	sxWarm := w.scrollX()
	sxRead := w.scrollXRead()
	if sxRead != sxWarm {
		t.Error("scrollXRead should return warm instance after lazy init")
	}
}
