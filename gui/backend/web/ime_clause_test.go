//go:build js && wasm

package web

import "testing"

func TestUTF16RuneIndex(t *testing.T) {
	cases := []struct {
		name string
		s    string
		n    int
		want int32
	}{
		{"empty", "", 5, 0},
		{"ascii", "hello", 2, 2},
		{"clamped past end", "hi", 99, 2},
		{"negative", "ab", -3, 0},
		{"bmp multibyte", "あい", 1, 1},
		{"after astral pair", "a𝄞b", 3, 2},
		// An offset inside a surrogate pair lands after the
		// astral character, matching the Win32 imeRuneIndex rule.
		{"inside astral pair", "a𝄞b", 2, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := utf16RuneIndex(tc.s, tc.n); got != tc.want {
				t.Errorf("utf16RuneIndex(%q, %d) = %d, want %d",
					tc.s, tc.n, got, tc.want)
			}
		})
	}
}
