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
		OnSelect: func([]string, *Event, *Window) {},
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
		OnSelect: func([]string, *Event, *Window) {},
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
		OnSelect: func([]string, *Event, *Window) {},
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
		OnSelect: func(s []string, _ *Event, _ *Window) {
			fired = true
			selected = s
		},
		TextStyle: DefaultTextStyle,
	}
	applySelectDefaults(cfg)
	v := selectOptionView(cfg, "B", 1, false)
	cv := v.(*containerView)

	w := &Window{}
	e := &Event{MouseButton: MouseLeft}
	layout := cv.GenerateLayout(w)
	if layout.Shape.events != nil && layout.Shape.events.OnClick != nil {
		layout.Shape.events.OnClick(nil, e, w)
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
		OnSelect: func([]string, *Event, *Window) {},
	}
	applySelectDefaults(&cfg)
	idScroll := cfg.ID + ".dropdown"

	// Open via space.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, idScroll, e, w)
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
	selectOnKeyDown(&cfg, idScroll, e, w)
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
	selectOnKeyDown(&cfg, idScroll, e, w)
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
		OnSelect: func(s []string, _ *Event, _ *Window) {
			selected = s
		},
	}
	applySelectDefaults(&cfg)
	idScroll := cfg.ID + ".dropdown"

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, idScroll, e, w)

	// Select current (A at index 0).
	e = &Event{KeyCode: KeyEnter}
	selectOnKeyDown(&cfg, idScroll, e, w)
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
		OnSelect: func([]string, *Event, *Window) {},
	}
	applySelectDefaults(&cfg)
	idScroll := cfg.ID + ".dropdown"

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, idScroll, e, w)

	// Navigate down past subheader.
	e = &Event{KeyCode: KeyDown}
	selectOnKeyDown(&cfg, idScroll, e, w)
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
		OnSelect: func([]string, *Event, *Window) {},
	}
	applySelectDefaults(&cfg)
	idScroll := cfg.ID + ".dropdown"

	// Open.
	e := &Event{KeyCode: KeySpace}
	selectOnKeyDown(&cfg, idScroll, e, w)

	// End → last selectable (C at index 3).
	e = &Event{KeyCode: KeyEnd}
	selectOnKeyDown(&cfg, idScroll, e, w)
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
	selectOnKeyDown(&cfg, idScroll, e, w)
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
		OnSelect: func([]string, *Event, *Window) {},
	}
	applySelectDefaults(&cfg)
	v := Select(cfg)
	sv := v.(*selectView)
	layout := sv.GenerateLayout(w)

	// Simulate click to open.
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(&layout, e, w)

	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	idx, _ := sh.Get("scr")
	if idx != 1 {
		t.Errorf("expected highlight 1 (B), got %d", idx)
	}
}

func TestSelectDefaultMinMaxWidth(t *testing.T) {
	cfg := SelectCfg{ID: "sdm"}
	applySelectDefaults(&cfg)
	if cfg.MinWidth != DefaultSelectStyle.MinWidth {
		t.Errorf("expected MinWidth %v, got %v",
			DefaultSelectStyle.MinWidth, cfg.MinWidth)
	}
	if cfg.MaxWidth != DefaultSelectStyle.MaxWidth {
		t.Errorf("expected MaxWidth %v, got %v",
			DefaultSelectStyle.MaxWidth, cfg.MaxWidth)
	}
}

func TestFnvSum32Consistency(t *testing.T) {
	a := FnvSum32("test")
	b := FnvSum32("test")
	if a != b {
		t.Error("FnvSum32 not consistent")
	}
	if FnvSum32("a") == FnvSum32("b") {
		t.Error("expected different hashes")
	}
}

func TestSelectPlaceholderWhenEmpty(t *testing.T) {
	w := &Window{}
	v := Select(SelectCfg{
		ID:          "s9",
		Placeholder: "Choose...",
		Options:     []string{"A"},
		OnSelect:    func([]string, *Event, *Window) {},
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
		OnSelect:       func([]string, *Event, *Window) {},
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
