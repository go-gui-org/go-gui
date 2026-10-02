package gl

import (
	"slices"
	"testing"
)

func TestApplyVSyncOff(t *testing.T) {
	tests := []struct {
		name       string
		vsyncOff   bool
		accept     bool
		wantSet    []int32
		wantReport []string
	}{
		// The default leaves the interval 1 from context creation alone.
		{"default keeps vsync", false, true, nil, nil},
		{"off sets interval 0", true, true, []int32{0}, nil},
		{"refused is reported", true, false, []int32{0},
			[]string{"eglSwapInterval(0) was refused by the driver"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var set []int32
			var reported []string
			applyVSyncOff(tt.vsyncOff,
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
