package gui

import (
	"maps"
	"math"
	"slices"
	"testing"
)

// rowHeaderTestData is a header row plus three data rows: the shape a
// row-header table usually takes, with row 0 left out of selection.
func rowHeaderTestData() []TableRowCfg {
	return []TableRowCfg{
		TR([]TableCellCfg{TH("Name"), TH("Age")}),
		TR([]TableCellCfg{TD("Alice"), TD("30")}),
		TR([]TableCellCfg{TD("Bob"), TD("25")}),
		TR([]TableCellCfg{TD("Cy"), TD("41")}),
	}
}

// rowHeaderCellText returns the glyph in a row's leading header cell.
func rowHeaderCellText(t *testing.T, row *Layout) string {
	t.Helper()
	if len(row.Children) == 0 || len(row.Children[0].Children) == 0 {
		t.Fatal("row has no header cell")
	}
	tc := row.Children[0].Children[0].Shape.TC
	if tc == nil {
		t.Fatal("header cell has no text")
	}
	return tc.Text
}

// clickRowHeader fires the OnClick of a row's leading header cell with
// the given modifiers and returns the event, so a test can check that
// the click was consumed.
func clickRowHeader(t *testing.T, row *Layout, w *Window, mods Modifier) *Event {
	t.Helper()
	cell := &row.Children[0]
	if !cell.Shape.hasEvents() || cell.Shape.events.OnClick == nil {
		t.Fatal("row header cell has no OnClick")
	}
	e := &Event{Modifiers: mods}
	cell.Shape.events.OnClick(EventCtx{cell, e, w})
	return e
}

func TestTableIsHeaderRow(t *testing.T) {
	data := rowHeaderTestData()
	if !tableIsHeaderRow(data[0]) {
		t.Error("all-HeadCell row: want header")
	}
	if tableIsHeaderRow(data[1]) {
		t.Error("data row: want not header")
	}
	if tableIsHeaderRow(TableRowCfg{}) {
		t.Error("empty row: want not header")
	}
	mixed := TR([]TableCellCfg{TH("a"), TD("b")})
	if tableIsHeaderRow(mixed) {
		t.Error("mixed row: want not header")
	}
}

func TestTableSelectAllSkipsHeaderRows(t *testing.T) {
	got := tableSelectAll(rowHeaderTestData())
	want := map[int]bool{1: true, 2: true, 3: true}
	if !maps.Equal(got, want) {
		t.Fatalf("select all = %v, want %v", got, want)
	}
}

func TestTableRowHeaderSelection(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	data := rowHeaderTestData()
	cur := map[int]bool{1: true}
	cases := []struct {
		name   string
		multi  bool
		row    int
		anchor int
		mods   Modifier
		want   map[int]bool
	}{
		{"plain replaces", true, 3, 1, ModNone, map[int]bool{3: true}},
		{"ctrl adds", true, 3, 1, ModCtrl, map[int]bool{1: true, 3: true}},
		{"ctrl removes", true, 1, 1, ModCtrl, map[int]bool{}},
		{"shift range", true, 3, 2, ModShift, map[int]bool{2: true, 3: true}},
		{"shift range up skips header", true, 0, 2, ModShift,
			map[int]bool{1: true, 2: true}},
		{"ctrl+shift extends", true, 3, 2, ModCtrlShift,
			map[int]bool{1: true, 2: true, 3: true}},
		// Held mouse buttons ride in Modifiers; they must not change
		// the outcome.
		{"mouse bit ignored", true, 2, 1, ModLMB, map[int]bool{2: true}},
		{"single ignores ctrl", false, 3, 1, ModCtrl, map[int]bool{3: true}},
		{"single ignores shift", false, 3, 1, ModShift, map[int]bool{3: true}},
		// The anchor comes from state and goes stale when Data shrinks;
		// the span clamps to the data instead of walking to it.
		{"stale anchor past end", true, 2, 1 << 30, ModShift,
			map[int]bool{2: true, 3: true}},
		{"negative anchor", true, 2, -5, ModShift,
			map[int]bool{1: true, 2: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tableRowHeaderSelection(data, cur, tc.multi,
				tc.row, tc.anchor, tc.mods)
			if !maps.Equal(got, tc.want) {
				t.Fatalf("sel = %v, want %v", got, tc.want)
			}
		})
	}
	// The input selection is the caller's; it must stay untouched.
	if !maps.Equal(cur, map[int]bool{1: true}) {
		t.Fatalf("input selection mutated: %v", cur)
	}
}

