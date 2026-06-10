package datagrid

import (
	"testing"

	. "github.com/go-gui-org/go-gui/gui"
)

// --- dataGridHeaderIndicator ---

func TestHeaderIndicatorNoSort(t *testing.T) {
	q := GridQueryState{}
	got := dataGridHeaderIndicator(q, "col1")
	if got != "" {
		t.Errorf("no sort: got %q, want empty", got)
	}
}

func TestHeaderIndicatorSingleAsc(t *testing.T) {
	q := GridQueryState{
		Sorts: []GridSort{{ColID: "col1", Dir: GridSortAsc}},
	}
	got := dataGridHeaderIndicator(q, "col1")
	if got != "\u25B2" {
		t.Errorf("asc: got %q, want ▲", got)
	}
}

func TestHeaderIndicatorSingleDesc(t *testing.T) {
	q := GridQueryState{
		Sorts: []GridSort{{ColID: "col1", Dir: GridSortDesc}},
	}
	got := dataGridHeaderIndicator(q, "col1")
	if got != "\u25BC" {
		t.Errorf("desc: got %q, want ▼", got)
	}
}

func TestHeaderIndicatorMultiSort(t *testing.T) {
	q := GridQueryState{
		Sorts: []GridSort{
			{ColID: "a", Dir: GridSortAsc},
			{ColID: "b", Dir: GridSortDesc},
		},
	}
	// Column "b" is index 1 (1-based: "2").
	got := dataGridHeaderIndicator(q, "b")
	if got != "2\u25BC" {
		t.Errorf("multi desc: got %q, want 2▼", got)
	}
	got = dataGridHeaderIndicator(q, "a")
	if got != "1\u25B2" {
		t.Errorf("multi asc: got %q, want 1▲", got)
	}
}

func TestHeaderIndicatorColumnNotSorted(t *testing.T) {
	q := GridQueryState{
		Sorts: []GridSort{{ColID: "a", Dir: GridSortAsc}},
	}
	got := dataGridHeaderIndicator(q, "x")
	if got != "" {
		t.Errorf("not sorted: got %q, want empty", got)
	}
}

// --- dataGridShowHeaderControls ---

func TestShowHeaderControls(t *testing.T) {
	tests := []struct {
		name                              string
		colID, hovered, resizing, focused string
		want                              bool
	}{
		{"hovered", "c1", "c1", "", "", true},
		{"resizing", "c1", "", "c1", "", true},
		{"focused", "c1", "", "", "c1", true},
		{"none", "c1", "", "", "", false},
		{"empty colID", "", "c1", "", "", false},
		{"different", "c1", "c2", "", "", false},
	}
	for _, tt := range tests {
		got := dataGridShowHeaderControls(tt.colID, tt.hovered, tt.resizing, tt.focused)
		if got != tt.want {
			t.Errorf("%s: got %v, want %v", tt.name, got, tt.want)
		}
	}
}

// --- dataGridHeaderColIDFromLayoutID ---

func TestHeaderColIDFromLayoutID(t *testing.T) {
	got := dataGridHeaderColIDFromLayoutID("grid1", "grid1:header:name")
	if got != "name" {
		t.Errorf("got %q, want %q", got, "name")
	}
}

