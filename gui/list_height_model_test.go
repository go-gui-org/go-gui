package gui

import (
	"math/rand"
	"testing"
)

// naivePrefix is the reference the Fenwick tree is checked against.
func naivePrefix(heights []float32, staticTop float32, i int) float32 {
	sum := float64(staticTop)
	for j := 0; j < i && j < len(heights); j++ {
		sum += float64(heights[j])
	}
	return float32(sum)
}

func TestListHeightModelPrefixMatchesNaive(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	const n = 500
	m := newVariableHeightModel(n, 20, 13)
	heights := make([]float32, n)
	for i := range n {
		heights[i] = float32(4 + rng.Intn(400))
		m.SetHeight(i, heights[i])
	}
	for i := 0; i <= n; i++ {
		got := m.Prefix(i)
		want := naivePrefix(heights, 13, i)
		if f32Abs(got-want) > 0.5 {
			t.Fatalf("Prefix(%d) = %v, want %v", i, got, want)
		}
	}
	if f32Abs(m.Total()-naivePrefix(heights, 13, n)) > 0.5 {
		t.Fatalf("Total = %v", m.Total())
	}
}

func TestListHeightModelIndexAtMonotonic(t *testing.T) {
	rng := rand.New(rand.NewSource(11))
	const n = 300
	m := newVariableHeightModel(n, 20, 7)
	for i := range n {
		// Fractional, deliberately: whole-pixel heights give prefixes
		// that are exact in float32, which hides every rounding
		// question the boundary assertion below exists to ask.
		m.SetHeight(i, float32(4+rng.Intn(120))+rng.Float32())
	}
	// Every item's own span must resolve back to that item.
	for i := range n {
		top := m.Prefix(i)
		mid := top + m.Height(i)/2
		if got := m.IndexAt(mid); got != i {
			t.Fatalf("IndexAt(mid of %d) = %d", i, got)
		}
		if got := m.IndexAt(top + 0.01); got != i {
			t.Fatalf("IndexAt(top of %d) = %d", i, got)
		}
		// Exactly on the boundary, which is the case that matters: the
		// scroll offset a re-anchor stores *is* a Prefix, rounded to
		// float32 on its way out while the tree sums in float64. Round
		// low by one ULP and the walk answers with the row above — one
		// row of displacement at the anchor, from an error too small to
		// see. Nudging by 0.01 first, as the assertion above does, hides
		// it.
		if got := m.IndexAt(top); got != i {
			t.Fatalf("IndexAt(Prefix(%d)) = %d — boundary rounding", i, got)
		}
	}
	// Out of range clamps rather than walking off either end.
	if got := m.IndexAt(-5000); got != 0 {
		t.Fatalf("IndexAt(negative) = %d, want 0", got)
	}
	if got := m.IndexAt(m.Total() + 5000); got != n-1 {
		t.Fatalf("IndexAt(past end) = %d, want %d", got, n-1)
	}
	// Monotonic across a fine sweep.
	prev := 0
	for y := float32(0); y < m.Total(); y += 3 {
		cur := m.IndexAt(y)
		if cur < prev {
			t.Fatalf("IndexAt not monotonic at y=%v: %d after %d", y, cur, prev)
		}
		prev = cur
	}
}

func TestListHeightModelUniformIsExact(t *testing.T) {
	m := newUniformHeightModel(1000, 24, 10, 0)
	if got := m.Prefix(0); got != 10 {
		t.Fatalf("Prefix(0) = %v, want staticTop 10", got)
	}
	if got := m.Prefix(4); got != 10+4*24 {
		t.Fatalf("Prefix(4) = %v", got)
	}
	if got := m.IndexAt(10 + 24*7.5); got != 7 {
		t.Fatalf("IndexAt = %d, want 7", got)
	}
	if got := m.IndexAt(0); got != 0 {
		t.Fatalf("IndexAt(0) = %d, want 0", got)
	}
	// A uniform model stores nothing per item.
	if m.heights != nil || m.tree != nil {
		t.Fatal("uniform model allocated per-item storage")
	}
}

func TestListHeightModelResizeKeepsMeasured(t *testing.T) {
	m := newVariableHeightModel(10, 20, 0)
	for i := range 10 {
		m.SetHeight(i, float32(30+i))
	}
	m.Resize(15)
	for i := range 10 {
		if got := m.Height(i); got != float32(30+i) {
			t.Fatalf("grow lost height %d: %v", i, got)
		}
	}
	for i := 10; i < 15; i++ {
		if got := m.Height(i); got != 20 {
			t.Fatalf("grown item %d = %v, want estimate 20", i, got)
		}
	}
	if want := naivePrefix(m.heights, 0, 15); f32Abs(m.Total()-want) > 0.5 {
		t.Fatalf("Total after grow = %v, want %v", m.Total(), want)
	}
	m.Resize(4)
	if m.Count() != 4 {
		t.Fatalf("Count after shrink = %d", m.Count())
	}
	if want := naivePrefix(m.heights, 0, 4); f32Abs(m.Total()-want) > 0.5 {
		t.Fatalf("Total after shrink = %v, want %v", m.Total(), want)
	}
}

func TestListHeightModelRebuildEqualsIncremental(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	const n = 257 // not a power of two, so the lifting walk skips
	inc := newVariableHeightModel(n, 20, 0)
	direct := newVariableHeightModel(n, 20, 0)
	for i := range n {
		h := float32(5 + rng.Intn(90))
		inc.SetHeight(i, h)
		direct.heights[i] = h
	}
	direct.rebuildTree()
	for i := 0; i <= n; i++ {
		if f32Abs(inc.Prefix(i)-direct.Prefix(i)) > 0.5 {
			t.Fatalf("prefix mismatch at %d: %v vs %v",
				i, inc.Prefix(i), direct.Prefix(i))
		}
	}
}

