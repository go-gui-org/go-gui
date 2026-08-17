package gui

import (
	"strconv"
	"testing"
)

// scrollIndexListWindow runs a real frame pipeline over a scrollable
// ListBox — the uniform retrofit path — so the index API is tested
// against the same offsets the widget itself virtualizes from.
func scrollIndexListWindow(id string, n int) *Window {
	w := NewWindow(WindowCfg{State: new(int), Width: 300, Height: 400})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{ListBox(ListBoxCfg{
				ID:         id,
				Scrollable: true,
				Height:     200,
				Sizing:     FillFixed,
				Data:       listBoxTestData(n),
			})},
		})
	}
	w.refreshLayout = true
	w.FrameFn()
	return w
}

func TestListHeightModelKeyMatchesScrollKey(t *testing.T) {
	// A model keyed differently from w.scrollY() fails silently — the
	// scroll lands somewhere plausible and wrong — so the equality is
	// asserted rather than assumed.
	w := scrollIndexListWindow("lb-key", 500)
	sc, ok := findScrollLayout(w, "lb-key")
	if !ok {
		t.Fatal("scrollable listbox not found in the layout")
	}
	key := sc.Shape.idKey()
	if _, ok = listHeightLookup(w, key); !ok {
		t.Fatalf("no height model under the scroll key %q", key)
	}
}

func TestScrollToIndexOnNeverScrolledList(t *testing.T) {
	// The regression this pass exists for: layoutAdjustScrollOffsets
	// is the first position pass and zero-fills any scrollable with no
	// stored entry, so an offset written before it is clobbered on a
	// list nobody has scrolled yet.
	w := scrollIndexListWindow("lb-jump", 500)
	m, ok := listHeightLookup(w, "lb-jump")
	if !ok {
		t.Fatal("height model not registered")
	}

	w.ScrollToIndex("lb-jump", 300)
	w.refreshLayout = true
	w.FrameFn()

	// Default 0: absent entry means unscrolled — which is the failure
	// this test is about.
	got := w.scrollY().GetOr("lb-jump", 0)
	if want := -m.Prefix(300); f32Abs(got-want) > 1 {
		t.Fatalf("offset = %v, want %v (row 300 at the viewport top)",
			got, want)
	}
}

func TestScrollIndexIntoViewIsANoOpWhenVisible(t *testing.T) {
	w := scrollIndexListWindow("lb-view", 500)
	before := w.scrollY().GetOr("lb-view", 0)
	w.ScrollIndexIntoView("lb-view", 1)
	w.refreshLayout = true
	w.FrameFn()
	if after := w.scrollY().GetOr("lb-view", 0); after != before {
		t.Fatalf("visible row moved the offset: %v -> %v", before, after)
	}

	// A row below the fold moves by the least amount: its bottom edge
	// lands on the viewport bottom, not its top on the viewport top.
	m, _ := listHeightLookup(w, "lb-view")
	w.ScrollIndexIntoView("lb-view", 40)
	w.refreshLayout = true
	w.FrameFn()
	sc, _ := findScrollLayout(w, "lb-view")
	viewH := sc.Shape.Height - sc.Shape.paddingHeight()
	got := w.scrollY().GetOr("lb-view", 0)
	want := -(m.Prefix(40) + m.Height(40) - viewH)
	if f32Abs(got-want) > 1 {
		t.Fatalf("offset = %v, want %v (nearest-edge landing)", got, want)
	}
}

func TestScrollToEndPinsTheBottom(t *testing.T) {
	w := scrollIndexListWindow("lb-end", 500)
	w.ScrollToEnd("lb-end")
	w.refreshLayout = true
	w.FrameFn()
	sc, _ := findScrollLayout(w, "lb-end")
	got := w.scrollY().GetOr("lb-end", 0)
	if want := scrollMaxOffsetY(sc); f32Abs(got-want) > 1 {
		t.Fatalf("offset = %v, want the bottom %v", got, want)
	}
	if got >= 0 {
		t.Fatalf("offset = %v, want a scrolled-down list", got)
	}
}

func TestScrollToIndexAtFraction(t *testing.T) {
	w := scrollIndexListWindow("lb-frac", 500)
	m, _ := listHeightLookup(w, "lb-frac")
	sc, _ := findScrollLayout(w, "lb-frac")
	viewH := sc.Shape.Height - sc.Shape.paddingHeight()

	w.ScrollToIndexAt("lb-frac", 200, 0.5)
	w.refreshLayout = true
	w.FrameFn()
	got := w.scrollY().GetOr("lb-frac", 0)
	want := -(m.Prefix(200) - 0.5*(viewH-m.Height(200)))
	if f32Abs(got-want) > 1 {
		t.Fatalf("centred offset = %v, want %v", got, want)
	}
}

