package gui

import "testing"

// resetMemImages clears the process-global registry so tests do not
// observe each other's entries or budget changes.
func resetMemImages(t *testing.T) {
	t.Helper()
	memImages.mu.Lock()
	memImages.data = nil
	memImages.order.Init()
	memImages.bytes = 0
	memImages.budget = defaultMemImageBudget
	memImages.mu.Unlock()
}

// pixels returns a w*h NRGBA8 buffer filled with fill.
func pixels(w, h int, fill byte) []byte {
	p := make([]byte, w*h*4)
	for i := range p {
		p[i] = fill
	}
	return p
}

func TestUseImageRoundTrip(t *testing.T) {
	resetMemImages(t)
	src := UseImage("round/trip", 2, 3, pixels(2, 3, 0x7f))
	if src != "mem:round/trip" {
		t.Fatalf("src = %q, want mem:round/trip", src)
	}
	w, h, pix, ok := LookupImage(src)
	if !ok {
		t.Fatal("LookupImage: not found")
	}
	if w != 2 || h != 3 {
		t.Errorf("dims = %dx%d, want 2x3", w, h)
	}
	if len(pix) != 2*3*4 || pix[0] != 0x7f {
		t.Errorf("pix len=%d [0]=%#x", len(pix), pix[0])
	}
	if !HasImage("round/trip") {
		t.Error("HasImage = false for a registered key")
	}
}

// LookupImage must reject anything that is not a mem: source so
// backends can call it first and fall through to path/URL handling.
func TestLookupImageRejectsNonMemSources(t *testing.T) {
	resetMemImages(t)
	UseImage("real", 1, 1, pixels(1, 1, 0xff))
	for _, src := range []string{
		"", "real", "/tmp/real.png", "https://example.com/a.png",
		"data:image/png;base64,AAAA", "mem:missing",
	} {
		if _, _, _, ok := LookupImage(src); ok {
			t.Errorf("LookupImage(%q) = ok, want not ok", src)
		}
	}
}

// A malformed buffer must register nothing rather than hand a
// backend a slice it will read past the end of.
func TestUseImageRejectsBadBuffers(t *testing.T) {
	resetMemImages(t)
	cases := []struct {
		name string
		key  string
		w, h int
		pix  []byte
	}{
		{"empty key", "", 1, 1, pixels(1, 1, 0)},
		{"zero width", "k", 0, 1, nil},
		{"negative height", "k", 1, -1, nil},
		{"short buffer", "k", 4, 4, pixels(2, 2, 0)},
		{"long buffer", "k", 2, 2, pixels(4, 4, 0)},
		{"oversized dimension", "k", maxMemImageDim + 1, 1, pixels(1, 1, 0)},
		{"wrapping dimension", "k", 1 << 30, 1 << 30, pixels(1, 1, 0)},
	}
	for _, tc := range cases {
		if got := UseImage(tc.key, tc.w, tc.h, tc.pix); got != "" {
			t.Errorf("%s: UseImage = %q, want \"\"", tc.name, got)
		}
	}
	if HasImage("k") {
		t.Error("a rejected buffer was registered")
	}
}

// Re-registering a key at the same size must not swap the stored
// buffer: backends cache an uploaded texture under that key and would
// never observe the change. Callers content-key instead.
func TestUseImageSameKeyKeepsFirstBuffer(t *testing.T) {
	resetMemImages(t)
	UseImage("stable", 1, 1, pixels(1, 1, 0x11))
	UseImage("stable", 1, 1, pixels(1, 1, 0x22))
	_, _, pix, ok := LookupImage("mem:stable")
	if !ok {
		t.Fatal("LookupImage: not found")
	}
	if pix[0] != 0x11 {
		t.Errorf("pix[0] = %#x, want 0x11 (first buffer)", pix[0])
	}
}

// A key re-registered at a different size is a different image, so
// the new buffer must replace the old one.
func TestUseImageSizeChangeReplaces(t *testing.T) {
	resetMemImages(t)
	UseImage("resize", 1, 1, pixels(1, 1, 0x11))
	UseImage("resize", 2, 2, pixels(2, 2, 0x22))
	w, h, pix, ok := LookupImage("mem:resize")
	if !ok {
		t.Fatal("LookupImage: not found")
	}
	if w != 2 || h != 2 || pix[0] != 0x22 {
		t.Errorf("dims = %dx%d pix[0] = %#x, want 2x2 0x22",
			w, h, pix[0])
	}
}

func TestDropImage(t *testing.T) {
	resetMemImages(t)
	UseImage("drop/me", 1, 1, pixels(1, 1, 0))
	DropImage("drop/me")
	if HasImage("drop/me") {
		t.Error("HasImage = true after DropImage")
	}
	if _, _, _, ok := LookupImage("mem:drop/me"); ok {
		t.Error("LookupImage found a dropped key")
	}
}

// The budget, not an explicit Drop, is what stops a content-keyed
// drag from growing memory without limit.
func TestMemImageBudgetEvictsLeastRecentlyUsed(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)

	const px = 16 // 2x2 NRGBA
	SetMemImageBudget(2 * px)

	UseImage("a", 2, 2, pixels(2, 2, 0xaa))
	UseImage("b", 2, 2, pixels(2, 2, 0xbb))
	// Touch "a" so "b" becomes the least recently used.
	if !HasImage("a") {
		t.Fatal("a evicted early")
	}
	UseImage("c", 2, 2, pixels(2, 2, 0xcc))

	if HasImage("b") {
		t.Error("b survived; want the least-recently-used evicted")
	}
	if !HasImage("a") || !HasImage("c") {
		t.Error("a and c should both still be registered")
	}
	if _, _, _, ok := LookupImage("mem:b"); ok {
		t.Error("LookupImage still resolves an evicted key")
	}
}

// A single buffer larger than the whole budget is still the one the
// caller is about to draw, so it must survive its own registration.
func TestMemImageOversizedEntrySurvives(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)

	SetMemImageBudget(4)
	UseImage("huge", 8, 8, pixels(8, 8, 0xff))
	if !HasImage("huge") {
		t.Error("an over-budget entry evicted itself")
	}
}

// The HasImage gate is the reason a repeat frame is cheap: it must
// not allocate.
func TestHasImageDoesNotAllocate(t *testing.T) {
	resetMemImages(t)
	UseImage("noalloc", 4, 4, pixels(4, 4, 0))
	got := testing.AllocsPerRun(100, func() {
		if !HasImage("noalloc") {
			t.Fatal("key missing")
		}
	})
	if got != 0 {
		t.Errorf("HasImage allocs = %v, want 0", got)
	}
}

func TestIsMemImage(t *testing.T) {
	if !isMemImage("mem:x") {
		t.Error("mem:x not recognized")
	}
	for _, s := range []string{"", "memx", "/mem:x", "data:mem:"} {
		if isMemImage(s) {
			t.Errorf("isMemImage(%q) = true", s)
		}
	}
}
