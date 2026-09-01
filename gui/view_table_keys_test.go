package gui

import "testing"

// tableTestRows builds a 3-row table whose rows carry no handlers, so
// tests can focus on selection and position movement.
func tableTestRows() []TableRowCfg {
	return []TableRowCfg{
		{Cells: []TableCellCfg{{Value: "r0"}}},
		{Cells: []TableCellCfg{{Value: "r1"}}},
		{Cells: []TableCellCfg{{Value: "r2"}}},
	}
}

func TestTableOnKeyDownMovesSelection(t *testing.T) {
	w := &Window{}
	var got map[int]bool
	var gotIdx int
	onSelect := func(sel map[int]bool, idx int, ctx EventCtx) {
		got = sel
		gotIdx = idx
	}
	key := func(k KeyCode) {
		e := &Event{KeyCode: k}
		tableOnKeyDown(tableTestRows(), nil, false, onSelect,
			"tbl", "", 0, 0, 0, soundCues{}, e, w)
	}

	key(KeyDown)
	if gotIdx != 1 || !got[1] {
		t.Fatalf("after Down: idx %d sel %v, want 1 {1}", gotIdx, got)
	}
	key(KeyDown)
	if gotIdx != 2 || !got[2] {
		t.Fatalf("after Down: idx %d sel %v, want 2 {2}", gotIdx, got)
	}
	key(KeyDown)
	if gotIdx != 2 {
		t.Fatalf("Down at last row: idx %d, want 2", gotIdx)
	}
	key(KeyUp)
	if gotIdx != 1 {
		t.Fatalf("Up: idx %d, want 1", gotIdx)
	}
	key(KeyHome)
	if gotIdx != 0 {
		t.Fatalf("Home: idx %d, want 0", gotIdx)
	}
	key(KeyEnd)
	if gotIdx != 2 {
		t.Fatalf("End: idx %d, want 2", gotIdx)
	}
	// Single-select movement replaces the selection outright.
	if len(got) != 1 {
		t.Fatalf("single select: sel %v, want 1 entry", got)
	}
}

func TestTableOnKeyDownShiftRange(t *testing.T) {
	w := &Window{}
	var got map[int]bool
	onSelect := func(sel map[int]bool, _ int, _ EventCtx) { got = sel }
	key := func(k KeyCode, mods Modifier) {
		e := &Event{KeyCode: k, Modifiers: mods}
		tableOnKeyDown(tableTestRows(), nil, true, onSelect,
			"tbl", "", 0, 0, 0, soundCues{}, e, w)
	}

	// Plain movement anchors; Shift extends from the anchor.
	key(KeyDown, ModNone)
	if len(got) != 1 || !got[1] {
		t.Fatalf("plain Down: sel %v, want {1}", got)
	}
	key(KeyDown, ModShift)
	if len(got) != 2 || !got[1] || !got[2] {
		t.Fatalf("Shift+Down: sel %v, want {1,2}", got)
	}
	key(KeyHome, ModShift)
	if len(got) != 2 || !got[0] || !got[1] {
		t.Fatalf("Shift+Home from anchor 1: sel %v, want {0,1}", got)
	}
	// A plain move re-anchors.
	key(KeyDown, ModNone)
	if len(got) != 1 || !got[1] {
		t.Fatalf("plain Down re-anchor: sel %v, want {1}", got)
	}
}

func TestTableOnKeyDownNoOnSelectStillMoves(t *testing.T) {
	w := &Window{}
	e := &Event{KeyCode: KeyDown}
	tableOnKeyDown(tableTestRows(), nil, false, nil,
		"tbl", "", 0, 0, 0, soundCues{}, e, w)
	if !e.IsHandled {
		t.Fatal("movement without OnSelect must still consume")
	}
	if got := StateReadOr(w, nsTableFocus, "tbl", -1); got != 1 {
		t.Fatalf("active row = %d, want 1", got)
	}
}

func TestTableOnKeyDownActivate(t *testing.T) {
	activated := 0
	rows := []TableRowCfg{
		{Cells: []TableCellCfg{{Value: "r0"}}},
		{Cells: []TableCellCfg{{Value: "r1"}},
			OnClick: func(EventCtx) { activated++ }},
		{Cells: []TableCellCfg{{Value: "r2"}}},
	}
	w := &Window{}
	key := func(k KeyCode) {
		tableOnKeyDown(rows, nil, false, nil,
			"tbl", "", 0, 0, 0, soundCues{}, &Event{KeyCode: k}, w)
	}

	key(KeyDown) // active -> 1
	key(KeyEnter)
	if activated != 1 {
		t.Fatalf("Enter activated %d rows, want 1", activated)
	}
	key(KeySpace)
	if activated != 2 {
		t.Fatalf("Space activated %d rows, want 2", activated)
	}
}

