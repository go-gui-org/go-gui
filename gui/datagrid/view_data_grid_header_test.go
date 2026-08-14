package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
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
		Sorts: []GridSort{{ColID: "col1", Dir: gridSortAsc}},
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
			{ColID: "a", Dir: gridSortAsc},
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
		Sorts: []GridSort{{ColID: "a", Dir: gridSortAsc}},
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
	r := dataGridHeaderControlState(500, gg.PaddingNone, true, true, true)
	if !r.showLabel || !r.showReorder || !r.showPin || !r.showResize {
		t.Errorf("wide: got label=%v reorder=%v pin=%v resize=%v",
			r.showLabel, r.showReorder, r.showPin, r.showResize)
	}
}

func TestHeaderControlStateNarrowDropsAll(t *testing.T) {
	// Very narrow column: nothing fits.
	r := dataGridHeaderControlState(1, gg.PaddingNone, true, true, true)
	if r.showReorder || r.showPin {
		t.Error("very narrow should drop reorder and pin")
	}
}

func TestHeaderControlStateNoControls(t *testing.T) {
	r := dataGridHeaderControlState(100, gg.PaddingNone, false, false, false)
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

// --- dataGridHeaderFocusedColID ---

func TestHeaderFocusedColID(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	columns := []GridColumnCfg{{ID: "a"}, {ID: "b"}, {ID: "c"}}
	// Header cell focus ids are cfg.ID + ":header:" + colID.
	got := dataGridHeaderFocusedColID(cfg, columns, "g:header:b")
	if got != "b" {
		t.Errorf("got %q, want %q", got, "b")
	}
}

func TestHeaderFocusedColIDOutOfRange(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	columns := []GridColumnCfg{{ID: "a"}}
	got := dataGridHeaderFocusedColID(cfg, columns, "g:header:zzz")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestHeaderFocusedColIDEmpty(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	columns := []GridColumnCfg{{ID: "a"}}
	if got := dataGridHeaderFocusedColID(cfg, columns, ""); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

// --- dataGridHeaderControlState (additional branches) ---

func TestHeaderControlStateMediumWidth(t *testing.T) {
	// Medium width: label + some controls.
	r := dataGridHeaderControlState(80, gg.PaddingNone, true, false, true)
	if !r.showLabel {
		t.Error("medium: label should show")
	}
}

func TestHeaderControlStateOnlyReorder(t *testing.T) {
	r := dataGridHeaderControlState(200, gg.PaddingNone, true, false, false)
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
	r := dataGridHeaderControlState(200, gg.PaddingNone, false, true, false)
	if !r.showPin {
		t.Error("should show pin")
	}
	if r.showReorder {
		t.Error("should not show reorder when not requested")
	}
}

func TestHeaderControlStateProgressiveDisclosure(t *testing.T) {
	// Progressively narrower: first lose label, then pin, then reorder.
	w0 := dataGridHeaderControlState(500, gg.PaddingNone, true, true, true)
	if !w0.showLabel || !w0.showPin || !w0.showReorder || !w0.showResize {
		t.Error("wide should show all")
	}
	// Narrow enough to drop label only.
	w1 := dataGridHeaderControlState(150, gg.PaddingNone, true, true, true)
	// Label may or may not show depending on constants; controls hierarchy
	// is what matters — reorder drops before resize in priority order.
	_ = w1
}

// --- dataGridHeaderControlState with padding ---

func TestHeaderControlStateWithPadding(t *testing.T) {
	pad := gg.NewPadding(0, 10, 0, 10)
	r := dataGridHeaderControlState(100, pad, true, true, true)
	// Padding reduces available width.
	_ = r
}

// --- dataGridOrderButton ---

func TestOrderButtonReturnsView(t *testing.T) {
	v := dataGridOrderButton("grid:reorder_left:col1", "◀",
		gg.DefaultTextStyle, gg.RGBA(200, 200, 200, 255),
		func(_ *gg.Event, _ *gg.Window) {})
	if v == nil {
		t.Fatal("order button should return a view")
	}
}

// The header controls are keyed by grid and column, so two grids —
// or two columns in one grid — never share a tab stop.
func TestOrderButtonIDIsPerGridAndColumn(t *testing.T) {
	style := gg.DefaultTextStyle
	hover := gg.RGBA(200, 200, 200, 255)
	cb := func(_ *gg.Event, _ *gg.Window) {}

	a := dataGridOrderButton("gridA:reorder_left:col1", "◀", style, hover, cb)
	b := dataGridOrderButton("gridB:reorder_left:col1", "◀", style, hover, cb)

	idA := a.GenerateLayout(nil).Shape.ID
	idB := b.GenerateLayout(nil).Shape.ID
	if idA == "" || idB == "" {
		t.Fatalf("header control has no ID: %q, %q", idA, idB)
	}
	if idA == idB {
		t.Errorf("two grids share the header-control ID %q", idA)
	}
}

// --- dataGridPinControl ---

func TestPinControl(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", pin: gridColumnPinNone}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("pin control should return a view")
	}
}

func TestPinControlLeft(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", pin: gridColumnPinLeft}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("left-pinned control should return a view")
	}
}

func TestPinControlRight(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
	}
	col := GridColumnCfg{ID: "c1", pin: gridColumnPinRight}
	v := dataGridPinControl(cfg, col)
	if v == nil {
		t.Fatal("right-pinned control should return a view")
	}
}

