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
