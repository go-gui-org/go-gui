package gui

import "testing"

func TestScratchSliceTakeEmpty(t *testing.T) {
	s := scratchSlice[RenderCmd]{retainMax: 1000, shrinkTo: 100}
	b := s.take(0)
	if len(b) != 0 {
		t.Fatalf("want len 0, got %d", len(b))
	}
}

func TestScratchSliceTakeRequiredCap(t *testing.T) {
	s := scratchSlice[RenderCmd]{retainMax: 1000, shrinkTo: 100}
	b := s.take(100)
	if cap(b) < 100 {
		t.Fatalf("want cap>=100, got %d", cap(b))
	}
}

func TestScratchSlicePutShrink(t *testing.T) {
	s := scratchSlice[RenderCmd]{retainMax: 1000, shrinkTo: 100}
	big := make([]RenderCmd, 0, 1001)
	s.put(big)
	if cap(s.buf) != 100 {
		t.Fatalf("want cap 100, got %d", cap(s.buf))
	}
}

func TestScratchSliceRoundTrip(t *testing.T) {
	s := scratchSlice[RenderCmd]{retainMax: 1000, shrinkTo: 100}
	b := s.take(16)
	b = append(b, RenderCmd{Kind: RenderRect})
	s.put(b)
	b2 := s.take(0)
	if len(b2) != 0 {
		t.Fatalf("want empty after take, got %d", len(b2))
	}
	if cap(b2) < 16 {
		t.Fatal("backing array should be reused")
	}
}

func TestScratchMapTakeEmpty(t *testing.T) {
	s := scratchMap[uint32, struct{}]{retainMax: 4096}
	m := s.take(0)
	if len(m) != 0 {
		t.Fatalf("want len 0, got %d", len(m))
	}
}

func TestScratchMapTakeReuse(t *testing.T) {
	s := scratchMap[uint32, struct{}]{retainMax: 4096}
	m := s.take(8)
	m[1] = struct{}{}
	s.put(m)
	m2 := s.take(0)
	if len(m2) != 0 {
		t.Fatal("should be cleared")
	}
}

func TestScratchMapPutDiscard(t *testing.T) {
	s := scratchMap[uint32, struct{}]{retainMax: 2}
	m := make(map[uint32]struct{}, 8)
	m[1] = struct{}{}
	m[2] = struct{}{}
	m[3] = struct{}{}
	s.put(m)
	if s.m != nil {
		t.Fatal("should discard oversized map")
	}
}

func TestTakeFloatingLayoutsResets(t *testing.T) {
	p := newScratchPools()
	s := p.takeFloatingLayouts(10)
	if len(s) != 0 || cap(s) < 10 {
		t.Fatal("unexpected size/cap")
	}
	if p.floatingPoolUsed != 0 {
		t.Fatal("floatingPoolUsed should reset")
	}
}

func TestAllocFloatingLayoutNew(t *testing.T) {
	p := newScratchPools()
	shape := Shape{Width: 42}
	src := Layout{Shape: &shape}
	l := p.allocFloatingLayout(src)
	if l.Shape.Width != 42 {
		t.Fatal("expected width 42")
	}
	if p.floatingPoolUsed != 1 {
		t.Fatal("pool used should be 1")
	}
}

func TestScratchObjPoolAlloc(t *testing.T) {
	pool := scratchObjPool[TextStyle]{retainMax: 64, shrinkTo: 8}
	s1 := TextStyle{Family: "sans", Size: 14}
	p1 := pool.alloc(s1)
	if p1.Family != "sans" || p1.Size != 14 {
		t.Fatal("alloc should copy value")
	}
	if pool.used != 1 {
		t.Fatal("used should be 1")
	}
}

func TestScratchObjPoolReuse(t *testing.T) {
	pool := scratchObjPool[TextStyle]{retainMax: 64, shrinkTo: 8}
	s1 := TextStyle{Family: "serif", Size: 12}
	first := pool.alloc(s1)

	pool.reset()

	s2 := TextStyle{Family: "mono", Size: 16}
	second := pool.alloc(s2)
	if first != second {
		t.Fatal("should reuse same pointer")
	}
	if second.Family != "mono" || second.Size != 16 {
		t.Fatal("reused slot should have new value")
	}
}

