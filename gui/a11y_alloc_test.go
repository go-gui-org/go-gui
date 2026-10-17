package gui

import "testing"

// A container with an accessible name is built each frame. Its
// accessInfo comes from the view-phase arena, like the shape that points
// at it, so a labelled container costs no heap allocation per frame.
// Before the arena held accessInfo, every labelled row (tree, listbox,
// menu, table) allocated one per frame.
func TestLabelledContainerA11YInfoDoesNotAllocate(t *testing.T) {
	w := &Window{scratch: newScratchPools()}
	v := Row(ContainerCfg{
		A11YCfg: A11YCfg{A11YLabel: "row", A11YDescription: "desc"},
	})
	// Warm the arena so its backing store exists before measuring.
	w.scratch.resetViewPools()
	_ = generateViewLayout(v, w)

	var layout Layout
	allocs := testing.AllocsPerRun(100, func() {
		w.scratch.resetViewPools()
		layout = generateViewLayout(v, w)
	})
	if allocs != 0 {
		t.Errorf("labelled container: %v allocs per frame, want 0", allocs)
	}
	info := layout.Shape.a11Y
	if info == nil || info.Label != "row" || info.Description != "desc" {
		t.Fatalf("a11Y = %+v, want label row, desc desc", info)
	}
}

// The A11YValue path fills a value into the info. It must also come from
// the arena, and must still carry the value.
func TestValuedContainerA11YInfoDoesNotAllocate(t *testing.T) {
	w := &Window{scratch: newScratchPools()}
	v := Row(ContainerCfg{
		A11YValue: AccessValue{Now: 3, Min: 0, Max: 10},
	})
	w.scratch.resetViewPools()
	_ = generateViewLayout(v, w)

	var layout Layout
	allocs := testing.AllocsPerRun(100, func() {
		w.scratch.resetViewPools()
		layout = generateViewLayout(v, w)
	})
	if allocs != 0 {
		t.Errorf("valued container: %v allocs per frame, want 0", allocs)
	}
	info := layout.Shape.a11Y
	if info == nil || info.ValueNum != 3 || info.ValueMax != 10 {
		t.Fatalf("a11Y = %+v, want value 3 of 0..10", info)
	}
}
