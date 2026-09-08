package gui

import (
	"testing"
)

// Regression tests for the take-focus predicate (Shape.canTakeFocus)
// and the per-frame focus fixup (fixupFocusLocked). Every widget maps
// an invisible Cfg to the disabled invisibleContainerView singleton,
// so invisibility never reaches a shape — these tests pin that
// invariant — and a widget disabled or removed while focused loses
// focus to the first tab stop instead of parking it in the void.

// focusFixupWindow builds a two-input window whose second input the
// test can disable by flipping *disabled and re-running Update.
func focusFixupWindow(disabled *bool) *Window {
	w := newTestWindow()
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Content: []View{
				Input(InputCfg{ID: "fix-a", Sizing: FillFit}),
				Input(InputCfg{
					ID:       "fix-b",
					Sizing:   FillFit,
					Disabled: *disabled,
				}),
			},
		})
	}
	return w
}

func TestCanTakeFocusPredicate(t *testing.T) {
	cases := []struct {
		name  string
		shape Shape
		want  bool
	}{
		{"focusable with ID", Shape{Focusable: true, ID: "a"},
			true},
		{"no ID", Shape{Focusable: true}, false},
		{"not focusable", Shape{ID: "a"}, false},
		{"disabled", Shape{Focusable: true, ID: "a", Disabled: true},
			false},
		// FocusSkip is tab-order-only: the widget still takes
		// click-focus, so the predicate stays true.
		{"focus skip", Shape{Focusable: true, ID: "a", FocusSkip: true},
			true},
	}
	for _, tc := range cases {
		if got := tc.shape.canTakeFocus(); got != tc.want {
			t.Errorf("%s: canTakeFocus() = %v, want %v",
				tc.name, got, tc.want)
		}
	}
}

func TestValidFocusSurvivesFrame(t *testing.T) {
	disabled := false
	w := focusFixupWindow(&disabled)
	w.SetFocus("fix-b")
	w.Update()
	if got := w.FocusID(); got != "fix-b" {
		t.Fatalf("FocusID() = %q, want %q", got, "fix-b")
	}
}

func TestDisabledFocusMovesToFirstTabStop(t *testing.T) {
	disabled := false
	w := focusFixupWindow(&disabled)
	w.SetFocus("fix-b")
	w.Update()
	if got := w.FocusID(); got != "fix-b" {
		t.Fatalf("setup: FocusID() = %q, want %q", got, "fix-b")
	}
	disabled = true
	w.refreshLayout = true
	w.Update()
	if got := w.FocusID(); got != "fix-a" {
		t.Fatalf("FocusID() = %q, want %q", got, "fix-a")
	}
}

func TestFocusClearedWhenNothingFocusable(t *testing.T) {
	disabled := false
	w := focusFixupWindow(&disabled)
	w.SetFocus("fix-a")
	w.Update()
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Content: []View{
				Input(InputCfg{
					ID:       "fix-a",
					Sizing:   FillFit,
					Disabled: true,
				}),
				Input(InputCfg{
					ID:       "fix-b",
					Sizing:   FillFit,
					Disabled: true,
				}),
			},
		})
	}
	w.refreshLayout = true
	w.Update()
	if got := w.FocusID(); got != "" {
		t.Fatalf("FocusID() = %q, want empty", got)
	}
}

func TestInvisibleInputTakesNoFocus(t *testing.T) {
	w := newTestWindow()
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Content: []View{
				Input(InputCfg{ID: "vis", Sizing: FillFit}),
				Input(InputCfg{ID: "hid", Sizing: FillFit, Invisible: true}),
			},
		})
	}
	w.Update()
	// The invisible input collapses to the disabled singleton: it
	// joins no tab stop.
	var candidates []focusCandidate
	seen := make(map[string]struct{})
	collectFocusCandidates(&w.layout, &candidates, seen)
	if len(candidates) != 1 || candidates[0].id != "vis" {
		t.Fatalf("tab stops = %v, want [vis]", candidates)
	}
	// Parking focus on the hidden leaf does not stick: the fixup
	// moves it to the visible field on the next frame.
	w.SetFocus("hid")
	w.Update()
	if got := w.FocusID(); got != "vis" {
		t.Fatalf("FocusID() = %q, want %q", got, "vis")
	}
}

func TestRemovedFocusMovesToFirstTabStop(t *testing.T) {
	disabled := false
	w := focusFixupWindow(&disabled)
	w.SetFocus("fix-b")
	w.Update()
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Content: []View{
				Input(InputCfg{ID: "fix-a", Sizing: FillFit}),
			},
		})
	}
	w.refreshLayout = true
	w.Update()
	if got := w.FocusID(); got != "fix-a" {
		t.Fatalf("FocusID() = %q, want %q", got, "fix-a")
	}
}