func TestScratchObjPoolResetShrink(t *testing.T) {
	pool := scratchObjPool[TextStyle]{retainMax: 2, shrinkTo: 1}
	for i := range 5 {
		pool.alloc(TextStyle{Size: float32(i)})
	}
	if len(pool.items) != 5 {
		t.Fatalf("want 5 items, got %d", len(pool.items))
	}
	pool.reset()
	if len(pool.items) != 0 || cap(pool.items) != 1 {
		t.Fatalf("want shrunk pool, got len=%d cap=%d",
			len(pool.items), cap(pool.items))
	}
}

func TestScratchObjPoolResetShrinkLowUsage(t *testing.T) {
	// After a spike of 1000 allocs, a frame using only 10 (1%)
	// should trigger the usage-based shrink.
	pool := scratchObjPool[TextStyle]{retainMax: 16384, shrinkTo: 32}
	for i := range 1000 {
		pool.alloc(TextStyle{Size: float32(i)})
	}
	if len(pool.items) != 1000 {
		t.Fatalf("pre: want 1000 items, got %d", len(pool.items))
	}
	// Simulate a normal frame: only 10 allocations.
	pool.used = 10
	pool.reset()
	if cap(pool.items) != 32 {
		t.Fatalf("low usage should trigger shrink: want cap 32, got %d",
			cap(pool.items))
	}
}

func TestScratchObjPoolResetNoShrinkWhenUsageHigh(t *testing.T) {
	pool := scratchObjPool[TextStyle]{retainMax: 16384, shrinkTo: 32}
	for i := range 1000 {
		pool.alloc(TextStyle{Size: float32(i)})
	}
	// 800 of 1000 slots used (80%) — should NOT shrink.
	pool.used = 800
	pool.reset()
	if len(pool.items) != 1000 {
		t.Fatalf("high usage should retain pool: want 1000 items, got %d",
			len(pool.items))
	}
}

func TestScratchObjPoolResetBoundaryNoShrinkAtQuarter(t *testing.T) {
	pool := scratchObjPool[TextStyle]{retainMax: 16384, shrinkTo: 32}
	for i := range 1000 {
		pool.alloc(TextStyle{Size: float32(i)})
	}
	// Exactly 25% usage — threshold is p.used < len/4 (strict less).
	// 250 < 250 is false, so pool must NOT shrink.
	pool.used = 250
	pool.reset()
	if len(pool.items) != 1000 {
		t.Fatalf("exactly 25%% usage should retain pool: want 1000 items, got %d",
			len(pool.items))
	}
}

func TestAllocFloatingLayoutReuse(t *testing.T) {
	p := newScratchPools()
	shape1 := Shape{Width: 1}
	src := Layout{Shape: &shape1}
	first := p.allocFloatingLayout(src)

	// Put back and take again to reset poolUsed.
	p.putFloatingLayouts(p.floatingLayouts)
	_ = p.takeFloatingLayouts(0)

	shape2 := Shape{Width: 2}
	src = Layout{Shape: &shape2}
	second := p.allocFloatingLayout(src)
	if first != second {
		t.Fatal("should reuse same pointer")
	}
	if second.Shape.Width != 2 {
		t.Fatal("expected width 2")
	}
}

// --- takeVColors ---

func TestTakeVColors_NonPositiveReturnsNil(t *testing.T) {
	var p scratchPools
	if p.takeVColors(0) != nil {
		t.Fatal("n=0 must return nil")
	}
	if p.takeVColors(-5) != nil {
		t.Fatal("negative n must return nil")
	}
}

