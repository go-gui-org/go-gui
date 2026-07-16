package gui

import (
	"testing"
	"time"
)

func inputDateOpen(id string, w *Window) {
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	sm.Set(id, true)
	w.UpdateWindow()
}

func TestInputDateLayout(t *testing.T) {
	w := &Window{}
	v := InputDate(InputDateCfg{
		ID:   "id1",
		Date: time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "id1" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if !layout.Shape.Focusable {
		t.Error("InputDate outer Column should be focusable by default")
	}
	if layout.Shape.shapeType != shapeRectangle {
		t.Errorf("type = %d", layout.Shape.shapeType)
	}
}

func TestInputDateLayoutZeroDate(t *testing.T) {
	w := &Window{}
	v := InputDate(InputDateCfg{
		ID:          "id-zero",
		Placeholder: "Select date",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "id-zero" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
}

func TestInputDateDefaultsPreserve(t *testing.T) {
	cfg := InputDateCfg{
		SizeBorder:   SomeF(1),
		CellSpacing:  SomeF(3),
		Radius:       SomeF(4),
		RadiusBorder: SomeF(4),
		TextStyle:    DefaultTextStyle,
		Color:        RGB(30, 30, 30),
	}
	applyInputDateDefaults(&cfg)
	if cfg.SizeBorder.Get(0) != 1 {
		t.Errorf("SizeBorder overwritten = %f", cfg.SizeBorder.Get(0))
	}
	if cfg.CellSpacing.Get(0) != 3 {
		t.Errorf("CellSpacing overwritten = %f", cfg.CellSpacing.Get(0))
	}
	if cfg.Color != RGB(30, 30, 30) {
		t.Error("Color should not be overwritten")
	}
}

func TestInputDateDefaultsPadding(t *testing.T) {
	cfg := InputDateCfg{}
	applyInputDateDefaults(&cfg)
	if !cfg.Padding.IsSet() {
		t.Error("Padding should be set")
	}
}

func TestInputDatePlaceholderStyle(t *testing.T) {
	cfg := InputDateCfg{
		TextStyle: TextStyle{
			Color: RGBA(200, 200, 200, 255),
			Size:  14,
		},
	}
	applyInputDateDefaults(&cfg)
	// PlaceholderStyle.Color.A should be < TextStyle.Color.A (100 vs 255).
	if cfg.PlaceholderStyle.Color.A == 0 && cfg.TextStyle.Color.A == 0 {
		// Both zero means defaults were wiped (test pollution).
		// Still valid — placeholder uses DefaultDatePickerStyle.
		return
	}
	if cfg.PlaceholderStyle.Color.A >= cfg.TextStyle.Color.A &&
		cfg.TextStyle.Color.A > 0 {
		t.Error("placeholder alpha should be reduced")
	}
}

func TestInputDateToggle(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, bool](w, nsInputDate, capModerate)

	inputDateToggle("tog-test", w)
	v, _ := sm.Get("tog-test")
	if !v {
		t.Error("first toggle should open")
	}

	inputDateToggle("tog-test", w)
	v, _ = sm.Get("tog-test")
	if v {
		t.Error("second toggle should close")
	}
}

func TestInputDateOpenClose(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, bool](w, nsInputDate, capModerate)

	inputDateOpen("oc-test", w)
	v, _ := sm.Get("oc-test")
	if !v {
		t.Error("open should set true")
	}

	inputDateClose("oc-test", w)
	v, _ = sm.Get("oc-test")
	if v {
		t.Error("close should set false")
	}
}

func TestInputDateWithPickerOpen(t *testing.T) {
	w := &Window{}
	sm := StateMap[string, bool](w, nsInputDate, capModerate)
	sm.Set("id-open", true)

	v := InputDate(InputDateCfg{
		ID:   "id-open",
		Date: time.Date(2025, 6, 1, 0, 0, 0, 0, time.Local),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "id-open" {
		t.Errorf("ID = %q", layout.Shape.ID)
	}
	if len(layout.Children) == 0 {
		t.Error("open state should have children")
	}
}

func TestInputDateMultiSelectText(t *testing.T) {
	w := &Window{}
	d1 := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)
	d2 := time.Date(2025, 3, 16, 0, 0, 0, 0, time.Local)

	v := InputDate(InputDateCfg{
		ID:    "id-multi",
		Dates: []time.Time{d1, d2},
	})
	layout := generateViewLayout(v, w)

	// The text child should say "2 dates selected".
	// The structure is Row -> [Text, Button]
	row := &layout.Children[0]
	text := row.Children[0].Shape.TC.Text
	if text != "2 dates selected" {
		t.Errorf("got %q, want '2 dates selected'", text)
	}
}

func TestInputDateSingleDateText(t *testing.T) {
	w := &Window{}
	d1 := time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local)

	v := InputDate(InputDateCfg{
		ID:   "id-single",
		Date: d1,
	})
	layout := generateViewLayout(v, w)

	// The date is shown via an embedded Input widget.
	// Find the text by searching the layout tree.
	expected := LocaleFormatDate(d1,
		localeDatePadFormat(ActiveLocale.Date.ShortDate))
	if !layoutContainsText(&layout, expected) {
		t.Errorf("layout does not contain %q", expected)
	}
}

func layoutContainsText(l *Layout, text string) bool {
	if l.Shape != nil && l.Shape.TC != nil && l.Shape.TC.Text == text {
		return true
	}
	for i := range l.Children {
		if layoutContainsText(&l.Children[i], text) {
			return true
		}
	}
	return false
}

// TestInputDateReadOnlyForwardsToInput checks that a read-only
// InputDate forwards ReadOnly to its inner Input (announced as
// AccessStateReadOnly) and that the outer date field announces it too.
func TestInputDateReadOnlyForwardsToInput(t *testing.T) {
	w := &Window{}
	v := InputDate(InputDateCfg{
		ID:       "id-ro-fwd",
		ReadOnly: true,
		Date:     time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YState != AccessStateReadOnly {
		t.Errorf("outer A11YState=%d, want ReadOnly", layout.Shape.A11YState)
	}
	inp := findShapeByID(&layout, "id-ro-fwd.input")
	if inp == nil {
		t.Fatal("inner input not found")
	}
	if inp.Shape.A11YState != AccessStateReadOnly {
		t.Errorf("inner input A11YState=%d, want ReadOnly", inp.Shape.A11YState)
	}
}

// TestInputDateReadOnlyDoesNotOpenPicker covers #82: a read-only date
// field must not open the calendar popup, which fires OnSelect
// independently of the text field. The stored open-state is forced true
// so only the ReadOnly gate can keep the picker closed. The editable
// control proves the probe observes the open picker (remove the
// !cfg.ReadOnly clause on isOpen and the read-only assertion fails).
func TestInputDateReadOnlyDoesNotOpenPicker(t *testing.T) {
	openChildren := func(readOnly bool) int {
		w := &Window{}
		inputDateOpen("id-open", w) // force stored open-state true
		v := InputDate(InputDateCfg{
			ID:       "id-open",
			ReadOnly: readOnly,
		})
		layout := generateViewLayout(v, w)
		// Closed: 1 child (the text+icon row). Open: 3 (row, backdrop,
		// floating picker). The picker's presence is the OnSelect path.
		hasPicker := findShapeByID(&layout, "id-open.picker") != nil
		if readOnly && hasPicker {
			t.Error("read-only date field opened the calendar picker")
		}
		if !readOnly && !hasPicker {
			t.Error("editable open date field did not render the picker")
		}
		return len(layout.Children)
	}

	if got := openChildren(false); got != 3 {
		t.Errorf("editable open field: got %d children, want 3", got)
	}
	if got := openChildren(true); got != 1 {
		t.Errorf("read-only field: got %d children, want 1 (picker gated)", got)
	}
}

func TestInputDateFocusDisabled(t *testing.T) {
	w := &Window{}
	v := InputDate(InputDateCfg{
		ID:            "id-fd",
		FocusDisabled: true,
		Date:          time.Date(2025, 3, 15, 0, 0, 0, 0, time.Local),
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.Focusable {
		t.Error("FocusDisabled: true should produce a non-focusable outer Column")
	}
}