func TestTableOnKeyDownActivateTogglesSelection(t *testing.T) {
	w := &Window{}
	var got map[int]bool
	onSelect := func(sel map[int]bool, _ int, _ EventCtx) { got = sel }
	// selected is the caller's current state; the handler synthesizes
	// the next one against it, and the caller adopts it (got).
	selected := map[int]bool{1: true}
	key := func(k KeyCode) {
		tableOnKeyDown(tableTestRows(), selected, true,
			onSelect, "tbl", "", 0, 0, 0, soundCues{}, &Event{KeyCode: k}, w)
		selected = got
	}

	key(KeyDown) // movement lands on row 1 and selects it
	if len(got) != 1 || !got[1] {
		t.Fatalf("movement: sel %v, want {1}", got)
	}
	key(KeySpace) // activation toggles row 1 (selected) off
	if len(got) != 0 {
		t.Fatalf("activate on selected row: sel %v, want empty", got)
	}
	key(KeySpace) // and back on
	if len(got) != 1 || !got[1] {
		t.Fatalf("activate again: sel %v, want {1}", got)
	}
}

func TestTableOnKeyDownIgnoresUnrelatedKeys(t *testing.T) {
	w := &Window{}
	for _, k := range []KeyCode{KeyEscape, KeyPageUp, KeyA} {
		e := &Event{KeyCode: k}
		tableOnKeyDown(tableTestRows(), nil, false, nil,
			"tbl", "", 0, 0, 0, soundCues{}, e, w)
		if e.IsHandled {
			t.Errorf("KeyCode %d consumed; want travel", k)
		}
	}
}

// Movement already clamped at an edge (Up at row 0, End at the last
// row) still consumes the key — ListBox and datagrid do the same — so
// it does not fall through to the page.
func TestTableOnKeyDownClampedEdgesConsume(t *testing.T) {
	w := &Window{}
	for _, k := range []KeyCode{KeyUp, KeyHome} {
		e := &Event{KeyCode: k}
		tableOnKeyDown(tableTestRows(), nil, false, nil,
			"tbl", "", 0, 0, 0, soundCues{}, e, w)
		if !e.IsHandled {
			t.Errorf("KeyCode %d at row 0 must consume", k)
		}
	}
	e := &Event{KeyCode: KeyEnd}
	tableOnKeyDown(tableTestRows(), nil, false, nil,
		"tbl", "", 0, 0, 0, soundCues{}, e, w)
	if !e.IsHandled {
		t.Error("End at the last row must consume")
	}
}

func TestTableOnKeyDownModifiedKeysTravel(t *testing.T) {
	w := &Window{}
	called := false
	e := &Event{KeyCode: KeyDown, Modifiers: ModCtrl}
	tableOnKeyDown(tableTestRows(), nil, false,
		func(_ map[int]bool, _ int, _ EventCtx) { called = true },
		"tbl", "", 0, 0, 0, soundCues{}, e, w)
	if e.IsHandled || called {
		t.Error("Ctrl+Down must travel untouched")
	}
}

func TestTableOnKeyDownEmptyDataTravels(t *testing.T) {
	w := &Window{}
	e := &Event{KeyCode: KeyDown}
	tableOnKeyDown(nil, nil, false, nil, "tbl", "", 0, 0, 0, soundCues{}, e, w)
	if e.IsHandled {
		t.Error("empty table must not consume keys")
	}
}

func TestTableOnKeyDownScrollsIntoView(t *testing.T) {
	w := &Window{}
	rows := make([]TableRowCfg, 10)
	for i := range rows {
		rows[i].Cells = []TableCellCfg{{Value: "r"}}
	}
	// rowH 20, listH 40: End lands on row 9, whose bottom (200) is
	// past the 40px viewport, so the scroll settles at 200-40.
	tableOnKeyDown(rows, nil, false, nil, "tbl", "sc", 20, 40, 0,
		soundCues{}, &Event{KeyCode: KeyEnd}, w)
	if got := w.scrollY().GetOr("sc", 0); got != -160 {
		t.Fatalf("scroll = %v, want -160", got)
	}
}

