package autoscroll

import (
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// tallView is a 100 px high scrollable over 300 px of content, and
// scrolls it by step on every frame.
func tallView(step float32) func(*gui.Window) gui.View {
	return func(w *gui.Window) gui.View {
		Step(w, "s", step)
		return gui.Column(gui.ContainerCfg{
			ID:         "s",
			Sizing:     gui.FillFixed,
			Height:     100,
			Scrollable: true,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{Sizing: gui.FillFixed, Height: 300}),
			},
		})
	}
}

// TestStepScrollsAndWraps: each frame moves the view down by the step,
// and the step after the last page jumps back to the top.
func TestStepScrollsAndWraps(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{Width: 200, Height: 200})
	w.TestRender(tallView(150))
	var ys []float32
	for range 4 {
		w.TestStep(16 * time.Millisecond)
		_, y, err := w.TestScrollOffset("s")
		if err != nil {
			t.Fatal(err)
		}
		ys = append(ys, y)
	}
	// From 0: -150, then -200 (clamped at the end), then the end again,
	// which wraps to 0, then -150.
	if ys[0] >= 0 || ys[1] > ys[0] {
		t.Fatalf("offsets %v: want the view to move down", ys)
	}
	wrapped := false
	for _, y := range ys[1:] {
		if y == 0 {
			wrapped = true
		}
	}
	if !wrapped {
		t.Errorf("offsets %v: want a jump back to 0 at the end", ys)
	}
}

// TestStepOffQueuesNothing: a zero step leaves the view where it is.
func TestStepOffQueuesNothing(t *testing.T) {
	w := gui.NewWindow(gui.WindowCfg{Width: 200, Height: 200})
	w.TestRender(tallView(0))
	w.TestStep(16 * time.Millisecond)
	if _, y, err := w.TestScrollOffset("s"); err != nil || y != 0 {
		t.Fatalf("offset %v, err %v; want 0, nil", y, err)
	}
}
