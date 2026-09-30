//go:build linux && !js && !android

package gl

import (
	"math"
	"os"
	"testing"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

// TestCrtcDPI covers the per-monitor DPI math headlessly (no X server).
func TestCrtcDPI(t *testing.T) {
	cases := []struct {
		name     string
		w, h     uint16
		rotation uint16
		mmW, mmH uint32
		wantDPI  float64 // 0 ⇒ expect ok=false
	}{
		// 1366x768 over 344x193mm ≈ 101 DPI (matches a real 15" laptop).
		{"laptop", 1366, 768, randr.RotationRotate0, 344, 193, 101},
		// 90° rotation swaps pixel axes; DPI must be unchanged.
		{"rotated90", 768, 1366, randr.RotationRotate90, 344, 193, 101},
		// 3840x2160 over 344x194mm ≈ 283 DPI (4K laptop panel).
		{"hidpi", 3840, 2160, randr.RotationRotate0, 344, 194, 283},
		{"no-physical-size", 1920, 1080, randr.RotationRotate0, 0, 0, 0},
		{"implausible-tiny-mm", 1920, 1080, randr.RotationRotate0, 5, 5, 0},
	}
	for _, c := range cases {
		info := &randr.GetCrtcInfoReply{Width: c.w, Height: c.h, Rotation: c.rotation}
		out := &randr.GetOutputInfoReply{MmWidth: c.mmW, MmHeight: c.mmH}
		dpi, ok := crtcDPI(info, out)
		if c.wantDPI == 0 {
			if ok {
				t.Errorf("%s: expected ok=false, got dpi=%.1f", c.name, dpi)
			}
			continue
		}
		if !ok {
			t.Errorf("%s: expected ok=true", c.name)
			continue
		}
		if math.Abs(dpi-c.wantDPI) > 1.5 {
			t.Errorf("%s: dpi=%.2f, want ~%.1f", c.name, dpi, c.wantDPI)
		}
	}
}

// TestParseXftDPIScale covers the Xft.dpi string to scale conversion. Only an
// integer in [minXftDPI, maxXftDPI] counts as set.
func TestParseXftDPIScale(t *testing.T) {
	cases := []struct {
		in     string
		want   float32
		wantOK bool
	}{
		{"192", 2, true},
		{"96", 1, true},
		{"120", 1.25, true},
		{"", 0, false},
		{"0", 0, false},
		{"-5", 0, false},
		{"abc", 0, false},
		{"96.5", 0, false},
		// Bounds: scale 0.5 to 8 is accepted, anything outside counts as unset.
		{"48", 0.5, true},
		{"768", 8, true},
		{"47", 0, false},
		{"769", 0, false},
		{"100000", 0, false},
		{"99999999999999999999", 0, false},
	}
	for _, c := range cases {
		got, ok := parseXftDPIScale(c.in)
		if ok != c.wantOK || got != c.want {
			t.Errorf("parseXftDPIScale(%q) = (%v, %v), want (%v, %v)",
				c.in, got, ok, c.want, c.wantOK)
		}
	}
}

// TestPickDPIScale pins the scale source order. Issue #871: under GNOME
// Wayland fractional scaling, XWayland reports a 2x virtual CRTC with the
// real panel millimetres, so RandR gives 2.62 while the desktop set
// Xft.dpi: 192 (2.0). Xft.dpi must win, and RandR must not even be queried.
func TestPickDPIScale(t *testing.T) {
	cases := []struct {
		name      string
		xft       float32
		xftOK     bool
		haveRandr bool
		rrScale   float32
		rrCrtc    randr.Crtc
		rrOK      bool
		wantScale float32
		wantCrtc  randr.Crtc
		wantRRUse bool
	}{
		{"xft-wins-over-randr", 2, true, true, 2.62, 7, true, 2, 0, false},
		// Xft.dpi: 96 on a 2x panel is kept on purpose. GNOME publishes 96 when
		// the user picks 100%, and GTK/Qt then draw at 1.0, so reading 96 as
		// "unset" and asking RandR would bring back the #871 mismatch.
		{"xft-96-hidpi-randr", 1, true, true, 2, 7, true, 1, 0, false},
		{"randr-when-xft-unset", 0, false, true, 1.5, 7, true, 1.5, 7, true},
		{"one-when-nothing", 0, false, true, 0, 0, false, 1, 0, true},
		{"no-randr-extension", 0, false, false, 1.5, 7, true, 1, 0, false},
		{"xft-without-randr", 1.25, true, false, 0, 0, false, 1.25, 0, false},
	}
	for _, c := range cases {
		called := false
		rr := func() (float32, randr.Crtc, bool) {
			called = true
			return c.rrScale, c.rrCrtc, c.rrOK
		}
		scale, crtc := pickDPIScale(c.xft, c.xftOK, c.haveRandr, rr)
		if scale != c.wantScale || crtc != c.wantCrtc {
			t.Errorf("%s: got (%v, %d), want (%v, %d)",
				c.name, scale, crtc, c.wantScale, c.wantCrtc)
		}
		if called != c.wantRRUse {
			t.Errorf("%s: randr queried = %v, want %v", c.name, called, c.wantRRUse)
		}
	}
}

// TestDPIScaleForWindowLive checks per-monitor detection against the live
// X server. Opt-in via GOGUI_X11_IT=1: it opens an X connection which, when
// sharing a process with the EGL/GL context test, can trip a native
// Mesa/Xlib threading crash unrelated to the DPI code.
func TestDPIScaleForWindowLive(t *testing.T) {
	if os.Getenv("GOGUI_X11_IT") == "" {
		t.Skip("set GOGUI_X11_IT=1 to run the live X11 DPI test")
	}
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no DISPLAY; live DPI test needs an X server")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		t.Skipf("x11 connect: %v", err)
	}
	defer conn.Close()

	root := xproto.Setup(conn).DefaultScreen(conn).Root
	haveRandr := randr.Init(conn) == nil
	scale, crtc := dpiScaleForWindow(conn, root, haveRandr, 0, 0)
	t.Logf("dpiScaleForWindow(0,0) = %.4f crtc=%d haveRandr=%v", scale, crtc, haveRandr)
	if scale <= 0 {
		t.Fatalf("scale = %v, want > 0", scale)
	}
	if scale < 0.5 || scale > 4.5 {
		t.Errorf("scale = %.4f outside plausible range [0.5,4.5]", scale)
	}
}
