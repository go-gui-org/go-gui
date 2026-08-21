package gui

import (
	"math"
	"strconv"
	"testing"
)

func TestListBoxIDPassthrough(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:   "lb1",
		Data: []ListBoxOption{{ID: "a", Name: "A"}},
	}), w)
	if layout.Shape.ID != "lb1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestListBoxChildCount(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb2",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
			{ID: "c", Name: "Gamma"},
		},
	}), w)
	if len(layout.Children) != 3 {
		t.Errorf("children: got %d, want 3", len(layout.Children))
	}
}

func TestListBoxSingleSelectClick(t *testing.T) {
	var selected []string
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb3",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
		},
		OnSelect: func(ids []string, ctx EventCtx) {
			selected = ids
		},
	}), w)
	if len(layout.Children) < 1 {
		t.Fatal("expected children")
	}
	item := &layout.Children[0]
	if item.Shape.events != nil && item.Shape.events.OnClick != nil {
		e := &Event{MouseButton: MouseLeft}
		item.Shape.events.OnClick(EventCtx{item, e, w})
		if len(selected) != 1 || selected[0] != "a" {
			t.Errorf("expected [a], got %v", selected)
		}
	}
}

func TestListBoxDisabledFlag(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:       "lb4",
		Disabled: true,
		Data:     []ListBoxOption{{ID: "a", Name: "A"}},
	}), w)
	if !layout.Shape.Disabled {
		t.Error("expected disabled")
	}
}

// A selected row paints the subtle wash, so its label keeps the
// body color — a tint needs no paired foreground
// (visual-refresh §4.3).
func TestListBoxSelectedRowTextColor(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb-sel-text",
		Data: []ListBoxOption{
			{ID: "x", Name: "Item X"},
			{ID: "y", Name: "Item Y"},
		},
		SelectedIDs: []string{"y"},
	}), w)
	if len(layout.Children) < 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	unselected := layout.Children[0].Children[0].Shape.TC
	selected := layout.Children[1].Children[0].Shape.TC
	if unselected == nil || selected == nil {
		t.Fatal("row text missing")
	}
	if !selected.TextStyle.Color.eq(DefaultTextStyle.Color) {
		t.Errorf("selected row color = %v, want body %v",
			selected.TextStyle.Color, DefaultTextStyle.Color)
	}
	if !unselected.TextStyle.Color.eq(DefaultTextStyle.Color) {
		t.Errorf("unselected row color = %v, want body %v",
			unselected.TextStyle.Color, DefaultTextStyle.Color)
	}
}

func TestListBoxItems(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-items",
		Items: []string{"Go", "Rust", "Zig"},
	}), w)
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3", len(layout.Children))
	}
}

func TestListBoxItemsPrecedence(t *testing.T) {
	// Items should take precedence over Data.
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-prec",
		Items: []string{"Alpha", "Beta"},
		Data:  []ListBoxOption{{ID: "ignored", Name: "Ignored"}},
	}), w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestListBoxItemsSelect(t *testing.T) {
	var selected []string
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-items-sel",
		Items: []string{"Alpha", "Beta"},
		OnSelect: func(ids []string, ctx EventCtx) {
			selected = ids
		},
	}), w)
	if len(layout.Children) < 1 {
		t.Fatal("expected children")
	}
	item := &layout.Children[0]
	if item.Shape.events != nil && item.Shape.events.OnClick != nil {
		e := &Event{MouseButton: MouseLeft}
		item.Shape.events.OnClick(EventCtx{item, e, w})
		if len(selected) != 1 || selected[0] != "Alpha" {
			t.Errorf("expected [Alpha], got %v", selected)
		}
	}
}

func TestListBoxItemsEmptyKeepsData(t *testing.T) {
	// Empty Items (non-nil, zero-length) should not overwrite Data.
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID:    "lb-empty-items",
		Items: []string{},
		Data:  []ListBoxOption{{ID: "a", Name: "Alpha"}},
	}), w)
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1 (Data preserved)", len(layout.Children))
	}
}