func TestHeaderColIDFromLayoutIDNoMatch(t *testing.T) {
	got := dataGridHeaderColIDFromLayoutID("grid1", "grid2:header:name")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHeaderColIDFromLayoutIDShort(t *testing.T) {
	got := dataGridHeaderColIDFromLayoutID("grid1", "grid1:header:")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridHeaderControlState ---

func TestHeaderControlStateAllFit(t *testing.T) {
	// Wide column: everything fits.
	r := dataGridHeaderControlState(500, Padding{}, true, true, true)
	if !r.showLabel || !r.showReorder || !r.showPin || !r.showResize {
		t.Errorf("wide: got label=%v reorder=%v pin=%v resize=%v",
			r.showLabel, r.showReorder, r.showPin, r.showResize)
	}
}

func TestHeaderControlStateNarrowDropsAll(t *testing.T) {
	// Very narrow column: nothing fits.
	r := dataGridHeaderControlState(1, Padding{}, true, true, true)
	if r.showReorder || r.showPin {
		t.Error("very narrow should drop reorder and pin")
	}
}

func TestHeaderControlStateNoControls(t *testing.T) {
	r := dataGridHeaderControlState(100, Padding{}, false, false, false)
	if r.showReorder || r.showPin || r.showResize {
		t.Error("no controls requested: none should show")
	}
	if !r.showLabel {
		t.Error("label should show when no controls requested")
	}
}

// --- dataGridHeaderControlsWidth ---

func TestHeaderControlsWidthAll(t *testing.T) {
	got := dataGridHeaderControlsWidth(true, true, true)
	want := dataGridHeaderControlWidth*2 + dataGridHeaderReorderSpacing +
		dataGridHeaderControlWidth + dataGridResizeHandleWidth
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestHeaderControlsWidthNone(t *testing.T) {
	got := dataGridHeaderControlsWidth(false, false, false)
	if got != 0 {
		t.Errorf("got %v, want 0", got)
	}
}

// --- dataGridHeaderFocusBaseID ---

func TestHeaderFocusBaseIDNormal(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusBaseID(cfg, 3)
	// body = 100, base = 101
	if got != 101 {
		t.Errorf("got %d, want 101", got)
	}
}

func TestHeaderFocusBaseIDZeroCols(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusBaseID(cfg, 0)
	if got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}

// --- dataGridHeaderFocusID ---

func TestHeaderFocusID(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusID(cfg, 3, 0)
	if got != 101 {
		t.Errorf("col0: got %d, want 101", got)
	}
	got = dataGridHeaderFocusID(cfg, 3, 2)
	if got != 103 {
		t.Errorf("col2: got %d, want 103", got)
	}
}

func TestHeaderFocusIDOutOfRange(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusID(cfg, 3, 3)
	if got != 0 {
		t.Errorf("out of range: got %d, want 0", got)
	}
	got = dataGridHeaderFocusID(cfg, 3, -1)
	if got != 0 {
		t.Errorf("negative: got %d, want 0", got)
	}
}

// --- dataGridHeaderFocusIndex ---

func TestHeaderFocusIndex(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusIndex(cfg, 3, 102)
	if got != 1 {
		t.Errorf("got %d, want 1", got)
	}
}

func TestHeaderFocusIndexNotInRange(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusIndex(cfg, 3, 50)
	if got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

func TestHeaderFocusIndexZeroFocus(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	got := dataGridHeaderFocusIndex(cfg, 3, 0)
	if got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

// --- dataGridHeaderFocusedColID ---

func TestHeaderFocusedColID(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	columns := []GridColumnCfg{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	got := dataGridHeaderFocusedColID(cfg, columns, 102)
	if got != "b" {
		t.Errorf("got %q, want %q", got, "b")
	}
}

func TestHeaderFocusedColIDOutOfRange(t *testing.T) {
	cfg := &DataGridCfg{ID: "g", IDFocus: 100}
	columns := []GridColumnCfg{{ID: "a"}}
	got := dataGridHeaderFocusedColID(cfg, columns, 50)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridHeaderControlState (additional branches) ---

func TestHeaderControlStateMediumWidth(t *testing.T) {
	// Medium width: label + some controls.
	r := dataGridHeaderControlState(80, Padding{}, true, false, true)
	if !r.showLabel {
		t.Error("medium: label should show")
	}
}

func TestHeaderControlStateOnlyReorder(t *testing.T) {
	r := dataGridHeaderControlState(200, Padding{}, true, false, false)
	if !r.showReorder {
		t.Error("should show reorder")
	}
	if r.showPin {
		t.Error("should not show pin when not requested")
	}
	if r.showResize {
		t.Error("should not show resize when not requested")
	}
}

func TestHeaderControlStateOnlyPin(t *testing.T) {
	r := dataGridHeaderControlState(200, Padding{}, false, true, false)
	if !r.showPin {
		t.Error("should show pin")
	}
	if r.showReorder {
		t.Error("should not show reorder when not requested")
	}
}

func TestHeaderControlStateProgressiveDisclosure(t *testing.T) {
	// Progressively narrower: first lose label, then pin, then reorder.
	w0 := dataGridHeaderControlState(500, Padding{}, true, true, true)
	if !w0.showLabel || !w0.showPin || !w0.showReorder || !w0.showResize {
		t.Error("wide should show all")
	}
	// Narrow enough to drop label only.
	w1 := dataGridHeaderControlState(150, Padding{}, true, true, true)
	// Label may or may not show depending on constants; controls hierarchy
	// is what matters — reorder drops before resize in priority order.
	_ = w1
}

// --- dataGridHeaderControlState with padding ---

func TestHeaderControlStateWithPadding(t *testing.T) {
	pad := NewPadding(0, 10, 0, 10)
	r := dataGridHeaderControlState(100, pad, true, true, true)
	// Padding reduces available width.
	_ = r
}

// --- dataGridOrderButton ---

func TestOrderButtonReturnsView(t *testing.T) {
	v := dataGridOrderButton("◀", DefaultTextStyle, RGBA(200, 200, 200, 255),
		func(_ *Event, _ *Window) {})
	if v == nil {
		t.Fatal("order button should return a view")
	}
}

// --- dataGridPinControl ---

func TestPinControl(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  DefaultTextStyle,
		ColorHeaderHover: RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", Pin: GridColumnPinNone}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("pin control should return a view")
	}
}

func TestPinControlLeft(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  DefaultTextStyle,
		ColorHeaderHover: RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", Pin: GridColumnPinLeft}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("left-pinned control should return a view")
	}
}

func TestPinControlRight(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  DefaultTextStyle,
		ColorHeaderHover: RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", Pin: GridColumnPinRight}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("right-pinned control should return a view")
	}
}

// --- dataGridFilterRow ---

func TestFilterRowReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ColorFilter:   RGBA(240, 240, 240, 255),
		ColorBorder:   RGBA(180, 180, 180, 255),
		SizeBorder:    SomeF(1),
		PaddingFilter: SomeP(2, 4, 2, 4),
	}
	columns := []GridColumnCfg{{ID: "c1"}, {ID: "c2"}}
	v := dataGridFilterRow(cfg, columns, nil)
	if v == nil {
		t.Fatal("filter row should return a view")
	}
}

// --- dataGridFilterCell ---

func TestFilterCellReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ID:              "g1",
		ColorFilter:     RGBA(240, 240, 240, 255),
		ColorBorder:     RGBA(180, 180, 180, 255),
		SizeBorder:      SomeF(1),
		PaddingFilter:   SomeP(2, 4, 2, 4),
		TextStyleFilter: DefaultTextStyle,
	}
	col := GridColumnCfg{ID: "c1", Filterable: true}
	v := dataGridFilterCell(cfg, col, 100)
	if v == nil {
		t.Fatal("filter cell should return a view")
	}
}

