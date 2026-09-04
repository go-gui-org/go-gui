package gui

import (
	"math"
	"testing"
)

func TestWindowSizeLimits(t *testing.T) {
	tests := []struct {
		name string
		cfg  WindowCfg
		want SizeLimits
	}{
		{
			name: "unset",
			cfg:  WindowCfg{Width: 800, Height: 600},
			want: SizeLimits{},
		},
		{
			name: "min only",
			cfg:  WindowCfg{MinWidth: 400, MinHeight: 300},
			want: SizeLimits{MinW: 400, MinH: 300},
		},
		{
			name: "max only",
			cfg:  WindowCfg{MaxWidth: 1200, MaxHeight: 900},
			want: SizeLimits{MaxW: 1200, MaxH: 900},
		},
		{
			name: "negative reads as unset",
			cfg:  WindowCfg{MinWidth: -10, MaxHeight: -1, MinHeight: 300},
			want: SizeLimits{MinH: 300},
		},
		{
			// The floor is the stronger promise, so the ceiling moves.
			name: "max below min is raised to min",
			cfg: WindowCfg{
				MinWidth: 500, MaxWidth: 200,
				MinHeight: 400, MaxHeight: 100,
			},
			want: SizeLimits{MinW: 500, MinH: 400, MaxW: 500, MaxH: 400},
		},
		{
			name: "fixed size collapses onto width/height",
			cfg: WindowCfg{
				Width: 640, Height: 480, FixedSize: true,
				MinWidth: 100, MaxWidth: 2000,
			},
			want: SizeLimits{MinW: 640, MinH: 480, MaxW: 640, MaxH: 480},
		},
		{
			// A FixedSize window with no size to pin to must not be
			// pinned to zero; the ordinary rules apply instead.
			name: "fixed size without dimensions falls through",
			cfg:  WindowCfg{FixedSize: true, MinWidth: 300},
			want: SizeLimits{MinW: 300},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := WindowSizeLimits(tc.cfg)
			if got != tc.want {
				t.Errorf("WindowSizeLimits() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestSizeLimitsNone(t *testing.T) {
	if !(SizeLimits{}).None() {
		t.Error("zero SizeLimits should report None")
	}
	if (SizeLimits{MinW: 1}).None() {
		t.Error("a set floor should not report None")
	}
	if (SizeLimits{MaxH: 1}).None() {
		t.Error("a set ceiling should not report None")
	}
}

func TestSizeLimitsScaled(t *testing.T) {
	got := SizeLimits{MinW: 400, MinH: 300, MaxW: 800}.Scaled(2)
	want := SizeLimits{MinW: 800, MinH: 600, MaxW: 1600}
	if got != want {
		t.Errorf("Scaled(2) = %+v, want %+v", got, want)
	}
	// Unset must survive scaling as unset, not become a 0-pixel bound.
	if (SizeLimits{}).Scaled(2) != (SizeLimits{}) {
		t.Error("scaling an unset limit should stay unset")
	}
	// A sub-pixel result must not round a real floor away.
	if got := (SizeLimits{MinW: 1}).Scaled(0.1); got.MinW != 1 {
		t.Errorf("Scaled(0.1).MinW = %d, want 1", got.MinW)
	}
	// Truncated, not rounded: the backends size a new window the same
	// way, so a floor equal to the created size must not land above it
	// and push the window a pixel wider the moment it opens.
	if got := (SizeLimits{MinW: 401}).Scaled(1.5); got.MinW != 601 {
		t.Errorf("Scaled(1.5).MinW = %d, want the truncated 601", got.MinW)
	}
	// Scaling must not carry a bound past what a backend can express:
	// X11 hints are 16-bit and the Win32 track sizes are int32.
	if got := (SizeLimits{MaxW: maxWindowExtent}).Scaled(4); got.MaxW != maxWindowExtent {
		t.Errorf("Scaled(4).MaxW = %d, want the %d cap",
			got.MaxW, maxWindowExtent)
	}
}

// A nonsense-large configured bound is held to the platform ceiling
// rather than wrapping into a negative when a backend narrows it.
func TestWindowSizeLimitsCapsHugeValues(t *testing.T) {
	got := WindowSizeLimits(WindowCfg{
		MinWidth: math.MaxInt, MaxHeight: math.MaxInt,
	})
	if got.MinW != maxWindowExtent || got.MaxH != maxWindowExtent {
		t.Errorf("WindowSizeLimits = %+v, want both axes at %d",
			got, maxWindowExtent)
	}
}
