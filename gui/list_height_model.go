package gui

import "math/bits"

// list_height_model.go — the per-list height model behind
// variable-height virtualization and index-addressed scrolling.
//
// Two shapes share one type:
//
//   - uniform: every row is rowH tall, so every query is arithmetic
//     and nothing is stored per item. This is exactly what the
//     existing ListBox/Table/Tree/Combobox virtualization already
//     assumes, which is why registering it costs those widgets one
//     line and buys them the index-addressed scroll API.
//   - variable: per-item heights in a Fenwick (binary indexed) tree,
//     seeded from an estimate and refined by measurement. Prefix and
//     IndexAt are O(log n), a point update O(log n), a full rebuild
//     O(n).
//
// The tree is float64 while the heights themselves are float32, and
// that is deliberate: at a million rows the running prefix reaches
// ~3e7 px where one float32 ULP is ~4 px. That is visible jitter,
// and — worse — it makes the prefix sequence non-monotonic, so the
// IndexAt search can return an index whose span does not contain y.
//
// Heights are keyed, positions are indexed. heights/tree hold the
// current frame's index order; byKey holds what was actually
// measured, under the caller's ItemKey. Rebuilding the tree from
// byKey is what lets an insert, a delete or a reorder keep every
// measured height on its own item instead of on whatever now sits at
// that index.

// listHeightModel is one list's height model. Pure data plus
// arithmetic — it never touches a Window.
type listHeightModel struct {
	// heights holds the current index order (variable only). It is
	// the "old" side of the delta an incremental update applies.
	heights []float32
	// tree is the Fenwick tree, 1-based, len n+1 (variable only).
	tree []float64
	// byKey holds measured heights under the caller's ItemKey, so
	// they survive a reorder. Nil when the list is index-keyed.
	byKey map[string]float32

	// heightFn is the caller's cheap path: when supplied, an item's
	// height is computed rather than measured, so a new item is seeded
	// exactly and the write-back is unnecessary. Stored on the model
	// rather than captured in a closure so re-registering it every
	// frame costs no allocation.
	heightFn func(i int, width float32) float32

	// keyAt is the caller's identity function. Held on the model so
	// both the rebuild and the measurement write-back read one source.
	keyAt func(i int) string

	rowH      float32 // uniform only
	staticTop float32 // content above item 0 inside the scrollable
	estimate  float32 // seed for items never measured
	width     float32 // inner width the heights were measured at

	// measuredW and viewportH are what the arrange pass recorded for
	// this list last frame — the same AmendLayout hook that writes row
	// heights back. The view phase cannot know either: under Fill
	// sizing both are decided after generation. hSeen distinguishes
	// "not laid out yet" from "laid out to nothing".
	measuredW float32
	viewportH float32

	// builtFirst/builtCount/leadOffset describe the row window the
	// view phase emitted, so the write-back can map a child position
	// back to an item index without parsing row IDs.
	builtFirst int
	builtCount int
	leadOffset int

	n         int
	indexBase int // caller index of model item 0 (table freeze)

	// widthRun counts consecutive frames the list widened while the
	// window stood still, and lastWinW is the window width those
	// frames were seen at. Together they separate a resize, which
	// stops, from a row that demands the width it was handed, which
	// does not; see virtualListNoteWidth.
	widthRun int
	lastWinW int

	themeID uint64
	uniform bool
	hSeen   bool
}

const (
	// listHeightMinRow floors a measured height. A zero would let the
	// IndexAt walk yield first > last, and a negative one would break
	// the tree's monotonicity outright.
	listHeightMinRow = float32(1)

	// listHeightEps is the write-back deadband, in pixels. Below it a
	// measurement is treated as noise: writing it would churn the tree
	// and re-anchor the scroll offset every frame for no visible gain.
	listHeightEps = float32(0.5)

	// listHeightMaxVariable caps per-item storage. Above it the model
	// falls back to uniform (12 B/item is ~50 MB at this size) and the
	// registry warns under Debug.
	listHeightMaxVariable = 1 << 22

	// listHeightKeyLoadFactor bounds byKey relative to the live item
	// count, so a long-running feed cannot grow the measured-height
	// map without limit.
	listHeightKeyLoadFactor = 4
)

// newUniformHeightModel returns an O(1) model for a list whose rows
// all have the same height.
func newUniformHeightModel(n int, rowH, staticTop float32, indexBase int) *listHeightModel {
	return &listHeightModel{
		uniform:   true,
		rowH:      f32Max(rowH, listHeightMinRow),
		staticTop: staticTop,
		estimate:  rowH,
		n:         max(n, 0),
		indexBase: indexBase,
	}
}

// newVariableHeightModel returns a model with per-item storage, every
// item seeded at estimate.
func newVariableHeightModel(n int, estimate, staticTop float32) *listHeightModel {
	m := &listHeightModel{
		estimate:  f32Max(estimate, listHeightMinRow),
		staticTop: staticTop,
	}
	m.Resize(n)
	return m
}

// Count returns the number of items the model covers.
func (m *listHeightModel) Count() int { return m.n }

