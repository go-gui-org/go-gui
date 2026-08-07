package gui

import "testing"

// Q6 phase gate (developer-ergonomics spec §9.6).
//
// §4.3 proposes collapsing the two event classes into one rule. Nested
// scroll is the one case where that collapse changes behavior with no
// compile error: OnMouseScroll is notify-class today, and an inner
// scrollable that has reached its limit stops consuming so the event
// cascades to the enclosing scrollable. Under a consume-by-default rule
// the inner container would swallow it and the outer would freeze —
// a silent regression of exactly the class §7.2 describes.
//
// These tests pin the current contract so phase 4 has something to
// break. If a change to the propagation model reds them, that is the
// gate firing, not a flaky test: the behavior below is the decision,
// and changing it is a deliberate act that needs its own justification
// in the spec.
//
// Written against the phase-2 test API (TestScroll/TestScrollOffset),
// which is why the gate could not be discharged before now.

// newNestedScrollWindow builds an outer scrollable containing a short
// inner scrollable plus enough sibling filler that the outer can scroll
// too. Both are taller than their viewports, so both have somewhere to
// go and "who moved" is unambiguous.
func newNestedScrollWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(_ *Window) View {
		rows := func(n int) []View {
			out := make([]View, 0, n)
			for range n {
				out = append(out, Column(ContainerCfg{
					Sizing: FillFixed,
					Height: 20,
				}))
			}
			return out
		}
		return Column(ContainerCfg{
			ID:         "outer",
			Scrollable: true,
			Sizing:     FixedFixed,
			Width:      300,
			Height:     200,
			Content: []View{
				Column(ContainerCfg{
					ID:         "inner",
					Scrollable: true,
					Sizing:     FixedFixed,
					Width:      280,
					Height:     100,
					Content:    rows(20),
				}),
				// Filler below the inner box, so the outer has content
				// well past its own 200px viewport and room to receive
				// a cascaded scroll.
				Column(ContainerCfg{
					Sizing:  FillFit,
					Content: rows(40),
				}),
			},
		})
	})
	return w
}

// Baseline: a scroll over the inner container moves the inner one and
// leaves the outer alone. Without this, the cascade test below could
// pass for the wrong reason (inner never receiving anything).
func TestNestedScrollInnerConsumesWhileItCan(t *testing.T) {
	w := newNestedScrollWindow(t)
	if err := w.TestScroll("inner", 0, -5); err != nil {
		t.Fatalf("TestScroll(inner) = %v, want nil", err)
	}
	_, inner, err := w.TestScrollOffset("inner")
	if err != nil {
		t.Fatalf("TestScrollOffset(inner) = %v, want nil", err)
	}
	_, outer, err := w.TestScrollOffset("outer")
	if err != nil {
		t.Fatalf("TestScrollOffset(outer) = %v, want nil", err)
	}
	if inner >= 0 {
		t.Fatalf("inner offset = %v, want < 0", inner)
	}
	if outer != 0 {
		t.Fatalf("outer offset = %v, want 0 — the inner had room and "+
			"should have absorbed the scroll", outer)
	}
}

// The gate itself. The scroll that the inner container can no longer
// absorb must reach the outer container in the same gesture.
//
// Asserted at the transition rather than after it. Once the outer
// scrolls, the inner box (100px at the top of a 200px viewport) is
// carried out of view, its clip goes empty, and a follow-up TestScroll
// aimed at it has no coordinate to use. So the loop watches for the
// first gesture the inner declines and checks the outer across exactly
// that gesture.
func TestNestedScrollCascadesToParentAtLimit(t *testing.T) {
	w := newNestedScrollWindow(t)

	var innerEnd, outerBefore, outerAfter float32
	cascaded := false
	for range 100 {
		// Snapshot both offsets, then scroll. The gesture of interest
		// is the one after which the inner has not moved.
		_, innerBefore, err := w.TestScrollOffset("inner")
		if err != nil {
			t.Fatalf("TestScrollOffset(inner) = %v, want nil", err)
		}
		_, ob, err := w.TestScrollOffset("outer")
		if err != nil {
			t.Fatalf("TestScrollOffset(outer) = %v, want nil", err)
		}
		// Errors are not assertable here: once the inner declines, the
		// outer accepts, so the event is still handled and the return is
		// still nil. The offsets are the signal.
		if err = w.TestScroll("inner", 0, -20); err != nil {
			t.Fatalf("TestScroll(inner) = %v, want nil", err)
		}
		_, innerNow, err := w.TestScrollOffset("inner")
		if err != nil {
			t.Fatalf("TestScrollOffset(inner) = %v, want nil", err)
		}
		if innerNow != innerBefore {
			continue // inner still had room; keep going
		}
		_, oa, err := w.TestScrollOffset("outer")
		if err != nil {
			t.Fatalf("TestScrollOffset(outer) = %v, want nil", err)
		}
		innerEnd, outerBefore, outerAfter = innerNow, ob, oa
		cascaded = true
		break
	}
	if !cascaded {
		t.Fatal("inner container never reached its end")
	}
	if innerEnd >= 0 {
		t.Fatalf("inner offset = %v at its end, want < 0", innerEnd)
	}
	if outerAfter >= outerBefore {
		t.Fatalf("outer offset %v -> %v: the pinned inner container did "+
			"not cascade the scroll to its parent. If this is intentional, "+
			"Q6 in docs/specs/developer-ergonomics.md is the decision "+
			"being changed", outerBefore, outerAfter)
	}
}
