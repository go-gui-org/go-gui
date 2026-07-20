package gui

import (
	"testing"
	"time"
)

// TestScrollVerticalToSmoothLiveLoop drives a smooth scroll through
// the real animation goroutine while the test goroutine plays the
// main loop's role, flushing commands until the ease settles. This
// is the production topology (animation goroutine queueing, main
// goroutine flushing), so the race detector can observe queue/flush
// interactions — regression cover for the command scratch buffer
// being handed back to writers while the flush loop still read it.
func TestScrollVerticalToSmoothLiveLoop(t *testing.T) {
	guiTheme.ScrollMultiplier = 1

	// NewWindow, not a bare Window: the animation goroutine only
	// starts when the lifecycle channels exist.
	w := NewWindow(WindowCfg{Width: 100, Height: 100})
	t.Cleanup(func() {
		w.stopAnimationLoop()
	})
	child := Layout{
		Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 300},
	}
	w.layout = Layout{
		Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{{
			Shape: &Shape{
				shapeType:  shapeRectangle,
				Scrollable: true,
				ID:         "live",
				Width:      100,
				Height:     100,
				Axis:       AxisTopToBottom,
			},
			Children: []Layout{child},
		}},
	}

	if !scrollSmoothTo(w, &w.layout.Children[0], scrollAxisY, -50) {
		t.Fatal("expected ease to arm")
	}

	// Flush slower than the 16ms animation tick so ticks land while
	// the flusher is between flushes — the timing where a reclaimed
	// scratch buffer is written while the previous flush's reads are
	// still the latest unsynchronized access.
	deadline := time.Now().Add(10 * time.Second)
	for {
		time.Sleep(25 * time.Millisecond)
		w.flushCommands()
		if v, _ := w.scrollY().Get("live"); v == -50 {
			return
		}
		if time.Now().After(deadline) {
			v, _ := w.scrollY().Get("live")
			t.Fatalf("ease did not settle; offset %v", v)
		}
	}
}