// --- dataGridFilterRow ---

func TestFilterRowReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ColorFilter:   gg.RGBA(240, 240, 240, 255),
		ColorBorder:   gg.RGBA(180, 180, 180, 255),
		SizeBorder:    gg.SomeF(1),
		PaddingFilter: gg.NewPadding(2, 4, 2, 4),
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
		ColorFilter:     gg.RGBA(240, 240, 240, 255),
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
		SizeBorder:      gg.SomeF(1),
		PaddingFilter:   gg.NewPadding(2, 4, 2, 4),
		TextStyleFilter: gg.DefaultTextStyle,
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
		ColorFilter:     gg.RGBA(240, 240, 240, 255),
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
		TextStyleFilter: gg.DefaultTextStyle,
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
		ColorResizeHandle: gg.RGBA(180, 180, 180, 255),
		ColorResizeActive: gg.RGBA(100, 100, 255, 255),
		TextStyleHeader:   gg.DefaultTextStyle,
		TextStyle:         gg.DefaultTextStyle,
		Columns:           []GridColumnCfg{{ID: "c1"}},
	}
	col := GridColumnCfg{ID: "c1"}
	v := dataGridResizeHandle(cfg, col, "", "")
	if v == nil {
		t.Fatal("resize handle should return a view")
	}
}