func TestListHeightModelKeysSurviveReorder(t *testing.T) {
	keys := []string{"a", "b", "c", "d"}
	m := newVariableHeightModel(len(keys), 20, 0)
	m.enableKeys(len(keys))
	want := map[string]float32{"a": 100, "b": 200, "c": 300, "d": 400}
	for i, k := range keys {
		m.RecordKeyed(i, k, want[k])
	}

	// Insert at the front: every measured height must follow its key.
	keys = append([]string{"z"}, keys...)
	m.keyAt = func(i int) string { return keys[i] }
	m.RebuildFromKeys(len(keys))
	if got := m.Height(0); got != 20 {
		t.Fatalf("new item took a neighbour's height: %v", got)
	}
	for i := 1; i < len(keys); i++ {
		if got := m.Height(i); got != want[keys[i]] {
			t.Fatalf("key %q height = %v, want %v",
				keys[i], got, want[keys[i]])
		}
	}

	// Full reverse: same requirement, no index survives.
	for i, j := 0, len(keys)-1; i < j; i, j = i+1, j-1 {
		keys[i], keys[j] = keys[j], keys[i]
	}
	m.keyAt = func(i int) string { return keys[i] }
	m.RebuildFromKeys(len(keys))
	for i := range keys {
		w, ok := want[keys[i]]
		if !ok {
			w = 20
		}
		if got := m.Height(i); got != w {
			t.Fatalf("after reverse, key %q = %v, want %v", keys[i], got, w)
		}
	}
}

func TestListHeightModelKeyEvictionBounded(t *testing.T) {
	m := newVariableHeightModel(1, 20, 0)
	m.enableKeys(1)
	// Measure 400 keys but keep only two live.
	for i := range 400 {
		m.n = 1
		m.RecordKeyed(0, string(rune('A'+i%26))+itoa(i), float32(30+i%40))
	}
	if len(m.byKey) < 100 {
		t.Fatalf("precondition: byKey too small (%d)", len(m.byKey))
	}
	live := []string{"A0", "B1"}
	m.keyAt = func(i int) string { return live[i] }
	m.RebuildFromKeys(len(live))
	if len(m.byKey) > listHeightKeyLoadFactor*len(live) {
		t.Fatalf("byKey not evicted: %d entries for %d live",
			len(m.byKey), len(live))
	}
}

func TestListHeightModelDeadband(t *testing.T) {
	m := newVariableHeightModel(3, 20, 0)
	if d := m.SetHeight(1, 60); d != 40 {
		t.Fatalf("first write delta = %v, want 40", d)
	}
	if d := m.SetHeight(1, 60.2); d != 0 {
		t.Fatalf("sub-deadband write returned %v, want 0", d)
	}
	if got := m.Height(1); got != 60 {
		t.Fatalf("deadband write mutated height: %v", got)
	}
	// A height below the floor is clamped, never zero or negative.
	m.SetHeight(2, -5)
	if got := m.Height(2); got != listHeightMinRow {
		t.Fatalf("negative height = %v, want %v", got, listHeightMinRow)
	}
}

func TestListHeightModelGrowTreeEqualsRebuild(t *testing.T) {
	// Growth extends the Fenwick tree in place (growTree); its sums
	// must match a from-scratch rebuild over the same heights.
	m := newVariableHeightModel(100, 20, 0)
	for i := range 100 {
		m.SetHeight(i, float32(5+i%37))
	}
	m.Resize(150) // append-only growth, twice
	m.Resize(1000)

	direct := newVariableHeightModel(1000, 20, 0)
	copy(direct.heights, m.heights)
	direct.rebuildTree()
	for i := 0; i <= 1000; i += 7 {
		if f32Abs(m.Prefix(i)-direct.Prefix(i)) > 0.5 {
			t.Fatalf("prefix mismatch at %d: %v vs %v",
				i, m.Prefix(i), direct.Prefix(i))
		}
	}
}

func TestListHeightModelEmptyAndOutOfRange(t *testing.T) {
	m := newVariableHeightModel(0, 20, 5)
	if got := m.IndexAt(100); got != 0 {
		t.Fatalf("IndexAt on empty model = %d, want 0", got)
	}
	if got := m.Total(); got != 5 {
		t.Fatalf("Total of empty model = %v, want the staticTop 5", got)
	}
	if got := m.Height(3); got != 20 {
		t.Fatalf("out-of-range Height = %v, want the estimate 20", got)
	}
	if got := m.SetHeight(3, 40); got != 0 {
		t.Fatalf("out-of-range SetHeight applied a delta %v, want 0", got)
	}

	m.Resize(10)
	// y above the content and y past the end clamp to the ends.
	if got := m.IndexAt(-50); got != 0 {
		t.Fatalf("IndexAt above content = %d, want 0", got)
	}
	if got := m.IndexAt(1e9); got != 9 {
		t.Fatalf("IndexAt past the end = %d, want 9", got)
	}
	// Prefix clamps its argument at both ends.
	if got := m.Prefix(-3); got != m.Prefix(0) {
		t.Fatalf("Prefix(-3) = %v, want Prefix(0) = %v", got, m.Prefix(0))
	}
	if got := m.Prefix(99); got != m.Total() {
		t.Fatalf("Prefix past the end = %v, want Total %v", got, m.Total())
	}
}

// itoa is a tiny local helper so the test needs no strconv import
// alongside the repo's own conventions.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