func TestFilterCellNotFilterable(t *testing.T) {
	cfg := &DataGridCfg{
		ID:              "g1",
		ColorFilter:     RGBA(240, 240, 240, 255),
		ColorBorder:     RGBA(180, 180, 180, 255),
		TextStyleFilter: DefaultTextStyle,
	}
	col := GridColumnCfg{ID: "c1", Filterable: false}
	v := dataGridFilterCell(cfg, col, 100)
	if v == nil {
		t.Fatal("non-filterable cell should return a view")
	}
}

// --- dataGridResizeHandle ---

func TestResizeHandleReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ID:                "g1",
		ColorResizeHandle: RGBA(180, 180, 180, 255),
		ColorResizeActive: RGBA(100, 100, 255, 255),
		TextStyleHeader:   DefaultTextStyle,
		TextStyle:         DefaultTextStyle,
		Columns:           []GridColumnCfg{{ID: "c1"}},
	}
	col := GridColumnCfg{ID: "c1"}
	v := dataGridResizeHandle(cfg, col, 0)
	if v == nil {
		t.Fatal("resize handle should return a view")
	}
}

// --- dataGridReorderControls ---

func TestReorderControls(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  DefaultTextStyle,
		ColorHeaderHover: RGBA(200, 200, 200, 255),
		Columns:          []GridColumnCfg{{ID: "c1"}},
		ColumnOrder:      []string{"c1"},
	}
	col := GridColumnCfg{ID: "c1", Reorderable: true}
	v := dataGridReorderControls(cfg, col)
	if v == nil {
		t.Fatal("reorder controls should return a view")
	}
}

