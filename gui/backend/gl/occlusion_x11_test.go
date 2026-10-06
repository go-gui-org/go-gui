//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// newOcclusionTestBackend returns an X11 backend whose window has run one
// frame, so a later FrameFn renders only if something marked a refresh.
func newOcclusionTestBackend(t *testing.T, win xproto.Window) *Backend {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 100, Height: 100})
	w.SetView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{})
	})
	w.FrameFn()
	b := &Backend{}
	b.plat.w = w
	b.plat.window = win
	return b
}

// Issue #954: ICCCM iconify unmaps the top-level window. UnmapNotify marks
// it occluded, so FrameFn draws nothing; MapNotify on restore shows it
// again, waking the main loop once and asking for one full frame.
func TestUnmapMapTogglesOcclusion(t *testing.T) {
	const win xproto.Window = 7
	b := newOcclusionTestBackend(t, win)
	w := b.plat.w
	// A pending refresh, so FrameFn would render were the window shown.
	// Before the wake counter: InvalidateLayout wakes the main loop too.
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	b.handleXEvent(xproto.UnmapNotifyEvent{Event: win, Window: win})
	if w.FrameFn() {
		t.Error("FrameFn rendered while unmapped")
	}
	if wakes != 0 {
		t.Errorf("unmap woke main %d times, want 0", wakes)
	}

	b.handleXEvent(xproto.MapNotifyEvent{Event: win, Window: win})
	if wakes != 1 {
		t.Errorf("map after unmap woke main %d times, want 1", wakes)
	}
	if !w.FrameFn() {
		t.Error("FrameFn drew nothing after restore")
	}
}

// The first MapNotify, when the window appears at startup, finds it
// already shown: no wake, no extra frame.
func TestInitialMapIsNoOp(t *testing.T) {
	const win xproto.Window = 7
	b := newOcclusionTestBackend(t, win)
	w := b.plat.w
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	b.handleXEvent(xproto.MapNotifyEvent{Event: win, Window: win})
	if wakes != 0 {
		t.Errorf("initial map woke main %d times, want 0", wakes)
	}
}

// Only the backend's own top-level window counts. A map or unmap reported
// for any other window (a child, should one ever select
// SubstructureNotify) must not hide or show this one.
func TestUnmapOtherWindowIgnored(t *testing.T) {
	const win, other xproto.Window = 7, 9
	b := newOcclusionTestBackend(t, win)
	w := b.plat.w
	w.InvalidateLayout()

	b.handleXEvent(xproto.UnmapNotifyEvent{Event: win, Window: other})
	if !w.FrameFn() {
		t.Error("unmap of another window hid this one")
	}
}
