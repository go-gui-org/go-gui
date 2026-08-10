//go:build windows && !js

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Issue #110: Win32 can revoke mouse capture without ever sending the
// matching button-up. Unhandled, that left a MouseLock drag locked for
// good — every later move kept driving it with no button held.
//
// The window is driven through gui.Window's exported surface, so these
// assert what a widget actually observes: whether moves keep arriving.

// dragCounts records what an in-flight MouseLock saw.
type dragCounts struct{ moves, ups, cancels int }

// newCaptureTestBackend returns a Backend over a headless window with a
// MouseLock in flight, plus the counters that lock feeds.
func newCaptureTestBackend(t *testing.T) (*Backend, *dragCounts) {
	t.Helper()
	b := newCharTestBackend(t)
	c := &dragCounts{}
	b.plat.w.MouseLock(gui.MouseLockCfg{
		MouseMove: func(gui.EventCtx) { c.moves++ },
		MouseUp:   func(ctx gui.EventCtx) { c.ups++; ctx.Window.MouseUnlock() },
		Cancel:    func(*gui.Window) { c.cancels++ },
	})
	return b, c
}

// moveForTest feeds a mouse move the way the message pump would.
func moveForTest(b *Backend) {
	b.emit(gui.Event{Type: gui.EventMouseMove, MouseX: 10, MouseY: 10})
}

func TestCaptureChangedCancelsInFlightDrag(t *testing.T) {
	b, c := newCaptureTestBackend(t)
	b.plat.capturing = true

	moveForTest(b)
	if c.moves != 1 {
		t.Fatalf("locked move not delivered: moves = %d", c.moves)
	}

	if _, handled := b.handleMessage(wmCaptureChanged, 0, 0); !handled {
		t.Error("WM_CAPTURECHANGED fell through to DefWindowProc")
	}
	if c.cancels != 1 {
		t.Errorf("Cancel ran %d times, want 1", c.cancels)
	}
	if c.ups != 0 {
		t.Error("cancel synthesised a MouseUp; that would commit the drag")
	}
	if b.plat.capturing {
		t.Error("capturing still set after capture was revoked")
	}

	// The point of the fix: the drag is over, so moves stop.
	moveForTest(b)
	if c.moves != 1 {
		t.Errorf("drag still tracking after cancel: moves = %d", c.moves)
	}
}

// Our own ReleaseCapture on button-up also raises WM_CAPTURECHANGED.
// That one must not cancel — MouseUp already ended the drag, and a
// spurious cancel would unwind a dock drop or reorder after the fact.
func TestCaptureChangedAfterOwnReleaseDoesNotCancel(t *testing.T) {
	b, c := newCaptureTestBackend(t)
	b.plat.capturing = false // cleared by mouseButton before ReleaseCapture

	b.handleMessage(wmCaptureChanged, 0, 0)
	if c.cancels != 0 {
		t.Errorf("deliberate release cancelled the lock: cancels = %d", c.cancels)
	}
	moveForTest(b)
	if c.moves != 1 {
		t.Errorf("lock did not survive a deliberate release: moves = %d", c.moves)
	}
}

// mouseButton owns the capturing flag, and must clear it *before*
// ReleaseCapture — that call reenters this wndproc synchronously, so a
// flag cleared afterwards would read as an involuntary loss.
func TestMouseButtonTracksCaptureOwnership(t *testing.T) {
	b, c := newCaptureTestBackend(t)

	b.mouseButton(gui.EventMouseDown, gui.MouseLeft, 0, true)
	if !b.plat.capturing {
		t.Error("button down did not take capture")
	}

	// Button up releases capture, then Windows delivers the message.
	b.mouseButton(gui.EventMouseUp, gui.MouseLeft, 0, false)
	if b.plat.capturing {
		t.Error("button up did not give up capture")
	}
	b.handleMessage(wmCaptureChanged, 0, 0)
	if c.ups != 1 {
		t.Errorf("MouseUp ran %d times, want 1", c.ups)
	}
	if c.cancels != 0 {
		t.Errorf("release-driven WM_CAPTURECHANGED cancelled: cancels = %d",
			c.cancels)
	}
}
