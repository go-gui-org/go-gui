//go:build darwin && arm64 && !ios

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

// IME offsets are UTF-16 units from AppKit and code points in the event.
// A surrogate pair counts once, and a boundary inside a pair rounds to
// its end (issue #169).
func TestUTF16UnitsToCodePoints(t *testing.T) {
	for _, tc := range []struct {
		s    string
		unit uint
		want uint
	}{
		{"あい", 0, 0},
		{"あい", 1, 1},
		{"あい", 2, 2},
		{"あい", 99, 2}, // past the end clamps
		{"あ😀", 1, 1},  // before the pair
		{"あ😀", 2, 2},  // inside the pair rounds to its end
		{"あ😀", 3, 2},  // after the pair
		{"𠮟", 2, 1},   // one code point, two units
		{"", 5, 0},
	} {
		if got := utf16UnitsToCodePoints(tc.s, tc.unit); got != tc.want {
			t.Errorf("utf16UnitsToCodePoints(%q, %d) = %d, want %d", tc.s, tc.unit, got, tc.want)
		}
	}
}
