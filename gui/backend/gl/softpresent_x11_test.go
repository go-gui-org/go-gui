//go:build linux && !js && !android

package gl

import (
	"testing"
	"time"

	"github.com/jezek/xgb/randr"
	"github.com/jezek/xgb/xproto"
)

// Pure helpers of the GOGUI_RENDERER=soft present path (#1265). None needs an
// X server: each works over data a test can build by hand.

// trueColor is a TrueColor visual with the masks soft's BGRX swizzle writes.
func trueColor(id xproto.Visualid) xproto.VisualInfo {
	return xproto.VisualInfo{VisualId: id, Class: xproto.VisualClassTrueColor,
		BitsPerRgbValue: 8, RedMask: 0xff0000, GreenMask: 0xff00, BlueMask: 0xff}
}

func TestPickSoftVisual(t *testing.T) {
	bgr := trueColor(0x30) // masks swapped: blue high, so swizzle would be wrong
	bgr.RedMask, bgr.BlueMask = 0xff, 0xff0000
	direct := trueColor(0x31)
	direct.Class = xproto.VisualClassDirectColor

	screen := []xproto.DepthInfo{
		{Depth: 24, Visuals: []xproto.VisualInfo{trueColor(0x21), trueColor(0x22)}},
		{Depth: 32, Visuals: []xproto.VisualInfo{direct, trueColor(0x41)}},
	}
	noARGB := []xproto.DepthInfo{
		{Depth: 24, Visuals: []xproto.VisualInfo{trueColor(0x21)}},
	}
	wrongMasks := []xproto.DepthInfo{
		{Depth: 24, Visuals: []xproto.VisualInfo{bgr}},
	}
	cases := []struct {
		name        string
		depths      []xproto.DepthInfo
		root        xproto.Visualid
		transparent bool
		wantVis     xproto.Visualid
		wantDepth   byte
		wantARGB    bool
		wantOK      bool
	}{
		// Opaque keeps the root visual when it fits, like the GL path
		// keeps the driver's first choice.
		{"opaque root", screen, 0x22, false, 0x22, 24, false, true},
		// Root visual unusable (masks swapped): first fitting depth-24.
		{"opaque root bgr", append([]xproto.DepthInfo{{Depth: 24,
			Visuals: []xproto.VisualInfo{bgr}}}, screen...), 0x30, false, 0x21, 24, false, true},
		// Transparent skips the DirectColor visual for the TrueColor one.
		{"transparent", screen, 0x21, true, 0x41, 32, true, true},
		// No depth-32 visual: degrade to opaque, report argb=false.
		{"transparent no argb", noARGB, 0x21, true, 0x21, 24, false, true},
		// Nothing soft can write into: fail rather than draw garbage.
		{"no usable visual", wrongMasks, 0x30, false, 0, 0, false, false},
		{"empty screen", nil, 0x21, false, 0, 0, false, false},
	}
	for _, c := range cases {
		vis, depth, argb, ok := pickSoftVisual(c.depths, c.root, c.transparent)
		if vis != c.wantVis || depth != c.wantDepth || argb != c.wantARGB || ok != c.wantOK {
			t.Errorf("%s: got (%#x, %d, argb=%v, ok=%v), want (%#x, %d, argb=%v, ok=%v)",
				c.name, vis, depth, argb, ok, c.wantVis, c.wantDepth, c.wantARGB, c.wantOK)
		}
	}
}

