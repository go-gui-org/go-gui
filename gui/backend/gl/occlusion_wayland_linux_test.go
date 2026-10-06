//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/decor"
	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// newWlOcclusionTestWindow returns a Wayland window with no surface whose
// gui window has run one frame, so a later FrameFn renders only if
// something marked a refresh.
func newWlOcclusionTestWindow(t *testing.T) *wlWindow {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 100, Height: 100})
	w.SetView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{})
	})
	w.FrameFn()
	b := &Backend{}
	b.plat.w = w
	return &wlWindow{d: &wlDisplay{}, b: b}
}

// wlStates packs xdg_toplevel states the way the wire carries them: a
// list of native-endian uint32 values.
func wlStates(states ...uint32) []byte {
	out := make([]byte, 0, 4*len(states))
	for _, s := range states {
		out = binary.LittleEndian.AppendUint32(out, s)
	}
	return out
}

// Issue #953: xdg_toplevel v6 reports a hidden window (minimized, on
// another workspace, screen locked) as suspended. The state takes effect
// with the configure sequence: FrameFn then draws nothing, and the
// configure that drops suspended wakes the loop once for a full frame.
func TestWlSuspendedTogglesOcclusion(t *testing.T) {
	ww := newWlOcclusionTestWindow(t)
	w := ww.b.plat.w
	// A pending refresh, so FrameFn would render were the window shown.
	// Before the wake counter: InvalidateLayout wakes the main loop too.
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	ww.toplevelConfigure(0, 0, wlStates(wl.XdgToplevelStateSuspended))
	ww.applyPending(1, 1)
	if w.FrameFn() {
		t.Error("FrameFn rendered while suspended")
	}
	if wakes != 0 {
		t.Errorf("suspend woke main %d times, want 0", wakes)
	}

	ww.toplevelConfigure(0, 0, wlStates(wl.XdgToplevelStateActivated))
	ww.applyPending(1, 1)
	if wakes != 1 {
		t.Errorf("show after suspend woke main %d times, want 1", wakes)
	}
	if !w.FrameFn() {
		t.Error("FrameFn drew nothing after the window came back")
	}
}

// A compositor before xdg-shell v6 never sends suspended. Its configures
// leave the window shown: no wake, and a pending refresh still draws.
func TestWlNoSuspendedStaysVisible(t *testing.T) {
	ww := newWlOcclusionTestWindow(t)
	w := ww.b.plat.w
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	ww.toplevelConfigure(0, 0, wlStates(wl.XdgToplevelStateActivated,
		wl.XdgToplevelStateMaximized))
	ww.applyPending(1, 1)
	if wakes != 0 {
		t.Errorf("configure without suspended woke main %d times, want 0", wakes)
	}
	if !w.FrameFn() {
		t.Error("configure without suspended hid the window")
	}
}

// The toplevel configure only records the state: it applies with the
// xdg_surface.configure that closes the sequence, like size and focus.
func TestWlSuspendedPendsUntilApplied(t *testing.T) {
	ww := newWlOcclusionTestWindow(t)
	w := ww.b.plat.w
	w.InvalidateLayout()

	ww.toplevelConfigure(0, 0, wlStates(wl.XdgToplevelStateSuspended))
	if !w.FrameFn() {
		t.Error("suspended applied before the configure sequence closed")
	}
}