func TestListBoxSubheadingCount(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb5",
		Data: []ListBoxOption{
			{ID: "h1", Name: "Section", isSubheading: true},
			{ID: "a", Name: "Alpha"},
		},
	}), w)
	if len(layout.Children) != 2 {
		t.Errorf("children: got %d, want 2", len(layout.Children))
	}
}

// listBoxTestData builds n rows of stable data.
func listBoxTestData(n int) []ListBoxOption {
	data := make([]ListBoxOption, n)
	for i := range n {
		s := strconv.Itoa(i)
		data[i] = NewListBoxOption("id-"+s, "Name "+s, "")
	}
	return data
}

func TestListBoxFillVirtualizesNextFrame(t *testing.T) {
	// Scrollable + Fill sizing must take the virtualizing path, and
	// once Arrange has resolved a height (captured by the amend
	// hook), the view phase builds only the visible rows.
	w := newTestWindow()
	cfg := ListBoxCfg{
		ID:         "lb-fill",
		Scrollable: true,
		Sizing:     FillFill,
		Data:       listBoxTestData(10_000),
	}
	v := ListBox(cfg)
	if _, ok := v.(*listBoxView); !ok {
		t.Fatal("Scrollable list with no Height must use the virtualizing path")
	}

	// Frame 1: no arranged height yet, so every row builds.
	first := generateViewLayout(v, w)
	if len(first.Children) < 9_000 {
		t.Fatalf("frame 1 children = %d, want all rows (unresolved height)",
			len(first.Children))
	}

	// Arrange resolves the height; the amend hook captures it.
	first.Shape.Height = 350
	first.Shape.events.AmendLayout(EventCtx{&first, nil, w})
	w.scrollY().Set(cfg.ID, 1000)

	// Frame 2: virtualized to the visible range plus spacers.
	second := generateViewLayout(v, w)
	if len(second.Children) > 500 {
		t.Fatalf("frame 2 children = %d, want visible range only",
			len(second.Children))
	}
	// Scrolled past row 0: leading spacer first, trailing spacer last.
	if len(second.Children) < 3 {
		t.Fatalf("frame 2 children = %d, want spacer+rows+spacer",
			len(second.Children))
	}
	if !second.Children[0].Shape.Color.eq(ColorTransparent) {
		t.Error("leading virtualization spacer missing")
	}
	if !second.Children[len(second.Children)-1].Shape.Color.
		eq(ColorTransparent) {
		t.Error("trailing virtualization spacer missing")
	}
}

func TestListBoxAmendStoresResolvedHeight(t *testing.T) {
	w := newTestWindow()
	cfg := ListBoxCfg{
		ID:         "lb-amend",
		Scrollable: true,
		Data:       listBoxTestData(10),
	}
	v := ListBox(cfg)
	layout := generateViewLayout(v, w)
	cache := listBoxEnsureCache(&cfg, w)

	// No arrange yet: height unknown, hSeen false.
	if cache.hSeen {
		t.Fatal("hSeen set before any amend")
	}

	layout.Shape.Height = 220
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if !cache.hSeen {
		t.Fatal("hSeen not set after amend")
	}
	if cache.resolvedH != 220 {
		t.Fatalf("resolvedH = %v, want 220", cache.resolvedH)
	}

	// A zero height must not clobber a previously resolved height.
	layout.Shape.Height = 0
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if cache.resolvedH != 220 {
		t.Fatalf("resolvedH = %v after zero amend, want 220", cache.resolvedH)
	}

	// A non-finite height must be rejected outright.
	layout.Shape.Height = float32(math.Inf(1))
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if cache.resolvedH != 220 {
		t.Fatalf("resolvedH = %v after Inf amend, want 220", cache.resolvedH)
	}
}

