package gui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

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
	sy.Set("1", float32(-50))
	sx := w.scrollX()
	sx.Set("2", float32(-30))
	w.hoverInside().Set("h", 1)
	w.overflow().Set("o", 5)

	// Verify values are present.
	if v, ok := sy.Get("1"); !ok || v != -50 {
		t.Errorf("scrollY before clear: got %v, %v", v, ok)
	}

	w.clearViewState()

	// After clear, the old maps should be gone and new
	// lazy-init maps should be empty.
	sy2 := w.scrollY()
	if sy2 == sy {
		t.Error("scrollY should be a new instance after clear")
	}
	if _, ok := sy2.Get("1"); ok {
		t.Error("scrollY value should be cleared")
	}
	if _, ok := w.scrollX().Get("2"); ok {
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
	sy1.Set("1", float32(-42))

	sy2 := w.scrollY()
	if sy1 != sy2 {
		t.Error("scrollY should return same instance across calls")
	}
	if v, ok := sy2.Get("1"); !ok || v != -42 {
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

func TestStateMapTypeMismatchPanicsWithNamespace(t *testing.T) {
	w := &Window{}
	StateMap[string, int](w, "test.mismatch", 10).Set("k", 1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic reusing a namespace with other types")
		}
		msg, isString := r.(string)
		if !isString {
			t.Fatalf("panic is %T, want string", r)
		}
		if !strings.Contains(msg, "test.mismatch") {
			t.Errorf("panic %q names no namespace", msg)
		}
		if !strings.Contains(msg, "BoundedMap[string,int]") ||
			!strings.Contains(msg, "BoundedMap[string,string]") {
			t.Errorf("panic %q names no types", msg)
		}
	}()
	StateMap[string, string](w, "test.mismatch", 10)
}

func TestStateMapReadTypeMismatchPanicsWithNamespace(t *testing.T) {
	w := &Window{}
	StateMap[string, int](w, "test.mismatch.read", 10).Set("k", 1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic reading a namespace with other types")
		}
		msg, isString := r.(string)
		if !isString {
			t.Fatalf("panic is %T, want string", r)
		}
		if !strings.Contains(msg, "test.mismatch.read") {
			t.Errorf("panic %q names no namespace", msg)
		}
	}()
	StateMapRead[string, string](w, "test.mismatch.read")
}

func TestClearViewStateResetsInteractionTargets(t *testing.T) {
	w := &Window{}
	w.viewState.hoverTargetID = "panel:button"
	w.viewState.pressTargetID = "panel:button"
	w.viewState.keyPressTargetID = "panel:field"

	w.clearViewState()

	if w.viewState.hoverTargetID != "" {
		t.Errorf("hoverTargetID: got %q, want empty", w.viewState.hoverTargetID)
	}
	if w.viewState.pressTargetID != "" {
		t.Errorf("pressTargetID: got %q, want empty", w.viewState.pressTargetID)
	}
	if w.viewState.keyPressTargetID != "" {
		t.Errorf("keyPressTargetID: got %q, want empty",
			w.viewState.keyPressTargetID)
	}
}

func TestBoundedMapClearZeroesOrderSlots(t *testing.T) {
	m := NewBoundedMap[string, int](4)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Clear()

	// Clear keeps the backing array; its slots must not pin old keys.
	for i, k := range m.order[:cap(m.order)] {
		if k != "" {
			t.Errorf("order[%d] = %q after Clear, want zero", i, k)
		}
	}
}

func TestBoundedMapRestoreAnyTypeMismatchPanics(t *testing.T) {
	m := NewBoundedMap[string, int](4)
	m.Set("keep", 1)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic restoring from another map type")
		}
		msg, isString := r.(string)
		if !isString {
			t.Fatalf("panic is %T, want string", r)
		}
		if !strings.Contains(msg, "BoundedMap[string,int]") ||
			!strings.Contains(msg, "BoundedMap[string,string]") {
			t.Errorf("panic %q names no types", msg)
		}
		// The assertion runs before Clear, so m keeps its entries.
		if v, ok := m.Get("keep"); !ok || v != 1 {
			t.Errorf("keep: got %d, %v; want 1, true", v, ok)
		}
	}()
	m.restoreAny(NewBoundedMap[string, string](4))
}

