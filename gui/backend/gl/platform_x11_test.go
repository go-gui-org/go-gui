//go:build linux && !js && !android

package gl

import (
	"math"
	"testing"
)

// The physical window side must stay inside what CreateWindow takes. A
// plain uint16 conversion wraps 9000 x 8 = 72000 to 6464, and 65536 to
// 0 (#984).
func TestX11ExtentClamps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		logical int32
		scale   float32
		want    int32
	}{
		{"one", 800, 1, 800},
		{"two", 800, 2, 1600},
		{"fraction truncates", 101, 1.5, 151},
		{"past uint16 clamps", 9000, 8, x11MaxExtent},
		{"exact wrap to zero clamps", 8192, 8, x11MaxExtent},
		{"past int32 clamps", math.MaxInt32, 8, x11MaxExtent},
		{"tiny raises to one", 1, 0.25, 1},
		{"zero raises to one", 0, 2, 1},
		{"NaN raises to one", 800, float32(math.NaN()), 1},
	}
	for _, tt := range tests {
		if got := x11Extent(tt.logical, tt.scale); got != tt.want {
			t.Errorf("%s: x11Extent(%d, %v) = %d, want %d",
				tt.name, tt.logical, tt.scale, got, tt.want)
		}
	}
}
