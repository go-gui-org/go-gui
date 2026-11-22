// Package autoscroll scrolls a scrollable by a fixed step on every frame,
// for the scroll workloads of the batching investigation (#1303). The
// examples take it through an -autoscroll flag. A run is then the same
// on every machine, with no hand on the wheel.
package autoscroll

import "github.com/go-gui-org/go-gui/gui"

// Step queues one scroll of px down the scrollable with effective ID
// id, and asks for the next frame. At the end it jumps back to the top,
// so the run never stalls on the last page. Call it from the view: a
// view holds the frame lock, so the scroll goes through QueueCommand.
// px <= 0 does nothing.
func Step(w *gui.Window, id string, px float32) {
	if px <= 0 {
		return
	}
	w.QueueCommand(func(w *gui.Window) {
		before := w.ScrollVerticalOffset(id)
		w.ScrollVerticalBy(id, -px)
		if w.ScrollVerticalOffset(id) == before {
			w.ScrollVerticalTo(id, 0) // at the end, or not yet laid out
		}
		w.InvalidateLayout()
	})
}
