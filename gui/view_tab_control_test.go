package gui

import "testing"

func TestTabControlBasic(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Selected: "b",
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
			{ID: "c", Label: "C"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Outer column: header row + content column.
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	// Header row has 3 buttons.
	header := layout.Children[0]
	if len(header.Children) != 3 {
		t.Errorf("header children = %d, want 3",
			len(header.Children))
	}
}

func TestTabsAlias(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Items:    []TabItemCfg{{ID: "a", Label: "A"}},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestTabSelectedIndex(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	if idx := tabSelectedIndex(ids, disabled, "b"); idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}
	// Missing falls back to first.
	if idx := tabSelectedIndex(ids, disabled, "z"); idx != 0 {
		t.Errorf("got %d, want 0", idx)
	}
}

func TestTabNavigationHelpers(t *testing.T) {
	disabled := []bool{true, false, false, true}
	if idx := tabFirstEnabledIndex(disabled); idx != 1 {
		t.Errorf("first = %d, want 1", idx)
	}
	if idx := tabLastEnabledIndex(disabled); idx != 2 {
		t.Errorf("last = %d, want 2", idx)
	}
	if idx := tabNextEnabledIndex(disabled, 1); idx != 2 {
		t.Errorf("next from 1 = %d, want 2", idx)
	}
	if idx := tabNextEnabledIndex(disabled, 2); idx != 1 {
		t.Errorf("next from 2 = %d, want 1 (wrap)", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 2); idx != 1 {
		t.Errorf("prev from 2 = %d, want 1", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 1); idx != 2 {
		t.Errorf("prev from 1 = %d, want 2 (wrap)", idx)
	}
}

func TestTabControlOnKeydown(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	var selected string
	onSelect := func(id string, ctx EventCtx) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	tabControlOnKeydown(false, ids, disabled, "a", onSelect, "", e, w)
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

func TestTabButtonID(t *testing.T) {
	id := tabButtonID("main", "settings")
	if id != "main:tab:settings" {
		t.Errorf("got %q", id)
	}
}

func TestNewTabItem(t *testing.T) {
	item := NewTabItem("t1", "Tab 1", nil)
	if item.ID != "t1" || item.Label != "Tab 1" {
		t.Errorf("got %+v", item)
	}
}

func TestTabControlDisabled(t *testing.T) {
	ids := []string{"a", "b", "c"}
	disabled := []bool{false, false, false}
	var selected string
	onSelect := func(id string, ctx EventCtx) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	tabControlOnKeydown(true, ids, disabled, "a", onSelect, "", e, w)
	if selected != "" {
		t.Errorf("expected no selection, got %q", selected)
	}
	if e.IsHandled {
		t.Error("event should not be handled when disabled")
	}
}

func TestTabControlEmptyItems(t *testing.T) {
	v := TabControl(TabControlCfg{ID: "tabs"})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	header := layout.Children[0]
	if len(header.Children) != 0 {
		t.Errorf("header children = %d, want 0",
			len(header.Children))
	}
}

func TestTabControlAllDisabledItems(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID: "tabs",
		Items: []TabItemCfg{
			{ID: "a", Label: "A", Disabled: true},
			{ID: "b", Label: "B", Disabled: true},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	content := layout.Children[1]
	if len(content.Children) != 0 {
		t.Errorf("content children = %d, want 0",
			len(content.Children))
	}
}

func TestTabNavEmptySlice(t *testing.T) {
	if idx := tabNextEnabledIndex(nil, 0); idx != -1 {
		t.Errorf("next on empty = %d, want -1", idx)
	}
	if idx := tabPrevEnabledIndex(nil, 0); idx != -1 {
		t.Errorf("prev on empty = %d, want -1", idx)
	}
	if idx := tabFirstEnabledIndex(nil); idx != -1 {
		t.Errorf("first on empty = %d, want -1", idx)
	}
	if idx := tabLastEnabledIndex(nil); idx != -1 {
		t.Errorf("last on empty = %d, want -1", idx)
	}
}

func TestTabNavAllDisabled(t *testing.T) {
	disabled := []bool{true, true, true}
	if idx := tabNextEnabledIndex(disabled, 0); idx != -1 {
		t.Errorf("next = %d, want -1", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, 0); idx != -1 {
		t.Errorf("prev = %d, want -1", idx)
	}
}

func TestTabNavOutOfBounds(t *testing.T) {
	disabled := []bool{false, false}
	if idx := tabNextEnabledIndex(disabled, 99); idx != 0 {
		t.Errorf("next from 99 = %d, want 0", idx)
	}
	if idx := tabPrevEnabledIndex(disabled, -5); idx != 1 {
		t.Errorf("prev from -5 = %d, want 1", idx)
	}
}

func TestTabSelectedContent(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Selected: "b",
		Items: []TabItemCfg{
			{ID: "a", Label: "A", Content: []View{
				Text(TextCfg{Text: "A"}),
			}},
			{ID: "b", Label: "B", Content: []View{
				Text(TextCfg{Text: "B1"}),
				Text(TextCfg{Text: "B2"}),
			}},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	content := layout.Children[1]
	if len(content.Children) != 2 {
		t.Errorf("content children = %d, want 2",
			len(content.Children))
	}
}

func TestTabControlReorderable(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:          "tabs",
		Selected:    "a",
		Reorderable: true,
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	header := layout.Children[0]
	if len(header.Children) != 2 {
		t.Errorf("header children = %d, want 2",
			len(header.Children))
	}
}

func TestTabOptZeroOverride(t *testing.T) {
	v := TabControl(TabControlCfg{
		ID:         "tabs",
		Selected:   "a",
		SizeBorder: NoBorder,
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
		},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %v, want 0",
			layout.Shape.SizeBorder)
	}
}

// Space activates the selected tab, driven the way a backend drives it:
// an EventKeyDown carrying KeyCode == KeySpace and CharCode == 0. The
// handler used to test CharCode, which backends populate only on
// EventChar, so the spacebar reached it as a no-op.
func TestTabControlKeydownSpaceSelects(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var selected string
	fires := 0
	w.TestRender(func(_ *Window) View {
		return TabControl(TabControlCfg{
			ID:        "tabs",
			Focusable: true,
			Selected:  "b",
			Items: []TabItemCfg{
				{ID: "a", Label: "A"},
				{ID: "b", Label: "B"},
				{ID: "c", Label: "C"},
			},
			OnSelect: func(id string, _ EventCtx) {
				selected = id
				fires++
			},
		})
	})
	if err := w.TestKey("tabs", KeySpace, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	// Space re-fires on the already-selected tab (the refire rule), so
	// both the ID and the callback firing are asserted.
	if fires != 1 {
		t.Errorf("OnSelect fired %d times, want 1", fires)
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

// A keydown never carries CharCode, so nothing may key off it.
func TestTabControlKeydownCharCodeOnlyIsInert(t *testing.T) {
	ids := []string{"a", "b"}
	disabled := []bool{false, false}
	fired := false
	onSelect := func(_ string, _ EventCtx) { fired = true }
	w := &Window{}
	e := &Event{CharCode: charSpace}

	tabControlOnKeydown(false, ids, disabled, "a", onSelect, "", e, w)
	if fired {
		t.Error("a CharCode-only keydown selected a tab")
	}
	if e.IsHandled {
		t.Error("a CharCode-only keydown was consumed")
	}
}

// A caller's ColorsTab must survive resolution and reach the tab
// button's own set, with ColorTab acting as the shorthand for Base
// (issue #720).
func TestTabControlColorsTabReachTabButton(t *testing.T) {
	base := RGB(1, 0, 0)
	hover := RGB(0, 1, 0)
	border := RGB(0, 0, 1)
	v := TabControl(TabControlCfg{
		ID:       "tabs",
		Selected: "a",
		Items: []TabItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
		},
		ColorTab:  base,
		ColorsTab: ColorSet{Hover: hover, Border: border},
		OnSelect:  func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	// Tab "b" is the unselected one, so it keeps the caller's set.
	tab := layout.Children[0].Children[1]
	if tab.Shape.bc == nil {
		t.Fatal("tab button carries no resolved color set")
	}
	got := tab.Shape.bc.colors
	if !got.Base.eq(base) {
		t.Errorf("Base = %v, want ColorTab %v", got.Base, base)
	}
	if !got.Hover.eq(hover) {
		t.Errorf("Hover = %v, want %v", got.Hover, hover)
	}
	if !got.Border.eq(border) {
		t.Errorf("Border = %v, want %v", got.Border, border)
	}
	// BorderFocus was never spelled, so it falls back to Border.
	if !got.BorderFocus.eq(border) {
		t.Errorf("BorderFocus = %v, want Border %v", got.BorderFocus, border)
	}
}

// A selected tab rests on ColorsTab.Selected, reacts to hover and
// press with the OKLCH step from it, and keeps the set's borders, so
// the focus ring survives selection (#741). Before #741 the selected
// color was pinned into every fill slot and the tab did not react.
func TestTabControlSelectedTabReactsAndKeepsBorders(t *testing.T) {
	sel := RGB(40, 90, 200)
	border := RGB(0, 0, 1)
	v := TabControl(TabControlCfg{
		ID:        "tabs",
		Selected:  "a",
		Items:     []TabItemCfg{{ID: "a", Label: "A"}},
		ColorsTab: ColorSet{Selected: sel, Border: border},
		OnSelect:  func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	tab := layout.Children[0].Children[0]
	if tab.Shape.bc == nil {
		t.Fatal("tab button carries no resolved color set")
	}
	if !tab.Shape.bc.selected {
		t.Fatal("selected tab's button is not marked selected")
	}
	if !tab.Shape.Color.eq(sel) {
		t.Errorf("resting fill = %v, want Selected %v", tab.Shape.Color, sel)
	}
	cs := tab.Shape.bc.colors
	hover, _ := cs.pick(stateFlags{selected: true, hovered: true})
	press, _ := cs.pick(stateFlags{selected: true, pressed: true})
	if want := accentShift(sel, oklchRampDelta); !hover.eq(want) {
		t.Errorf("selected hover = %v, want %v", hover, want)
	}
	if want := accentShift(sel, -oklchRampDelta); !press.eq(want) {
		t.Errorf("selected press = %v, want %v", press, want)
	}
	if hover.eq(sel) || press.eq(sel) {
		t.Error("selected tab does not react to hover or press")
	}
	if !cs.Border.eq(border) {
		t.Errorf("Border = %v, want %v", cs.Border, border)
	}
}

// A disabled tab paints ColorsTab.Disabled, and the renderer does not
// dim that fill again (#741).
func TestTabControlDisabledTabPaintsExplicitColor(t *testing.T) {
	dis := RGB(10, 200, 30)
	v := TabControl(TabControlCfg{
		ID:        "tabs",
		Selected:  "a",
		Items:     []TabItemCfg{{ID: "a", Label: "A"}, {ID: "b", Label: "B", Disabled: true}},
		ColorsTab: ColorSet{Disabled: dis},
		OnSelect:  func(_ string, ctx EventCtx) {},
	})
	layout := generateViewLayout(v, &Window{})
	layoutDisables(&layout, false)
	tab := layout.Children[0].Children[1]
	if !tab.Shape.Disabled {
		t.Fatal("disabled tab shape is not disabled")
	}
	if !tab.Shape.Color.eq(dis) {
		t.Errorf("disabled fill = %v, want %v", tab.Shape.Color, dis)
	}
	if fillDims(tab.Shape) {
		t.Error("explicit disabled fill still takes the render dim")
	}
}