// The freeze layout scrolls a body that holds data rows 1..n, so a
// data index must map to a body row one below it.
func TestTableOnKeyDownFreezeBodyOffset(t *testing.T) {
	w := &Window{}
	rows := make([]TableRowCfg, 10)
	for i := range rows {
		rows[i].Cells = []TableCellCfg{{Value: "r"}}
	}
	// Move to row 1: body row 0, no scroll. Then End: data 9 ->
	// body 8, bottom 180 > 40 -> 180-40.
	tableOnKeyDown(rows, nil, false, nil, "tbl", "sc", 20, 40, 1,
		soundCues{}, &Event{KeyCode: KeyDown}, w)
	if got := w.scrollY().GetOr("sc", 0); got != 0 {
		t.Fatalf("scroll after row 1 = %v, want 0", got)
	}
	tableOnKeyDown(rows, nil, false, nil, "tbl", "sc", 20, 40, 1,
		soundCues{}, &Event{KeyCode: KeyEnd}, w)
	if got := w.scrollY().GetOr("sc", 0); got != -140 {
		t.Fatalf("scroll after End = %v, want -140", got)
	}
}

func TestTableFocusWiring(t *testing.T) {
	focused := generateViewLayout(Table(TableCfg{
		ID:        "tbl",
		Focusable: true,
		Data:      tableTestRows(),
	}), &Window{})
	if !focused.Shape.Focusable {
		t.Error("focusable table must join the tab order")
	}
	ev := focused.Shape.events
	if ev == nil || ev.OnKeyDown == nil {
		t.Fatal("focusable table needs a key handler")
	}
	if ev.AmendLayout == nil {
		t.Fatal("focusable table needs a focus ring")
	}

	plain := generateViewLayout(Table(TableCfg{
		ID:   "tbl2",
		Data: tableTestRows(),
	}), &Window{})
	if plain.Shape.Focusable {
		t.Error("table without Focusable must not join the tab order")
	}
	if ev := plain.Shape.events; ev != nil &&
		(ev.OnKeyDown != nil || ev.AmendLayout != nil) {
		t.Error("non-focusable table must not wire key handling")
	}
}

// A click is also a keyboard position change: the next arrow key
// moves from the clicked row, not from wherever keys last went.
func TestTableClickSyncsKeyboardPosition(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(Table(TableCfg{
		ID:        "tbl",
		Focusable: true,
		Data:      tableTestRows(),
	}), w)
	// children[1] is data row 1 (row 0 is the header row).
	row := l.Children[1]
	row.Shape.events.OnClick(EventCtx{&row, &Event{}, w})
	if got := StateReadOr(w, nsTableFocus, "tbl", -1); got != 1 {
		t.Fatalf("active row after click = %d, want 1", got)
	}
}

// A non-focusable table never reads the keyboard position back, so
// its row clicks must not write it: no dead state, and no surprises
// if the table becomes focusable later with a stale position.
func TestTableNonFocusableClickSkipsStateSync(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(Table(TableCfg{
		ID:   "tbl",
		Data: tableTestRows(),
	}), w)
	row := l.Children[1]
	row.Shape.events.OnClick(EventCtx{&row, &Event{}, w})
	if got := StateReadOr(w, nsTableFocus, "tbl", -1); got != -1 {
		t.Fatalf("non-focusable click wrote position %d", got)
	}
}

// An empty table still carries the tab stop and ring when focusable,
// so a keyboard user lands on it instead of skipping past a control
// with nothing to navigate yet; without Focusable it stays inert.
func TestTableEmptyFocusWiring(t *testing.T) {
	focused := generateViewLayout(Table(TableCfg{
		ID:        "e1",
		Focusable: true,
	}), &Window{})
	if !focused.Shape.Focusable {
		t.Error("focusable empty table must join the tab order")
	}
	if ev := focused.Shape.events; ev == nil || ev.AmendLayout == nil {
		t.Error("focusable empty table needs a focus ring")
	}

	plain := generateViewLayout(Table(TableCfg{ID: "e2"}), &Window{})
	if plain.Shape.Focusable {
		t.Error("plain empty table must not join the tab order")
	}
}

// The freeze layout must wire the focus surface on its outer
// container too: tab stop, ring, and the key handler that navigates
// the scrollable body with the row-offset shift.
func TestTableFreezeFocusWiring(t *testing.T) {
	w := &Window{}
	l := generateViewLayout(Table(TableCfg{
		ID:           "ft",
		Focusable:    true,
		Scrollable:   true,
		MaxHeight:    100,
		FreezeHeader: true,
		Data:         tableTestRows(),
	}), w)
	if !l.Shape.Focusable {
		t.Error("freeze focusable table must join the tab order")
	}
	ev := l.Shape.events
	if ev == nil || ev.OnKeyDown == nil {
		t.Fatal("freeze focusable table needs a key handler")
	}
	if ev.AmendLayout == nil {
		t.Fatal("freeze focusable table needs a focus ring")
	}
}
