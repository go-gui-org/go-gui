package gui

import "testing"

// PumpFrame is the entry point backends use from a nested platform
// runloop (modal dialog, menu tracking, live resize), where the backend's
// own event loop is blocked and delivers no frames. These tests cover the
// three behaviours that matter there: it does a real frame, it declines
// rather than deadlocks when the window is already locked up-stack, and
// it refuses to re-enter itself.

func newPumpTestWindow() *Window {
	w := NewWindow(WindowCfg{
		State:  new(int),
		Width:  100,
		Height: 100,
	})
	w.viewGenerator = func(_ *Window) View {
		return Text(TextCfg{Text: "x"})
	}
	return w
}

func TestPumpFrameFlushesAndRebuilds(t *testing.T) {
	w := newPumpTestWindow()
	w.refreshLayout = false
	w.refreshRenderOnly = false

	// A command queued while the backend loop is blocked — e.g. a
	// terminal's debounced resize wake — must still be flushed.
	flushed := false
	w.QueueCommand(func(_ *Window) {
		flushed = true
		w.markLayoutRefresh()
	})

	if !w.PumpFrame() {
		t.Fatal("PumpFrame should report a rebuild after the queued refresh")
	}
	if !flushed {
		t.Error("queued command not flushed")
	}
	if w.refreshLayout {
		t.Error("refreshLayout should be cleared by the pumped frame")
	}
}

func TestPumpFrameSkipsWhenLocked(t *testing.T) {
	w := newPumpTestWindow()
	w.refreshLayout = true

	// Stand in for a caller further up the stack — an event handler
	// that opened the modal dialog from inside locked window code.
	// Blocking here would wedge the main thread.
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.PumpFrame() {
		t.Error("PumpFrame should decline the frame while the window is locked")
	}
	if !w.refreshLayout {
		t.Error("pending refresh must survive a declined pump")
	}
}

func TestPumpFrameRejectsReentry(t *testing.T) {
	w := newPumpTestWindow()
	w.refreshLayout = true

	// The timer firing again inside a command callback that itself
	// spins a runloop.
	var inner, innerRan bool
	w.QueueCommand(func(_ *Window) {
		innerRan = true
		inner = w.PumpFrame()
	})

	w.PumpFrame()
	if !innerRan {
		t.Fatal("queued command did not run")
	}
	if inner {
		t.Error("nested PumpFrame should be rejected")
	}
	if w.pumping {
		t.Error("pumping flag not cleared after the frame")
	}
}