func TestScrollToIndexConvergesOnAVariableList(t *testing.T) {
	// A far jump into never-measured rows lands estimate-accurate.
	// The request stays live for a few frames so the rows it built get
	// measured and the landing is corrected.
	cfg := virtualListCfg("vl-jump", 600, 200)
	w := virtualListWindow(t, cfg)
	virtualListFrame(w)

	w.ScrollToIndex("vl-jump", 300)
	for range virtualScrollMaxAge + 2 {
		virtualListFrame(w)
	}
	m, _ := listHeightLookup(w, "vl-jump")
	got := w.scrollY().GetOr("vl-jump", 0)
	if want := -m.Prefix(300); f32Abs(got-want) > 2 {
		t.Fatalf("offset = %v, want %v after convergence", got, want)
	}
	// The row itself must be at the viewport top, which is the claim a
	// caller actually cares about.
	row, ok := w.layout.FindByID(ScopeIDN("vl-jump", "row", 300))
	if !ok {
		t.Fatal("row 300 not built after the jump")
	}
	sc, _ := findScrollLayout(w, "vl-jump")
	top := sc.Shape.Y + sc.Shape.PaddingTop()
	if f32Abs(row.Shape.Y-top) > 2 {
		t.Fatalf("row 300 at y=%v, want the viewport top %v",
			row.Shape.Y, top)
	}
	// And the queue drains rather than re-applying forever.
	if len(w.virtualScrolls) != 0 {
		t.Fatalf("pending requests = %d, want 0", len(w.virtualScrolls))
	}
}

func TestUserScrollDropsPendingIndexScroll(t *testing.T) {
	// A variable-height jump stays queued for a few frames so it can
	// converge — but a direct offset write (wheel, scrollbar drag) is
	// the later intent, and the still-pending request must not snap
	// the user back on the next frame.
	cfg := virtualListCfg("vl-drop", 600, 200)
	w := virtualListWindow(t, cfg)
	virtualListFrame(w)

	w.ScrollToIndex("vl-drop", 300)
	virtualListFrame(w)
	if len(w.virtualScrolls) == 0 {
		t.Fatal("request settled immediately; the test needs it pending")
	}

	w.scrollVerticalBy("vl-drop", 120)
	if len(w.virtualScrolls) != 0 {
		t.Fatalf("pending requests = %d after a direct scroll, want 0",
			len(w.virtualScrolls))
	}
	m, _ := listHeightLookup(w, "vl-drop")
	topBefore := m.IndexAt(-w.scrollY().GetOr("vl-drop", 0))
	virtualListFrame(w)
	// The raw offset may legitimately move — the write-back re-anchors
	// as newly built rows are measured — but the row under the viewport
	// top must stay the user's, not snap back to the jump target.
	topAfter := m.IndexAt(-w.scrollY().GetOr("vl-drop", 0))
	if absInt(topAfter-topBefore) > 1 {
		t.Fatalf("viewport top moved row %d -> %d after the user scroll",
			topBefore, topAfter)
	}
	if len(w.virtualScrolls) != 0 {
		t.Fatalf("request re-queued: %d pending", len(w.virtualScrolls))
	}
}

// absInt is a tiny local helper for index distances.
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestScrollToIndexOnTableRespectsFreeze(t *testing.T) {
	// With Freeze the first data row is the header and sits outside
	// the scrollable, which indexBase records. A caller passing a data
	// index must still land on that row.
	data := make([]TableRowCfg, 200)
	for i := range data {
		s := strconv.Itoa(i)
		data[i] = TableRowCfg{ID: "r" + s, Cells: []TableCellCfg{
			{Value: "row " + s}, {Value: "value"},
		}}
	}
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Table(TableCfg{
				ID:           "tbl",
				Scrollable:   true,
				FreezeHeader: true,
				Height:       200,
				Sizing:       FillFixed,
				Data:         data,
			})},
		})
	}
	w.refreshLayout = true
	w.FrameFn()

	scrollID := tableScrollID(&TableCfg{ID: "tbl"}, true)
	m, ok := listHeightLookup(w, scrollID)
	if !ok {
		t.Fatalf("no height model under %q", scrollID)
	}
	if m.indexBase != 1 {
		t.Fatalf("indexBase = %d, want 1 (frozen header)", m.indexBase)
	}
	w.ScrollToIndex(scrollID, 100)
	w.refreshLayout = true
	w.FrameFn()
	got := w.scrollY().GetOr(scrollID, 0)
	// Data index 100 is model item 99: the header is not scrollable.
	if want := -m.Prefix(99); f32Abs(got-want) > 1 {
		t.Fatalf("offset = %v, want %v", got, want)
	}
}
