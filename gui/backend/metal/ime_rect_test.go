//go:build darwin && cgo && !ios

package metal

import "testing"

// The framework reports the IME caret rect with a top-left origin, but
// AppKit's content view has a bottom-left origin. Without the flip the
// candidate window is mirrored top to bottom: a caret on the first line
// of a terminal puts the candidate list at the bottom of the window.
func TestIMERectViewY_FlipsToBottomLeft(t *testing.T) {
	cases := []struct {
		name              string
		y, h, viewH, want float32
	}{
		{"caret on top row", 0, 20, 600, 580},
		{"caret on bottom row", 580, 20, 600, 0},
		{"caret mid view", 100, 16, 600, 484},
		{"zero height view", 10, 20, 0, -30},
	}
	for _, tc := range cases {
		if got := imeRectViewY(tc.y, tc.h, tc.viewH); got != tc.want {
			t.Errorf("%s: imeRectViewY(%v, %v, %v) = %v, want %v",
				tc.name, tc.y, tc.h, tc.viewH, got, tc.want)
		}
	}
}
