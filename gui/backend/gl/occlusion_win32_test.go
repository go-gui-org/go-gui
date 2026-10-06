//go:build windows && !js

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Issue #943: WM_SIZE with SIZE_MINIMIZED marks the window occluded, so
// running animations stop asking for frames. The flag is unexported, so
// the test reads it through the show side: un-occluding a window that
// the minimize did occlude wakes the main loop once.
func TestMinimizeMarksWindowOccluded(t *testing.T) {
	b := newCharTestBackend(t)
	w := b.plat.w
	// newCharTestBackend already ran FrameFn, so without a pending
	// refresh FrameFn returns false occluded or not. Before the wake
	// counter: InvalidateLayout wakes the main loop too.
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	if _, handled := b.handleMessage(wmSize, sizeMinimized, 0); !handled {
		t.Fatal("WM_SIZE/SIZE_MINIMIZED fell through to DefWindowProc")
	}
	if w.FrameFn() {
		t.Error("FrameFn rendered while minimized")
	}
	gui.DispatchWindowOccluded(w, false)
	if wakes != 1 {
		t.Errorf("show after minimize woke main %d times, want 1", wakes)
	}
}