// TestSoftImageFormatOK: soft writes 4 bytes per pixel in little-endian
// BGRX/BGRA order, so the depth's ZPixmap format must be 32 bpp and the
// server's image byte order LSBFirst. A big-endian server or a packed 24 bpp
// format is refused, not drawn with swapped channels.
func TestSoftImageFormatOK(t *testing.T) {
	formats := []xproto.Format{
		{Depth: 1, BitsPerPixel: 1, ScanlinePad: 32},
		{Depth: 24, BitsPerPixel: 32, ScanlinePad: 32},
		{Depth: 32, BitsPerPixel: 32, ScanlinePad: 32},
	}
	packed := []xproto.Format{{Depth: 24, BitsPerPixel: 24, ScanlinePad: 32}}
	cases := []struct {
		name    string
		formats []xproto.Format
		order   byte
		depth   byte
		want    bool
	}{
		{"depth 24", formats, xproto.ImageOrderLSBFirst, 24, true},
		{"depth 32", formats, xproto.ImageOrderLSBFirst, 32, true},
		{"msb server", formats, xproto.ImageOrderMSBFirst, 24, false},
		{"packed 24bpp", packed, xproto.ImageOrderLSBFirst, 24, false},
		{"depth missing", formats, xproto.ImageOrderLSBFirst, 16, false},
	}
	for _, c := range cases {
		if got := softImageFormatOK(c.formats, c.order, c.depth); got != c.want {
			t.Errorf("%s: softImageFormatOK = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestBandRows: a PutImage request holds a 24-byte header plus rows of
// width*4 bytes and may not exceed the server's maximum request length, so a
// frame goes up in bands. At least one row always goes, even when a single
// row is over the limit (the request then fails at the server, which is
// louder than drawing nothing).
func TestBandRows(t *testing.T) {
	const maxReq = 65535 * 4 // the common MaximumRequestLength, in bytes
	cases := []struct {
		width, maxReq, want int
	}{
		{1024, maxReq, (maxReq - 24) / 4096},
		{1, maxReq, (maxReq - 24) / 4},
		{70000, maxReq, 1}, // one row is over the limit
		{1024, 0, 1},       // unknown limit
	}
	for _, c := range cases {
		if got := bandRows(c.width, c.maxReq); got != c.want {
			t.Errorf("bandRows(%d, %d) = %d, want %d", c.width, c.maxReq, got, c.want)
		}
		if c.want > 1 && 24+c.want*c.width*4 > c.maxReq {
			t.Errorf("bandRows(%d, %d) = %d overflows the request", c.width, c.maxReq, c.want)
		}
	}
}

// TestPutImageRequestHeader pins the core PutImage header putCore writes in
// place of xproto.PutImage (#1265 step 5), byte for byte against the X11
// protocol's request 72: opcode, format, length in 4-byte units, drawable,
// gc, width, height, dst-x, dst-y, left-pad, depth, two unused bytes, all
// little-endian as xgb sends them. The live pixel tests cover the server
// accepting it.
func TestPutImageRequestHeader(t *testing.T) {
	const w, h = 3, 2
	req := make([]byte, putImageHeader+w*h*4)
	for i := range req {
		req[i] = 0xee // the header must overwrite every byte it owns
	}
	putImageRequestHeader(req, 0x01020304, 0x0a0b0c0d, w, h, 0x0506, 24)
	want := []byte{
		72, xproto.ImageFormatZPixmap, // opcode, format
		byte(len(req) / 4), 0, // length: 12 units
		0x04, 0x03, 0x02, 0x01, // drawable
		0x0d, 0x0c, 0x0b, 0x0a, // gc
		w, 0, h, 0, // width, height
		0, 0, // dst-x
		0x06, 0x05, // dst-y
		0, 24, 0, 0, // left-pad, depth, unused
	}
	if got := req[:putImageHeader]; string(got) != string(want) {
		t.Fatalf("header\n got % x\nwant % x", got, want)
	}
	for i, c := range req[putImageHeader:] {
		if c != 0xee {
			t.Fatalf("header wrote into the pixel data at byte %d", putImageHeader+i)
		}
	}
}

// TestSwizzleBGRA: premultiplied RGBA in, B G R A out. For a depth-24 visual
// the fourth byte is padding the server ignores; for depth 32 it is the
// premultiplied alpha the compositor reads, so it is kept either way.
func TestSwizzleBGRA(t *testing.T) {
	src := []byte{
		10, 20, 30, 255,
		1, 2, 3, 4,
		0, 0, 0, 0,
	}
	dst := make([]byte, len(src))
	swizzleBGRA(dst, src)
	want := []byte{
		30, 20, 10, 255,
		3, 2, 1, 4,
		0, 0, 0, 0,
	}
	for i := range want {
		if dst[i] != want[i] {
			t.Fatalf("byte %d = %d, want %d (got %v)", i, dst[i], want[i], dst)
		}
	}
	// A short dst writes only the whole pixels it can hold.
	short := make([]byte, 6)
	swizzleBGRA(short, src)
	if short[0] != 30 || short[4] != 0 {
		t.Fatalf("short dst = %v, want first pixel only", short)
	}
}

// TestModeRefresh: the frame interval soft paces to comes from the CRTC's
// mode (dot clock / (htotal * vtotal)). A mode with no timing falls back to
// 60 Hz, as does an absurd rate.
func TestModeRefresh(t *testing.T) {
	m60 := randr.ModeInfo{DotClock: 148500000, Htotal: 2200, Vtotal: 1125} // 1080p60
	m144 := randr.ModeInfo{DotClock: 356500000, Htotal: 2200, Vtotal: 1125}
	cases := []struct {
		name string
		mode randr.ModeInfo
		want time.Duration
	}{
		{"1080p60", m60, time.Second / 60},
		{"144Hz", m144, time.Duration(2200 * 1125 * int64(time.Second) / 356500000)},
		{"no timing", randr.ModeInfo{}, time.Second / 60},
		{"absurd", randr.ModeInfo{DotClock: 1 << 31, Htotal: 1, Vtotal: 1}, time.Second / 60},
	}
	for _, c := range cases {
		got := modeFrameInterval(c.mode)
		diff := got - c.want
		if diff < -time.Microsecond || diff > time.Microsecond {
			t.Errorf("%s: interval %v, want %v", c.name, got, c.want)
		}
	}
}

// TestSetMonitorReadsOncePerCrtc: a window drag sends a ConfigureNotify per
// step, each reaching setMonitor. The interval is read once per CRTC, also
// with VSyncOff and before the first frame, when pace has never set next.
// A planted interval surviving the repeat call proves no re-read happened.
func TestSetMonitorReadsOncePerCrtc(t *testing.T) {
	const marker = 7 * time.Millisecond
	sp := &softPresent{vsyncOff: true}
	sp.setMonitor(0, false, 5)
	if sp.interval != defaultFrameInterval {
		t.Fatalf("first read: interval %v, want %v", sp.interval, defaultFrameInterval)
	}
	sp.interval = marker
	sp.setMonitor(0, false, 5)
	if sp.interval != marker {
		t.Fatalf("same CRTC re-read the interval (%v)", sp.interval)
	}
	sp.setMonitor(0, false, 6)
	if sp.interval != defaultFrameInterval {
		t.Fatalf("new CRTC kept the old interval (%v)", sp.interval)
	}
}
