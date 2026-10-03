//go:build linux && !js && !android

package gl

import (
	"errors"
	"strings"
	"testing"
	"unsafe"
)

// eglOpener builds a fake display opener for eglFirstDesktopGL. No
// EGL library is needed: the openers stand in for the real
// eglGetDisplay / eglGetPlatformDisplay paths.
func eglOpener(dpy uintptr, cands []eglConfigVisual, err error) func() (uintptr, []eglConfigVisual, error) {
	return func() (uintptr, []eglConfigVisual, error) { return dpy, cands, err }
}

// TestEGLFallbackOnGLESOnlyDisplay is the #916 case: the default
// display (libhybris on a phone) has no desktop-GL config, and the
// Mesa X11 platform display has one. The fallback display wins and
// the default display is released.
func TestEGLFallbackOnGLESOnlyDisplay(t *testing.T) {
	want := []eglConfigVisual{{config: 7, visualID: 0x23}}
	var released []uintptr
	dpy, cands, err := eglFirstDesktopGL(
		[]func() (uintptr, []eglConfigVisual, error){
			eglOpener(1, nil, errNoDesktopGLConfig),
			eglOpener(2, want, nil),
		},
		func(d uintptr) { released = append(released, d) })
	if err != nil {
		t.Fatalf("err = %v, want fallback to succeed", err)
	}
	if dpy != 2 || len(cands) != 1 || cands[0] != want[0] {
		t.Fatalf("got dpy %d cands %v, want the fallback display", dpy, cands)
	}
	if len(released) != 1 || released[0] != 1 {
		t.Fatalf("released %v, want only the GLES-only display 1", released)
	}
}

// TestEGLNoFallbackOnOtherError keeps a hard failure (no display,
// eglInitialize error) from being hidden by a second attempt.
func TestEGLNoFallbackOnOtherError(t *testing.T) {
	cause := errors.New("eglInitialize failed (egl error 0x3001)")
	called := false
	_, _, err := eglFirstDesktopGL(
		[]func() (uintptr, []eglConfigVisual, error){
			eglOpener(1, nil, cause),
			func() (uintptr, []eglConfigVisual, error) {
				called = true
				return 2, []eglConfigVisual{{config: 1}}, nil
			},
		},
		func(uintptr) {})
	if !errors.Is(err, cause) {
		t.Fatalf("err = %v, want %v", err, cause)
	}
	if called {
		t.Fatal("fallback ran after a non-GLES error")
	}
}

// TestEGLAllDisplaysGLESOnly checks the error names the real cause
// when no display offers desktop GL: OpenGL ES only, not an old driver.
func TestEGLAllDisplaysGLESOnly(t *testing.T) {
	var released []uintptr
	_, _, err := eglFirstDesktopGL(
		[]func() (uintptr, []eglConfigVisual, error){
			eglOpener(1, nil, errNoDesktopGLConfig),
			eglOpener(0, nil, errors.New("eglGetPlatformDisplay(X11): no display")),
		},
		func(d uintptr) { released = append(released, d) })
	if !errors.Is(err, errNoDesktopGLConfig) {
		t.Fatalf("err = %v, want errNoDesktopGLConfig", err)
	}
	if !strings.Contains(err.Error(), "OpenGL ES") {
		t.Fatalf("err = %q, want it to name OpenGL ES", err)
	}
	if !strings.Contains(err.Error(), "X11") {
		t.Fatalf("err = %q, want the fallback failure kept", err)
	}
	if len(released) != 1 || released[0] != 1 {
		t.Fatalf("released %v, want display 1 only (0 is no display)", released)
	}
}

// stubEGL swaps the EGL entry points eglOpenDisplay calls for fakes and
// restores them when the test ends. bind and choose set what
// eglBindAPI and eglChooseConfig return; n is the match count.
func stubEGL(t *testing.T, bind, choose uint32, n int32) {
	t.Helper()
	oldInit, oldBind, oldChoose := eglInitialize, eglBindAPI, eglChooseConfig
	oldErr, oldAttrib := eglGetError, eglGetConfigAttrib
	t.Cleanup(func() {
		eglInitialize, eglBindAPI, eglChooseConfig = oldInit, oldBind, oldChoose
		eglGetError, eglGetConfigAttrib = oldErr, oldAttrib
	})
	eglInitialize = func(uintptr, unsafe.Pointer, unsafe.Pointer) uint32 { return 1 }
	eglBindAPI = func(uint32) uint32 { return bind }
	eglChooseConfig = func(_ uintptr, _, cfgs unsafe.Pointer, _ int32, np unsafe.Pointer) uint32 {
		*(*int32)(np) = n
		if n > 0 {
			*(*uintptr)(cfgs) = 0x42
		}
		return choose
	}
	eglGetError = func() int32 { return 0x3000 }
	eglGetConfigAttrib = func(_, _ uintptr, _ int32, v unsafe.Pointer) uint32 {
		*(*int32)(v) = 0x23
		return 1
	}
}

