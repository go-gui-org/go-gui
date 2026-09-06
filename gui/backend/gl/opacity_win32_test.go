//go:build windows && !js

package gl

import (
	"math"
	"testing"
)

// The guard is the whole Windows story for #516: a Transparent window
// already composites through DwmEnableBlurBehindWindow, and adding
// WS_EX_LAYERED on top of it is untested against the GL swap chain.
func TestLayeredOpacityAllowed(t *testing.T) {
	if !layeredOpacityAllowed(false) {
		t.Fatal("an ordinary window must accept the fade")
	}
	if layeredOpacityAllowed(true) {
		t.Fatal("a Transparent window must refuse the fade")
	}
}

// SetLayeredWindowAttributes takes a 0-255 alpha, so the ends must be
// exact: 255 is opaque and anything less is composited.
func TestOpacityAlphaByte(t *testing.T) {
	tests := []struct {
		name    string
		opacity float32
		want    byte
	}{
		{"invisible", 0, 0},
		{"opaque", 1, 255},
		{"below zero clamps", -1, 0},
		{"above one clamps", 2, 255},
		{"NaN stays opaque", float32(math.NaN()), 255},
		{"half", 0.5, 128},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := opacityAlphaByte(tc.opacity); got != tc.want {
				t.Fatalf("opacityAlphaByte(%v) = %d, want %d",
					tc.opacity, got, tc.want)
			}
		})
	}
}

// GWL_EXSTYLE is -20 as a signed word; the call takes a uintptr, so the
// two's-complement spelling is pinned here rather than trusted.
func TestWin32OpacityConstants(t *testing.T) {
	if gwlExStyle != ^uintptr(0)-19 {
		t.Fatalf("gwlExStyle = %#x, want -20 as a uintptr", gwlExStyle)
	}
	if wsExLayered != 0x00080000 {
		t.Fatalf("wsExLayered = %#x, want 0x00080000", wsExLayered)
	}
	if lwaAlpha != 0x2 {
		t.Fatalf("lwaAlpha = %#x, want 0x2", lwaAlpha)
	}
}