// The libdecor path: decorStates maps libdecor's state bits to the same
// pending flags, and applyPending, shared with the xdg path, applies them.
// The bit values are spelled from libdecor.h (enum libdecor_window_state),
// not from the decor constants, so a wrong constant fails here.
func TestWlDecorSuspendedTogglesOcclusion(t *testing.T) {
	const (
		libdecorActive    = 1 << 0
		libdecorSuspended = 1 << 7
	)
	ww := newWlOcclusionTestWindow(t)
	w := ww.b.plat.w
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	ww.decorStates(libdecorActive | libdecorSuspended)
	if !ww.pendActive {
		t.Error("active bit not read")
	}
	ww.applyPending(1, 1)
	if w.FrameFn() {
		t.Error("FrameFn rendered while libdecor reports suspended")
	}

	ww.decorStates(libdecorActive)
	ww.applyPending(1, 1)
	if wakes != 1 {
		t.Errorf("show after suspend woke main %d times, want 1", wakes)
	}
	if !w.FrameFn() {
		t.Error("FrameFn drew nothing after the window came back")
	}
	if decor.StateSuspended != libdecorSuspended {
		t.Errorf("decor.StateSuspended = %#x, libdecor.h has %#x",
			decor.StateSuspended, libdecorSuspended)
	}
}

// Issue #959: a stalled frame callback and suspended are two reasons for
// one occluded flag. Clearing either alone leaves the window hidden; only
// when both are gone does it show, waking the loop once.
func TestWlStallCombinesWithSuspended(t *testing.T) {
	ww := newWlOcclusionTestWindow(t)
	w := ww.b.plat.w
	w.InvalidateLayout()
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	ww.setStalled(true)
	if w.FrameFn() {
		t.Error("FrameFn rendered while the frame callback is stalled")
	}
	ww.toplevelConfigure(0, 0, wlStates(wl.XdgToplevelStateSuspended))
	ww.applyPending(1, 1)

	ww.setStalled(false) // still suspended
	if wakes != 0 || w.FrameFn() {
		t.Errorf("stall cleared while suspended showed the window (wakes %d)", wakes)
	}
	ww.setStalled(true)
	ww.toplevelConfigure(0, 0, nil)
	ww.applyPending(1, 1) // still stalled
	if wakes != 0 || w.FrameFn() {
		t.Errorf("suspended cleared while stalled showed the window (wakes %d)", wakes)
	}

	ww.setStalled(false)
	if wakes != 1 {
		t.Errorf("show woke main %d times, want 1", wakes)
	}
	if !w.FrameFn() {
		t.Error("FrameFn drew nothing after the window came back")
	}
}

// Issue #959, on a live compositor: a frame callback outstanding past
// wlFrameTimeout marks the window occluded, as a covered window's does
// when the compositor stops painting it. Its done shows the window again.
// The callback is asked for without a commit, so nothing answers it
// until renderFrame commits: that stands in for the cover and uncover.
func TestWaylandFrameStallOccludes(t *testing.T) {
	b := newWaylandTestBackend(t, gui.WindowCfg{})
	t.Cleanup(b.Destroy)
	settleWindow(t, b)
	ww, w := b.plat.wl, b.plat.w
	waitFor(t, ww.d, "settle frame callback", func() bool { return !ww.frameCb.Valid() })
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	now := time.Now()
	ww.requestFrame(now)
	ww.checkStall(now)
	if ww.stalled {
		t.Fatal("stalled before the timeout")
	}
	ww.checkStall(now.Add(wlFrameTimeout))
	if !ww.stalled {
		t.Fatal("not stalled at the timeout")
	}
	w.InvalidateLayout()
	wakes = 0
	if w.FrameFn() {
		t.Error("FrameFn rendered while stalled")
	}

	b.renderFrame(w) // commits: the compositor now answers the callback
	waitFor(t, ww.d, "frame callback", func() bool { return !ww.frameCb.Valid() })
	if ww.stalled {
		t.Error("done left the window stalled")
	}
	if wakes != 1 {
		t.Errorf("done woke main %d times, want 1", wakes)
	}
	if !w.FrameFn() {
		t.Error("FrameFn drew nothing after done")
	}
}

// With no outstanding callback (a VSyncOff window never asks for one)
// nothing can stall, however long the window goes without a frame.
func TestWlNoCallbackNeverStalls(t *testing.T) {
	ww := newWlOcclusionTestWindow(t)
	ww.checkStall(time.Now().Add(time.Hour))
	if ww.stalled {
		t.Error("stalled with no frame callback outstanding")
	}
}