func TestListBoxVisibleRangeHeightPriority(t *testing.T) {
	// The height for virtualization resolves Height, then MaxHeight,
	// then the resolved height from the last arranged frame.
	w := newTestWindow()
	data := listBoxTestData(100)

	cases := []struct {
		name              string
		height, maxHeight float32
		resolvedH         float32
		wantListH         float32
		wantVirtualize    bool
	}{
		{"height_wins", 300, 200, 100, 300, true},
		{"max_height_wins", 0, 200, 100, 200, true},
		{"resolved_wins", 0, 0, 100, 100, true},
		{"none_zero", 0, 0, 0, 0, false},
		// A negative resolved height is used verbatim as the final
		// fallback; downstream treats any <=0 as "no height".
		{"negative_resolved", 0, 0, -5, -5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := ListBoxCfg{
				ID:         "lb-prio-" + tc.name,
				Scrollable: true,
				Height:     tc.height,
				MaxHeight:  tc.maxHeight,
				Data:       data,
			}
			cache := &listBoxCache{resolvedH: tc.resolvedH, hSeen: true}
			first, last, virtualize, listH, _ :=
				listBoxVisibleRange(&cfg, cache, w)
			if listH != tc.wantListH {
				t.Errorf("listH = %v, want %v", listH, tc.wantListH)
			}
			if virtualize != tc.wantVirtualize {
				t.Errorf("virtualize = %v, want %v", virtualize,
					tc.wantVirtualize)
			}
			// Non-virtualizing lists keep the full range: the caller
			// builds every row.
			if !tc.wantVirtualize && (first != 0 || last != len(data)-1) {
				t.Errorf("range [%d,%d], want full [0,%d]", first, last,
					len(data)-1)
			}
		})
	}
}

func TestListBoxCacheRefreshIgnoresNameValue(t *testing.T) {
	// The cache derives from ID and IsSubheading only, so mutating
	// Name or Value must not refresh it, while mutating ID must.
	w := newTestWindow()
	data := listBoxTestData(5)
	cfg := ListBoxCfg{ID: "lb-cache-hash", Data: data}
	cache := listBoxEnsureCache(&cfg, w)
	wantIDs := []string{"id-0", "id-1", "id-2", "id-3", "id-4"}
	if len(cache.itemIDs) != 5 || cache.itemIDs[0] != wantIDs[0] {
		t.Fatalf("cache not seeded: %v", cache.itemIDs)
	}

	mutate := func(fn func(*ListBoxOption)) {
		for i := range cfg.Data {
			fn(&cfg.Data[i])
		}
	}

	// Name mutation: no refresh (the reduced hash ignores it).
	mutate(func(o *ListBoxOption) { o.Name = "renamed" })
	listBoxEnsureCache(&cfg, w)
	if len(cache.itemIDs) != 5 {
		t.Fatalf("Name mutation refreshed cache: %v", cache.itemIDs)
	}

	// Value mutation: no refresh.
	mutate(func(o *ListBoxOption) { o.Value = "new value" })
	listBoxEnsureCache(&cfg, w)
	if len(cache.itemIDs) != 5 {
		t.Fatalf("Value mutation refreshed cache: %v", cache.itemIDs)
	}

	// IsSubheading mutation: refresh.
	cfg.Data[0].isSubheading = true
	listBoxEnsureCache(&cfg, w)
	if len(cache.itemIDs) != 4 {
		t.Fatalf("IsSubheading mutation not refreshed: %v", cache.itemIDs)
	}

	// ID mutation: refresh with the new IDs.
	cfg.Data[0].isSubheading = false
	cfg.Data[0].ID = "renamed-id"
	listBoxEnsureCache(&cfg, w)
	if len(cache.itemIDs) != 5 || cache.itemIDs[0] != "renamed-id" {
		t.Fatalf("ID mutation not refreshed: %v", cache.itemIDs)
	}
}

func TestListBoxNoHeightWarning(t *testing.T) {
	prevOn := DebugEnabled()
	Debug(true)
	defer Debug(prevOn)

	// Persistent zero height: the layout arranged but gave the list
	// nothing, so virtualization is off. Warn once, only after hSeen.
	w := newTestWindow()
	var found []string
	w.debug.collect = &found
	cfg := ListBoxCfg{
		ID:         "lb-warn",
		Scrollable: true,
		Data:       listBoxTestData(500),
	}
	v := ListBox(cfg)

	// First frame, never arranged: no warning yet.
	layout := generateViewLayout(v, w)
	if len(found) != 0 {
		t.Fatalf("warned before first arrange: %v", found)
	}

	// Arrange gives zero height; amend marks hSeen.
	layout.Shape.Height = 0
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	generateViewLayout(v, w)
	if len(found) != 1 {
		t.Fatalf("found %d findings, want 1: %v", len(found), found)
	}

	// Warn-once: later frames do not repeat it.
	generateViewLayout(v, w)
	if len(found) != 1 {
		t.Fatalf("warning repeated: %v", found)
	}
}