func TestTableRowHeaderOffByDefault(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:   "rh-off",
		Data: rowHeaderTestData(),
	}), w)
	rows := tableRows(&layout)
	if got := len(rows[1].Children); got != 2 {
		t.Fatalf("cells = %d, want 2 (no row header)", got)
	}
}

func TestTableRowHeaderAddsLeadingCell(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:        "rh-cells",
		RowHeader: true,
		Data:      rowHeaderTestData(),
	}), w)
	rows := tableRows(&layout)
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
	for i := range rows {
		if got := len(rows[i].Children); got != 3 {
			t.Fatalf("row %d cells = %d, want 3", i, got)
		}
		if got := rows[i].Children[0].Shape.Width; got != tableRowHeaderWidthDefault {
			t.Errorf("row %d header width = %v, want %v",
				i, got, tableRowHeaderWidthDefault)
		}
	}
	if got := rowHeaderCellText(t, &rows[0]); got != tableRowHeaderCorner {
		t.Errorf("corner text = %q, want %q", got, tableRowHeaderCorner)
	}
	// No row is current before any click, so no data row shows the
	// marker.
	if got := rowHeaderCellText(t, &rows[1]); got != "" {
		t.Errorf("row 1 header text = %q, want empty", got)
	}
}

func TestTableRowHeaderWidth(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:             "rh-width",
		RowHeader:      true,
		RowHeaderWidth: 40,
		Data:           rowHeaderTestData(),
	}), w)
	rows := tableRows(&layout)
	if got := rows[1].Children[0].Shape.Width; got != 40 {
		t.Fatalf("header width = %v, want 40", got)
	}
}

// A width that is not a finite positive number falls back to the
// default, so it cannot poison the row layout.
func TestTableRowHeaderWidthInvalidFallsBack(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, width := range []float32{-3, nan, inf} {
		rh := tableRowHeaderOf(&TableCfg{
			RowHeader:      true,
			RowHeaderWidth: width,
		}, -1, nil)
		if rh.width != tableRowHeaderWidthDefault {
			t.Errorf("width %v: got %v, want %v",
				width, rh.width, tableRowHeaderWidthDefault)
		}
	}
}

func TestTableRowHeaderEmptyDataNoPanic(t *testing.T) {
	w := &Window{}
	generateViewLayout(Table(TableCfg{
		ID:        "rh-empty",
		RowHeader: true,
		Focusable: true,
	}), w)
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	if tableSelectionKeys(nil, nil, true, true,
		func(map[int]bool, int, EventCtx) {}, "rh-empty", e, w) {
		t.Fatal("empty data: key consumed, want travel on")
	}
}

func TestTableRowHeaderClickSelects(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	w := &Window{}
	var got map[int]bool
	var gotIdx int
	sel := map[int]bool{}
	rowClicks := 0
	data := rowHeaderTestData()
	data[1].OnClick = func(EventCtx) { rowClicks++ }
	build := func() []Layout {
		layout := generateViewLayout(Table(TableCfg{
			ID:          "rh-click",
			RowHeader:   true,
			MultiSelect: true,
			Data:        data,
			Selected:    sel,
			OnSelect: func(s map[int]bool, idx int, _ EventCtx) {
				got, gotIdx = s, idx
				sel = s
			},
		}), w)
		return tableRows(&layout)
	}

	// Plain click on row 1: selects it alone and anchors there. The
	// row's own OnClick fires, as for a click on its cells.
	rows := build()
	e := clickRowHeader(t, &rows[1], w, ModLMB)
	if !e.IsHandled {
		t.Error("row header click not consumed")
	}
	if gotIdx != 1 || !maps.Equal(got, map[int]bool{1: true}) {
		t.Fatalf("plain click: idx %d sel %v, want 1 {1}", gotIdx, got)
	}
	if rowClicks != 1 {
		t.Errorf("row OnClick calls = %d, want 1", rowClicks)
	}

	// Shift click on row 3: range from the anchor.
	rows = build()
	clickRowHeader(t, &rows[3], w, ModShift|ModLMB)
	if !maps.Equal(got, map[int]bool{1: true, 2: true, 3: true}) {
		t.Fatalf("shift click: sel %v, want {1,2,3}", got)
	}

	// Ctrl click on row 2: toggles it out, keeps the rest.
	rows = build()
	clickRowHeader(t, &rows[2], w, ModCtrl|ModLMB)
	if !maps.Equal(got, map[int]bool{1: true, 3: true}) {
		t.Fatalf("ctrl click: sel %v, want {1,3}", got)
	}

	// The last clicked row is current and shows the marker.
	rows = build()
	if txt := rowHeaderCellText(t, &rows[2]); txt != tableRowHeaderCurrent {
		t.Errorf("row 2 header text = %q, want %q", txt, tableRowHeaderCurrent)
	}
	if txt := rowHeaderCellText(t, &rows[1]); txt != "" {
		t.Errorf("row 1 header text = %q, want empty", txt)
	}
}