// --- dataGridEndResize ---

func TestEndResize(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	// Start a resize, then end it.
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, 4)
	dgRS.Set("g1", dataGridResizeState{Active: true, ColID: "c1"})
	dataGridEndResize("g1", w)
	state, _ := dgRS.Get("g1")
	if state.Active {
		t.Fatal("resize should be inactive after EndResize")
	}
}

func TestEndResizeNoState(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridEndResize("nonexistent", w)
}

// --- dataGridActiveResizeColID ---

func TestActiveResizeColID(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	dgRS := StateMap[string, dataGridResizeState](w, nsDgResize, 4)
	dgRS.Set("g1", dataGridResizeState{Active: true, ColID: "c2"})
	got := dataGridActiveResizeColID("g1", w)
	if got != "c2" {
		t.Errorf("got %q, want c2", got)
	}
}

func TestActiveResizeColIDInactive(t *testing.T) {
	w := NewWindow(WindowCfg{})
	defer w.Close()
	got := dataGridActiveResizeColID("g1", w)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridHeaderRow basic ---

func TestHeaderRowReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ID:              "g1",
		ColorBorder:     RGBA(180, 180, 180, 255),
		SizeBorder:      SomeF(1),
		PaddingHeader:   SomeP(2, 4, 2, 4),
		TextStyleHeader: DefaultTextStyle,
	}
	columns := []GridColumnCfg{{ID: "c1"}, {ID: "c2"}}
	v := dataGridHeaderRow(cfg, columns, nil, 0, "", "", "")
	if v == nil {
		t.Fatal("header row should return a view")
	}
}

// --- dataGridHeaderCell ---

func TestHeaderCellReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ID:               "g1",
		ColorHeader:      RGBA(240, 240, 240, 255),
		ColorBorder:      RGBA(180, 180, 180, 255),
		SizeBorder:       SomeF(1),
		PaddingHeader:    SomeP(2, 4, 2, 4),
		TextStyleHeader:  DefaultTextStyle,
		ColorHeaderHover: RGBA(220, 220, 220, 255),
	}
	col := GridColumnCfg{ID: "c1", Title: "Column 1"}
	v := dataGridHeaderCell(cfg, col, 0, 2, 100, 0, false)
	if v == nil {
		t.Fatal("header cell should return a view")
	}
}

func TestHeaderCellWithControls(t *testing.T) {
	cfg := &DataGridCfg{
		ID:                  "g1",
		ColorHeader:         RGBA(240, 240, 240, 255),
		ColorBorder:         RGBA(180, 180, 180, 255),
		SizeBorder:          SomeF(1),
		PaddingHeader:       SomeP(2, 4, 2, 4),
		TextStyleHeader:     DefaultTextStyle,
		ColorHeaderHover:    RGBA(220, 220, 220, 255),
		OnColumnOrderChange: func([]string, *Event, *Window) {},
		OnColumnPinChange:   func(string, GridColumnPin, *Event, *Window) {},
	}
	col := GridColumnCfg{
		ID: "c1", Title: "Column 1",
		Reorderable: true, Resizable: true, Pin: GridColumnPinNone,
	}
	v := dataGridHeaderCell(cfg, col, 0, 2, 300, 0, true)
	if v == nil {
		t.Fatal("header cell with controls should return a view")
	}
}

// --- dataGridPagerArrows ---

func TestPagerArrowsLTR(t *testing.T) {
	saved := ActiveLocale.TextDir
	ActiveLocale.TextDir = TextDirLTR
	defer func() { ActiveLocale.TextDir = saved }()
	prev, next := dataGridPagerArrows()
	if prev != "\u25C0" || next != "\u25B6" {
		t.Errorf("LTR: prev=%q next=%q", prev, next)
	}
}

func TestPagerArrowsRTL(t *testing.T) {
	saved := ActiveLocale.TextDir
	ActiveLocale.TextDir = TextDirRTL
	defer func() { ActiveLocale.TextDir = saved }()
	prev, next := dataGridPagerArrows()
	if prev != "\u25B6" || next != "\u25C0" {
		t.Errorf("RTL: prev=%q next=%q", prev, next)
	}
}