// The listbox warning is its own category: a mask without it must stay
// silent at the site, and the category alone must fire.
func TestListBoxNoHeightGatedByCategory(t *testing.T) {
	prevMask := DebugCategory(debugMask.Load())
	defer DebugCategories(prevMask)
	w := newTestWindow()
	cfg := ListBoxCfg{
		ID:         "lb-cat",
		Scrollable: true,
		Data:       listBoxTestData(500),
	}
	v := ListBox(cfg)

	DebugCategories(DebugMissingIDs)
	var found []string
	w.debug.collect = &found
	layout := generateViewLayout(v, w)
	layout.Shape.Height = 0
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	generateViewLayout(v, w)
	if len(found) != 0 {
		t.Fatalf("missing-ID mask must not warn, got %v", found)
	}

	DebugCategories(DebugListBoxNoHeight)
	generateViewLayout(v, w)
	if len(found) != 1 {
		t.Fatalf("listbox category must warn, got %d findings: %v", len(found), found)
	}
}

// --- drag-reorder plumbing ---

func TestListBoxDragIndexByRowDisabled(t *testing.T) {
	// Non-reorderable lists build no drag index map at all.
	got := listBoxDragIndexByRow(&ListBoxCfg{}, false)
	if got != nil {
		t.Fatalf("canReorder=false: got %v, want nil", got)
	}
}