// Total returns the full content height, staticTop included.
func (m *listHeightModel) Total() float32 { return m.Prefix(m.n) }

// Prefix returns the offset of item i from the top of the scrollable
// content: staticTop plus the heights of items [0, i). i is clamped,
// so Prefix(0) is staticTop and Prefix(n) is the total.
func (m *listHeightModel) Prefix(i int) float32 {
	i = clampInt(i, 0, m.n)
	if m.uniform {
		return m.staticTop + float32(i)*m.rowH
	}
	sum := float64(0)
	for j := i; j > 0; j -= j & (-j) {
		sum += m.tree[j]
	}
	return m.staticTop + float32(sum)
}

// Height returns item i's height, or the estimate when i is out of
// range (a caller mid-mutation may ask about an item the model has
// not seen yet).
func (m *listHeightModel) Height(i int) float32 {
	if m.uniform {
		return m.rowH
	}
	if i < 0 || i >= m.n {
		return m.estimate
	}
	return m.heights[i]
}

// IndexAt returns the item whose span contains y, where y is measured
// from the top of the scrollable content (the same origin as Prefix).
// y above the first item returns 0; y past the end returns n-1. An
// empty model returns 0.
func (m *listHeightModel) IndexAt(y float32) int {
	if m.n == 0 {
		return 0
	}
	y -= m.staticTop
	if y <= 0 {
		return 0
	}
	if m.uniform {
		// Same boundary tolerance as the variable path below: i*rowH
		// rounded to float32 and divided back can land just under i.
		return clampInt(int(float64(y)/float64(m.rowH)+
			(1.0/(1<<23))), 0, m.n-1)
	}
	// Fenwick binary lifting: walk the highest power-of-two stride
	// down, taking a step whenever the running prefix stays <= y. The
	// result is the count of items that fit strictly above y, i.e. the
	// index of the item containing it.
	//
	// y arrives as a float32 — a stored scroll offset, or a Prefix
	// result rounded on its way out — while the tree accumulates in
	// float64. On an exact row boundary that rounding lands a hair
	// *below* the boundary and the walk answers with the row above.
	// One ULP of error, a whole row of displacement: the anchor the
	// write-back re-seats on is then the wrong row, and the correction
	// moves the reader by that row's height. The tolerance is one
	// float32 ULP of y plus a sub-pixel floor.
	target := float64(y)
	eps := target*(1.0/(1<<23)) + 1.0/1024
	idx := 0
	running := float64(0)
	for stride := m.highBit(); stride > 0; stride >>= 1 {
		next := idx + stride
		if next > m.n {
			continue
		}
		if running+m.tree[next] <= target+eps {
			idx = next
			running += m.tree[next]
		}
	}
	return clampInt(idx, 0, m.n-1)
}

// highBit returns the largest power of two <= n, the starting stride
// for the IndexAt walk.
func (m *listHeightModel) highBit() int {
	if m.n < 1 {
		return 1
	}
	return 1 << (bits.Len(uint(m.n)) - 1)
}

// SetHeight records a measured height for item i and returns the
// signed change applied. A change smaller than the deadband is
// ignored and returns 0, so a stable list stops writing after the
// first measured frame.
func (m *listHeightModel) SetHeight(i int, h float32) float32 {
	if m.uniform || i < 0 || i >= m.n || !f32IsFinite(h) {
		return 0
	}
	h = f32Max(h, listHeightMinRow)
	old := m.heights[i]
	delta := h - old
	if f32Abs(delta) < listHeightEps {
		return 0
	}
	m.heights[i] = h
	m.add(i, float64(delta))
	return delta
}

// add applies a delta to item i in the Fenwick tree.
func (m *listHeightModel) add(i int, delta float64) {
	for j := i + 1; j <= m.n; j += j & (-j) {
		m.tree[j] += delta
	}
}

// Resize grows or shrinks the model to n items. Growth seeds the new
// items at the estimate, reuses the heights capacity when it can, and
// extends the Fenwick tree in place — an existing node's span does
// not change when the tree grows — so the append-only path (a feed
// appending at the bottom) costs O(added · log n) with no per-append
// reallocation, never a full rebuild.
func (m *listHeightModel) Resize(n int) {
	n = max(n, 0)
	if m.uniform {
		m.n = n
		return
	}
	if n == m.n && m.tree != nil {
		return
	}
	if n < m.n {
		// Shrink: rebuilding is O(n) and simpler than unwinding the
		// tree's partial sums item by item.
		m.heights = m.heights[:n]
		m.n = n
		m.rebuildTree()
		return
	}
	if cap(m.heights) >= n {
		m.heights = m.heights[:n]
	} else {
		grown := make([]float32, n, growCap(cap(m.heights), n))
		copy(grown, m.heights)
		m.heights = grown
	}
	for i := m.n; i < n; i++ {
		m.heights[i] = m.seed(i)
	}
	m.n = n
	if len(m.tree) <= 1 {
		// Never built (or shrunk to nothing): the O(n) rebuild beats
		// n incremental O(log n) insertions.
		m.rebuildTree()
	} else {
		m.growTree()
	}
}

