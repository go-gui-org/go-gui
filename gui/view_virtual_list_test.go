package gui

import (
	"math"
	"strings"
	"testing"
)

// virtualListRowH is the test corpus: a tall row every tenth item, so
// no single scalar row height describes the list and the arithmetic
// the uniform path uses would land in the wrong place.
func virtualListRowH(i int) float32 {
	if i%10 == 0 {
		return 400
	}
	return 20
}

// virtualListCfg builds a list over n rows of virtualListRowH. Each
// row is a fixed-height rectangle inside an identified container, so
// a test can find a named row's arranged position.
func virtualListCfg(id string, n int, h float32) VirtualListCfg {
	return VirtualListCfg{
		ID:        id,
		ItemCount: n,
		Height:    h,
		// FillFixed, not a bare Height: a Fit-sized container grows to
		// its content, and a scrollable that grew has nothing to clip.
		Sizing:  FillFixed,
		Padding: PaddingNone,
		ItemView: func(i int, _ float32) View {
			return Column(ContainerCfg{
				ID:         ScopeIDN(id, "row", i),
				SizeBorder: NoBorder,
				Padding:    NoPadding,
				Sizing:     FillFixed,
				Height:     virtualListRowH(i),
				Content: []View{Rectangle(RectangleCfg{
					Color:  RGB(20, 20, 20),
					Height: virtualListRowH(i),
					Sizing: FillFixed,
				})},
			})
		},
	}
}

func TestVirtualListBuildsOnlyTheVisibleRange(t *testing.T) {
	w := newTestWindow()
	cfg := virtualListCfg("vl-range", 10_000, 300)
	// The cheap path: heights are exact from frame 1, so the range is
	// too and the test does not depend on convergence.
	cfg.ItemHeight = func(i int, _ float32) float32 {
		return virtualListRowH(i)
	}
	v := VirtualList(cfg)

	layout := generateViewLayout(v, w)
	if n := len(layout.Children); n > 120 {
		t.Fatalf("children = %d, want the visible range only", n)
	}
	// Row 0 is 400 px tall, so a 300 px viewport with half a viewport
	// of overscan cannot reach row 2.
	if n := len(layout.Children); n < 2 {
		t.Fatalf("children = %d, want rows plus a trailing spacer", n)
	}
	last := layout.Children[len(layout.Children)-1]
	if !last.Shape.Color.eq(ColorTransparent) {
		t.Error("trailing spacer missing")
	}

	m, ok := listHeightLookup(w, "vl-range")
	if !ok {
		t.Fatal("height model not registered")
	}
	// 1000 tall rows + 9000 short ones.
	if want := float32(1000*400 + 9000*20); f32Abs(m.Total()-want) > 1 {
		t.Fatalf("modelled content height = %v, want %v", m.Total(), want)
	}
}

func TestVirtualListNaNOverscanFallsBackToDefault(t *testing.T) {
	// NaN escapes an `<= 0` check, and a NaN overscan would poison
	// every range comparison into false — first and last both 0.
	w := newTestWindow()
	cfg := virtualListCfg("vl-nan", 1000, 200)
	cfg.OverscanPx = float32(math.NaN())
	cfg.ItemHeight = func(i int, _ float32) float32 {
		return virtualListRowH(i)
	}
	generateViewLayout(VirtualList(cfg), w)

	m, _ := listHeightLookup(w, "vl-nan")
	w.scrollY().Set("vl-nan", -m.Prefix(500))
	first, last, _, _, probe := virtualListRange(&cfg, m, w)
	if probe {
		t.Fatal("configured Height must not take the probe path")
	}
	if first == 0 || last <= first {
		t.Fatalf("range [%d, %d] ignored the scroll position", first, last)
	}
}

func TestVirtualListScrolledRangeSkipsToTheRightRow(t *testing.T) {
	w := newTestWindow()
	cfg := virtualListCfg("vl-scroll", 1000, 200)
	cfg.OverscanPx = 1 // pin the window to the viewport itself
	cfg.ItemHeight = func(i int, _ float32) float32 {
		return virtualListRowH(i)
	}
	v := VirtualList(cfg)
	generateViewLayout(v, w) // register the model

	m, _ := listHeightLookup(w, "vl-scroll")
	// Put the viewport top exactly on row 25.
	w.scrollY().Set("vl-scroll", -m.Prefix(25))

	first, last, lead, _, probe := virtualListRange(&cfg, m, w)
	if probe {
		t.Fatal("configured Height must not take the probe path")
	}
	// A 1 px overscan still reaches one row above the viewport top,
	// so the window starts at 24 and must cover 25.
	if first != 24 {
		t.Fatalf("first = %d, want 24", first)
	}
	if last < 25 {
		t.Fatalf("last = %d, want at least 25", last)
	}
	if f32Abs(lead-m.Prefix(first)) > 0.5 {
		t.Fatalf("leading spacer = %v, want %v", lead, m.Prefix(first))
	}
}