func TestListBoxDragIndexByRowSkipsSubheadings(t *testing.T) {
	cfg := &ListBoxCfg{Data: []ListBoxOption{
		{ID: "a", Name: "A"},
		NewListBoxSubheading("h", "Header"),
		{ID: "b", Name: "B"},
		{ID: "c", Name: "C"},
		NewListBoxSubheading("h2", "Header 2"),
		{ID: "d", Name: "D"},
	}}
	got := listBoxDragIndexByRow(cfg, true)
	want := []int{0, -1, 1, 2, -1, 3}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("dragIdx[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestListBoxDragIndexByRowAllSubheadings(t *testing.T) {
	cfg := &ListBoxCfg{Data: []ListBoxOption{
		NewListBoxSubheading("h1", "One"),
		NewListBoxSubheading("h2", "Two"),
	}}
	got := listBoxDragIndexByRow(cfg, true)
	if len(got) != 2 || got[0] != -1 || got[1] != -1 {
		t.Fatalf("got %v, want [-1 -1]", got)
	}
}

func TestListBoxItemLayoutIDsDisabled(t *testing.T) {
	lids, moff := listBoxItemLayoutIDs(&ListBoxCfg{}, false, 0, 0)
	if lids != nil || moff != 0 {
		t.Fatalf("canReorder=false: got (%v, %d), want (nil, 0)", lids, moff)
	}
}

func TestListBoxItemLayoutIDsVirtualized(t *testing.T) {
	// Five rows, one subheading between row 0 and 1; the virtual
	// window shows data rows 1..3 (flat indices 1..3).
	cfg := &ListBoxCfg{
		ID: "lb",
		Data: []ListBoxOption{
			{ID: "a", Name: "A"},
			{ID: "b", Name: "B"},
			NewListBoxSubheading("h", "Header"),
			{ID: "c", Name: "C"},
			{ID: "d", Name: "D"},
		},
	}
	lids, moff := listBoxItemLayoutIDs(cfg, true, 1, 3)
	// Row 0 ("a") is a draggable row above the window → midsOffset 1.
	// Inside the window: b (1), h skipped, c (2) → two layout IDs.
	if moff != 1 {
		t.Fatalf("midsOffset = %d, want 1", moff)
	}
	want := []string{"lb:item:b", "lb:item:c"}
	if len(lids) != len(want) {
		t.Fatalf("layoutIDs = %v, want %v", lids, want)
	}
	for i := range want {
		if lids[i] != want[i] {
			t.Fatalf("layoutIDs = %v, want %v", lids, want)
		}
	}
}

func TestListBoxItemID(t *testing.T) {
	if got := listBoxItemID("lb", "opt"); got != "lb:item:opt" {
		t.Fatalf("listBoxItemID() = %q, want lb:item:opt", got)
	}
}

func TestListBoxReorderItemView(t *testing.T) {
	// The reorder row carries its layout ID and a11y role.
	cfg := ListBoxCfg{ID: "lb"}
	view := listBoxReorderItemView(
		ListBoxOption{ID: "a", Name: "Alpha"},
		cfg,
		nil,
		"",
		0,
		[]string{"a"},
		[]string{"lb:item:a"},
		0,
		"",
	)
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	layout := generateViewLayout(view, w)
	if layout.Shape.ID != "lb:item:a" {
		t.Fatalf("row ID = %q, want lb:item:a", layout.Shape.ID)
	}
	if layout.Shape.A11YRole != AccessRoleListItem {
		t.Fatalf("row role = %d, want ListItem", layout.Shape.A11YRole)
	}
	if layout.Shape.events == nil || layout.Shape.events.OnClick == nil {
		t.Fatal("reorder row must carry an OnClick handler")
	}
}

func TestListBoxReorderItemViewSelectedState(t *testing.T) {
	cfg := ListBoxCfg{ID: "lb", SelectedIDs: []string{"a"}}
	view := listBoxReorderItemView(
		ListBoxOption{ID: "a", Name: "Alpha"},
		cfg,
		listCoreSelectedSet(cfg.SelectedIDs),
		"",
		0,
		[]string{"a"},
		[]string{"lb:item:a"},
		0,
		"",
	)
	layout := generateViewLayout(view, newTestWindow())
	if !layout.Shape.A11YState.Has(AccessStateSelected) {
		t.Fatal("selected row should expose AccessStateSelected")
	}
}

func TestListBoxReorderItemViewClickArmsDrag(t *testing.T) {
	// Clicking a reorderable row arms the drag-reorder state and
	// reports the selection through OnSelect.
	cfg := ListBoxCfg{
		ID: "lb-drag",
	}
	var selected []string
	cfg.OnSelect = func(ids []string, ctx EventCtx) { selected = ids }
	view := listBoxReorderItemView(
		ListBoxOption{ID: "a", Name: "A"},
		cfg,
		nil,
		"",
		0,
		[]string{"a", "b"},
		[]string{"lb-drag:item:a", "lb-drag:item:b"},
		0,
		"",
	)
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	layout := generateViewLayout(view, w)

	e := &Event{MouseX: 5, MouseY: 5}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})

	state := dragReorderGet(w, "lb-drag")
	if !state.started {
		t.Fatal("click should arm the drag-reorder state")
	}
	if state.itemID != "a" || state.sourceIndex != 0 {
		t.Fatalf("drag state = %+v, want itemID a at index 0", state)
	}
	// Single-select: the clicked row becomes the selection.
	if len(selected) != 1 || selected[0] != "a" {
		t.Fatalf("onSelect = %v, want [a]", selected)
	}
}

// A click must move the keyboard focus row onto the clicked item.
// Without it the list paints the focus highlight on one row and the
// selection wash on another, and the next arrow key resumes from the
// stale row.
func TestListBoxClickMovesKeyboardFocus(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ListBox(ListBoxCfg{
		ID: "lb-focus-click",
		Data: []ListBoxOption{
			{ID: "a", Name: "Alpha"},
			{ID: "b", Name: "Beta"},
			{ID: "c", Name: "Gamma"},
		},
		OnSelect: func(ids []string, ctx EventCtx) {},
	}), w)

	// Third row: index 2 in itemIDs, not the default 0.
	item := &layout.Children[2]
	if item.Shape.events == nil || item.Shape.events.OnClick == nil {
		t.Fatal("expected third row to have OnClick")
	}
	item.Shape.events.OnClick(
		EventCtx{item, &Event{MouseButton: MouseLeft}, w})

	if got := StateReadOr(w, nsListBoxFocus, "lb-focus-click", -1); got != 2 {
		t.Errorf("focus index = %d, want 2", got)
	}
}

