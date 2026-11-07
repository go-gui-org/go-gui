package gui

import (
	"math"
	"testing"
)

// With no report, the safe area is a set zero Padding: used as a Cfg's
// Padding on desktop it pads nothing, rather than reading as unset and
// falling back to the theme's padding.
func TestSafeAreaPaddingDefaultsToSetZero(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	got := w.SafeAreaPadding()
	if got != PaddingNone || !got.IsSet() {
		t.Errorf("SafeAreaPadding() = %+v, want PaddingNone", got)
	}
}

func TestDispatchSafeAreaReportsEachEdge(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	DispatchSafeArea(w, 59, 62, 34, 20)
	if got, want := w.SafeAreaPadding(), NewPadding(59, 62, 34, 20); got != want {
		t.Errorf("SafeAreaPadding() = %+v, want %+v", got, want)
	}
}

// The keyboard covers the home bar: the bottom edge gives way to it, so
// an app padding by both counts the overlap once.
func TestSafeAreaBottomYieldsToSoftKeyboard(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		keyboard   float32
		wantBottom float32
	}{
		{"keyboard down", 0, 34},
		{"keyboard lower than the home bar", 20, 14},
		{"keyboard covers the home bar", 300, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := NewTestWindow(t, WindowCfg{})
			DispatchSafeArea(w, 59, 0, 34, 0)
			w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: tt.keyboard})
			got := w.SafeAreaPadding()
			if got.Bottom != tt.wantBottom {
				t.Errorf("Bottom = %v, want %v", got.Bottom, tt.wantBottom)
			}
			if got.Top != 59 {
				t.Errorf("Top = %v, want 59: only the bottom edge yields", got.Top)
			}
		})
	}
}

func TestDispatchSafeAreaClampsBadEdges(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	DispatchSafeArea(w, -5, nan, inf, 12)
	if got, want := w.SafeAreaPadding(), NewPadding(0, 0, 0, 12); got != want {
		t.Errorf("SafeAreaPadding() = %+v, want %+v", got, want)
	}
}

// A change marks one refresh and wakes the loop, so a view padding by
// the safe area redraws; a repeat of the same values does neither, and
// neither does a zero report on a window that never had one.
func TestDispatchSafeAreaRefreshesOnChange(t *testing.T) {
	t.Parallel()
	w := NewTestWindow(t, WindowCfg{})
	wakes := 0
	w.SetWakeMainFn(func() { wakes++ })

	w.refresh.Store(0)
	DispatchSafeArea(w, 0, 0, 0, 0)
	if w.refresh.Load() != 0 || wakes != 0 {
		t.Errorf("zero on a fresh window: refresh %#x, wakes %d", w.refresh.Load(), wakes)
	}
	DispatchSafeArea(w, 59, 0, 34, 0)
	if refreshReason(w.refresh.Load())&refreshSafeArea == 0 {
		t.Errorf("change marked refresh %#x", w.refresh.Load())
	}
	if wakes != 1 {
		t.Errorf("change woke main %d times, want 1", wakes)
	}
	w.refresh.Store(0)
	DispatchSafeArea(w, 59, 0, 34, 0)
	if w.refresh.Load() != 0 || wakes != 1 {
		t.Errorf("repeat: refresh %#x, wakes %d", w.refresh.Load(), wakes)
	}
	// Back to zero is a change too: the notch side rotated away.
	DispatchSafeArea(w, 0, 0, 0, 0)
	if wakes != 2 || w.SafeAreaPadding() != PaddingNone {
		t.Errorf("back to zero: wakes %d, padding %+v", wakes, w.SafeAreaPadding())
	}
}

func TestDispatchSafeAreaNilWindow(t *testing.T) {
	t.Parallel()
	DispatchSafeArea(nil, 1, 2, 3, 4) // must not panic
}
