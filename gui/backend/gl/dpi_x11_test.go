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
// Issue #918: under XWayland with no Xft.dpi (Phosh), the compositor already
// scales the X11 buffer by the output scale. RandR physical DPI (1.81 on the
// FLX1s panel) would apply the scale a second time, so the scale is 1.0 and
// RandR is not queried.
func TestPickDPIScale(t *testing.T) {
	cases := []struct {
		name      string
		xft       float32
		xftOK     bool
		haveRandr bool
		xwayland  bool
		rrScale   float32
		rrCrtc    randr.Crtc
		rrOK      bool
		wantScale float32
		wantCrtc  randr.Crtc
		wantRRUse bool
	}{
		{"xft-wins-over-randr", 2, true, true, false, 2.62, 7, true, 2, 0, false},
		// Xft.dpi: 96 on a 2x panel is kept on purpose. GNOME publishes 96 when
		// the user picks 100%, and GTK/Qt then draw at 1.0, so reading 96 as
		// "unset" and asking RandR would bring back the #871 mismatch.
		{"xft-96-hidpi-randr", 1, true, true, false, 2, 7, true, 1, 0, false},
		{"randr-when-xft-unset", 0, false, true, false, 1.5, 7, true, 1.5, 7, true},
		{"one-when-nothing", 0, false, true, false, 0, 0, false, 1, 0, true},
		{"no-randr-extension", 0, false, false, false, 1.5, 7, true, 1, 0, false},
		{"xft-without-randr", 1.25, true, false, false, 0, 0, false, 1.25, 0, false},
		{"xwayland-no-xft-skips-randr", 0, false, true, true, 1.81, 7, true, 1, 0, false},
		{"xwayland-xft-wins", 1.5, true, true, true, 1.81, 7, true, 1.5, 0, false},
	}
	for _, c := range cases {
		called := false
		rr := func() (float32, randr.Crtc, bool) {
			called = true
			return c.rrScale, c.rrCrtc, c.rrOK
		}
		scale, crtc := pickDPIScale(c.xft, c.xftOK, c.haveRandr, c.xwayland, rr)
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
	xwayland := detectXWayland(conn, root, haveRandr)
	scale, crtc := dpiScaleForWindow(conn, root, haveRandr, xwayland, 0, 0)
	t.Logf("dpiScaleForWindow(0,0) = %.4f crtc=%d haveRandr=%v xwayland=%v",
		scale, crtc, haveRandr, xwayland)
	if scale <= 0 {
		t.Fatalf("scale = %v, want > 0", scale)
	}
	if scale < 0.5 || scale > 4.5 {
		t.Errorf("scale = %.4f outside plausible range [0.5,4.5]", scale)
	}
}

// TestIsXWaylandOutputName pins the fallback XWayland check for servers
// older than Xwayland 23.1, which do not advertise the XWAYLAND extension
// but name every RandR output XWAYLAND<n>.
func TestIsXWaylandOutputName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"XWAYLAND0", true},
		{"XWAYLAND12", true},
		{"eDP-1", false},
		{"HDMI-A-1", false},
		{"", false},
		{"XWAY", false},
	}
	for _, c := range cases {
		if got := isXWaylandOutputName([]byte(c.name)); got != c.want {
			t.Errorf("isXWaylandOutputName(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestPerMonitorDPI pins the gate for per-monitor RandR rescans: they need
// RandR and are off under Xwayland, where the compositor owns the scale (#918).
func TestPerMonitorDPI(t *testing.T) {
	cases := []struct {
		haveRandr, xwayland, want bool
	}{
		{true, false, true},
		{true, true, false},
		{false, false, false},
		{false, true, false},
	}
	for _, c := range cases {
		p := platformState{haveRandr: c.haveRandr, xwayland: c.xwayland}
		if got := p.perMonitorDPI(); got != c.want {
			t.Errorf("perMonitorDPI(haveRandr=%v, xwayland=%v) = %v, want %v",
				c.haveRandr, c.xwayland, got, c.want)
		}
	}
}
