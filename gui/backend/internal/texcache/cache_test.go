package texcache

import "testing"

func TestGetMiss(t *testing.T) {
	c := New[string, int](4, nil)
	if _, ok := c.Get("x"); ok {
		t.Fatal("expected miss")
	}
}

func TestSetGet(t *testing.T) {
	c := New[string, int](4, nil)
	c.Set("a", 1)
	v, ok := c.Get("a")
	if !ok || v != 1 {
		t.Fatalf("got %v %v", v, ok)
	}
}

func TestEvictsOldest(t *testing.T) {
	var destroyed []int
	c := New[string, int](2, func(v int) {
		destroyed = append(destroyed, v)
	})
	c.Set("a", 1)
	c.Set("b", 2)
	c.Set("c", 3) // evicts "a"
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if len(destroyed) != 1 || destroyed[0] != 1 {
		t.Fatalf("destroyed = %v", destroyed)
	}
}

func TestPromotePreservesEntry(t *testing.T) {
	var destroyed []string
	c := New[string, string](2, func(v string) {
		destroyed = append(destroyed, v)
	})
	c.Set("a", "A")
	c.Set("b", "B")
	c.Get("a")      // promote a → most recent
	c.Set("c", "C") // evicts b (oldest)
	if _, ok := c.Get("b"); ok {
		t.Fatal("b should be evicted")
	}
	if v, ok := c.Get("a"); !ok || v != "A" {
		t.Fatal("a should survive")
	}
}

func TestUpdateExisting(t *testing.T) {
	c := New[string, int](2, nil)
	c.Set("a", 1)
	c.Set("a", 2)
	v, _ := c.Get("a")
	if v != 2 {
		t.Fatalf("got %d", v)
	}
}

func TestDestroyAll(t *testing.T) {
	var count int
	c := New[string, int](4, func(int) { count++ })
	c.Set("a", 1)
	c.Set("b", 2)
	c.DestroyAll()
	if count != 2 {
		t.Fatalf("destroyed %d", count)
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("should be empty")
	}
}

func TestDestroyAllAllowsReuse(t *testing.T) {
	c := New[string, int](4, nil)
	c.Set("a", 1)
	c.DestroyAll()
	c.Set("b", 2)
	v, ok := c.Get("b")
	if !ok || v != 2 {
		t.Fatalf("got %v %v", v, ok)
	}
}

func TestUint64Key(t *testing.T) {
	c := New[uint64, string](2, nil)
	c.Set(42, "hello")
	v, ok := c.Get(42)
	if !ok || v != "hello" {
		t.Fatalf("got %v %v", v, ok)
	}
}

func TestUpdateCallsDestroy(t *testing.T) {
	var destroyed []int
	c := New[string, int](4, func(v int) {
		destroyed = append(destroyed, v)
	})
	c.Set("a", 1)
	c.Set("a", 2) // should destroy old value 1
	if len(destroyed) != 1 || destroyed[0] != 1 {
		t.Fatalf("destroyed = %v", destroyed)
	}
}

func TestEvictOldest(t *testing.T) {
	var destroyed []string
	c := New[string, string](2, func(v string) {
		destroyed = append(destroyed, v)
	})
	c.Set("a", "A")
	c.Set("b", "B")
	if !c.EvictOldest() {
		t.Fatal("expected eviction")
	}
	if c.Len() != 1 {
		t.Fatalf("len = %d", c.Len())
	}
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if v, ok := c.Get("b"); !ok || v != "B" {
		t.Fatalf("got %v %v", v, ok)
	}
	if len(destroyed) != 1 || destroyed[0] != "A" {
		t.Fatalf("destroyed = %v", destroyed)
	}
}

func TestNonPositiveMaxSizeClamped(t *testing.T) {
	for _, n := range []int{0, -3} {
		c := New[string, int](n, nil)
		c.Set("a", 1)
		c.Set("b", 2) // must evict a, not grow
		if _, ok := c.Get("a"); ok {
			t.Fatalf("New(%d): a should be evicted", n)
		}
		if c.Len() != 1 {
			t.Fatalf("New(%d): len = %d, want 1", n, c.Len())
		}
	}
}
