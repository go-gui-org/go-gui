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
			{ID: "h1", Name: "Section", IsSubheading: true},
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
	if !second.Children[0].Shape.Color.Eq(ColorTransparent) {
		t.Error("leading virtualization spacer missing")
	}
	if !second.Children[len(second.Children)-1].Shape.Color.
		Eq(ColorTransparent) {
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
	cfg.Data[0].IsSubheading = true
	listBoxEnsureCache(&cfg, w)
	if len(cache.itemIDs) != 4 {
		t.Fatalf("IsSubheading mutation not refreshed: %v", cache.itemIDs)
	}

	// ID mutation: refresh with the new IDs.
	cfg.Data[0].IsSubheading = false
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
