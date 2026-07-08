//go:build windows && !js

package gl

import (
	"testing"

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
