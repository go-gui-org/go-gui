package gui

import "testing"

// registryTestSpec is a variable list of n rows seeded at 20 px.
func registryTestSpec(n int, width float32) listHeightSpec {
	return listHeightSpec{n: n, estimate: 20, width: width}
}

func TestListHeightRegistryReusesTheModel(t *testing.T) {
	w := newTestWindow()
	m1 := listHeightEnsureVariable(w, "lst", registryTestSpec(10, 100))
	m1.SetHeight(3, 200)
	m2 := listHeightEnsureVariable(w, "lst", registryTestSpec(10, 100))
	if m1 != m2 {
		t.Fatal("re-registering replaced the model")
	}
	if got := m2.Height(3); got != 200 {
		t.Fatalf("measured height lost on re-registration: %v", got)
	}
}

func TestListHeightRegistryWidthChangeKeepsMeasuredHeights(t *testing.T) {
	w := newTestWindow()
	m := listHeightEnsureVariable(w, "lst", registryTestSpec(100, 100))
	for i := range 100 {
		m.SetHeight(i, 50)
	}
	// Sit on row 40 with a 7 px offset into it.
	w.scrollY().Set("lst", -(m.Prefix(40) + 7))

	// A width change makes every measured height stale — they describe
	// a wrap that no longer applies — but it must not throw them away.
	// A height measured for *this* row predicts it better than one
	// estimate shared by the corpus, and dropping them re-seated the
	// reader from the estimate on every frame of a window resize.
	m = listHeightEnsureVariable(w, "lst", registryTestSpec(100, 250))
	if got := m.Height(40); got != 50 {
		t.Fatalf("measured height dropped on a width change: %v", got)
	}
	// Default 0: absent entry means unscrolled.
	got := w.scrollY().GetOr("lst", 0)
	if want := -(m.Prefix(40) + 7); f32Abs(got-want) > 0.5 {
		t.Fatalf("anchor not preserved across the width change: %v, "+
			"want %v", got, want)
	}
}

func TestListHeightRegistryCountChangeKeepsKeyedHeights(t *testing.T) {
	w := newTestWindow()
	keys := []string{"a", "b", "c"}
	spec := listHeightSpec{
		n: len(keys), estimate: 20, width: 100,
		keyAt: func(i int) string { return keys[i] },
	}
	m := listHeightEnsureVariable(w, "lst", spec)
	for i, k := range keys {
		m.RecordKeyed(i, k, float32(100+i))
	}

	keys = append([]string{"new"}, keys...)
	spec.n = len(keys)
	m = listHeightEnsureVariable(w, "lst", spec)
	if got := m.Height(0); got != 20 {
		t.Fatalf("inserted row = %v, want the estimate", got)
	}
	for i := 1; i < len(keys); i++ {
		if want := float32(100 + i - 1); m.Height(i) != want {
			t.Fatalf("key %q = %v, want %v", keys[i], m.Height(i), want)
		}
	}
}

func TestInvalidateListHeightsResetsAndHoldsAnchor(t *testing.T) {
	w := newTestWindow()
	m := listHeightEnsureVariable(w, "lst", registryTestSpec(50, 100))
	for i := range 50 {
		m.SetHeight(i, 80)
	}
	w.scrollY().Set("lst", -m.Prefix(20))

	w.InvalidateListHeights("lst")
	if got := m.Height(20); got != 20 {
		t.Fatalf("height survived the invalidate: %v", got)
	}
	if got := w.scrollY().GetOr("lst", 0); f32Abs(got+m.Prefix(20)) > 0.5 {
		t.Fatalf("anchor not preserved: %v, want %v", got, -m.Prefix(20))
	}
	// A uniform model has nothing to invalidate and must not panic.
	listHeightRegisterUniform(w, "uni", 10, 24, 0, 0)
	w.InvalidateListHeights("uni")
	w.InvalidateListHeights("nobody")
}

func TestListHeightRegisterUniformDoesNotReallocate(t *testing.T) {
	w := newTestWindow()
	listHeightRegisterUniform(w, "uni", 100, 24, 6, 1)
	m1, _ := listHeightLookup(w, "uni")
	listHeightRegisterUniform(w, "uni", 120, 24, 6, 1)
	m2, _ := listHeightLookup(w, "uni")
	if m1 != m2 {
		t.Fatal("re-registering a uniform model replaced it")
	}
	if m2.Count() != 120 {
		t.Fatalf("count = %d, want 120", m2.Count())
	}
	if m2.Prefix(0) != 6 {
		t.Fatalf("staticTop lost: %v", m2.Prefix(0))
	}
	if m2.indexBase != 1 {
		t.Fatalf("indexBase = %d, want 1", m2.indexBase)
	}
}

func TestListHeightRegistryCapsPerItemStorage(t *testing.T) {
	// Past the cap the model must fall back to uniform rather than
	// allocating tens of megabytes of per-item heights.
	w := newTestWindow()
	m := listHeightEnsureVariable(w, "huge",
		registryTestSpec(listHeightMaxVariable+1, 100))
	if m == nil {
		t.Fatal("no model registered past the cap")
	}
	if !m.uniform {
		t.Fatal("model past the cap kept per-item storage")
	}
	if m.heights != nil || m.tree != nil {
		t.Fatal("capped model allocated per-item storage")
	}
}
