//go:build windows && !js

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Issue #587: WM_MOUSELEAVE reaches the window as EventMouseLeave, so
// nothing stays hovered after the pointer leaves the client area, and it
// consumes the one-shot TrackMouseEvent request so the next move re-arms.
func TestMouseLeaveEmitsEventAndDisarms(t *testing.T) {
	b := newCharTestBackend(t)
	var got []gui.EventType
	b.plat.w.OnEvent = func(e *gui.Event, _ *gui.Window) { got = append(got, e.Type) }
	b.plat.trackingLeave = true

	if _, handled := b.handleMessage(wmMouseLeave, 0, 0); !handled {
		t.Error("WM_MOUSELEAVE fell through to DefWindowProc")
	}
	if b.plat.trackingLeave {
		t.Error("trackingLeave still set after WM_MOUSELEAVE")
	}
	if len(got) != 1 || got[0] != gui.EventMouseLeave {
		t.Errorf("events = %v, want [EventMouseLeave]", got)
	}
}