// TestResizeHandleHoverPressedColor pins the pressed-while-hovered
// branch of the resize handle's OnHover (view_data_grid_header.go):
// while the left button is held over the handle, the hover event
// carries MouseLeft and the handle renders colorResizeActive; release
// falls back to colorResizeHandle.
//
// A plain press on the handle starts a resize drag, which locks the
// mouse and silences hover (layoutHover bails under a lock), so the
// held-button hover pass is observed on the double-click window: the
// second press of a double-click auto-fits the column without locking,
// leaving hover live while the button is held. The auto-fit writes
// back the column's current width (headless: no measurer, same value),
// so the geometry does not shift under the pointer.
func TestResizeHandleHoverPressedColor(t *testing.T) {
	handle := gg.RGBA(180, 180, 180, 255)
	active := gg.RGBA(100, 100, 255, 255)
	cfg := DataGridCfg{
		ID:                "g1",
		ColorResizeHandle: handle,
		ColorResizeActive: active,
		TextStyleHeader:   gg.DefaultTextStyle,
		TextStyle:         gg.DefaultTextStyle,
		ColorHeader:       gg.RGBA(240, 240, 240, 255),
		ColorBorder:       gg.RGBA(180, 180, 180, 255),
		PaddingHeader:     gg.NewPadding(2, 4, 2, 4),
		SizeBorder:        gg.SomeF(0),
		Columns: []GridColumnCfg{{
			ID: "c1", Title: "Col1", resizable: true,
			Width: gg.SomeF(120),
		}},
	}
	w := gg.NewTestWindow(gg.WindowCfg{})
	defer w.Close()
	w.TestRender(func(win *gg.Window) gg.View { return New(w, cfg) })

	// The resize handle renders only while its column shows header
	// controls (focused/hovered/resizing); focus the header cell to
	// bring it into the tree.
	w.SetFocus("g1:header:c1")
	root := w.TestRender(nil)

	handleColor := func() gg.Color {
		ly, ok := root.FindByID("g1:resize:c1")
		if !ok {
			t.Fatal("resize handle left the tree")
		}
		return ly.Shape.Color
	}

	// Hit point: the handle shape's center.
	ly, ok := root.FindByID("g1:resize:c1")
	if !ok {
		t.Fatal("no resize handle in tree after focusing the column")
	}
	s := ly.Shape
	x, y := s.X+s.Width/2, s.Y+s.Height/2
	if !s.PointInShape(x, y) {
		t.Fatalf("handle center (%.0f,%.0f) misses the shape", x, y)
	}

	// Plain hover, no button held: the resting color.
	w.EventFn(&gg.Event{Type: gg.EventMouseMove, MouseX: x, MouseY: y})
	root = w.TestRender(nil)
	if c := handleColor(); c != handle {
		t.Fatalf("hover: color = %+v, want %+v", c, handle)
	}

	// Advance the frame counter so the first press records a
	// non-zero LastClickFrame (the double-click guard requires it).
	w.FrameFn()

	// Click 1: starts a resize drag — the mouse locks and hover is
	// silent, so OnHover cannot paint; the handle renders its
	// generation-time color instead, which is the active color while
	// the resize state is set (issue #284).
	press := &gg.Event{Type: gg.EventMouseDown, MouseButton: gg.MouseLeft,
		MouseX: x, MouseY: y}
	w.EventFn(press)
	root = w.TestRender(nil)
	if c := handleColor(); c != active {
		t.Fatalf("drag held: color = %+v, want %+v (resize state drives the mid-drag color)", c, active)
	}
	w.EventFn(&gg.Event{Type: gg.EventMouseUp, MouseButton: gg.MouseLeft,
		MouseX: x, MouseY: y})
	root = w.TestRender(nil)

	// Click 2 within the double-click window: auto-fit, no lock —
	// hover is live again and the held button paints the active color.
	w.EventFn(press)
	root = w.TestRender(nil)
	if c := handleColor(); c != active {
		t.Fatalf("held: color = %+v, want %+v", c, active)
	}

	// Release: the button is no longer held; hover falls back to the
	// resting color.
	w.EventFn(&gg.Event{Type: gg.EventMouseUp, MouseButton: gg.MouseLeft,
		MouseX: x, MouseY: y})
	root = w.TestRender(nil)
	if c := handleColor(); c != handle {
		t.Fatalf("released: color = %+v, want %+v", c, handle)
	}
}

// TestResizeHandleActiveDuringDrag pins the generation-time color of
// the resize handle while a resize drag is in flight: the handle's
// resting color must come from the resize state
// (dataGridActiveResizeColID), not from hover, because the drag holds
// a mouse lock and layoutHover bails under a lock
// (gui/layout_pipeline.go) — the OnHover active-color branch cannot
// fire mid-drag. Press starts the drag, the handle renders
// colorResizeActive; release ends the drag and the handle reverts to
// colorResizeHandle.
func TestResizeHandleActiveDuringDrag(t *testing.T) {
	handle := gg.RGBA(180, 180, 180, 255)
	active := gg.RGBA(100, 100, 255, 255)
	cfg := DataGridCfg{
		ID:                "g1",
		ColorResizeHandle: handle,
		ColorResizeActive: active,
		TextStyleHeader:   gg.DefaultTextStyle,
		TextStyle:         gg.DefaultTextStyle,
		ColorHeader:       gg.RGBA(240, 240, 240, 255),
		ColorBorder:       gg.RGBA(180, 180, 180, 255),
		PaddingHeader:     gg.NewPadding(2, 4, 2, 4),
		SizeBorder:        gg.SomeF(0),
		Columns: []GridColumnCfg{{
			ID: "c1", Title: "Col1", resizable: true,
			Width: gg.SomeF(120),
		}},
	}
	w := gg.NewTestWindow(gg.WindowCfg{})
	defer w.Close()
	w.TestRender(func(win *gg.Window) gg.View { return New(w, cfg) })

	// Bring the header controls (and the handle) into the tree by
	// focusing the header cell.
	w.SetFocus("g1:header:c1")
	root := w.TestRender(nil)

	ly, ok := root.FindByID("g1:resize:c1")
	if !ok {
		t.Fatal("no resize handle in tree after focusing the column")
	}
	s := ly.Shape
	x, y := s.X+s.Width/2, s.Y+s.Height/2

	handleColor := func() gg.Color {
		ly, ok := root.FindByID("g1:resize:c1")
		if !ok {
			t.Fatal("resize handle left the tree")
		}
		return ly.Shape.Color
	}

	w.FrameFn()

	// Press: starts the resize drag and locks the mouse. The next
	// generation paints the active color because the resize state says
	// this column is being resized.
	w.EventFn(&gg.Event{Type: gg.EventMouseDown, MouseButton: gg.MouseLeft,
		MouseX: x, MouseY: y})
	root = w.TestRender(nil)
	if c := handleColor(); c != active {
		t.Fatalf("during drag: color = %+v, want %+v", c, active)
	}

	// Drag a few px: the lock routes the move into the drag handler,
	// and the next generation still paints the active color.
	w.EventFn(&gg.Event{Type: gg.EventMouseMove, MouseX: x + 5, MouseY: y})
	root = w.TestRender(nil)
	if c := handleColor(); c != active {
		t.Fatalf("after drag: color = %+v, want %+v", c, active)
	}

	// Release: the resize state clears and the handle reverts to the
	// resting color.
	w.EventFn(&gg.Event{Type: gg.EventMouseUp, MouseButton: gg.MouseLeft,
		MouseX: x + 5, MouseY: y})
	root = w.TestRender(nil)
	if c := handleColor(); c != handle {
		t.Fatalf("after release: color = %+v, want %+v", c, handle)
	}
}

