package gui

import "testing"

func TestComboboxClosedLayout(t *testing.T) {
	w := &Window{}
	v := Combobox(ComboboxCfg{
		ID:          "cb1",
		Value:       "Apple",
		Options:     []string{"Apple", "Banana", "Cherry"},
		Placeholder: "Pick fruit",
		OnSelect:    func(_ string, _ *Event, _ *Window) {},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "cb1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if layout.Shape.shapeType != shapeRectangle {
		t.Errorf("type = %d", layout.Shape.shapeType)
	}
}

func TestComboboxOpenLayout(t *testing.T) {
	w := &Window{}
	// Pre-set open state.
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	ss.Set("cb-open", true)

	v := Combobox(ComboboxCfg{
		ID:       "cb-open",
		Options:  []string{"A", "B", "C"},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	layout := generateViewLayout(v, w)
	// Should have children (input, spacer, arrow, dropdown).
	if len(layout.Children) < 3 {
		t.Errorf("children = %d, want >= 3", len(layout.Children))
	}
}

func TestComboboxOpenClose(t *testing.T) {
	w := &Window{}
	comboboxOpen("test-oc", "", w)
	isOpen := StateReadOr(w, nsCombobox, "test-oc", false)
	if !isOpen {
		t.Error("expected open")
	}
	comboboxClose("test-oc", w)
	isOpen = StateReadOr(w, nsCombobox, "test-oc", false)
	if isOpen {
		t.Error("expected closed")
	}
}

func TestComboboxKeyDownOpenClose(t *testing.T) {
	w := &Window{}
	called := ""
	onSel := func(id string, _ *Event, _ *Window) { called = id }

	// Open via Enter.
	e := &Event{KeyCode: KeyEnter}
	comboboxOnKeyDown("cb-kd", onSel, "", []string{"x", "y"}, "", 0, 0, e, w)
	if !StateReadOr(w, nsCombobox, "cb-kd", false) {
		t.Error("enter should open")
	}

	// Select via Enter.
	e = &Event{KeyCode: KeyEnter}
	comboboxOnKeyDown("cb-kd", onSel, "", []string{"x", "y"}, "", 0, 0, e, w)
	if called != "x" {
		t.Errorf("selected = %q, want x", called)
	}
}

func TestComboboxKeyNavScrolls(t *testing.T) {
	w := &Window{}
	idScroll := "77"
	rowH := float32(26)
	listH := float32(187)
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"}
	onSel := func(_ string, _ *Event, _ *Window) {}

	// Open the combobox.
	comboboxOpen("cb-nav", "", w)

	// Navigate down 8 times (past visible ~7 items).
	for range 8 {
		e := &Event{KeyCode: KeyDown}
		comboboxOnKeyDown("cb-nav", onSel, "", ids, idScroll, rowH, listH, e, w)
	}

	hl := StateReadOr(w, nsComboboxHighlight, "cb-nav", -1)
	if hl != 8 {
		t.Fatalf("highlight = %d, want 8", hl)
	}

	sy, _ := w.scrollY().Get(idScroll)
	if sy >= 0 {
		t.Fatalf("scrollY = %f, want negative (scrolled down)", sy)
	}
}

func TestComboboxDefaults(t *testing.T) {
	cfg := ComboboxCfg{}
	applyComboboxDefaults(&cfg)
	if cfg.MaxDropdownHeight != 200 {
		t.Errorf("max dropdown = %f", cfg.MaxDropdownHeight)
	}
	if cfg.MinWidth != 75 {
		t.Errorf("min width = %f", cfg.MinWidth)
	}
}

func TestScrollEnsureVisible(t *testing.T) {
	w := &Window{}
	idScroll := "50"
	rowH := float32(26)
	listH := float32(187)

	// Initially scrollY = 0. Item 7: bottom = 208 > 187. Should scroll.
	scrollEnsureVisible(idScroll, 7, rowH, listH, w)
	sy, _ := w.scrollY().Get(idScroll)
	if sy >= 0 {
		t.Errorf("expected negative scrollY, got %f", sy)
	}
	want := -(float32(8)*rowH - listH) // -(208-187) = -21
	if sy != want {
		t.Errorf("scrollY = %f, want %f", sy, want)
	}

	// Scroll back up to item 0.
	scrollEnsureVisible(idScroll, 0, rowH, listH, w)
	sy, _ = w.scrollY().Get(idScroll)
	if sy != 0 {
		t.Errorf("scrollY = %f, want 0", sy)
	}
}

func TestComboboxBackspaceNotDelete(t *testing.T) {
	w := &Window{}
	id := "cb-del"

	// Open and set a query.
	comboboxOpen(id, "", w)
	sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
	sq.Set(id, "abc")

	// Delete key should not remove characters.
	e := &Event{KeyCode: KeyDelete}
	comboboxOnKeyDown(id, nil, "", []string{"a"}, "", 0, 0, e, w)
	q, _ := sq.Get(id)
	if q != "abc" {
		t.Errorf("Delete modified query: got %q, want abc", q)
	}

	// Backspace should remove the last character.
	e = &Event{KeyCode: KeyBackspace}
	comboboxOnKeyDown(id, nil, "", []string{"a"}, "", 0, 0, e, w)
	q, _ = sq.Get(id)
	if q != "ab" {
		t.Errorf("Backspace: got %q, want ab", q)
	}
}

func TestScrollEnsureVisibleSurvivesPipeline(t *testing.T) {
	w := &Window{}
	idScroll := "60"
	rowH := float32(26)
	listH := float32(187)

	// Set scroll state (simulates scrollEnsureVisible after key nav).
	scrollEnsureVisible(idScroll, 7, rowH, listH, w)
	want := -(float32(8)*rowH - listH) // -21

	// Build a layout tree mimicking the dropdown Column.
	// 10 children at 26px each = 260px total content.
	children := make([]Layout, 10)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Height:    rowH,
			shapeType: shapeRectangle,
			Sizing:    FillFixed,
		}}
	}
	dropdown := Layout{
		Shape: &Shape{
			Scrollable: true,
			ID:         idScroll,
			Height:     200, // MaxHeight clamped
			Axis:       AxisTopToBottom,
			Padding:    Padding{Top: 5, Right: 5, Bottom: 5, Left: 5},
			SizeBorder: 1.5,
		},
		Children: children,
	}

	// Run the scroll offset adjustment.
	layoutAdjustScrollOffsets(&dropdown, w)

	sy, _ := w.scrollY().Get(idScroll)
	if sy != want {
		t.Fatalf("scrollY after pipeline = %f, want %f", sy, want)
	}
}