func TestTableRowHeaderCornerSelectsAll(t *testing.T) {
	w := &Window{}
	var got map[int]bool
	layout := generateViewLayout(Table(TableCfg{
		ID:          "rh-corner",
		RowHeader:   true,
		MultiSelect: true,
		Data:        rowHeaderTestData(),
		OnSelect:    func(s map[int]bool, _ int, _ EventCtx) { got = s },
	}), w)
	rows := tableRows(&layout)
	e := clickRowHeader(t, &rows[0], w, ModLMB)
	if !e.IsHandled {
		t.Error("corner click not consumed")
	}
	if !maps.Equal(got, map[int]bool{1: true, 2: true, 3: true}) {
		t.Fatalf("corner click: sel %v, want {1,2,3}", got)
	}
}

// Single select cannot hold every row, so the corner stays inert there.
func TestTableRowHeaderCornerInertSingleSelect(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:        "rh-corner-single",
		RowHeader: true,
		Data:      rowHeaderTestData(),
		OnSelect:  func(map[int]bool, int, EventCtx) {},
	}), w)
	rows := tableRows(&layout)
	cell := rows[0].Children[0].Shape
	if cell.hasEvents() && cell.events.OnClick != nil {
		t.Fatal("corner has OnClick under single select")
	}
}

func TestTableRowHeaderFreeze(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &tableTestMeasurer{}
	layout := generateViewLayout(w.Table(TableCfg{
		ID:           "rh-freeze",
		RowHeader:    true,
		MaxHeight:    200,
		FreezeHeader: true,
		Data:         rowHeaderTestData(),
	}), w)
	header := layout.Children[0].Children[0]
	if got := rowHeaderCellText(t, &header); got != tableRowHeaderCorner {
		t.Errorf("frozen header corner = %q, want %q", got, tableRowHeaderCorner)
	}
	body := layout.Children[1].Children[0]
	if got := len(body.Children); got != 3 {
		t.Errorf("body row cells = %d, want 3", got)
	}
}

// rowHeaderKey drives a focusable row-header table's key handler, the
// way the focused outer container would.
func rowHeaderKey(t *testing.T, w *Window, cfg TableCfg, k KeyCode, mods Modifier) *Event {
	t.Helper()
	layout := generateViewLayout(w.Table(cfg), w)
	if !layout.Shape.hasEvents() || layout.Shape.events.OnKeyDown == nil {
		t.Fatal("table has no OnKeyDown")
	}
	e := &Event{KeyCode: k, Modifiers: mods}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
	return e
}

func TestTableKeySelectAll(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	w := &Window{}
	var got map[int]bool
	cfg := TableCfg{
		ID:          "rh-keys-all",
		Focusable:   true,
		MultiSelect: true,
		Data:        rowHeaderTestData(),
		OnSelect:    func(s map[int]bool, _ int, _ EventCtx) { got = s },
	}
	// Select all needs no row header: it is the platform shortcut any
	// multi-select list answers.
	e := rowHeaderKey(t, w, cfg, KeyA, ModCtrl)
	if !e.IsHandled {
		t.Error("Ctrl+A not consumed")
	}
	if !maps.Equal(got, map[int]bool{1: true, 2: true, 3: true}) {
		t.Fatalf("Ctrl+A: sel %v, want {1,2,3}", got)
	}

	// Single select has nothing to select all into: the key travels on.
	got = nil
	cfg.MultiSelect = false
	e = rowHeaderKey(t, w, cfg, KeyA, ModCtrl)
	if e.IsHandled || got != nil {
		t.Fatalf("single select Ctrl+A: handled %v sel %v, want travel on",
			e.IsHandled, got)
	}
}

