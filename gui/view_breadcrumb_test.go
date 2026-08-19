package gui

import "testing"

func TestBreadcrumbBasic(t *testing.T) {
	v := Breadcrumb(BreadcrumbCfg{
		ID:       "bc",
		Selected: "b",
		Items: []BreadcrumbItemCfg{
			{ID: "a", Label: "A"},
			{ID: "b", Label: "B"},
			{ID: "c", Label: "C"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// Outer column has 1 child: trail row (no content views).
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
}

func TestBreadcrumbWithContent(t *testing.T) {
	v := Breadcrumb(BreadcrumbCfg{
		ID:       "bc",
		Selected: "a",
		Items: []BreadcrumbItemCfg{
			{ID: "a", Label: "A", Content: []View{
				Text(TextCfg{Text: "content A"}),
			}},
			{ID: "b", Label: "B"},
		},
		OnSelect: func(_ string, ctx EventCtx) {},
	})
	w := &Window{}
	layout := generateViewLayout(v, w)
	// trail row + content column.
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
}

func TestNewBreadcrumbItem(t *testing.T) {
	content := []View{Text(TextCfg{Text: "x"})}
	item := NewBreadcrumbItem("id-1", "Label", content)
	if item.ID != "id-1" || item.Label != "Label" {
		t.Fatalf("item = %+v, want ID id-1 Label Label", item)
	}
	if len(item.Content) != 1 {
		t.Fatalf("Content = %d views, want 1", len(item.Content))
	}
	// Empty content is a valid call (text-only crumb).
	empty := NewBreadcrumbItem("id-2", "L", nil)
	if empty.ID != "id-2" || empty.Content != nil {
		t.Fatalf("empty-content item = %+v", empty)
	}
}

func TestBcSelectedIndex(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	if idx := bcSelectedIndex(items, "b"); idx != 1 {
		t.Errorf("got %d, want 1", idx)
	}
	// Falls back to last enabled.
	if idx := bcSelectedIndex(items, "missing"); idx != 2 {
		t.Errorf("got %d, want 2", idx)
	}
}

func TestBcSelectedIndexDisabled(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B", Disabled: true},
		{ID: "c", Label: "C"},
	}
	// Selecting disabled item falls back.
	if idx := bcSelectedIndex(items, "b"); idx != 2 {
		t.Errorf("got %d, want 2", idx)
	}
}

func TestBcNavigationHelpers(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A", Disabled: true},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
		{ID: "d", Label: "D", Disabled: true},
	}
	if idx := bcFirstEnabledIndex(items); idx != 1 {
		t.Errorf("first = %d, want 1", idx)
	}
	if idx := bcLastEnabledIndex(items); idx != 2 {
		t.Errorf("last = %d, want 2", idx)
	}
	if idx := bcNextEnabledIndex(items, 1); idx != 2 {
		t.Errorf("next from 1 = %d, want 2", idx)
	}
	// Wrap around.
	if idx := bcNextEnabledIndex(items, 2); idx != 1 {
		t.Errorf("next from 2 = %d, want 1 (wrap)", idx)
	}
	if idx := bcPrevEnabledIndex(items, 2); idx != 1 {
		t.Errorf("prev from 2 = %d, want 1", idx)
	}
	// Wrap around.
	if idx := bcPrevEnabledIndex(items, 1); idx != 2 {
		t.Errorf("prev from 1 = %d, want 2 (wrap)", idx)
	}
}

func TestBcHasAnyContent(t *testing.T) {
	if bcHasAnyContent(nil) {
		t.Error("nil should be false")
	}
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}}
	if bcHasAnyContent(items) {
		t.Error("no content should be false")
	}
	items[0].Content = []View{Text(TextCfg{Text: "x"})}
	if !bcHasAnyContent(items) {
		t.Error("has content should be true")
	}
}

func TestBcCrumbID(t *testing.T) {
	id := bcCrumbID("nav", "home")
	if id != "nav:crumb:home" {
		t.Errorf("got %q", id)
	}
}

func TestBcOnKeydown(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	var selected string
	onSelect := func(id string, ctx EventCtx) {
		selected = id
	}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}

	bcOnKeydown(false, items, "a", onSelect, "", e, w)
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