func TestVirtualListProbesWhenHeightUnresolved(t *testing.T) {
	w := newTestWindow()
	cfg := virtualListCfg("vl-probe", 100_000, 0)
	cfg.Sizing = FillFill
	cfg.ItemHeight = func(i int, _ float32) float32 {
		return virtualListRowH(i)
	}
	v := VirtualList(cfg)

	// Frame 1: no arranged height. The ListBox precedent builds every
	// row here; at this corpus size that is a hang, so the probe
	// window is the whole point.
	first := generateViewLayout(v, w)
	if n := len(first.Children); n > virtualListProbeRows+2 {
		t.Fatalf("frame 1 children = %d, want a bounded probe", n)
	}
	if len(first.Children) < 2 {
		t.Fatal("probe frame built nothing")
	}

	// Arrange resolves a height; the amend hook records it.
	first.Shape.Height = 300
	first.Shape.Width = 200
	first.Shape.events.AmendLayout(EventCtx{&first, nil, w})

	m, _ := listHeightLookup(w, "vl-probe")
	if want := 300 - first.Shape.paddingHeight(); m.viewportH != want {
		t.Fatalf("recorded viewport height = %v, want %v",
			m.viewportH, want)
	}
	_, _, _, _, probe := virtualListRange(&cfg, m, w)
	if probe {
		t.Fatal("still probing after arrange recorded a height")
	}
}

// virtualListWindow runs a real frame pipeline over a virtual list,
// which is what makes the measurement write-back observable: the
// heights come from arrange, not from the view phase.
func virtualListWindow(t *testing.T, cfg VirtualListCfg) *Window {
	t.Helper()
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	w.viewGenerator = func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{VirtualList(cfg)},
		})
	}
	return w
}

func virtualListFrame(w *Window) {
	w.refreshLayout = true
	w.FrameFn()
}

func TestVirtualListMeasurementConverges(t *testing.T) {
	cfg := virtualListCfg("vl-measure", 500, 200)
	w := virtualListWindow(t, cfg)

	virtualListFrame(w)
	m, ok := listHeightLookup(w, "vl-measure")
	if !ok {
		t.Fatal("height model not registered")
	}
	// Frame 1 estimates every row; frame 1's amend measures the built
	// ones. Row 0 is 400 px and no estimate is close to that.
	if got := m.Height(0); f32Abs(got-400) > 1 {
		t.Fatalf("row 0 measured %v after one frame, want 400", got)
	}
	virtualListFrame(w)
	if got := m.Height(1); f32Abs(got-20) > 1 {
		t.Fatalf("row 1 measured %v, want 20", got)
	}
	// A settled list stops writing: a third frame must not move
	// anything the second frame already recorded.
	before := m.Total()
	virtualListFrame(w)
	if f32Abs(m.Total()-before) > 0.5 {
		t.Fatalf("total moved on a settled frame: %v -> %v",
			before, m.Total())
	}
}

func TestVirtualListWriteBackHoldsTheAnchorRow(t *testing.T) {
	// The layout-level guard on the re-anchor rule: a row under the
	// viewport top must not move when the rows above it are re-
	// measured. Keying the correction on `first` instead of on the
	// anchor makes this fail — the delta there is structurally zero,
	// because rows above the built window sit inside the leading
	// spacer and are never measured, so the content slides instead.
	cfg := virtualListCfg("vl-anchor", 500, 200)
	w := virtualListWindow(t, cfg)
	virtualListFrame(w)

	m, _ := listHeightLookup(w, "vl-anchor")
	// Land the viewport top on row 41 by hand rather than through
	// ScrollToIndex: a queued jump keeps re-targeting as the model
	// converges, which is a different behaviour (see the scroll-index
	// tests) and would mask this one.
	w.scrollY().Set("vl-anchor", -m.Prefix(41))
	virtualListFrame(w) // builds rows around 41 and measures them

	anchorID := ScopeIDN("vl-anchor", "row", 41)
	yOf := func() (float32, bool) {
		ly, ok := w.layout.FindByID(anchorID)
		if !ok {
			return 0, false
		}
		return ly.Shape.Y, true
	}
	y1, ok := yOf()
	if !ok {
		t.Fatalf("anchor row %q not built", anchorID)
	}
	virtualListFrame(w)
	y2, ok := yOf()
	if !ok {
		t.Fatalf("anchor row %q left the built range", anchorID)
	}
	if f32Abs(y1-y2) > 1 {
		t.Fatalf("anchor row moved across the write-back frame: "+
			"%v -> %v", y1, y2)
	}
}

