//go:build linux && !js && !android

package gl

import (
	"errors"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// stubEGLTeardown replaces the EGL calls platformState.destroy makes with
// counters, so destroy runs with no libEGL loaded. It returns
// the count of eglTerminate calls per display.
func stubEGLTeardown(t *testing.T) map[uintptr]int {
	t.Helper()
	terminated := map[uintptr]int{}
	oldMake, oldTerm := eglMakeCurrent, eglTerminate
	eglMakeCurrent = func(uintptr, uintptr, uintptr, uintptr) uint32 { return 1 }
	eglTerminate = func(dpy uintptr) uint32 {
		terminated[dpy]++
		return 1
	}
	t.Cleanup(func() { eglMakeCurrent, eglTerminate = oldMake, oldTerm })
	return terminated
}

// TestEGLDisplayRefCount pins the counting rule: only the release that
// drops the last hold reports true.
func TestEGLDisplayRefCount(t *testing.T) {
	const dpy = uintptr(0x1284)
	eglDisplayAcquire(dpy)
	eglDisplayAcquire(dpy)
	if eglDisplayRelease(dpy) {
		t.Fatal("first release of two holds reported last")
	}
	if !eglDisplayRelease(dpy) {
		t.Fatal("second release of two holds did not report last")
	}
	// A release with no hold (a display acquired by nobody) reports last,
	// so a window that skipped acquire still terminates what it opened.
	if !eglDisplayRelease(dpy) {
		t.Fatal("release with no hold did not report last")
	}
}

// TestX11DestroySharedEGLDisplay is #1284: EGL hands every X11 window the
// same EGLDisplay, so closing one window must not terminate it under the
// others. Before the fix the first destroy terminated it, and every swap of
// the surviving window failed with EGL_NOT_INITIALIZED.
func TestX11DestroySharedEGLDisplay(t *testing.T) {
	terminated := stubEGLTeardown(t)
	const dpy = uintptr(0x1284)
	main, child := &platformState{eglDpy: dpy}, &platformState{eglDpy: dpy}
	eglDisplayAcquire(dpy)
	eglDisplayAcquire(dpy)

	child.destroy()
	if n := terminated[dpy]; n != 0 {
		t.Fatalf("closing one of two windows terminated the shared display %d times", n)
	}
	main.destroy()
	if n := terminated[dpy]; n != 1 {
		t.Fatalf("closing the last window terminated the display %d times, want 1", n)
	}
}

// TestEGLRejectedDisplayKeepsHeld covers the open path of #1284: a second
// window's eglInitialize returns the display the first window already
// holds, and a later eglBindAPI or eglChooseConfig failure must not
// terminate it under that window. A display nobody holds is still
// terminated, and the rejection leaves the holder's count untouched.
func TestEGLRejectedDisplayKeepsHeld(t *testing.T) {
	terminated := stubEGLTeardown(t)
	const held, unheld = uintptr(0x1284), uintptr(0x1285)
	eglDisplayAcquire(held)
	t.Cleanup(func() { eglDisplayRelease(held) })

	eglReleaseRejected(held)
	if n := terminated[held]; n != 0 {
		t.Fatalf("rejecting a held display terminated it %d times", n)
	}
	// The hold is still the first window's: its own close is the last one.
	if !eglDisplayRelease(held) {
		t.Fatal("rejection consumed the holder's count")
	}
	eglDisplayAcquire(held) // restore for the Cleanup release

	eglReleaseRejected(unheld)
	if n := terminated[unheld]; n != 1 {
		t.Fatalf("rejecting an unheld display terminated it %d times, want 1", n)
	}
}

// TestNewX11HoldsEGLDisplay drives the real newX11 path so the hold it takes
// is pinned, not hand-written: the tests above acquire by hand and would pass
// with newX11's acquire removed, and without it the first window to close
// terminates the shared display again (#1284). The display init is faked and
// the surface creation fails, which sends newX11 through b.plat.destroy, the
// same teardown a closing window runs. Needs only an X connection (CI has
// Xvfb); without one the test skips.
func TestNewX11HoldsEGLDisplay(t *testing.T) {
	t.Setenv("GOGUI_RENDERER", "gl") // auto-pick would probe real EGL
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("no X display: %v", err)
	}
	rootVisual := xproto.Setup(conn).DefaultScreen(conn).RootVisual
	conn.Close()

	terminated := stubEGLTeardown(t)
	const dpy = uintptr(0x1284)
	oldInit, oldSurf := eglInitDisplayNFunc, eglCreateSurfaceContextFunc
	t.Cleanup(func() { eglInitDisplayNFunc, eglCreateSurfaceContextFunc = oldInit, oldSurf })
	eglInitDisplayNFunc = func() (uintptr, []eglConfigVisual, error) {
		// The root visual, so pickVisual chooses a visual CreateWindow takes.
		return dpy, []eglConfigVisual{{config: 1, visualID: uint32(rootVisual)}}, nil
	}
	holdsAtSurface := -1
	eglCreateSurfaceContextFunc = func(uintptr, uintptr, uintptr) (uintptr, uintptr, error) {
		eglDisplayRefs.mu.Lock()
		holdsAtSurface = eglDisplayRefs.n[dpy]
		eglDisplayRefs.mu.Unlock()
		return 0, 0, errors.New("injected surface failure")
	}

	// newX11, not New: New tries Wayland first when WAYLAND_DISPLAY is set,
	// and that path never reaches the stubbed display init.
	b, err := newX11(gui.NewWindow(gui.WindowCfg{State: new(int), Width: 100, Height: 100}))
	if err == nil {
		b.Destroy()
		t.Fatal("New with a failing surface returned nil error")
	}
	if holdsAtSurface != 1 {
		t.Fatalf("newX11 held the EGL display %d times while creating the surface, want 1",
			holdsAtSurface)
	}
	if n := terminated[dpy]; n != 1 {
		t.Fatalf("teardown of the only window terminated the display %d times, want 1", n)
	}
	eglDisplayRefs.mu.Lock()
	left := eglDisplayRefs.n[dpy]
	eglDisplayRefs.mu.Unlock()
	if left != 0 {
		t.Fatalf("teardown left %d holds on the display", left)
	}
}
