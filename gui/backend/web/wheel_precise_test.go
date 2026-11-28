package web

import "testing"

func TestIsWebWheelPrecise(t *testing.T) {
	cases := []struct {
		name        string
		deltaMode   int
		dx, dy      float64
		sinceLastMs float64
		want        bool
	}{
		{"discrete_chrome_100px", 0, 0, 100, 200, false},
		{"discrete_windows_120px", 0, 0, -120, 150, false},
		{"discrete_line_mode", 1, 0, 3, 10, false},
		{"discrete_page_mode", 2, 0, 1, 10, false},
		{"discrete_53px_notch", 0, 0, 53, 200, false},
		{"trackpad_small_pixel", 0, 0, 4, 16, true},
		{"trackpad_fractional_pixel", 0, 0, 12.5, 16, true},
		{"trackpad_horizontal_swipe", 0, -8, 0, 16, true},
		{"trackpad_rapid_momentum_stream", 0, 0, 64, 16.6, true},
		{"discrete_fast_spin_100px", 0, 0, 100, 20, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isWebWheelPrecise(tc.deltaMode, tc.dx, tc.dy, tc.sinceLastMs)
			if got != tc.want {
				t.Errorf("isWebWheelPrecise(%d, %v, %v, %v) = %v, want %v",
					tc.deltaMode, tc.dx, tc.dy, tc.sinceLastMs, got, tc.want)
			}
		})
	}
}

func TestNormalizeWebWheel(t *testing.T) {
	lastTS := -1.0
	dx, dy, precise := normalizeWebWheel(0, 0, 17.7, 100, &lastTS)
	if dx != 0 || dy != 1 || !precise {
		t.Fatalf("pixel mode = (%v, %v, %v), want (0, 1, true)", dx, dy, precise)
	}
	dx, dy, precise = normalizeWebWheel(1, 0, 3, 200, &lastTS)
	if dx != 0 || dy != 3 || precise {
		t.Fatalf("line mode = (%v, %v, %v), want (0, 3, false)", dx, dy, precise)
	}
	dx, dy, precise = normalizeWebWheel(2, 0, 1, 300, &lastTS)
	if dx != 0 || dy != 30 || precise {
		t.Fatalf("page mode = (%v, %v, %v), want (0, 30, false)", dx, dy, precise)
	}
}
