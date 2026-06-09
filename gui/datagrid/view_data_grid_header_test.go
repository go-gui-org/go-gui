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