func TestScrollPositionsShiftChildren(t *testing.T) {
	w := &Window{}
	idScroll := "61"
	rowH := float32(26)

	// Set scroll state to -21 (scrolled down).
	sm := w.scrollY()
	sm.Set(idScroll, -21)

	// Build dropdown layout.
	children := make([]Layout, 10)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Height:    rowH,
			shapeType: shapeRectangle,
			Sizing:    FillFixed,
		}}
	}
	dropdown := Layout{
		Shape: &Shape{
			Scrollable: true,
			ID:         idScroll,
			Height:     200,
			Axis:       AxisTopToBottom,
			Padding:    Padding{Top: 5, Right: 5, Bottom: 5, Left: 5},
			SizeBorder: 1.5,
		},
		Children: children,
	}

	layoutAdjustScrollOffsets(&dropdown, w)
	layoutPositions(&dropdown, 0, 0, w)

	// First child Y should be negative (shifted up by scroll).
	firstY := dropdown.Children[0].Shape.Y
	expectedFirstY := float32(5 + 1.5 - 21) // paddingTop + sizeBorder + scrollY
	if firstY != expectedFirstY {
		t.Fatalf("first child Y = %f, want %f", firstY, expectedFirstY)
	}
}

func TestListCoreRowHeightEstimate(t *testing.T) {
	w := &Window{}
	items := []listCoreItem{{ID: "a", Label: "Test"}}
	cfg := listCoreCfg{
		TextStyle:   DefaultTextStyle,
		PaddingItem: PaddingSmall,
	}
	views := listCoreViews(items, cfg, 0, 0, -1, nil, 26)
	if len(views) == 0 {
		t.Fatal("no views")
	}
	layout := generateViewLayout(views[0], w)
	layoutWidths(&layout)
	layoutHeights(&layout)
	est := listCoreRowHeightEstimate(DefaultTextStyle, PaddingSmall)
	actual := layout.Shape.Height
	if est != actual {
		t.Errorf("estimate=%f, actual=%f", est, actual)
	}
}