// TestEGLOpenDisplayErrorMapping pins which eglOpenDisplay failures
// count as "no desktop GL" and so let eglFirstDesktopGL fall back
// (#916). If this mapping broke, the fallback would never run.
func TestEGLOpenDisplayErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		bind, choose uint32
		n            int32
		wantGLESOnly bool
	}{
		{"bind API refused", 0, 1, 0, true},
		{"no config matched", 1, 1, 0, true},
		{"choose config failed", 1, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubEGL(t, tc.bind, tc.choose, tc.n)
			dpy, cands, err := eglOpenDisplay(9, "test")
			if err == nil {
				t.Fatalf("err = nil, cands %v", cands)
			}
			if got := errors.Is(err, errNoDesktopGLConfig); got != tc.wantGLESOnly {
				t.Fatalf("errors.Is(%v, errNoDesktopGLConfig) = %v, want %v",
					err, got, tc.wantGLESOnly)
			}
			// The display was initialized, so it must come back for
			// eglFirstDesktopGL to release it.
			if dpy != 9 {
				t.Fatalf("dpy = %d, want 9 so the caller can release it", dpy)
			}
		})
	}
}

// TestEGLOpenDisplaySuccess checks a matching config comes back with
// its X visual id.
func TestEGLOpenDisplaySuccess(t *testing.T) {
	stubEGL(t, 1, 1, 1)
	dpy, cands, err := eglOpenDisplay(9, "test")
	if err != nil {
		t.Fatal(err)
	}
	if dpy != 9 || len(cands) != 1 || cands[0] != (eglConfigVisual{config: 0x42, visualID: 0x23}) {
		t.Fatalf("got dpy %d cands %v", dpy, cands)
	}
}

// TestEGLFirstDesktopGLNoOpeners keeps the "configs never empty on a
// nil error" contract when there is nothing to open.
func TestEGLFirstDesktopGLNoOpeners(t *testing.T) {
	dpy, cands, err := eglFirstDesktopGL(nil, func(uintptr) {})
	if err == nil || dpy != 0 || cands != nil {
		t.Fatalf("got dpy %d cands %v err %v, want an error", dpy, cands, err)
	}
}

// TestEGLBothDisplaysGLESOnly releases both displays and keeps the
// OpenGL ES cause when the fallback is GL ES-only too.
func TestEGLBothDisplaysGLESOnly(t *testing.T) {
	var released []uintptr
	_, _, err := eglFirstDesktopGL(
		[]func() (uintptr, []eglConfigVisual, error){
			eglOpener(1, nil, errNoDesktopGLConfig),
			eglOpener(2, nil, errNoDesktopGLConfig),
		},
		func(d uintptr) { released = append(released, d) })
	if !errors.Is(err, errNoDesktopGLConfig) || !strings.Contains(err.Error(), "fallback") {
		t.Fatalf("err = %v, want errNoDesktopGLConfig with the fallback cause", err)
	}
	if len(released) != 2 {
		t.Fatalf("released %v, want both displays", released)
	}
}

// TestEGLOpenDisplayNoDisplay: a zero handle is a hard error, not GL
// ES-only, and returns no handle to release.
func TestEGLOpenDisplayNoDisplay(t *testing.T) {
	stubEGL(t, 1, 1, 1)
	dpy, _, err := eglOpenDisplay(0, "test")
	if err == nil || errors.Is(err, errNoDesktopGLConfig) || dpy != 0 {
		t.Fatalf("got dpy %d err %v, want a hard error and no handle", dpy, err)
	}
}

// TestEGLOpenDisplayInitFails: an uninitialized display is a hard
// error and is not handed back for eglTerminate.
func TestEGLOpenDisplayInitFails(t *testing.T) {
	stubEGL(t, 1, 1, 1)
	eglInitialize = func(uintptr, unsafe.Pointer, unsafe.Pointer) uint32 { return 0 }
	dpy, _, err := eglOpenDisplay(9, "test")
	if err == nil || errors.Is(err, errNoDesktopGLConfig) || dpy != 0 {
		t.Fatalf("got dpy %d err %v, want a hard error and no handle", dpy, err)
	}
}

// TestEGLOpenDisplayClampsCount: a driver that reports more configs
// than the buffer holds must not make cfgs[:n] panic.
func TestEGLOpenDisplayClampsCount(t *testing.T) {
	stubEGL(t, 1, 1, eglMaxConfigs+10)
	_, cands, err := eglOpenDisplay(9, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != eglMaxConfigs {
		t.Fatalf("len(cands) = %d, want %d", len(cands), eglMaxConfigs)
	}
}
