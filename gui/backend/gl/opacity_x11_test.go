//go:build linux && !js && !android

package gl

import (
	"math"
	"testing"
)

// The property is a single 32-bit CARDINAL spanning the whole range, so
// the ends have to be exact: a compositor treats 0xFFFFFFFF as "leave
// it alone" and anything less as a real fade.
func TestOpacityCardinal(t *testing.T) {
	tests := []struct {
		name    string
		opacity float32
		want    uint32
	}{
		{"invisible", 0, 0},
		{"opaque", 1, 0xFFFFFFFF},
		{"below zero clamps", -1, 0},
		{"above one clamps", 2, 0xFFFFFFFF},
		{"NaN stays opaque", float32(math.NaN()), 0xFFFFFFFF},
		{"half", 0.5, 0x80000000},
		{"quarter", 0.25, 0x40000000},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := opacityCardinal(tc.opacity); got != tc.want {
				t.Fatalf("opacityCardinal(%v) = %#08x, want %#08x",
					tc.opacity, got, tc.want)
			}
		})
	}
}

// The encoding has to be monotonic across the slider range, or a fade
// animation would step backwards somewhere in the middle.
func TestOpacityCardinalMonotonic(t *testing.T) {
	prev := opacityCardinal(0)
	for i := 1; i <= 100; i++ {
		got := opacityCardinal(float32(i) / 100)
		if got <= prev {
			t.Fatalf("opacityCardinal(%v) = %#08x, not above %#08x",
				float32(i)/100, got, prev)
		}
		prev = got
	}
}
