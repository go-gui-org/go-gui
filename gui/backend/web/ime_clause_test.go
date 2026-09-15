//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"
	"testing"
	"unicode/utf8"
)

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

func TestIMESanitizeText(t *testing.T) {
	if got := imeSanitizeText("a�b"); got != "ab" {
		t.Errorf("imeSanitizeText stripped to %q, want %q", got, "ab")
	}
	if got := imeSanitizeText(""); got != "" {
		t.Errorf("imeSanitizeText(\"\") = %q, want empty", got)
	}
	if got := imeSanitizeText("�"); got != "" {
		t.Errorf("imeSanitizeText lone failure = %q, want empty", got)
	}
	big := strings.Repeat("あ", maxIMETextRunes+10)
	if got := imeSanitizeText(big); utf8.RuneCountInString(got) != maxIMETextRunes {
		t.Errorf("sanitized runes = %d, want %d",
			utf8.RuneCountInString(got), maxIMETextRunes)
	}
}

// A missing composition data value must not coerce to a literal:
// Value.String on null yields "null", which would arrive as a bogus
// one-word preedit.
func TestJSStringNoCoercion(t *testing.T) {
	cases := []struct {
		name string
		v    js.Value
		want string
	}{
		{"null", js.Null(), ""},
		{"undefined", js.Undefined(), ""},
		{"number", js.ValueOf(42), ""},
		{"string", js.ValueOf("かん"), "かん"},
		{"empty string", js.ValueOf(""), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsString(tc.v); got != tc.want {
				t.Errorf("jsString = %q, want %q", got, tc.want)
			}
		})
	}
}
