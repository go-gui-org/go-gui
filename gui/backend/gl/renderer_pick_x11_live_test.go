//go:build linux && !js && !android

package gl

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"

	"github.com/go-gui-org/go-gui/gui"
)

// Live tests of auto-pick (#1265 step 3): GOGUI_RENDERER unset, a real X
// server. tools/glrendertest runs each in its own process under Xvfb, whose GL
// is llvmpipe; without an X server they skip (softSkip).

// newAutoX11 opens one X11 window with GOGUI_RENDERER unset and a fresh probe.
func newAutoX11(t *testing.T) *Backend {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		softSkip(t, "no DISPLAY; the test needs an X server")
	}
	t.Setenv("GOGUI_RENDERER", "")
	resetAutoProbe(t, probeGLRenderer)
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := newX11(w)
	if err != nil {
		softSkip(t, "x11 window: %v", err)
	}
	t.Cleanup(b.Destroy)
	return b
}

// TestAutoPickSoftOnSoftwareRasterizer runs the real probe. On a GL whose
// GL_RENDERER names a software rasterizer the window must come up on the CPU
// renderer with no EGL state, and on hardware GL it must stay on GL. The
// probe's string decides which of the two is checked, so the test is
// meaningful on a desktop with a GPU too; CI's Xvfb is the llvmpipe case.
func TestAutoPickSoftOnSoftwareRasterizer(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		softSkip(t, "no DISPLAY; the test needs an X server")
	}
	// The expected renderer is probed before the window exists, on this
	// test's own probe. Probing after newX11 would use the window's EGL
	// display on the window's thread, and the probe's release of its own
	// context would leave the window's GL context not current for Destroy.
	//
	// The probe holds an X connection of its own meanwhile: Xvfb resets
	// when its last client leaves, as the previous glrendertest process just
	// did, and a probe with no X client open raced that reset (eglInitialize
	// 0x3001). newX11 probes after its X connect for the same reason.
	conn, err := xgb.NewConn()
	if err != nil {
		softSkip(t, "x11 connect: %v", err)
	}
	// Closed after the window's Destroy (cleanups run last-first): closing it
	// before newAutoX11 connects would start the same reset.
	t.Cleanup(conn.Close)
	// eglMakeCurrent binds to the OS thread, and glGetString must run on the
	// same one; newX11 holds this lock for the real probe.
	runtime.LockOSThread()
	res := probeGLRenderer()
	runtime.UnlockOSThread()
	if res.eglErr != nil || res.probeErr != nil {
		softSkip(t, "probe found no GL: egl %v, probe %v", res.eglErr, res.probeErr)
	}
	b := newAutoX11(t)
	wantSoft := isSoftwareRasterizer(res.renderer)
	if b.plat.softMode != wantSoft {
		t.Fatalf("GL_RENDERER %q: softMode = %v, want %v",
			res.renderer, b.plat.softMode, wantSoft)
	}
	if wantSoft && (b.plat.eglDpy != 0 || b.plat.eglContext != 0) {
		t.Fatalf("auto-picked soft window holds EGL state: dpy=%#x context=%#x",
			b.plat.eglDpy, b.plat.eglContext)
	}
	if os.Getenv("GOGUI_REQUIRE_GL") != "" && !wantSoft {
		// The CI job provides llvmpipe; GL there means the matcher missed it.
		t.Fatalf("GOGUI_REQUIRE_GL is set but GL_RENDERER %q was not taken "+
			"for a software rasterizer", res.renderer)
	}
}

// TestAutoPickSoftOnEGLFailure: with no EGL display, an unset GOGUI_RENDERER
// opens the window on the CPU renderer instead of failing with
// ErrNoGPUContext, which only a named GOGUI_RENDERER=gl now reports
// (TestNewWrapsGPUDisplayError).
func TestAutoPickSoftOnEGLFailure(t *testing.T) {
	old := eglInitDisplayNFunc
	eglInitDisplayNFunc = func() (uintptr, []eglConfigVisual, error) {
		return 0, nil, errors.New("eglInitialize failed (egl error 0x3001)")
	}
	t.Cleanup(func() { eglInitDisplayNFunc = old })
	b := newAutoX11(t)
	if !b.plat.softMode || b.plat.soft == nil {
		t.Fatal("EGL failure with GOGUI_RENDERER unset did not open a soft window")
	}
}

// TestAutoPickSoftInitFailureFallsBackToGL: auto-pick chose soft from
// GL_RENDERER (here a stubbed probe says llvmpipe) and the CPU renderer then
// fails to start. GL drew on this machine before auto-pick, so the window
// opens on GL. Without a working EGL the test skips.
func TestAutoPickSoftInitFailureFallsBackToGL(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		softSkip(t, "no DISPLAY; the test needs an X server")
	}
	t.Setenv("GOGUI_RENDERER", "")
	resetAutoProbe(t, func() probeResult { return probeResult{renderer: "llvmpipe"} })
	old := newSoftPresentFunc
	newSoftPresentFunc = func(*xgb.Conn, xproto.Window, byte, *gui.Window, float32) (*softPresent, error) {
		return nil, errors.New("injected soft present failure")
	}
	t.Cleanup(func() { newSoftPresentFunc = old })
	// The failed soft window closes its X connection before GL reconnects.
	// Xvfb resets when its last client leaves, so a second client keeps the
	// server up across the retry (a real server has other clients).
	keep, kerr := xgb.NewConn()
	if kerr != nil {
		softSkip(t, "x11 connect: %v", kerr)
	}
	t.Cleanup(keep.Close)
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := newX11(w)
	if err != nil {
		softSkip(t, "gl window: %v", err)
	}
	t.Cleanup(b.Destroy)
	if b.plat.softMode {
		t.Fatal("soft init failed but the window is still soft")
	}
}

// TestAutoPickSoftInitFailureAfterEGLFailure: EGL failed, so auto-pick chose
// soft, and soft fails too. The error matches ErrNoGPUContext, keeps the EGL
// cause and does not advise GOGUI_RENDERER=soft.
func TestAutoPickSoftInitFailureAfterEGLFailure(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		softSkip(t, "no DISPLAY; the test needs an X server")
	}
	t.Setenv("GOGUI_RENDERER", "")
	eglErr := errors.New("eglInitialize failed (egl error 0x3001)")
	resetAutoProbe(t, func() probeResult { return probeResult{eglErr: eglErr} })
	old := newSoftPresentFunc
	newSoftPresentFunc = func(*xgb.Conn, xproto.Window, byte, *gui.Window, float32) (*softPresent, error) {
		return nil, errors.New("injected soft present failure")
	}
	t.Cleanup(func() { newSoftPresentFunc = old })
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 64, Height: 64})
	w.SetView(func(_ *gui.Window) gui.View { return gui.Column(gui.ContainerCfg{}) })
	b, err := newX11(w)
	if err == nil {
		b.Destroy()
		t.Fatal("expected an error when neither renderer can run")
	}
	if !errors.Is(err, ErrNoGPUContext) || !errors.Is(err, eglErr) {
		t.Errorf("err %v: want ErrNoGPUContext with the EGL cause", err)
	}
	if strings.Contains(err.Error(), "GOGUI_RENDERER=soft") {
		t.Errorf("err %q advises soft, which just failed", err)
	}
}
