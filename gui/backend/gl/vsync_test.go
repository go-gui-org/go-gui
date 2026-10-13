package gl

import (
	"slices"
	"testing"
)

func TestApplyVSync(t *testing.T) {
	tests := []struct {
		name       string
		vsyncOff   bool
		accept     bool
		wantSet    []int32
		wantReport []string
	}{
		{"default sets interval 1", false, true, []int32{1}, nil},
		// The animation loop ticks on presented frames (#1018), so an
		// unpaced present is worth a report even when nobody asked for it.
		{"refused 1 is reported", false, false, []int32{1},
			[]string{"eglSwapInterval(1) was refused by the driver; " +
				"frames are not paced to the display"}},
		{"off sets interval 0", true, true, []int32{0}, nil},
		{"refused 0 is reported", true, false, []int32{0},
			[]string{"VSyncOff requested but eglSwapInterval(0) " +
				"was refused by the driver"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var set []int32
			var reported []string
			applyVSync(tt.vsyncOff,
				func(n int32) bool {
					set = append(set, n)
					return tt.accept
				},
				func(reason string) { reported = append(reported, reason) },
				"eglSwapInterval")
			if !slices.Equal(set, tt.wantSet) {
				t.Fatalf("intervals set = %v, want %v", set, tt.wantSet)
			}
			if !slices.Equal(reported, tt.wantReport) {
				t.Fatalf("reported = %q, want %q", reported, tt.wantReport)
			}
		})
	}
}
