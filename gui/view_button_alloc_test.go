package gui

import "testing"

// A button's shapeButtonColors comes from the frame pool
// (scratch.buttonColors), so generating a built button allocates
// nothing once the pools are warm. Before the fix, the nil-window
// fallback took &bc, which moved bc to the heap on every call, and the
// pooled path paid one allocation per button per frame as well.
//
// The view is built outside the measured func: this pins generation
// only, not the factory.
func TestButtonGenerateLayoutAllocsNothing(t *testing.T) {
	w := &Window{scratch: newScratchPools()}
	view := Button(ButtonCfg{
		ID:      "ok",
		OnClick: func(EventCtx) {},
	})

	// The pooled path must be the one measured: a button shape has an
	// events record (OpticalCenterText guarantees one), so bc is set.
	w.scratch.resetViewPools()
	if got := generateViewLayout(view, w); got.Shape.bc == nil {
		t.Fatal("button shape has no bc; the pooled path did not run")
	}

	if got := testing.AllocsPerRun(100, func() {
		w.scratch.resetViewPools()
		_ = generateViewLayout(view, w)
	}); got != 0 {
		t.Fatalf("button GenerateLayout allocs = %v, want 0", got)
	}
}