func TestTableKeyShiftSpaceAndEscape(t *testing.T) {
	w := &Window{}
	var got map[int]bool
	cfg := TableCfg{
		ID:          "rh-keys",
		Focusable:   true,
		MultiSelect: true,
		RowHeader:   true,
		Data:        rowHeaderTestData(),
		Selected:    map[int]bool{1: true, 3: true},
		OnSelect:    func(s map[int]bool, _ int, _ EventCtx) { got = s },
	}
	StateMap[string, int](w, nsTableFocus, capModerate).
		Set(w.EffID(cfg.ID), 2)

	// Shift+Space: the current row becomes the whole selection.
	e := rowHeaderKey(t, w, cfg, KeySpace, ModShift)
	if !e.IsHandled {
		t.Error("Shift+Space not consumed")
	}
	if !maps.Equal(got, map[int]bool{2: true}) {
		t.Fatalf("Shift+Space: sel %v, want {2}", got)
	}

	// Escape clears a selection.
	e = rowHeaderKey(t, w, cfg, KeyEscape, ModNone)
	if !e.IsHandled {
		t.Error("Escape not consumed")
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("Escape: sel %v, want empty", got)
	}

	// With nothing selected, Escape travels on (a dialog may want it).
	got = nil
	cfg.Selected = nil
	e = rowHeaderKey(t, w, cfg, KeyEscape, ModNone)
	if e.IsHandled || got != nil {
		t.Fatalf("Escape, empty sel: handled %v sel %v, want travel on",
			e.IsHandled, got)
	}

	// Without RowHeader, Escape keeps its old meaning: it travels on.
	cfg.RowHeader = false
	cfg.Selected = map[int]bool{1: true}
	e = rowHeaderKey(t, w, cfg, KeyEscape, ModNone)
	if e.IsHandled || got != nil {
		t.Fatalf("Escape, no row header: handled %v sel %v, want travel on",
			e.IsHandled, got)
	}
}

// A header click consumes the press, so the table container never sees
// the press that would focus it. The handlers focus it themselves, or
// the table's keys (Ctrl+A, Shift+Space, Escape) stay dead after a click.
func TestTableRowHeaderClickFocusesTable(t *testing.T) {
	for _, row := range []int{0, 1} { // corner, then a data row
		w := &Window{}
		layout := generateViewLayout(w.Table(TableCfg{
			ID:          "rh-focus",
			Focusable:   true,
			RowHeader:   true,
			MultiSelect: true,
			Data:        rowHeaderTestData(),
			OnSelect:    func(map[int]bool, int, EventCtx) {},
		}), w)
		rows := tableRows(&layout)
		clickRowHeader(t, &rows[row], w, ModLMB)
		if got, want := w.FocusID(), w.EffID("rh-focus"); got != want {
			t.Errorf("row %d: focus = %q, want %q", row, got, want)
		}
	}
}

// A table that is not focusable has nothing to focus.
func TestTableRowHeaderClickNoFocusWhenNotFocusable(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(w.Table(TableCfg{
		ID:        "rh-nofocus",
		RowHeader: true,
		Data:      rowHeaderTestData(),
		OnSelect:  func(map[int]bool, int, EventCtx) {},
	}), w)
	rows := tableRows(&layout)
	clickRowHeader(t, &rows[1], w, ModLMB)
	if got := w.FocusID(); got != "" {
		t.Errorf("focus = %q, want none", got)
	}
}

// rowHeaderCellStyle returns the text style of a row's leading header
// cell glyph.
func rowHeaderCellStyle(t *testing.T, row *Layout) TextStyle {
	t.Helper()
	tc := row.Children[0].Children[0].Shape.TC
	if tc == nil || tc.TextStyle == nil {
		t.Fatal("header cell has no text style")
	}
	return *tc.TextStyle
}

// RowHeaderMarker replaces the default glyph on the current row; an
// empty value keeps the default (issue #1156 follow-up).
func TestTableRowHeaderMarker(t *testing.T) {
	for _, tc := range []struct {
		name, marker, want string
	}{
		{"default", "", tableRowHeaderCurrent},
		{"custom", ">", ">"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &Window{}
			layout := generateViewLayout(Table(TableCfg{
				ID:              "rh-marker",
				RowHeader:       true,
				RowHeaderMarker: tc.marker,
				CurrentRow:      Some(2),
				Data:            rowHeaderTestData(),
			}), w)
			rows := tableRows(&layout)
			if got := rowHeaderCellText(t, &rows[2]); got != tc.want {
				t.Errorf("marker = %q, want %q", got, tc.want)
			}
		})
	}
}

