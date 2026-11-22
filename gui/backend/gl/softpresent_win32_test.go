//go:build windows && !js

package gl

import (
	"bytes"
	"fmt"
	"math/rand/v2"
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

// TestLayeredAlphaFloor: a layered window's frame has no alpha 0, which
// would let the mouse through (#1310, decision 4). Alpha 0 becomes 1, in
// either half of a two-pixel word and in the odd last pixel; every other
// alpha, and every color byte, is swizzleBGRA's.
func TestLayeredAlphaFloor(t *testing.T) {
	src := []byte{
		0, 0, 0, 0, // alpha 0 in the word's low pixel
		10, 20, 30, 2,
		1, 2, 3, 1,
		0, 0, 0, 0, // alpha 0 in the word's high pixel
		40, 50, 60, 128,
		70, 80, 90, 255,
		0, 0, 0, 0, // the odd pixel
	}
	want := []byte{
		0, 0, 0, 1,
		30, 20, 10, 2,
		3, 2, 1, 1,
		0, 0, 0, 1,
		60, 50, 40, 128,
		90, 80, 70, 255,
		0, 0, 0, 1,
	}
	got := make([]byte, len(src))
	swizzleBGRALayered(got, src)
	if !bytes.Equal(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

// TestSwizzleBGRALayeredMatchesByteLoop: the word form, floor included,
// writes what a per-pixel byte loop writes, on seeded noise and on every
// alpha value in both halves of a word, for the lengths
// TestSwizzleBGRAMatchesByteLoop covers and in place.
func TestSwizzleBGRALayeredMatchesByteLoop(t *testing.T) {
	byteLoop := func(dst, src []byte) {
		n := min(len(dst), len(src)) &^ 3
		for i := 0; i < n; i += 4 {
			dst[i], dst[i+1], dst[i+2], dst[i+3] = src[i+2], src[i+1], src[i], max(src[i+3], 1)
		}
	}
	const guard = 0xa5
	fill := func(n int) []byte {
		rng := rand.New(rand.NewPCG(1310, uint64(n)))
		p := make([]byte, n)
		for i := range p {
			p[i] = byte(rng.Uint32())
		}
		// Noise has alpha 0 in one pixel of 256; put it in some on purpose.
		for i := 3; i < n; i += 12 {
			p[i] = 0
		}
		return p
	}
	for srcLen := 0; srcLen <= 41; srcLen++ {
		for _, dstLen := range []int{srcLen, srcLen + 5, max(srcLen-3, 0), max(srcLen-8, 0)} {
			src := fill(srcLen)
			got, want := make([]byte, dstLen), make([]byte, dstLen)
			for i := range got {
				got[i], want[i] = guard, guard
			}
			swizzleBGRALayered(got, src)
			byteLoop(want, src)
			if !bytes.Equal(got, want) {
				t.Fatalf("src %d bytes, dst %d bytes:\n got %v\nwant %v", srcLen, dstLen, got, want)
			}
		}
		inPlace, want := fill(srcLen), fill(srcLen)
		swizzleBGRALayered(inPlace, inPlace)
		byteLoop(want, fill(srcLen))
		if !bytes.Equal(inPlace, want) {
			t.Fatalf("in place, %d bytes:\n got %v\nwant %v", srcLen, inPlace, want)
		}
	}
	// Every pair of alphas in one word, with colors that differ per pixel.
	src := make([]byte, 0, 256*256*8)
	for a := range 256 {
		for b := range 256 {
			src = append(src, byte(a), byte(b), 7, byte(a), byte(b), byte(a), 9, byte(b))
		}
	}
	got, want := make([]byte, len(src)), make([]byte, len(src))
	swizzleBGRALayered(got, src)
	byteLoop(want, src)
	if !bytes.Equal(got, want) {
		t.Fatal("an alpha pair differs from the byte loop")
	}
}

// BenchmarkSwizzleBGRALayered times the layered window's conversion,
// beside BenchmarkSwizzleBGRA, the opaque one.
func BenchmarkSwizzleBGRALayered(b *testing.B) {
	for _, sz := range [][2]int{{1920, 1080}, {3840, 2160}} {
		b.Run(fmt.Sprintf("%dx%d", sz[0], sz[1]), func(b *testing.B) {
			src := make([]byte, sz[0]*sz[1]*4)
			for i := range src {
				src[i] = byte(i)
			}
			dst := make([]byte, len(src))
			b.SetBytes(int64(len(src)))
			for b.Loop() {
				swizzleBGRALayered(dst, src)
			}
		})
	}
}
