//go:build windows && !js

package gl

import (
	"testing"
	"time"
	"unsafe"
)

// Pure helpers of the GOGUI_RENDERER=soft present path on Windows (#1310).
// None needs a window.

// TestDIBHeader: the frame store is top-down, so soft's row 0 is the
// window's top row, and 32 bpp BI_RGB, whose B, G, R, x byte order is what
// swizzleBGRA writes. A positive height would show every frame upside
// down; a wrong biSize makes CreateDIBSection fail.
func TestDIBHeader(t *testing.T) {
	h := dibHeader(320, 200)
	if h.biSize != 40 || unsafe.Sizeof(h) != 40 {
		t.Fatalf("biSize %d, struct %d bytes; BITMAPINFOHEADER is 40", h.biSize, unsafe.Sizeof(h))
	}
	if h.biWidth != 320 || h.biHeight != -200 {
		t.Fatalf("size %dx%d, want 320x-200 (top-down)", h.biWidth, h.biHeight)
	}
	if h.biPlanes != 1 || h.biBitCount != 32 || h.biCompression != biRGB {
		t.Fatalf("planes %d, bpp %d, compression %d; want 1, 32, BI_RGB",
			h.biPlanes, h.biBitCount, h.biCompression)
	}
}

// TestDisplayFrameInterval: soft paces to the monitor's display
// frequency. 0 and 1 are the "hardware default" answers of
// EnumDisplaySettingsW, and a rate outside 20–1000 Hz is a misreport: all
// of them pace at 60 Hz. The DEVMODEW offset pins the field the rate is
// read from.
func TestDisplayFrameInterval(t *testing.T) {
	cases := []struct {
		hz   uint32
		want time.Duration
	}{
		{0, defaultFrameInterval},
		{1, defaultFrameInterval},
		{19, defaultFrameInterval},
		{20, time.Second / 20},
		{60, time.Second / 60},
		{144, time.Second / 144},
		{1000, time.Second / 1000},
		{1001, defaultFrameInterval},
	}
	for _, c := range cases {
		if got := displayFrameInterval(c.hz); got != c.want {
			t.Errorf("displayFrameInterval(%d) = %v, want %v", c.hz, got, c.want)
		}
	}
	var dm devModeW
	if off, size := unsafe.Offsetof(dm.dmDisplayFrequency), unsafe.Sizeof(dm); off != 184 || size != 220 {
		t.Fatalf("DEVMODEW: dmDisplayFrequency at %d, size %d; want 184 and 220", off, size)
	}
}

// TestPaceDeadline: a frame ahead of its deadline sleeps until it, and the
// next deadline is one interval after the old one, so a steady loop holds
// the rate. A late frame does not sleep, and the next deadline counts from
// now: a slow frame must not bank time for a burst after it. The first
// frame (zero deadline) does not sleep.
func TestPaceDeadline(t *testing.T) {
	const iv = 16 * time.Millisecond
	t0 := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		next     time.Time
		now      time.Time
		wantWait time.Duration
		wantNext time.Time
	}{
		{"first frame", time.Time{}, t0, 0, t0.Add(iv)},
		{"early", t0.Add(iv), t0.Add(5 * time.Millisecond), 11 * time.Millisecond, t0.Add(2 * iv)},
		{"on time", t0.Add(iv), t0.Add(iv), 0, t0.Add(2 * iv)},
		{"late", t0.Add(iv), t0.Add(50 * time.Millisecond), 0, t0.Add(50*time.Millisecond + iv)},
	}
	for _, c := range cases {
		wait, next := paceDeadline(c.next, c.now, iv)
		if wait != c.wantWait || !next.Equal(c.wantNext) {
			t.Errorf("%s: wait %v, next %v; want %v, %v", c.name, wait,
				next.Sub(t0), c.wantWait, c.wantNext.Sub(t0))
		}
	}
}

// TestSoftWin32PresentEmptyFrame: a client area 0 wide or high (a
// frameless window resized to 0, or a window dragged down to its title bar)
// has nothing to draw, as GL has an empty viewport. present returns before
// Draw, so no "frame skipped" line is logged for a state that is not a
// fault. The nil window proves Draw is not reached: Lock on it would panic.
func TestSoftWin32PresentEmptyFrame(t *testing.T) {
	for _, size := range [][2]int32{{0, 0}, {640, 0}, {0, 480}, {-1, 480}} {
		sp := &softPresentWin32{vsyncOff: true}
		if sp.present(nil, size[0], size[1]) {
			t.Errorf("present(%dx%d) = true, want false: nothing reached the window",
				size[0], size[1])
		}
		if sp.skipLogged {
			t.Errorf("present(%dx%d) logged a skipped frame", size[0], size[1])
		}
	}
}
