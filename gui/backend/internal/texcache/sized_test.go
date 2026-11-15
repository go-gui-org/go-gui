package texcache

import "testing"

// sizeIdent makes a test value its own byte size, so a test reads the
// byte total straight from the values it set.
func sizeIdent(v int) int64 { return int64(v) }

// newSizedInts returns a sized cache of int values that records every
// destroyed value in *destroyed.
func newSizedInts(maxCount int, maxBytes int64, destroyed *[]int) Cache[string, int] {
	return NewSized[string, int](maxCount, maxBytes, sizeIdent, func(v int) {
		*destroyed = append(*destroyed, v)
	})
}

// advance runs n frames with no draws, so every entry stops being in use.
func advance(c *Cache[string, int], n int) {
	for range n {
		c.BeginFrame()
	}
}

func TestSizedEvictsByBytesOldestFirst(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("a", 40)
	c.Set("b", 40)
	advance(&c, 2)
	c.Set("c", 40) // 120 > 100: evicts a, the oldest
	if len(destroyed) != 1 || destroyed[0] != 40 {
		t.Fatalf("destroyed = %v", destroyed)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if got := c.Bytes(); got != 80 {
		t.Fatalf("Bytes = %d, want 80", got)
	}
}

func TestSizedKeepsEntriesInUse(t *testing.T) {
	// Every entry is drawn this frame, so none is evicted, though the
	// total is over the budget. Evicting one would mean uploading it
	// again later in the same frame.
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("a", 60)
	c.Set("b", 60)
	c.Set("c", 60)
	if len(destroyed) != 0 {
		t.Fatalf("destroyed = %v, want none", destroyed)
	}
	if got := c.Bytes(); got != 180 {
		t.Fatalf("Bytes = %d, want 180", got)
	}
}

func TestSizedKeepsEntriesUsedLastFrame(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("a", 60)
	c.Set("b", 60)
	c.BeginFrame() // a and b were used last frame: still in use
	c.Set("c", 60)
	if len(destroyed) != 0 {
		t.Fatalf("destroyed = %v, want none", destroyed)
	}
	c.BeginFrame()
	c.Get("c")
	c.BeginFrame()
	// a and b are now two frames old; c was used last frame.
	c.Set("d", 10)
	if len(destroyed) != 2 {
		t.Fatalf("destroyed = %v, want a and b", destroyed)
	}
	if _, ok := c.Get("c"); !ok {
		t.Fatal("c should be kept")
	}
}

func TestSizedSteadyWorkingSetDoesNotThrash(t *testing.T) {
	// A frame draws three 60-byte images against a 100-byte budget. Each
	// later frame draws the same three: no frame may upload again.
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	uploads := 0
	for range 10 {
		c.BeginFrame()
		for _, k := range []string{"a", "b", "c"} {
			if _, ok := c.Get(k); !ok {
				uploads++
				c.Set(k, 60)
			}
		}
	}
	if uploads != 3 {
		t.Fatalf("uploads = %d, want 3", uploads)
	}
	if len(destroyed) != 0 {
		t.Fatalf("destroyed = %v, want none", destroyed)
	}
}

func TestSizedOversizeEntryKept(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("big", 500)
	if _, ok := c.Get("big"); !ok {
		t.Fatal("an entry larger than the budget should be kept while in use")
	}
	advance(&c, 2)
	c.Set("small", 1) // big is no longer in use: evicted now
	if len(destroyed) != 1 || destroyed[0] != 500 {
		t.Fatalf("destroyed = %v", destroyed)
	}
}

func TestSizedCountBoundStillApplies(t *testing.T) {
	var destroyed []int
	c := newSizedInts(2, 1000, &destroyed)
	c.Set("a", 1)
	c.Set("b", 1)
	advance(&c, 2)
	c.Set("c", 1)
	if c.Len() != 2 || len(destroyed) != 1 {
		t.Fatalf("Len = %d destroyed = %v", c.Len(), destroyed)
	}
}

func TestSizedZeroBytesMeansCountOnly(t *testing.T) {
	var destroyed []int
	c := newSizedInts(2, 0, &destroyed)
	c.Set("a", 1000)
	c.Set("b", 1000)
	if len(destroyed) != 0 {
		t.Fatalf("destroyed = %v, want none", destroyed)
	}
}

func TestSizedReplaceUpdatesBytes(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("a", 0) // a web image: size unknown while it loads
	c.Set("a", 70)
	if got := c.Bytes(); got != 70 {
		t.Fatalf("Bytes = %d, want 70", got)
	}
}

func TestSizedReplaceCanTriggerEviction(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 100, &destroyed)
	c.Set("old", 50)
	advance(&c, 2)
	c.Set("a", 0)
	c.Set("a", 70) // the size became known: 120 > 100, evicts old
	if _, ok := c.Get("old"); ok {
		t.Fatal("old should be evicted")
	}
	if got := c.Bytes(); got != 70 {
		t.Fatalf("Bytes = %d, want 70", got)
	}
}

