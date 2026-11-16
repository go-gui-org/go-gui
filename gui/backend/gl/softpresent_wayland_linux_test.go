//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"errors"
	"image"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/wl"
)

// Pure tests for the Wayland soft present path (#1265 step 4). The live
// tests that need a compositor are in softpresent_wayland_live_linux_test.go.

// fakeRing is a wlSlotRing whose slots are plain structs: alloc counts and
// sizes them, free records what was freed. No compositor, no memfd.
func fakeRing(t *testing.T) (r *wlSlotRing, allocs *int, freed *[]*wlSoftSlot) {
	t.Helper()
	n := 0
	var gone []*wlSoftSlot
	r = &wlSlotRing{
		alloc: func(w, h int32) (*wlSoftSlot, error) {
			n++
			return &wlSoftSlot{w: w, h: h}, nil
		},
		free: func(s *wlSoftSlot) { gone = append(gone, s) },
	}
	return r, &n, &gone
}

// TestSlotRingReusesReleasedSlot: a slot the compositor released is drawn
// into again; no new buffer is made while one of the right size is free.
func TestSlotRingReusesReleasedSlot(t *testing.T) {
	r, allocs, _ := fakeRing(t)
	a, err := r.acquire(100, 50)
	if err != nil || a == nil {
		t.Fatalf("acquire = %v, %v", a, err)
	}
	a.busy = true
	a.busy = false // wl_buffer.release
	b, _ := r.acquire(100, 50)
	if b != a {
		t.Fatal("a released slot of the right size was not reused")
	}
	if *allocs != 1 {
		t.Fatalf("allocs = %d, want 1", *allocs)
	}
}

// TestSlotRingGrowsToCapThenSkips: while the compositor holds every slot, a
// new one is made up to wlSoftMaxSlots; past that acquire returns nil (the
// frame is skipped and drawn after the next release), never a held slot.
func TestSlotRingGrowsToCapThenSkips(t *testing.T) {
	r, allocs, _ := fakeRing(t)
	seen := map[*wlSoftSlot]bool{}
	for i := range wlSoftMaxSlots {
		s, err := r.acquire(10, 10)
		if err != nil || s == nil {
			t.Fatalf("acquire %d = %v, %v", i, s, err)
		}
		if seen[s] {
			t.Fatalf("acquire %d returned a held slot", i)
		}
		seen[s] = true
		s.busy = true
	}
	s, err := r.acquire(10, 10)
	if err != nil || s != nil {
		t.Fatalf("acquire with every slot held = %v, %v; want nil, nil", s, err)
	}
	if *allocs != wlSoftMaxSlots {
		t.Fatalf("allocs = %d, want %d", *allocs, wlSoftMaxSlots)
	}
}

// TestSlotRingResizeDropsFreeSlots: after a resize, free slots of the old
// size are freed at once; a held one is kept until it is released, and
// freed by the first acquire after that.
func TestSlotRingResizeDropsFreeSlots(t *testing.T) {
	r, _, freed := fakeRing(t)
	held, _ := r.acquire(10, 10)
	held.busy = true
	free, _ := r.acquire(10, 10) // a second, then released
	if free == held {
		t.Fatal("acquire returned the held slot")
	}
	n, _ := r.acquire(20, 20)
	if n == nil || n.w != 20 || n.h != 20 {
		t.Fatalf("acquire(20,20) = %+v", n)
	}
	if len(*freed) != 1 || (*freed)[0] != free {
		t.Fatalf("freed = %v, want only the free old-size slot", *freed)
	}
	held.busy = false
	n.busy = true
	if _, err := r.acquire(20, 20); err != nil {
		t.Fatal(err)
	}
	if len(*freed) != 2 || (*freed)[1] != held {
		t.Fatalf("freed = %v, want the released old-size slot too", *freed)
	}
}

// TestSlotRingAllocError: an allocation failure is reported, and the ring
// keeps no half-made slot.
func TestSlotRingAllocError(t *testing.T) {
	cause := errors.New("memfd_create: out of memory")
	r := &wlSlotRing{
		alloc: func(int32, int32) (*wlSoftSlot, error) { return nil, cause },
		free:  func(*wlSoftSlot) { t.Fatal("free of a slot that was never made") },
	}
	if s, err := r.acquire(10, 10); !errors.Is(err, cause) || s != nil {
		t.Fatalf("acquire = %v, %v; want nil, %v", s, err, cause)
	}
	for _, s := range r.slots {
		if s != nil {
			t.Fatal("ring kept a slot after a failed alloc")
		}
	}
}