// The keyboard-focus row takes an accent ring, not a fill: a fill is
// the same grey the mouse hover paints, so the two cursors could not
// be told apart.
func TestListBoxFocusRowRingNotFill(t *testing.T) {
	w := &Window{}
	view := ListBox(ListBoxCfg{
		ID:    "lb-ring",
		Items: []string{"one", "two"},
		OnSelect: func(ids []string, ctx EventCtx) {
		},
	})
	layout := generateViewLayout(view, w)

	// Focus index defaults to 0, so the first row is the active one.
	row := &layout.Children[0]
	if row.Shape.Color != ColorTransparent {
		t.Errorf("focus row fill = %v, want transparent", row.Shape.Color)
	}
	if row.Shape.events == nil || row.Shape.events.AmendLayout == nil {
		t.Fatal("focus row has no AmendLayout ring hook")
	}
	amend := row.Shape.events.AmendLayout
	// The hook is a no-op until the list itself holds focus.
	amend(EventCtx{row, nil, w})
	if row.Shape.SizeBorder != 0 {
		t.Errorf("unfocused list drew a ring: %v", row.Shape.SizeBorder)
	}

	w.SetFocus("lb-ring")
	amend(EventCtx{row, nil, w})
	if row.Shape.SizeBorder != focusRingBorderWidth {
		t.Errorf("ring width = %v, want %v",
			row.Shape.SizeBorder, float32(focusRingBorderWidth))
	}
	if !row.Shape.ColorBorder.IsSet() {
		t.Error("ring colour unset on the focused row")
	}

	// A non-focus row takes no hook at all.
	if e := layout.Children[1].Shape.events; e != nil &&
		e.AmendLayout != nil {
		t.Error("non-focus row carries a ring hook")
	}
}

// The reorderable row takes the same keyboard-focus ring as a plain
// one; it carried no focus indication of any kind before.
func TestListBoxReorderItemViewFocusRing(t *testing.T) {
	cfg := ListBoxCfg{ID: "lb-reo-ring"}
	applyListBoxDefaults(&cfg)
	view := listBoxReorderItemView(
		ListBoxOption{ID: "a", Name: "Alpha"},
		cfg,
		nil,
		"a", // this row holds the keyboard focus
		0,
		[]string{"a"},
		[]string{"lb-reo-ring:item:a"},
		0,
		"",
	)
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	layout := generateViewLayout(view, w)

	if layout.Shape.events == nil || layout.Shape.events.AmendLayout == nil {
		t.Fatal("focused reorder row has no ring hook")
	}
	amend := layout.Shape.events.AmendLayout
	amend(EventCtx{&layout, nil, w})
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("unfocused list drew a ring: %v", layout.Shape.SizeBorder)
	}

	w.SetFocus("lb-reo-ring")
	amend(EventCtx{&layout, nil, w})
	if layout.Shape.SizeBorder != focusRingBorderWidth {
		t.Errorf("ring width = %v, want %v",
			layout.Shape.SizeBorder, float32(focusRingBorderWidth))
	}
}

// listBoxFocusOnClick is reached from a per-row OnClick, so it must
// no-op rather than panic on the degenerate inputs a caller can
// produce: no list ID, no window, an empty item list, or an item that
// is not in the list at all.
func TestListBoxFocusOnClickDegenerateInputs(t *testing.T) {
	w := newTestWindow()

	// No panic, and nothing recorded, for each degenerate case.
	listBoxFocusOnClick("", false, []string{"a"}, "a", w)
	listBoxFocusOnClick("lb", false, []string{"a"}, "a", nil)
	listBoxFocusOnClick("lb", false, nil, "a", w)
	if got := StateReadOr(w, nsListBoxFocus, "lb", -1); got != -1 {
		t.Errorf("degenerate call recorded focus %d, want none", got)
	}

	// An ID that is not in the list leaves the index alone. The list
	// still takes window focus: the click landed on it either way.
	StateMap[string, int](w, nsListBoxFocus, capModerate).Set("lb", 2)
	listBoxFocusOnClick("lb", false, []string{"a", "b"}, "zz", w)
	if got := StateReadOr(w, nsListBoxFocus, "lb", -1); got != 2 {
		t.Errorf("unknown item moved focus to %d, want 2 unchanged", got)
	}
	if !w.IsFocus("lb") {
		t.Error("click should still give the list window focus")
	}
}