func TestSizedRemovalsKeepByteTotal(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 1000, &destroyed)
	c.Set("a", 10)
	c.Set("b", 20)
	c.Set("c", 30)
	c.Set("d", 40)

	c.EvictOldest() // a
	if got := c.Bytes(); got != 90 {
		t.Fatalf("after EvictOldest Bytes = %d, want 90", got)
	}
	c.Sweep(func(k string, _ int) bool { return k != "b" })
	if got := c.Bytes(); got != 70 {
		t.Fatalf("after Sweep Bytes = %d, want 70", got)
	}
	c.DiscardAll(func(v int) bool { return v == 40 })
	if got := c.Bytes(); got != 40 {
		t.Fatalf("after DiscardAll(keep) Bytes = %d, want 40", got)
	}
	c.DestroyAll()
	if got := c.Bytes(); got != 0 {
		t.Fatalf("after DestroyAll Bytes = %d, want 0", got)
	}
	c.Set("e", 5)
	c.DiscardAll(nil)
	if got := c.Bytes(); got != 0 {
		t.Fatalf("after DiscardAll(nil) Bytes = %d, want 0", got)
	}
}

func TestUnsizedBytesIsZero(t *testing.T) {
	c := New[string, int](4, nil)
	c.Set("a", 100)
	c.BeginFrame() // allowed on an unsized cache; does nothing visible
	if got := c.Bytes(); got != 0 {
		t.Fatalf("Bytes = %d, want 0", got)
	}
}

func TestBudget(t *testing.T) {
	if got := Budget(0, 7); got != 7 {
		t.Fatalf("Budget(0, 7) = %d", got)
	}
	if got := Budget(-1, 7); got != 7 {
		t.Fatalf("Budget(-1, 7) = %d", got)
	}
	if got := Budget(3, 7); got != 3 {
		t.Fatalf("Budget(3, 7) = %d", got)
	}
}

func TestSizedHotPathAllocs(t *testing.T) {
	c := NewSized[string, int](8, 100, sizeIdent, nil)
	c.Set("a", 10)
	allocs := testing.AllocsPerRun(100, func() {
		c.BeginFrame()
		c.Get("a")
	})
	if allocs != 0 {
		t.Fatalf("BeginFrame+Get allocs = %v, want 0", allocs)
	}
}

func TestRemove(t *testing.T) {
	var destroyed []int
	c := newSizedInts(100, 1000, &destroyed)
	c.Set("a", 10)
	c.Set("b", 20)
	if !c.Remove("a") {
		t.Fatal("Remove(a) = false, want true")
	}
	if c.Remove("a") {
		t.Fatal("second Remove(a) = true, want false")
	}
	if _, ok := c.Get("a"); ok || c.Len() != 1 {
		t.Fatalf("a still cached, Len = %d", c.Len())
	}
	if c.Bytes() != 20 || len(destroyed) != 1 || destroyed[0] != 10 {
		t.Fatalf("Bytes = %d destroyed = %v", c.Bytes(), destroyed)
	}
}

func TestSizedNonPositiveMaxSizeClamped(t *testing.T) {
	// NewSized shares New's clamp: a zero count bound still evicts, so the
	// cache cannot grow without limit when the byte bound is also off.
	var destroyed []int
	c := newSizedInts(0, 0, &destroyed)
	c.Set("a", 1)
	advance(&c, 2)
	c.Set("b", 1)
	if c.Len() != 1 || len(destroyed) != 1 || destroyed[0] != 1 {
		t.Fatalf("Len = %d destroyed = %v", c.Len(), destroyed)
	}
}
