package gui

import "testing"

// Issue #1273: Close wakes an idle event loop. Without the wake the
// backend sees the request only at the next OS message, and a hidden
// window gets none.
func TestCloseWakesTheEventLoop(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	w.Close()

	if !w.CloseRequested() {
		t.Error("CloseRequested = false after Close")
	}
	if wakes != 1 {
		t.Errorf("Close woke the loop %d times, want 1", wakes)
	}
}
