package gui

import "testing"

// amendAll composes the one AmendLayout slot a shape has. The
// composition contract: nil hooks are dropped, nothing live means no
// hook at all, and a single live hook runs without a wrapper.
func TestAmendAllComposition(t *testing.T) {
	var order []string
	a := func(EventCtx) { order = append(order, "a") }
	b := func(EventCtx) { order = append(order, "b") }

	if got := amendAll(); got != nil {
		t.Error("empty call: want nil hook")
	}
	if got := amendAll(nil, nil); got != nil {
		t.Error("all-nil: want nil hook")
	}

	if got := amendAll(nil, a); got == nil {
		t.Fatal("one live hook: want non-nil")
	} else {
		got(EventCtx{})
	}
	if len(order) != 1 || order[0] != "a" {
		t.Errorf("single hook ran %v, want [a]", order)
	}

	order = nil
	h := amendAll(a, nil, b)
	if h == nil {
		t.Fatal("mixed hooks: want non-nil")
	}
	h(EventCtx{})
	if len(order) != 2 || order[0] != "a" || order[1] != "b" {
		t.Errorf("mixed hooks ran %v, want [a b] in call order", order)
	}
}

// focusRingAmend with both colors unset must produce no hook, so a
// widget that never fills either keeps its plain AmendLayout slot.
func TestFocusRingAmendUnsetIsNil(t *testing.T) {
	if got := focusRingAmend(Color{}, Color{}); got != nil {
		t.Error("both colors unset: want nil hook")
	}
	if got := focusRingAmend(RGB(1, 2, 3), Color{}); got == nil {
		t.Error("fill set: want non-nil hook")
	}
	if got := focusRingAmend(Color{}, RGB(1, 2, 3)); got == nil {
		t.Error("border set: want non-nil hook")
	}
}