// Space activates the selected crumb, driven the way a backend drives
// it: an EventKeyDown carrying KeyCode == KeySpace and CharCode == 0.
// The handler used to test CharCode, which backends populate only on
// EventChar, so the spacebar reached it as a no-op.
func TestBcKeydownSpaceSelects(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	var selected string
	fires := 0
	w.TestRender(func(_ *Window) View {
		return Breadcrumb(BreadcrumbCfg{
			ID:        "bc",
			Focusable: true,
			Selected:  "b",
			Items: []BreadcrumbItemCfg{
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
	if err := w.TestKey("bc", KeySpace, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	// Space re-fires on the already-selected crumb (the refire rule),
	// so both the ID and the callback firing are asserted.
	if fires != 1 {
		t.Errorf("OnSelect fired %d times, want 1", fires)
	}
	if selected != "b" {
		t.Errorf("selected = %q, want b", selected)
	}
}

// A keydown never carries CharCode, so nothing may key off it.
func TestBcKeydownCharCodeOnlyIsInert(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A"},
		{ID: "b", Label: "B"},
	}
	fired := false
	onSelect := func(_ string, _ EventCtx) { fired = true }
	w := &Window{}
	e := &Event{CharCode: charSpace}

	bcOnKeydown(false, items, "a", onSelect, "", e, w)
	if fired {
		t.Error("a CharCode-only keydown selected a crumb")
	}
	if e.IsHandled {
		t.Error("a CharCode-only keydown was consumed")
	}
}

// TestBreadcrumbHoverPressedColor pins the pressed-while-hovered branch
// of makeBcOnHover (view_breadcrumb.go): a press-and-hold on a crumb
// renders the click color, release falls back to the hover color. The
// crumb under test is unselected — a selected crumb paints the selected
// color in every hover state, so it cannot exercise the branch.
func TestBreadcrumbHoverPressedColor(t *testing.T) {
	hover := RGBA(40, 200, 40, 255)
	click := RGBA(200, 40, 40, 255)
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View {
		return Breadcrumb(BreadcrumbCfg{
			ID:       "bc",
			OnSelect: func(string, EventCtx) {},
			Items: []BreadcrumbItemCfg{
				{ID: "one", Label: "One"},
				{ID: "two", Label: "Two"},
			},
			Selected:        "two",
			ColorCrumbHover: hover,
			ColorCrumbClick: click,
		})
	})

	const crumbID = "bc:crumb:one"
	x, y := hoverOver(t, w, crumbID)
	if c := mustShape(t, w, crumbID).Color; c != hover {
		t.Fatalf("hover: color = %+v, want %+v", c, hover)
	}
	pressAt(w, MouseLeft, x, y)
	if c := mustShape(t, w, crumbID).Color; c != click {
		t.Fatalf("held: color = %+v, want %+v", c, click)
	}
	releaseAt(w, MouseLeft, x, y)
	if c := mustShape(t, w, crumbID).Color; c != hover {
		t.Fatalf("released: color = %+v, want %+v", c, hover)
	}
}

// --- bcOnKeydown guard paths ---

func TestBcKeydownDisabled(t *testing.T) {
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}}
	fired := false
	onSelect := func(_ string, _ EventCtx) { fired = true }
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	bcOnKeydown(true, items, "a", onSelect, "", e, w)
	if fired {
		t.Error("disabled breadcrumb must not fire OnSelect")
	}
	if e.IsHandled {
		t.Error("disabled breadcrumb must not consume the event")
	}
}

func TestBcKeydownEmptyItems(t *testing.T) {
	fired := false
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	bcOnKeydown(false, nil, "", func(_ string, _ EventCtx) { fired = true }, "", e, w)
	if fired || e.IsHandled {
		t.Error("empty breadcrumb must be inert")
	}
}

func TestBcKeydownModifierGuard(t *testing.T) {
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
	fired := false
	w := &Window{}
	e := &Event{KeyCode: KeyRight, Modifiers: ModCtrl}
	bcOnKeydown(false, items, "a", func(_ string, _ EventCtx) { fired = true }, "", e, w)
	if fired {
		t.Error("modified key must not navigate the breadcrumb")
	}
	if e.IsHandled {
		t.Error("modified key must not be consumed")
	}
}

func TestBcKeydownHomeEnd(t *testing.T) {
	items := []BreadcrumbItemCfg{
		{ID: "a", Label: "A", Disabled: true},
		{ID: "b", Label: "B"},
		{ID: "c", Label: "C"},
	}
	var selected string
	onSelect := func(id string, _ EventCtx) { selected = id }
	w := &Window{}

	// Home jumps to the first enabled crumb (skips disabled "a" → "b").
	bcOnKeydown(false, items, "c", onSelect, "", &Event{KeyCode: KeyHome}, w)
	if selected != "b" {
		t.Errorf("Home selected %q, want b", selected)
	}
	// End jumps to the last enabled crumb.
	bcOnKeydown(false, items, "b", onSelect, "", &Event{KeyCode: KeyEnd}, w)
	if selected != "c" {
		t.Errorf("End selected %q, want c", selected)
	}
}

func TestBcKeydownLeftNoSelection(t *testing.T) {
	// With no selection, the breadcrumb anchors on the last enabled
	// item, so Left moves to the previous one from there.
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
	var selected string
	onSelect := func(id string, _ EventCtx) { selected = id }
	w := &Window{}
	bcOnKeydown(false, items, "", onSelect, "", &Event{KeyCode: KeyLeft}, w)
	if selected != "a" {
		t.Errorf("Left without selection = %q, want a (prev of last enabled)", selected)
	}
}

func TestBcKeydownEnterRefires(t *testing.T) {
	// Enter on the already-selected crumb re-fires (activation).
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}, {ID: "b", Label: "B"}}
	fires := 0
	onSelect := func(_ string, _ EventCtx) { fires++ }
	w := &Window{}
	e := &Event{KeyCode: KeyEnter}
	bcOnKeydown(false, items, "b", onSelect, "", e, w)
	if fires != 1 {
		t.Errorf("Enter fires = %d, want 1", fires)
	}
	if !e.IsHandled {
		t.Error("Enter should be handled")
	}
}

func TestBcKeydownSetsFocus(t *testing.T) {
	items := []BreadcrumbItemCfg{{ID: "a", Label: "A"}}
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	bcOnKeydown(false, items, "", nil, "bc-focus", e, w)
	if w.FocusID() != "bc-focus" {
		t.Errorf("focus = %q, want bc-focus", w.FocusID())
	}
	if !e.IsHandled {
		t.Error("navigating key should be handled")
	}
}

func TestBcKeydownEmptyTargetID(t *testing.T) {
	// An item with an empty ID cannot be selected: no OnSelect, but
	// the event is still consumed (the key was a navigation key).
	items := []BreadcrumbItemCfg{{ID: "", Label: "No ID"}}
	fired := false
	w := &Window{}
	e := &Event{KeyCode: KeyRight}
	bcOnKeydown(false, items, "", func(_ string, _ EventCtx) { fired = true }, "", e, w)
	if fired {
		t.Error("empty target ID must not fire OnSelect")
	}
}