func TestTakeVColors_BasicReservation(t *testing.T) {
	var p scratchPools
	a := p.takeVColors(4)
	if len(a) != 4 || cap(a) != 4 {
		t.Fatalf("want len=cap=4, got len=%d cap=%d", len(a), cap(a))
	}
	// Appending to 'a' must not bleed into the next reservation:
	// cap is pinned to its length.
	b := p.takeVColors(3)
	if len(b) != 3 || cap(b) != 3 {
		t.Fatalf("want len=cap=3, got len=%d cap=%d", len(b), cap(b))
	}
	// Writes to b must not overwrite a.
	for i := range b {
		b[i] = Color{1, 2, 3, 4, true}
	}
	for i := range a {
		if a[i] != (Color{}) {
			t.Fatalf("a[%d] clobbered by b: %+v", i, a[i])
		}
	}
}

// Arena growth must preserve slices returned before the realloc.
// Prior reservations reference the old backing array; Go's GC
// keeps it alive.
func TestTakeVColors_ArenaGrowthPreservesPriorSlices(t *testing.T) {
	var p scratchPools
	first := p.takeVColors(4)
	for i := range first {
		first[i] = Color{byte(i + 1), 0, 0, 255, true}
	}
	// Force at least one realloc by requesting a huge chunk.
	_ = p.takeVColors(1024)
	// first must still hold its original data.
	for i := range first {
		want := Color{byte(i + 1), 0, 0, 255, true}
		if first[i] != want {
			t.Fatalf("first[%d]=%+v clobbered by arena growth, want %+v",
				i, first[i], want)
		}
	}
}

func TestTakeVColors_OversizeBypassesArena(t *testing.T) {
	var p scratchPools
	// Large n must bypass arena; returned slice has len=n but
	// arena remains empty (no capacity held across frames).
	a := p.takeVColors(maxVColReservation + 10)
	if len(a) != maxVColReservation+10 {
		t.Fatalf("expected len=%d, got %d",
			maxVColReservation+10, len(a))
	}
	if len(p.svgVColArena) != 0 {
		t.Fatalf("arena should stay empty for oversize requests, "+
			"got len=%d", len(p.svgVColArena))
	}
}

func TestResetRenderPools_TruncatesVColArena(t *testing.T) {
	var p scratchPools
	_ = p.takeVColors(32)
	if len(p.svgVColArena) == 0 {
		t.Fatal("precondition: arena should have content")
	}
	prevCap := cap(p.svgVColArena)
	p.resetRenderPools()
	if len(p.svgVColArena) != 0 {
		t.Fatalf("arena len should reset to 0, got %d",
			len(p.svgVColArena))
	}
	if cap(p.svgVColArena) != prevCap {
		t.Fatal("small arena cap should be retained across reset")
	}
}

// A spike frame must not hold megabytes of vertex-color capacity
// forever; the arena shrinks back to svgVColShrinkTo when it has
// grown past svgVColRetainMax.
func TestResetRenderPools_ShrinksOversizedVColArena(t *testing.T) {
	var p scratchPools
	_ = p.takeVColors(svgVColRetainMax + 1)
	p.resetRenderPools()
	if cap(p.svgVColArena) > svgVColShrinkTo {
		t.Fatalf("expected shrink to <=%d, got cap=%d",
			svgVColShrinkTo, cap(p.svgVColArena))
	}
}

// --- growCap ---

func TestGrowCap_NormalDoubling(t *testing.T) {
	if growCap(4, 5) != 8 {
		t.Fatalf("expected doubling from 4 → 8, got %d", growCap(4, 5))
	}
	if growCap(0, 10) != 10 {
		t.Fatalf("expected need when oldCap=0, got %d", growCap(0, 10))
	}
}

// Overflow of oldCap*2 must not return a negative/absurd capacity;
// the helper falls back to need when doubling would overflow.
func TestGrowCap_HandlesOverflow(t *testing.T) {
	huge := int(^uint(0) >> 1) // math.MaxInt
	// oldCap*2 overflows; growCap should return need, not a
	// negative or tiny value.
	got := growCap(huge, huge)
	if got < huge {
		t.Fatalf("overflow must not produce shrunken cap: got %d, want >= %d",
			got, huge)
	}
}
