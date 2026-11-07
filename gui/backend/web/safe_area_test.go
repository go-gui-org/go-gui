//go:build js && wasm

package web

import "testing"

func TestCSSPixels(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want float32
	}{
		{"47px", 47},
		{"0px", 0},
		{"20.5px", 20.5},
		{"", 0},
		{"auto", 0},
	}
	for _, tt := range tests {
		if got := cssPixels(tt.in); got != tt.want {
			t.Errorf("cssPixels(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
