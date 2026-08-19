package gui

import (
	"strings"
	"testing"
)

func TestSelectGeneratesClosedLayout(t *testing.T) {
	w := &Window{}
	v := Select(SelectCfg{
		ID:       "s1",
		Options:  []string{"A", "B", "C"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.ID != "s1" {
		t.Errorf("expected ID s1, got %s", layout.Shape.ID)
	}
}

func TestSelectGeneratesDropdownWhenOpen(t *testing.T) {
	w := &Window{}
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	ss.Set("s2", true)
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	sh.Set("s2", 0)

	v := Select(SelectCfg{
		ID:       "s2",
		Options:  []string{"A", "B", "C"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	sv := v.(*selectView)
	layout := sv.GenerateLayout(w)

	// When open, layout should have 4 children: text, spacer,
	// arrow, and the float dropdown.
	if len(layout.Children) != 4 {
		t.Errorf("expected 4 children when open, got %d",
			len(layout.Children))
	}
	last := layout.Children[len(layout.Children)-1]
	if last.Shape == nil || !last.Shape.Float {
		t.Error("expected last child to be a float dropdown")
	}
}

func TestSelectArrowChangesWithState(t *testing.T) {
	w := &Window{}
	v := Select(SelectCfg{
		ID:       "s3",
		Options:  []string{"X"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	})
	sv := v.(*selectView)

	// Closed: 3 children (text, spacer, arrow).
	layout := sv.GenerateLayout(w)
	if len(layout.Children) != 3 {
		t.Fatalf("expected 3 children when closed, got %d",
			len(layout.Children))
	}

	// Open: 4 children (text, spacer, arrow, dropdown).
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	ss.Set("s3", true)
	layout = sv.GenerateLayout(w)
	if len(layout.Children) != 4 {
		t.Errorf("expected 4 children when open, got %d",
			len(layout.Children))
	}
}

func TestSelectOptionViewOnClickFires(t *testing.T) {
	fired := false
	var selected []string
	cfg := &SelectCfg{
		ID:      "s4",
		Options: []string{"A", "B"},
		OnSelect: func(s []string, ctx EventCtx) {
			fired = true
			selected = s
		},
		TextStyle: DefaultTextStyle,
	}
	applySelectDefaults(cfg)
	v := selectOptionView(cfg, cfg.ID, "B", 1, false)
	cv := v.(*containerView)

	w := &Window{}
	e := &Event{MouseButton: MouseLeft}
	layout := cv.GenerateLayout(w)
	if layout.Shape.events != nil && layout.Shape.events.OnClick != nil {
		layout.Shape.events.OnClick(EventCtx{nil, e, w})
	}
	if !fired {
		t.Error("OnSelect not fired")
	}
	if len(selected) != 1 || selected[0] != "B" {
		t.Errorf("expected [B], got %v", selected)
	}
}

func TestSelectSubHeaderView(t *testing.T) {
	cfg := &SelectCfg{
		ID: "s5",
		SubheadingStyle: TextStyle{
			Color: RGB(180, 180, 180),
			Size:  14,
		},
	}
	v := selectSubHeaderView(cfg, "---Section")
	cv := v.(*containerView)
	if len(cv.content) != 2 {
		t.Errorf("expected 2 children (header + underline), got %d",
			len(cv.content))
	}
}

func TestSelectKeyboardNavigation(t *testing.T) {
	w := &Window{}
	cfg := SelectCfg{
		ID:       "s6",
		Options:  []string{"A", "B", "C"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")

	// Open via space.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected space open to be handled")
	}
	ss := StateMap[string, bool](w, nsSelect, capModerate)
	isOpen, _ := ss.Get("s6")
	if !isOpen {
		t.Error("expected open after space")
	}

	// Navigate down.
	e = &Event{KeyCode: KeyDown}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected down navigation to be handled")
	}
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	idx, _ := sh.Get("s6")
	if idx != 1 {
		t.Errorf("expected highlight 1, got %d", idx)
	}

	// Close via escape.
	e = &Event{KeyCode: KeyEscape}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected escape close to be handled")
	}
	isOpen, _ = ss.Get("s6")
	if isOpen {
		t.Error("expected closed after escape")
	}
}

func TestSelectKeyboardSelectItem(t *testing.T) {
	w := &Window{}
	var selected []string
	cfg := SelectCfg{
		ID:      "s7",
		Options: []string{"A", "B"},
		OnSelect: func(s []string, ctx EventCtx) {
			selected = s
		},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)

	// Select current (A at index 0).
	e = &Event{KeyCode: KeyEnter}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected enter select to be handled")
	}
	if len(selected) != 1 || selected[0] != "A" {
		t.Errorf("expected [A], got %v", selected)
	}
}

func TestSelectSkipsSubHeaders(t *testing.T) {
	w := &Window{}
	cfg := SelectCfg{
		ID:       "s8",
		Options:  []string{"A", "---Section", "B"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)

	// Navigate down past subheader.
	e = &Event{KeyCode: KeyDown}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected navigation to be handled")
	}
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	idx, _ := sh.Get("s8")
	// Should skip "---Section" and land on "B" (index 2).
	if idx != 2 {
		t.Errorf("expected 2 (skip subheader), got %d", idx)
	}
}

func TestSelectHomeEndKeys(t *testing.T) {
	w := &Window{}
	cfg := SelectCfg{
		ID:       "she",
		Options:  []string{"A", "---S", "B", "C"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	}
	applySelectDefaults(&cfg)
	idScroll := ScopeID(cfg.ID, "dropdown")

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)

	// End → last selectable (C at index 3).
	e = &Event{KeyCode: KeyEnd}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected End to be handled")
	}
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	idx, _ := sh.Get("she")
	if idx != 3 {
		t.Errorf("expected 3 (C), got %d", idx)
	}

	// Home → first selectable (A at index 0).
	e = &Event{KeyCode: KeyHome}
	selectOnKeyDown(&cfg, cfg.ID, idScroll, e, w)
	if !e.IsHandled {
		t.Error("expected Home to be handled")
	}
	idx, _ = sh.Get("she")
	if idx != 0 {
		t.Errorf("expected 0 (A), got %d", idx)
	}
}

func TestSelectClickOpenResetsHighlight(t *testing.T) {
	w := &Window{}
	cfg := SelectCfg{
		ID:       "scr",
		Options:  []string{"A", "B", "C"},
		Selected: []string{"B"},
		OnSelect: func(_ []string, ctx EventCtx) {},
	}
	applySelectDefaults(&cfg)
	v := Select(cfg)
	sv := v.(*selectView)
	layout := sv.GenerateLayout(w)

	// Simulate click to open.
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(EventCtx{&layout, e, w})

	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	idx, _ := sh.Get("scr")
	if idx != 1 {
		t.Errorf("expected highlight 1 (B), got %d", idx)
	}
}

func TestSelectDefaultMinMaxWidth(t *testing.T) {
	cfg := SelectCfg{ID: "sdm"}
	applySelectDefaults(&cfg)
	if cfg.MinWidth != defaultSelectStyle.MinWidth {
		t.Errorf("expected MinWidth %v, got %v",
			defaultSelectStyle.MinWidth, cfg.MinWidth)
	}
	if cfg.MaxWidth != defaultSelectStyle.MaxWidth {
		t.Errorf("expected MaxWidth %v, got %v",
			defaultSelectStyle.MaxWidth, cfg.MaxWidth)
	}
}

func TestFnvSum32Consistency(t *testing.T) {
	a := fnvSum32("test")
	b := fnvSum32("test")
	if a != b {
		t.Error("FnvSum32 not consistent")
	}
	if fnvSum32("a") == fnvSum32("b") {
		t.Error("expected different hashes")
	}
}

func TestSelectPlaceholderWhenEmpty(t *testing.T) {
	w := &Window{}
	v := Select(SelectCfg{
		ID:          "s9",
		Placeholder: "Choose...",
		Options:     []string{"A"},
		OnSelect:    func(_ []string, ctx EventCtx) {},
	})
	sv := v.(*selectView)
	layout := sv.GenerateLayout(w)
	// First child is the text; should show placeholder.
	if len(layout.Children) < 1 || layout.Children[0].Shape == nil {
		t.Fatal("expected text child")
	}
	if layout.Children[0].Shape.TC == nil ||
		layout.Children[0].Shape.TC.Text != "Choose..." {
		t.Error("expected placeholder text 'Choose...'")
	}
}

func TestSelectMultipleJoinsSelected(t *testing.T) {
	w := &Window{}
	v := Select(SelectCfg{
		ID:             "s10",
		Selected:       []string{"A", "B"},
		Options:        []string{"A", "B", "C"},
		SelectMultiple: true,
		OnSelect:       func(_ []string, ctx EventCtx) {},
	})
	sv := v.(*selectView)
	layout := sv.GenerateLayout(w)
	_ = layout
	// The text should be "A, B".
	txt := strings.Join(sv.cfg.Selected, ", ")
	if txt != "A, B" {
		t.Errorf("expected 'A, B', got %s", txt)
	}
}

// selectLabelY renders a Select and returns the arranged Y of its label
// shape — the one number optical centring moves.
func selectLabelY(t *testing.T, cfg SelectCfg) float32 {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	cfg.ID = "s"
	w.TestRender(func(*Window) View { return Select(cfg) })
	field, ok := w.layout.FindByID("s")
	if !ok {
		t.Fatal("no select in the rendered window")
	}
	if len(field.Children) == 0 || field.Children[0].Shape == nil ||
		field.Children[0].Shape.shapeType != shapeText {
		t.Fatal("first child of the select is not its label")
	}
	return field.Children[0].Shape.Y
}

// A Select swaps its label as the selection changes, so the correction
// must be content-free: a descender-bearing label and a cap-only one
// have to land on the same baseline, or picking an option would step
// the label vertically in a control that has not moved (issue #346).
func TestSelectOpticalCenterDoesNotFollowContent(t *testing.T) {
	base := selectLabelY(t, SelectCfg{Placeholder: "PICK"})
	for _, label := range []string{
		"Pick a language", // descends
		"gypsy",           // descends, no caps
		"Go",
	} {
		got := selectLabelY(t, SelectCfg{Placeholder: label})
		if got != base {
			t.Errorf("label %q: text Y %v, want %v (content-free)",
				label, got, base)
		}
	}
}

// And it is applied at all: the label sits below where metric centring
// alone would put it. The plain Input is the counter-case that must not
// move — see TestInputOpticalCenterIsOptIn.
func TestSelectOpticalCenterIsApplied(t *testing.T) {
	// A wrapping multi-select is the uncorrected reference: it is the
	// one spelling of this widget the hook deliberately skips, and it
	// centres the same label in the same row otherwise.
	uncorrected := selectLabelY(t, SelectCfg{
		Placeholder: "PICK", SelectMultiple: true,
	})
	corrected := selectLabelY(t, SelectCfg{Placeholder: "PICK"})
	if corrected <= uncorrected {
		t.Errorf("corrected label Y %v, want below uncorrected %v",
			corrected, uncorrected)
	}
}