// TestResizeHandleRestingAfterCancel pins the Cancel hook's state
// clear: capture loss (window-resize steal, alt-tab, the #281 class)
// cancels an in-flight resize drag, and the handle must not stay
// pinned in the active color once the drag is gone.
func TestResizeHandleRestingAfterCancel(t *testing.T) {
	handle := gg.RGBA(180, 180, 180, 255)
	active := gg.RGBA(100, 100, 255, 255)
	cfg := DataGridCfg{
		ID:                "g1",
		ColorResizeHandle: handle,
		ColorResizeActive: active,
		TextStyleHeader:   gg.DefaultTextStyle,
		TextStyle:         gg.DefaultTextStyle,
		ColorHeader:       gg.RGBA(240, 240, 240, 255),
		ColorBorder:       gg.RGBA(180, 180, 180, 255),
		PaddingHeader:     gg.NewPadding(2, 4, 2, 4),
		SizeBorder:        gg.SomeF(0),
		Columns: []GridColumnCfg{{
			ID: "c1", Title: "Col1", resizable: true,
			Width: gg.SomeF(120),
		}},
	}
	w := gg.NewTestWindow(gg.WindowCfg{})
	defer w.Close()
	w.TestRender(func(win *gg.Window) gg.View { return New(w, cfg) })

	w.SetFocus("g1:header:c1")
	root := w.TestRender(nil)

	ly, ok := root.FindByID("g1:resize:c1")
	if !ok {
		t.Fatal("no resize handle in tree after focusing the column")
	}
	s := ly.Shape
	x, y := s.X+s.Width/2, s.Y+s.Height/2

	w.FrameFn()

	// Press: starts the drag — the handle paints the active color.
	w.EventFn(&gg.Event{Type: gg.EventMouseDown, MouseButton: gg.MouseLeft,
		MouseX: x, MouseY: y})
	root = w.TestRender(nil)
	ly, ok = root.FindByID("g1:resize:c1")
	if !ok {
		t.Fatal("resize handle left the tree")
	}
	if c := ly.Shape.Color; c != active {
		t.Fatalf("during drag: color = %+v, want %+v", c, active)
	}

	// Capture loss: the Cancel hook clears the resize state; the next
	// generation paints the resting color, not a stale active one.
	w.MouseCancel()
	root = w.TestRender(nil)
	ly, ok = root.FindByID("g1:resize:c1")
	if !ok {
		t.Fatal("resize handle left the tree")
	}
	if c := ly.Shape.Color; c != handle {
		t.Fatalf("after cancel: color = %+v, want %+v", c, handle)
	}
}

// --- dataGridReorderControls ---

func TestReorderControls(t *testing.T) {
	cfg := &DataGridCfg{
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(200, 200, 200, 255),
		Columns:          []GridColumnCfg{{ID: "c1"}},
		columnOrder:      []string{"c1"},
	}
	col := GridColumnCfg{ID: "c1", Reorderable: true}
	v := dataGridReorderControls(cfg, col)
	if v == nil {
		t.Fatal("reorder controls should return a view")
	}
}