// A FocusDisabled list is not in the tab order, so a click on one must
// not pull window focus away from whatever holds it.
func TestListBoxFocusOnClickFocusDisabledKeepsWindowFocus(t *testing.T) {
	w := newTestWindow()
	w.SetFocus("somewhere-else")

	listBoxFocusOnClick("lb", true, []string{"a", "b"}, "b", w)

	// The row index still moves — the list tracks its own active row
	// even when it cannot hold window focus.
	if got := StateReadOr(w, nsListBoxFocus, "lb", -1); got != 1 {
		t.Errorf("focus index = %d, want 1", got)
	}
	if !w.IsFocus("somewhere-else") {
		t.Error("FocusDisabled list stole window focus")
	}
}

// An ID-less option must not match the ID-less "no focus row"
// sentinel. Without the guard every row in an ID-less list reads as
// the focus row and the list paints a ring on all of them.
func TestListBoxItemViewIDLessRowsTakeNoRing(t *testing.T) {
	cfg := ListBoxCfg{ID: "lb-noid"}
	applyListBoxDefaults(&cfg)
	w := newTestWindow()
	for _, dat := range []ListBoxOption{{Name: "one"}, {Name: "two"}} {
		view := listBoxItemView(dat, cfg, nil, "", nil)
		layout := generateViewLayout(view, w)
		if e := layout.Shape.events; e != nil && e.AmendLayout != nil {
			t.Fatalf("ID-less row %q took a focus ring", dat.Name)
		}
	}
}

// The ring hook is built only when there is something to paint with,
// and it tolerates the degenerate contexts an AmendLayout slot can be
// invoked with.
func TestListBoxItemRingAmendGuards(t *testing.T) {
	ring := RGB(0, 0, 255)
	if listBoxItemRingAmend(false, "lb", ring) != nil {
		t.Error("non-focus row should take no hook")
	}
	if listBoxItemRingAmend(true, "", ring) != nil {
		t.Error("ID-less list should take no hook")
	}
	if listBoxItemRingAmend(true, "lb", Color{}) != nil {
		t.Error("unset ring colour should take no hook")
	}

	amend := listBoxItemRingAmend(true, "lb", ring)
	if amend == nil {
		t.Fatal("focused row should take a hook")
	}
	w := newTestWindow()
	w.SetFocus("lb")
	// Nil layout, nil shape and nil window must all no-op, not panic.
	amend(EventCtx{nil, nil, w})
	amend(EventCtx{&Layout{}, nil, w})
	amend(EventCtx{&Layout{Shape: &Shape{}}, nil, nil})

	// A disabled row never shows focus even while the list holds it.
	disabled := Layout{Shape: &Shape{Disabled: true}}
	amend(EventCtx{&disabled, nil, w})
	if disabled.Shape.SizeBorder != 0 {
		t.Errorf("disabled row drew a ring: %v", disabled.Shape.SizeBorder)
	}
}

// The reorder path shares the focus-follows-click wiring, not just the
// ring: clicking a draggable row must move the keyboard focus index.
func TestListBoxReorderClickMovesKeyboardFocus(t *testing.T) {
	cfg := ListBoxCfg{ID: "lb-reo-click"}
	cfg.OnSelect = func([]string, EventCtx) {}
	applyListBoxDefaults(&cfg)
	view := listBoxReorderItemView(
		ListBoxOption{ID: "b", Name: "Beta"},
		cfg, nil, "", 1,
		[]string{"a", "b"},
		[]string{"lb-reo-click:item:a", "lb-reo-click:item:b"},
		0, "",
	)
	w := newTestWindow()
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	layout := generateViewLayout(view, w)

	layout.Shape.events.OnClick(
		EventCtx{&layout, &Event{MouseX: 5, MouseY: 5}, w})

	if got := StateReadOr(w, nsListBoxFocus, "lb-reo-click", -1); got != 1 {
		t.Errorf("focus index = %d, want 1", got)
	}
}