func TestBoundedMapDeleteAllThenClearZeroesOrderSlots(t *testing.T) {
	m := NewBoundedMap[string, int](4)
	m.Set("a", 1)
	m.Set("b", 2)
	m.Delete("a")
	m.Delete("b") // last live key: order resets to [:0]
	m.Clear()

	// Slots past len must be zero on every path, not just Set→Clear.
	for i, k := range m.order[:cap(m.order)] {
		if k != "" {
			t.Errorf("order[%d] = %q after delete-all+Clear, want zero", i, k)
		}
	}
}

// A full StateMap namespace evicts an entry not used in the current view
// pass, not simply the oldest inserted (#1263). One key is read every view pass, as an on-screen
// widget reads its state; each pass also inserts a new key, as paging adds
// row IDs. Under FIFO the read key went on the third insert.
func TestStateMapKeepsStateUsedThisPass(t *testing.T) {
	w := newTestWindow()
	sm := StateMap[string, int](w, "test-lrs", 3)
	sm.Set("live", 1)
	for i := range 10 {
		w.viewPass++
		if _, ok := sm.Get("live"); !ok {
			t.Fatalf("pass %d: live key evicted", i)
		}
		sm.Set(fmt.Sprintf("row-%d", i), i)
		if sm.Len() > 3 {
			t.Fatalf("pass %d: len %d exceeds cap 3", i, sm.Len())
		}
	}
	// The two newest rows stay with the live key; Keys keeps insertion
	// order, so "live" still comes first.
	want := []string{"live", "row-8", "row-9"}
	if got := sm.Keys(); !slices.Equal(got, want) {
		t.Errorf("keys: got %v, want %v", got, want)
	}
}

// GetOr and an update through Set also count as use. When every entry was
// used in this pass, the oldest inserted goes, as FIFO did.
func TestStateMapUseStampsAndAllUsedFallback(t *testing.T) {
	w := newTestWindow()
	sm := StateMap[string, int](w, "test-lrs-ties", 3)
	sm.Set("a", 1)
	sm.Set("b", 2)
	sm.Set("c", 3)
	w.viewPass++
	_ = sm.GetOr("a", 0)
	sm.Set("b", 20)
	sm.Set("d", 4) // c is the only entry not seen this pass
	if sm.Contains("c") {
		t.Error("c should be evicted: it is the only entry not used this pass")
	}
	sm.Set("e", 5) // a, b, d all seen this pass: oldest inserted (a) goes
	want := []string{"b", "d", "e"}
	if got := sm.Keys(); !slices.Equal(got, want) {
		t.Errorf("keys: got %v, want %v", got, want)
	}
	// A key evicted and inserted again holds one order slot.
	sm.Set("a", 6)
	if got := len(sm.Keys()); got != 3 {
		t.Errorf("keys after re-insert: got %d, want 3", got)
	}
}

// A map built by NewBoundedMap has no view pass and stays FIFO: reads do
// not protect an entry.
func TestBoundedMapStandaloneStaysFIFO(t *testing.T) {
	m := NewBoundedMap[string, int](2)
	m.Set("a", 1)
	m.Set("b", 2)
	_, _ = m.Get("a")
	m.Set("c", 3)
	if m.Contains("a") {
		t.Error("standalone map: a should be evicted (FIFO)")
	}
}

// The hot scroll map is widget state too: an on-screen scrollable reads
// its offset every pass and must not be evicted while newer IDs go in.
func TestScrollYKeepsOffsetUsedThisPass(t *testing.T) {
	w := newTestWindow()
	sy := w.ScrollY()
	sy.Set("live", 10)
	for i := range capScroll + 10 {
		w.viewPass++
		if _, ok := sy.Get("live"); !ok {
			t.Fatalf("pass %d: live scroll offset evicted", i)
		}
		sy.Set(fmt.Sprintf("pane-%d", i), float32(i))
	}
	if sy.Len() != capScroll {
		t.Errorf("len: got %d, want %d", sy.Len(), capScroll)
	}
}