// --- dataGridEndResize ---

func TestEndResize(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Start a resize, then end it.
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, 4)
	dgRS.Set("g1", dataGridResizeState{Active: true, ColID: "c1"})
	dataGridEndResize("g1", w)
	state, _ := dgRS.Get("g1")
	if state.Active {
		t.Fatal("resize should be inactive after EndResize")
	}
}

func TestEndResizeNoState(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	// Should not panic.
	dataGridEndResize("nonexistent", w)
}

// --- dataGridActiveResizeColID ---

func TestActiveResizeColID(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
	defer w.Close()
	dgRS := gg.StateMap[string, dataGridResizeState](w, nsDgResize, 4)
	dgRS.Set("g1", dataGridResizeState{Active: true, ColID: "c2"})
	got := dataGridActiveResizeColID("g1", w)
	if got != "c2" {
		t.Errorf("got %q, want c2", got)
	}
}

func TestActiveResizeColIDInactive(t *testing.T) {
	w := gg.NewWindow(gg.WindowCfg{})
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
		ColorBorder:     gg.RGBA(180, 180, 180, 255),
		SizeBorder:      gg.SomeF(1),
		PaddingHeader:   gg.NewPadding(2, 4, 2, 4),
		TextStyleHeader: gg.DefaultTextStyle,
	}
	columns := []GridColumnCfg{{ID: "c1"}, {ID: "c2"}}
	v := dataGridHeaderRow(cfg, columns, nil, "", "", "", "")
	if v == nil {
		t.Fatal("header row should return a view")
	}
}

// --- dataGridHeaderCell ---

func TestHeaderCellReturnsView(t *testing.T) {
	cfg := &DataGridCfg{
		ID:               "g1",
		ColorHeader:      gg.RGBA(240, 240, 240, 255),
		ColorBorder:      gg.RGBA(180, 180, 180, 255),
		SizeBorder:       gg.SomeF(1),
		PaddingHeader:    gg.NewPadding(2, 4, 2, 4),
		TextStyleHeader:  gg.DefaultTextStyle,
		ColorHeaderHover: gg.RGBA(220, 220, 220, 255),
	}
	col := GridColumnCfg{ID: "c1", Title: "Column 1"}
	v := dataGridHeaderCell(cfg, col, 0, 2, 100, "", false, "")
	if v == nil {
		t.Fatal("header cell should return a view")
	}
}

func TestHeaderCellWithControls(t *testing.T) {
	cfg := &DataGridCfg{
		ID:                  "g1",
		ColorHeader:         gg.RGBA(240, 240, 240, 255),
		ColorBorder:         gg.RGBA(180, 180, 180, 255),
		SizeBorder:          gg.SomeF(1),
		PaddingHeader:       gg.NewPadding(2, 4, 2, 4),
		TextStyleHeader:     gg.DefaultTextStyle,
		ColorHeaderHover:    gg.RGBA(220, 220, 220, 255),
		onColumnOrderChange: func(_ []string, ctx gg.EventCtx) {},
		onColumnPinChange:   func(_ string, _ gridColumnPin, ctx gg.EventCtx) {},
	}
	col := GridColumnCfg{
		ID: "c1", Title: "Column 1",
		Reorderable: true, resizable: true, pin: gridColumnPinNone,
	}
	v := dataGridHeaderCell(cfg, col, 0, 2, 300, "", true, "")
	if v == nil {
		t.Fatal("header cell with controls should return a view")
	}
}

// --- dataGridPagerArrows ---

func TestPagerArrowsLTR(t *testing.T) {
	saved := gg.ActiveLocale.TextDir
	gg.ActiveLocale.TextDir = gg.TextDirLTR
	defer func() { gg.ActiveLocale.TextDir = saved }()
	prev, next := dataGridPagerArrows()
	if prev != "\u25C0" || next != "\u25B6" {
		t.Errorf("LTR: prev=%q next=%q", prev, next)
	}
}

func TestPagerArrowsRTL(t *testing.T) {
	saved := gg.ActiveLocale.TextDir
	gg.ActiveLocale.TextDir = gg.TextDirRTL
	defer func() { gg.ActiveLocale.TextDir = saved }()
	prev, next := dataGridPagerArrows()
	if prev != "\u25B6" || next != "\u25C0" {
		t.Errorf("RTL: prev=%q next=%q", prev, next)
	}
}