func TestComboboxScrollEndToEnd(t *testing.T) {
	w := &Window{}
	w.windowWidth = 800
	w.windowHeight = 600

	idScroll := "cb-e2e.dropdown"
	options := []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J"}

	// Open the combobox.
	comboboxOpen("cb-e2e", "", w)

	// Navigate down 8 times to trigger scroll.
	for range 8 {
		e := &Event{KeyCode: KeyDown}
		comboboxOnKeyDown("cb-e2e", func(_ string, _ *Event, _ *Window) {},
			"", options, idScroll, 26, 187, e, w)
	}

	// Verify scroll state was set.
	syBefore, _ := w.scrollY().Get(idScroll)
	if syBefore >= 0 {
		t.Fatalf("scrollY before = %f, want negative", syBefore)
	}

	// Generate the layout (simulates next frame).
	v := Combobox(ComboboxCfg{
		ID:         "cb-e2e",
		Scrollable: true,
		Options:    options,
		OnSelect:   func(_ string, _ *Event, _ *Window) {},
	})
	layout := generateViewLayout(v, w)

	// Set parent pointers.
	layoutParents(&layout, nil)

	// Extract float (dropdown) like layoutArrange does.
	var floats []*Layout
	layoutRemoveFloatingLayouts(&layout, nil, &floats)
	if len(floats) == 0 {
		t.Fatal("no floats extracted")
	}

	// Find the dropdown float (has IDScroll).
	var dropdown *Layout
	for _, f := range floats {
		if f.Shape.Scrollable && f.Shape.ID == idScroll {
			dropdown = f
			break
		}
	}
	if dropdown == nil {
		t.Fatal("dropdown float not found")
	}

	// Run pipeline on the float like the real renderer does.
	layoutPipeline(dropdown, w)

	// Check scroll state survived pipeline.
	syAfter, _ := w.scrollY().Get(idScroll)
	t.Logf("scrollY: before=%f, after=%f, dropdownH=%f, contentH=%f",
		syBefore, syAfter, dropdown.Shape.Height,
		contentHeight(dropdown))

	if syAfter >= 0 {
		t.Fatalf("scrollY after pipeline = %f, want negative (clamped to 0!)", syAfter)
	}

	// Verify children are positioned with scroll offset.
	if len(dropdown.Children) < 2 {
		t.Fatalf("dropdown children = %d", len(dropdown.Children))
	}
	// With scroll, the first non-overdraw child's Y should be < dropdown.Shape.Y + padding.
	for i, c := range dropdown.Children {
		if !c.Shape.OverDraw {
			t.Logf("child[%d] Y=%f, dropdown Y=%f", i, c.Shape.Y, dropdown.Shape.Y)
			break
		}
	}
}

func TestComboboxItemsCacheInvalidatesOnOptionsChange(t *testing.T) {
	w := &Window{}
	id := "cb-cache"
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	ss.Set(id, true)

	v := Combobox(ComboboxCfg{
		ID:       id,
		Options:  []string{"A"},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	_ = generateViewLayout(v, w)

	cm := StateMapRead[string, *comboboxItemsCache](w, nsComboboxItems)
	if cm == nil {
		t.Fatal("expected combobox items cache map")
	}
	cache, ok := cm.Get(id)
	if !ok || cache == nil {
		t.Fatal("expected combobox cache entry")
	}
	if got := len(cache.items); got != 1 {
		t.Fatalf("cache items len = %d, want 1", got)
	}

	v = Combobox(ComboboxCfg{
		ID:       id,
		Options:  []string{"A", "B"},
		OnSelect: func(_ string, _ *Event, _ *Window) {},
	})
	_ = generateViewLayout(v, w)
	cache, _ = cm.Get(id)
	if got := len(cache.items); got != 2 {
		t.Fatalf("cache items len = %d, want 2", got)
	}
}