// RowHeaderTextStyle styles the marker and the corner glyph. Left
// zero, the marker takes TextStyle and the corner TextStyleHead, as
// before the field existed.
func TestTableRowHeaderTextStyle(t *testing.T) {
	body := TextStyle{Size: 28, Color: RGB(1, 2, 3)}
	head := TextStyle{Size: 30, Color: RGB(4, 5, 6)}
	small := TextStyle{Size: 10, Color: RGB(7, 8, 9)}
	build := func(rhs TextStyle) []Layout {
		w := &Window{}
		layout := generateViewLayout(Table(TableCfg{
			ID:                 "rh-style",
			RowHeader:          true,
			RowHeaderTextStyle: rhs,
			TextStyle:          body,
			TextStyleHead:      head,
			CurrentRow:         Some(1),
			Data:               rowHeaderTestData(),
		}), w)
		return tableRows(&layout)
	}

	rows := build(TextStyle{})
	if got := rowHeaderCellStyle(t, &rows[1]); got != body {
		t.Errorf("unset: marker style = %+v, want TextStyle", got)
	}
	if got := rowHeaderCellStyle(t, &rows[0]); got != head {
		t.Errorf("unset: corner style = %+v, want TextStyleHead", got)
	}

	rows = build(small)
	if got := rowHeaderCellStyle(t, &rows[1]); got != small {
		t.Errorf("set: marker style = %+v, want RowHeaderTextStyle", got)
	}
	if got := rowHeaderCellStyle(t, &rows[0]); got != small {
		t.Errorf("set: corner style = %+v, want RowHeaderTextStyle", got)
	}
}

// The header cell centres its glyph vertically, so a glyph smaller than
// the row sits in the middle rather than at the top.
func TestTableRowHeaderCellCentred(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(Table(TableCfg{
		ID:        "rh-valign",
		RowHeader: true,
		Data:      rowHeaderTestData(),
	}), w)
	rows := tableRows(&layout)
	for i := range rows {
		if got := rows[i].Children[0].Shape.VAlign; got != VAlignMiddle {
			t.Errorf("row %d header VAlign = %v, want VAlignMiddle", i, got)
		}
	}
}

// CurrentRow lets the app own the marker row: it wins over the row a
// click stored, on a focusable table or not. -1 or an index past the
// data shows no marker.
func TestTableRowHeaderCurrentRow(t *testing.T) {
	markers := func(rows []Layout) []int {
		var at []int
		for i := 1; i < len(rows); i++ {
			if rowHeaderCellText(t, &rows[i]) != "" {
				at = append(at, i)
			}
		}
		return at
	}
	for _, focusable := range []bool{false, true} {
		for _, tc := range []struct {
			name    string
			current Opt[int]
			want    []int
		}{
			{"unset follows click", Opt[int]{}, []int{1}},
			{"set overrides click", Some(3), []int{3}},
			{"minus one hides", Some(-1), nil},
			{"past data hides", Some(99), nil},
			{"negative hides", Some(-5), nil},
		} {
			name := tc.name
			if focusable {
				name += " focusable"
			}
			t.Run(name, func(t *testing.T) {
				w := &Window{}
				cfg := TableCfg{
					ID:         "rh-current",
					RowHeader:  true,
					Focusable:  focusable,
					CurrentRow: tc.current,
					Data:       rowHeaderTestData(),
					OnSelect:   func(map[int]bool, int, EventCtx) {},
				}
				layout := generateViewLayout(Table(cfg), w)
				rows := tableRows(&layout)
				// A click stores row 1 as current.
				clickRowHeader(t, &rows[1], w, ModLMB)
				layout = generateViewLayout(Table(cfg), w)
				rows = tableRows(&layout)
				if got := markers(rows); !slices.Equal(got, tc.want) {
					t.Errorf("marker rows = %v, want %v", got, tc.want)
				}
			})
		}
	}
}

// A RowHeaderTextStyle taller than TextStyle makes the header cell set
// the row height, so the virtualization estimate must count it. A
// smaller one, or a table with no row header, leaves the estimate at
// TextStyle.
func TestTableEstimateRowHeightCountsRowHeaderStyle(t *testing.T) {
	body := TextStyle{Size: 14}
	big := TextStyle{Size: 30}
	small := TextStyle{Size: 8}
	pad := NewPadding(2, 0, 3, 0)
	for _, tc := range []struct {
		name      string
		rowHeader bool
		rhs       TextStyle
		want      float32
	}{
		{"no row header ignores style", false, big, 14 + 5},
		{"taller style wins", true, big, 30 + 5},
		{"smaller style ignored", true, small, 14 + 5},
		{"unset style", true, TextStyle{}, 14 + 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tableEstimateRowHeight(&TableCfg{
				TextStyle:          body,
				CellPadding:        pad,
				RowHeader:          tc.rowHeader,
				RowHeaderTextStyle: tc.rhs,
			}, nil)
			if got != tc.want {
				t.Errorf("estimate = %v, want %v", got, tc.want)
			}
		})
	}
}