// TestSlotRingDestroyAll frees every slot, held ones included: the surface
// goes with them, so no release will come.
func TestSlotRingDestroyAll(t *testing.T) {
	r, _, freed := fakeRing(t)
	a, _ := r.acquire(10, 10)
	a.busy = true
	if _, err := r.acquire(10, 10); err != nil {
		t.Fatal(err)
	}
	r.destroyAll()
	if len(*freed) != 2 {
		t.Fatalf("freed %d slots, want 2", len(*freed))
	}
	for _, s := range r.slots {
		if s != nil {
			t.Fatal("destroyAll left a slot")
		}
	}
	r.destroyAll() // a second call is a no-op
	if len(*freed) != 2 {
		t.Fatal("a second destroyAll freed again")
	}
}

// TestWlShmSize pins the buffer size check: positive sides within
// wlMaxWindowSize, and a byte count that fits wl_shm's int32 pool size.
func TestWlShmSize(t *testing.T) {
	for _, c := range []struct {
		w, h int32
		n    int
		ok   bool
	}{
		{640, 480, 640 * 480 * 4, true},
		{wlMaxWindowSize, wlMaxWindowSize, wlMaxWindowSize * wlMaxWindowSize * 4, true},
		{0, 480, 0, false},
		{640, -1, 0, false},
		{wlMaxWindowSize + 1, 1, 0, false},
	} {
		n, err := wlShmSize(c.w, c.h)
		if (err == nil) != c.ok || (c.ok && n != c.n) {
			t.Errorf("wlShmSize(%d,%d) = %d, %v; want %d ok=%v", c.w, c.h, n, err, c.n, c.ok)
		}
	}
}

// TestWlShmFormat: an opaque window's buffer is XRGB8888, so the compositor
// ignores alpha (soft's alpha is not forced to 1, as GL's finishFrameAlpha
// does); a transparent one is ARGB8888, premultiplied, which soft writes.
// Both formats are ones every wl_shm must offer.
func TestWlShmFormat(t *testing.T) {
	if got := wlShmFormat(false); got != wl.ShmFormatXrgb8888 {
		t.Errorf("opaque: format %#x, want XRGB8888", got)
	}
	if got := wlShmFormat(true); got != wl.ShmFormatArgb8888 {
		t.Errorf("transparent: format %#x, want ARGB8888", got)
	}
}

// TestCopyBGRA: the frame lands in the buffer as B, G, R, A per pixel, the
// memory order of the little-endian [A]RGB8888 formats, for a tight image
// and for one whose stride is wider than its rows.
func TestCopyBGRA(t *testing.T) {
	tight := image.NewRGBA(image.Rect(0, 0, 2, 2))
	copy(tight.Pix, []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
	})
	want := []byte{
		3, 2, 1, 4, 7, 6, 5, 8,
		11, 10, 9, 12, 15, 14, 13, 16,
	}
	dst := make([]byte, 16)
	copyBGRA(dst, tight)
	if string(dst) != string(want) {
		t.Fatalf("tight: %v, want %v", dst, want)
	}

	// A sub-image of a 4-wide image: stride 16 bytes, rows of 8.
	wide := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for i := range wide.Pix {
		wide.Pix[i] = 0xee
	}
	copy(wide.Pix[0:8], tight.Pix[0:8])
	copy(wide.Pix[16:24], tight.Pix[8:16])
	sub := wide.SubImage(image.Rect(0, 0, 2, 2)).(*image.RGBA)
	dst = make([]byte, 16)
	copyBGRA(dst, sub)
	if string(dst) != string(want) {
		t.Fatalf("strided: %v, want %v", dst, want)
	}
}

// TestWaylandAutoSoft pins the Wayland auto-pick table. The display is
// already open, so there is no EGL failure case: a failed display is the
// Wayland setup failing, which falls back to X11 as before.
func TestWaylandAutoSoft(t *testing.T) {
	cases := []struct {
		name    string
		res     probeResult
		haveShm bool
		soft    bool
		logs    string
	}{
		{"hardware", probeResult{renderer: "Mesa Intel(R) UHD Graphics 620 (WHL GT2)"},
			true, false, ""},
		{"llvmpipe", probeResult{renderer: "llvmpipe (LLVM 20.1.2, 256 bits)"},
			true, true, "GOGUI_RENDERER=gl keeps GL"},
		// No wl_shm: soft cannot present, and GL works, if slowly.
		{"llvmpipe, no shm", probeResult{renderer: "llvmpipe (LLVM 20.1.2, 256 bits)"},
			false, false, "wl_shm"},
		{"probe failure", probeResult{probeErr: errors.New("no glGetString")},
			true, false, "using GL"},
	}
	for _, c := range cases {
		soft, why := waylandAutoSoft(c.res, c.haveShm)
		if soft != c.soft {
			t.Errorf("%s: soft = %v, want %v", c.name, soft, c.soft)
		}
		if c.logs == "" && why != "" {
			t.Errorf("%s: logs %q, want nothing", c.name, why)
		}
		if c.logs != "" && !strings.Contains(why, c.logs) {
			t.Errorf("%s: log %q does not contain %q", c.name, why, c.logs)
		}
	}
}