// growTree appends Fenwick nodes for items seeded since the tree was
// last built, leaving existing nodes untouched. Node i covers the
// span (i-lowbit(i), i], which decomposes into heights[i-1] plus the
// already-final nodes i-1, i-2, i-4, … above the span's start — so
// each appended node costs O(log n).
func (m *listHeightModel) growTree() {
	if len(m.tree) == 0 {
		m.tree = append(m.tree[:0], 0)
	}
	for i := len(m.tree); i <= m.n; i++ {
		s := float64(m.heights[i-1])
		for step := 1; step < i&(-i); step <<= 1 {
			s += m.tree[i-step]
		}
		m.tree = append(m.tree, s)
	}
}

// rebuildTree rebuilds the Fenwick tree from heights in O(n) by
// propagating each node into its parent once — never n·log n.
func (m *listHeightModel) rebuildTree() {
	if cap(m.tree) >= m.n+1 {
		m.tree = m.tree[:m.n+1]
		clear(m.tree)
	} else {
		m.tree = make([]float64, m.n+1)
	}
	for i := 1; i <= m.n; i++ {
		m.tree[i] += float64(m.heights[i-1])
	}
	for i := 1; i <= m.n; i++ {
		if parent := i + (i & (-i)); parent <= m.n {
			m.tree[parent] += m.tree[i]
		}
	}
}

// ResetToEstimate drops every measured height and reseeds the model.
// Used when the measurement premise changed — a width or theme change
// invalidates heights that are still perfectly well-formed.
func (m *listHeightModel) ResetToEstimate(estimate float32) {
	m.estimate = f32Max(estimate, listHeightMinRow)
	if m.uniform {
		m.rowH = m.estimate
		return
	}
	m.reseed()
	clear(m.byKey)
}

// RebuildFromKeys re-seats the model at n items, taking each item's
// height from byKey under keyAt(i) and falling back to the estimate
// for keys never measured. This is what makes an insert, a delete or
// a reorder free: heights follow their key, not their index.
//
// It also bounds byKey — when the measured map has outgrown the live
// count by the load factor, it is replaced by one holding only the
// live keys, so a long-running feed does not accumulate forever.
func (m *listHeightModel) RebuildFromKeys(n int) {
	n = max(n, 0)
	if m.uniform || m.keyAt == nil || m.byKey == nil {
		m.Resize(n)
		return
	}
	if m.heightFn != nil {
		// The caller computes heights, so a rebuild is a reseed and
		// the keyed store has nothing better to offer.
		m.Resize(n)
		m.reseed()
		return
	}
	if cap(m.heights) >= n {
		m.heights = m.heights[:n]
	} else {
		m.heights = make([]float32, n)
	}
	evict := len(m.byKey) > listHeightKeyLoadFactor*max(n, 1)
	var live map[string]float32
	if evict {
		live = make(map[string]float32, n)
	}
	for i := range n {
		k := m.keyAt(i)
		h, ok := m.byKey[k]
		if !ok || h <= 0 {
			h = m.seed(i)
		}
		m.heights[i] = h
		if evict && ok {
			live[k] = h
		}
	}
	if evict {
		m.byKey = live
	}
	m.n = n
	m.rebuildTree()
}

// RecordKeyed writes a measured height through both the indexed tree
// and the keyed store, so the measurement survives the next rebuild.
// Returns the change the tree took, for the re-anchor delta.
func (m *listHeightModel) RecordKeyed(i int, key string, h float32) float32 {
	delta := m.SetHeight(i, h)
	m.recordKey(key, h)
	return delta
}

// recordKey writes a measured height into the keyed store alone. The
// write-back calls it only when the tree actually changed, so a
// settled list stops paying keyAt and a map write per row per frame.
func (m *listHeightModel) recordKey(key string, h float32) {
	if m.byKey != nil && key != "" && f32IsFinite(h) {
		m.byKey[key] = f32Max(h, listHeightMinRow)
	}
}

// seed returns the height a never-measured item starts at: the
// caller's own answer when it has one, the estimate otherwise.
func (m *listHeightModel) seed(i int) float32 {
	if m.heightFn != nil {
		if h := m.heightFn(i, m.width); h > 0 && f32IsFinite(h) {
			return f32Max(h, listHeightMinRow)
		}
	}
	return m.estimate
}

// reseed recomputes every height from seed: the caller's height
// function when there is one, the estimate otherwise. O(n) arithmetic
// plus a tree rebuild.
func (m *listHeightModel) reseed() {
	for i := range m.heights {
		m.heights[i] = m.seed(i)
	}
	m.rebuildTree()
}

// enableKeys allocates the keyed store. Called when the caller
// supplies an ItemKey; an index-keyed list never pays for it.
func (m *listHeightModel) enableKeys(n int) {
	if m.byKey == nil {
		m.byKey = make(map[string]float32, max(n, 8))
	}
}

// clampInt clamps v into [lo, hi]. hi < lo returns lo.
func clampInt(v, lo, hi int) int {
	if hi < lo {
		return lo
	}
	return min(max(v, lo), hi)
}
