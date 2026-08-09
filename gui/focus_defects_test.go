package gui

import (
	"slices"
	"strings"
	"testing"
)

// Regression tests for the first-party focus defects found by
// ergoaudit (developer-ergonomics §4.2). Each of these widgets shipped
// a button carrying an OnClick that no keyboard user could reach,
// because focus traversal is keyed by ID and the button had none.
//
// The assertions run through the gui.Debug gate rather than poking at
// shapes directly: the gate is the thing that decides what counts as a
// focus defect, so a test that agreed with it by reimplementing it
// would not notice the two drifting apart.

// assertNoFocusDefects renders v and fails if the debug gate reports
// anything. captureDebug (debug_test.go) turns the gate on and
// redirects its output for the duration of the test.
//
// Uses generateViewLayout, not v.GenerateLayout: the latter builds one
// node, and every button under test is a descendant reached through
// Content().
func assertNoFocusDefects(t *testing.T, w *Window, v View) Layout {
	t.Helper()
	buf := captureDebug(t)
	layout := generateViewLayout(v, w)
	w.debugAudit(&layout)
	if got := buf.String(); got != "" {
		t.Errorf("debug gate reported defects:\n%s", got)
	}
	return layout
}

// focusableIDs collects the ID of every shape that joins the tab
// order, in tree order.
func focusableIDs(layout *Layout) []string {
	var out []string
	var walk func(*Layout)
	walk = func(l *Layout) {
		if s := l.Shape; s != nil && s.Focusable && !s.FocusSkip && !s.Disabled {
			out = append(out, s.ID)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(layout)
	return out
}

// Two toasts on screen at once is the case a fixed ID would break:
// both dismiss buttons would claim one identity, collapsing them to a
// single tab stop sharing one state slot.
func TestToastButtonsGetPerToastIDs(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 1, animFrac: 1, cfg: ToastCfg{
			Title: "A", actionLabel: "Undo", OnAction: func(ctx EventCtx) {},
		}},
		{id: 2, animFrac: 1, cfg: ToastCfg{
			Title: "B", actionLabel: "Undo", OnAction: func(ctx EventCtx) {},
		}},
	}

	layout := assertNoFocusDefects(t, w, toastContainerView(w))

	ids := focusableIDs(&layout)
	want := []string{
		"toast:1:action", "toast:1:dismiss",
		"toast:2:action", "toast:2:dismiss",
	}
	for _, id := range want {
		if !slices.Contains(ids, id) {
			t.Errorf("missing focusable %q; got %v", id, ids)
		}
	}
	assertUnique(t, ids)
}

// A toast with no action label ships only a dismiss button, and it
// must still be reachable.
func TestToastDismissOnlyIsReachable(t *testing.T) {
	w := &Window{}
	w.toasts = []toastNotification{
		{id: 7, animFrac: 1, cfg: ToastCfg{Title: "Saved"}},
	}

	layout := assertNoFocusDefects(t, w, toastContainerView(w))

	if ids := focusableIDs(&layout); !slices.Contains(ids, "toast:7:dismiss") {
		t.Errorf("dismiss button not keyboard-reachable; got %v", ids)
	}
}

func TestToastBtnIDIsPerToast(t *testing.T) {
	t.Parallel()
	if a, b := toastBtnID(1, "dismiss"), toastBtnID(2, "dismiss"); a == b {
		t.Fatalf("two toasts share the button ID %q", a)
	}
	if got := toastBtnID(3, "action"); !strings.Contains(got, "3") {
		t.Errorf("toastBtnID lost the toast number: %q", got)
	}
}

// The overflow panel's trigger is namespaced by the panel's own ID, so
// a toolbar holding several panels keeps them distinct.
func TestOverflowPanelTriggerIsReachable(t *testing.T) {
	w := &Window{}
	v := OverflowPanel(w, OverflowPanelCfg{
		ID: "toolbar",
		Items: []OverflowItem{
			{View: Text(TextCfg{Text: "one"})},
		},
	})

	layout := assertNoFocusDefects(t, w, v)

	if ids := focusableIDs(&layout); !slices.Contains(ids, "toolbar:trigger") {
		t.Errorf("overflow trigger not keyboard-reachable; got %v", ids)
	}
}

// The out-of-month calendar cell is blank, disabled, and has no
// handler. It opts out of focus rather than inventing an ID for empty
// space — the decorative case FocusDisabled exists for.
func TestDatePickerBlankCellOptsOutOfFocus(t *testing.T) {
	t.Parallel()
	v := Button(ButtonCfg{
		FocusDisabled: true,
		Disabled:      true,
		Content:       []View{Text(TextCfg{Text: " "})},
	})
	w := &Window{}
	layout := v.GenerateLayout(w)
	if layout.Shape.Focusable {
		t.Error("FocusDisabled must clear Focusable on the shape")
	}
}

// assertUnique fails on a repeated ID. Duplicates are the failure mode
// a per-instance namespace exists to prevent.
func assertUnique(t *testing.T, ids []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate focusable ID %q in %v", id, ids)
		}
		seen[id] = true
	}
}
