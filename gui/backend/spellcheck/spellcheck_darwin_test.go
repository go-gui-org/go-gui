//go:build darwin && !ios && cgo

package spellcheck

import (
	"math"
	"slices"
	"testing"
)

// Out-of-range Suggest args clamp to the text remainder, the same
// as the nativehost forwarder. Each clamped call must equal the
// call spelled with the in-range form.
func TestSuggestClampsToRemainder(t *testing.T) {
	const text = "helo"
	tests := []struct {
		name             string
		start, length    int
		wantStart, wantL int
	}{
		{"negative start", -1, 3, 0, 3},
		{"huge length", 1, math.MaxInt, 1, 3},
		{"negative length", 1, -5, 1, 3},
		{"zero length", 1, 0, 1, 3},
		{"overlong length", 0, 100, 0, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Suggest(text, tc.start, tc.length)
			want := Suggest(text, tc.wantStart, tc.wantL)
			if !slices.Equal(got, want) {
				t.Errorf("Suggest(%q, %d, %d) = %q, want clamped %q",
					text, tc.start, tc.length, got, want)
			}
		})
	}
}

func TestSuggestPastEndIsNil(t *testing.T) {
	if got := Suggest("helo", 4, 1); got != nil {
		t.Errorf("Suggest past end = %q, want nil", got)
	}
	if got := Suggest("", 0, 0); got != nil {
		t.Errorf("Suggest empty = %q, want nil", got)
	}
}

func TestCheckEmptyIsNil(t *testing.T) {
	if got := Check(""); got != nil {
		t.Errorf("Check empty = %v, want nil", got)
	}
}

func TestLearnEmptyNoPanic(t *testing.T) {
	Learn("")
}
