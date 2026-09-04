//go:build windows && !js

package gl

import (
	"testing"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// newCharTestBackend returns a Backend wired to a minimal headless
// window so charInput's b.emit path can run without a real GL context.
// charInput stores each emitted event in b.plat.evt (the reused event),
// so tests observe that field to distinguish emit from suppress.
func newCharTestBackend(t *testing.T) *Backend {
	t.Helper()
	w := gui.NewWindow(gui.WindowCfg{State: new(int), Width: 100, Height: 100})
	w.UpdateView(func(_ *gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{})
	})
	w.FrameFn()
	b := &Backend{}
	b.plat.w = w
	return b
}

func TestCharInputSurrogatePairEmitsRune(t *testing.T) {
	b := newCharTestBackend(t)

	// U+1F600 GRINNING FACE = high 0xD83D, low 0xDE00.
	b.plat.evt = gui.Event{Type: gui.EventInvalid}
	b.charInput(0xD83D)
	if b.plat.evt.Type != gui.EventInvalid {
		t.Fatalf("high surrogate emitted an event: %v", b.plat.evt.Type)
	}
	if b.plat.highSurr != 0xD83D {
		t.Fatalf("high surrogate not stored: got %#x", b.plat.highSurr)
	}

	b.charInput(0xDE00)
	if b.plat.evt.Type != gui.EventChar {
		t.Fatalf("low surrogate did not emit EventChar: %v", b.plat.evt.Type)
	}
	if b.plat.evt.CharCode != 0x1F600 {
		t.Errorf("CharCode = %#x, want 0x1F600", b.plat.evt.CharCode)
	}
	if b.plat.highSurr != 0 {
		t.Errorf("highSurr not cleared after pair: %#x", b.plat.highSurr)
	}
}

func TestCharInputEmitsBMPRune(t *testing.T) {
	b := newCharTestBackend(t)
	b.plat.evt = gui.Event{Type: gui.EventInvalid}
	b.charInput('A')
	if b.plat.evt.Type != gui.EventChar || b.plat.evt.CharCode != 'A' {
		t.Fatalf("charInput('A') = {%v, %#x}, want {EventChar, 0x41}",
			b.plat.evt.Type, b.plat.evt.CharCode)
	}
}

func TestCharInputSuppressed(t *testing.T) {
	cases := []struct {
		name   string
		wparam uintptr
	}{
		{"lone low surrogate", 0xDE00},
		{"null", 0x00},
		{"backspace", 0x08},
		{"unit separator", 0x1F},
		{"delete", 0x7F},
		{"replacement char", 0xFFFD},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := newCharTestBackend(t)
			b.plat.evt = gui.Event{Type: gui.EventInvalid}
			b.charInput(tc.wparam)
			if b.plat.evt.Type != gui.EventInvalid {
				t.Errorf("charInput(%#x) emitted %v, want no event",
					tc.wparam, b.plat.evt.Type)
			}
		})
	}
}

func TestLoHiWordSSignedExtraction(t *testing.T) {
	// Low/high words are signed 16-bit: mouse coords can be negative
	// when the pointer is captured outside the window. 0xFFFD = -3,
	// 0xFFF9 = -7 as int16.
	lparam := uintptr(0xFFF9FFFD)
	if got := loWordS(lparam); got != -3 {
		t.Errorf("loWordS = %d, want -3", got)
	}
	if got := hiWordS(lparam); got != -7 {
		t.Errorf("hiWordS = %d, want -7", got)
	}

	lparam = uintptr(uint32(1200) | uint32(800)<<16)
	if got := loWordS(lparam); got != 1200 {
		t.Errorf("loWordS = %d, want 1200", got)
	}
	if got := hiWordS(lparam); got != 800 {
		t.Errorf("hiWordS = %d, want 800", got)
	}
}

// TestNotchesToLines_HonoursSystemSetting pins the wheel-speed fix: a
// WM_MOUSEWHEEL delta must be converted to gui.Event's line unit using
// SPI_GETWHEELSCROLLLINES, not emitted as a bare notch count. Reporting
// 1.0 per notch is what made the terminal grid crawl a quarter of a row
// per notch on Windows while macOS moved 2.5x further for the same
// gesture.
func TestNotchesToLines_HonoursSystemSetting(t *testing.T) {
	lines := wheelScrollLines()
	if lines == wheelPageScroll {
		t.Skip("system set to page-scroll; per-line math not exercised")
	}
	if lines == 0 {
		t.Fatal("wheelScrollLines returned 0; fallback did not apply")
	}
	// One notch up.
	if got, want := notchesToLines(wheelDelta), float32(lines); got != want {
		t.Errorf("one notch = %v lines, want %v", got, want)
	}
	// Direction is preserved.
	if got, want := notchesToLines(-wheelDelta), -float32(lines); got != want {
		t.Errorf("one notch down = %v lines, want %v", got, want)
	}
	// A partial (high-resolution) notch scales proportionally rather than
	// rounding away, so precision wheels stay smooth.
	if got, want := notchesToLines(wheelDelta/2), float32(lines)/2; got != want {
		t.Errorf("half notch = %v lines, want %v", got, want)
	}
}

// TestSysParamUint_FallsBackOnBogusAction verifies the fallback path: an
// unknown SystemParametersInfo action must yield the documented default
// rather than zero, which would make the wheel completely dead.
func TestSysParamUint_FallsBackOnBogusAction(t *testing.T) {
	if got := sysParamUint(0xFFFF, defaultScrollLines); got != defaultScrollLines {
		t.Errorf("fallback = %d, want %d", got, defaultScrollLines)
	}
}

// WM_DPICHANGED carries the rect Windows wants the window moved to. The
// decode must survive negative screen coordinates, which is what a
// monitor placed left of or above the primary one produces.
func TestDPIChangedBounds(t *testing.T) {
	rc := rectW{left: -1920, top: -200, right: -640, bottom: 520}

	x, y, cx, cy, ok := dpiChangedBounds(uintptr(unsafe.Pointer(&rc)))

	if !ok {
		t.Fatal("ok = false, want true for a non-nil rect")
	}
	if x != -1920 || y != -200 {
		t.Errorf("position = (%d,%d), want (-1920,-200)", x, y)
	}
	if cx != 1280 || cy != 720 {
		t.Errorf("size = (%d,%d), want (1280,720)", cx, cy)
	}
}

// A null lParam must be reported, not dereferenced.
func TestDPIChangedBoundsNil(t *testing.T) {
	if _, _, _, _, ok := dpiChangedBounds(0); ok {
		t.Error("ok = true for a nil lParam, want false")
	}
}

func TestDPIChangedBoundsDegenerate(t *testing.T) {
	cases := []struct {
		name string
		rc   rectW
	}{
		{"zero width", rectW{left: 0, top: 0, right: 0, bottom: 100}},
		{"zero height", rectW{left: 0, top: 0, right: 100, bottom: 0}},
		{"negative width", rectW{left: 100, top: 0, right: 0, bottom: 100}},
		{"negative height", rectW{left: 0, top: 100, right: 100, bottom: 0}},
		{"too large", rectW{left: 0, top: 0, right: 20000, bottom: 20000}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, _, _, ok := dpiChangedBounds(uintptr(unsafe.Pointer(&tc.rc))); ok {
				t.Errorf("dpiChangedBounds(%v) = ok true, want false", tc.rc)
			}
		})
	}
}
