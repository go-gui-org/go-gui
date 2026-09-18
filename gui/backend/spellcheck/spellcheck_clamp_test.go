package spellcheck

import (
	"math"
	"testing"
)

// The clamp runs on every platform, so its table test has no build
// tag. Backend tests pin the same behavior end to end through
// Suggest where the platform allows it.
func TestClampRange(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		start      int
		length     int
		wantStart  int
		wantLength int
		wantOK     bool
	}{
		{"in range", "helo", 1, 3, 1, 3, true},
		{"negative start", "helo", -1, 3, 0, 3, true},
		{"huge length", "helo", 1, math.MaxInt, 1, 3, true},
		{"negative length", "helo", 1, -5, 1, 3, true},
		{"zero length", "helo", 1, 0, 1, 3, true},
		{"overlong length", "helo", 0, 100, 0, 4, true},
		{"empty text", "", 0, 0, 0, 0, false},
		{"start at end", "helo", 4, 1, 0, 0, false},
		{"start past end", "helo", 9, 1, 0, 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			start, length, ok := clampRange(tc.text, tc.start, tc.length)
			if start != tc.wantStart || length != tc.wantLength || ok != tc.wantOK {
				t.Errorf("clampRange(%q, %d, %d) = (%d, %d, %v), want (%d, %d, %v)",
					tc.text, tc.start, tc.length, start, length, ok,
					tc.wantStart, tc.wantLength, tc.wantOK)
			}
		})
	}
}