func TestVirtualListItemKeySurvivesInsert(t *testing.T) {
	keys := []string{"a", "b", "c", "d", "e"}
	cfg := VirtualListCfg{
		ID:        "vl-key",
		Height:    200,
		ItemCount: len(keys),
		ItemKey:   func(i int) string { return keys[i] },
		ItemView: func(i int, _ float32) View {
			return Rectangle(RectangleCfg{
				Height: 30, Sizing: FillFixed,
			})
		},
	}
	w := newTestWindow()
	generateViewLayout(VirtualList(cfg), w)
	m, ok := listHeightLookup(w, "vl-key")
	if !ok {
		t.Fatal("model not registered")
	}
	for i, k := range keys {
		m.RecordKeyed(i, k, float32(100+10*i))
	}

	// Prepend, then re-generate: every measured height must still be
	// on its own key.
	keys = append([]string{"z"}, keys...)
	cfg.ItemCount = len(keys)
	generateViewLayout(VirtualList(cfg), w)
	for i := 1; i < len(keys); i++ {
		want := float32(100 + 10*(i-1))
		if got := m.Height(i); f32Abs(got-want) > 0.5 {
			t.Fatalf("key %q height = %v, want %v", keys[i], got, want)
		}
	}
}

func TestVirtualListFocusedIndexNavigates(t *testing.T) {
	cfg := virtualListCfg("vl-focus", 100, 200)
	w := virtualListWindow(t, cfg)
	virtualListFrame(w)

	if got := w.VirtualListFocusedIndex("vl-focus"); got != 0 {
		t.Fatalf("initial focused index = %d, want 0", got)
	}
	if got := virtualListNavigate(KeyDown, 0, 100); got != 1 {
		t.Fatalf("KeyDown = %d, want 1", got)
	}
	if got := virtualListNavigate(KeyEnd, 0, 100); got != 99 {
		t.Fatalf("KeyEnd = %d, want 99", got)
	}
	if got := virtualListNavigate(KeyUp, 0, 100); got != 0 {
		t.Fatalf("KeyUp at the top = %d, want 0", got)
	}

	w.SetVirtualListFocusedIndex("vl-focus", 60)
	virtualListFrame(w)
	if got := w.VirtualListFocusedIndex("vl-focus"); got != 60 {
		t.Fatalf("focused index = %d, want 60", got)
	}
	// Moving focus must have brought the row into view.
	m, _ := listHeightLookup(w, "vl-focus")
	// Default 0: absent entry means unscrolled.
	off := w.scrollY().GetOr("vl-focus", 0)
	if -off > m.Prefix(60) || -off+m.viewportH < m.Prefix(60) {
		t.Fatalf("row 60 not in view: offset %v, row top %v, viewport %v",
			off, m.Prefix(60), m.viewportH)
	}
}

func TestVirtualListHasNoDuplicateIDs(t *testing.T) {
	// The spacers carry no ID and the rows carry one each, so a
	// rendered list must be clean. A spacer that acquired the list's
	// ID, or a row window that built an index twice, shows up here.
	cfg := virtualListCfg("vl-ids", 400, 200)
	w := virtualListWindow(t, cfg)
	virtualListFrame(w)
	if found := w.TestDuplicateIDs(); len(found) != 0 {
		t.Fatalf("duplicate effective IDs: %v", found)
	}
}

func TestVirtualListWidthRatchetWarns(t *testing.T) {
	// The failure this catches has no error state to observe: rows
	// that turn the width they are handed into a width they demand
	// widen the list a little every frame, and because a width change
	// re-wraps them, the list never settles. examples/virtual_list
	// shipped with exactly that bug — MinWidth: width - padding, which
	// omits the row's own border — so the widget reports it.
	//
	// Driven through the recorder rather than a laid-out list: what
	// makes a row overflow its allocation is a property of the app's
	// whole tree, and the rule under test is only "growth the window
	// does not explain, on consecutive frames".
	prevOn := DebugEnabled()
	Debug(true)
	defer Debug(prevOn)

	w := newTestWindow()
	var found []string
	w.debug.collect = &found
	m := &listHeightModel{}
	w.windowWidth = 800
	for i := range virtualListWidthRatchetRuns + 1 {
		virtualListNoteWidth(w, m, "vl-ratchet", 400+float32(i)*3)
	}
	if len(found) != 1 {
		t.Fatalf("want one ratchet warning, got %v", found)
	}
	if !strings.Contains(found[0], "vl-ratchet") {
		t.Errorf("warning does not name the list: %q", found[0])
	}

	// A resize widens the list too, and must stay quiet: the window
	// moving is the explanation.
	w2 := newTestWindow()
	var quiet []string
	w2.debug.collect = &quiet
	m2 := &listHeightModel{}
	for i := range virtualListWidthRatchetRuns + 4 {
		w2.windowWidth = 800 + i*3
		virtualListNoteWidth(w2, m2, "vl-resize", 400+float32(i)*3)
	}
	// A settled list is quiet too.
	for range virtualListWidthRatchetRuns + 1 {
		virtualListNoteWidth(w2, m2, "vl-resize", m2.measuredW)
	}
	if len(quiet) != 0 {
		t.Fatalf("warned on a resize: %v", quiet)
	}
}
