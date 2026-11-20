//go:build darwin && !ios

package spellcheck

import (
	"math"
	"slices"
	"testing"
)

// Out-of-range Suggest args clamp to the text remainder, the same
// as the nativehost forwarder. Each clamped call must equal the
// call spelled with the in-range form.
func TestSuggestClampsToRemainder(t *testing.T) {
	const text = "helo"
	tests := []struct {
		name             string
		start, length    int
		wantStart, wantL int
	}{
		{"negative start", -1, 3, 0, 3},
		{"huge length", 1, math.MaxInt, 1, 3},
		{"negative length", 1, -5, 1, 3},
		{"zero length", 1, 0, 1, 3},
		{"overlong length", 0, 100, 0, 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Suggest(text, tc.start, tc.length)
			want := Suggest(text, tc.wantStart, tc.wantL)
			if !slices.Equal(got, want) {
				t.Errorf("Suggest(%q, %d, %d) = %q, want clamped %q",
					text, tc.start, tc.length, got, want)
			}
		})
	}
}

func TestSuggestPastEndIsNil(t *testing.T) {
	if got := Suggest("helo", 4, 1); got != nil {
		t.Errorf("Suggest past end = %q, want nil", got)
	}
	if got := Suggest("", 0, 0); got != nil {
		t.Errorf("Suggest empty = %q, want nil", got)
	}
}

func TestCheckEmptyIsNil(t *testing.T) {
	if got := Check(""); got != nil {
		t.Errorf("Check empty = %v, want nil", got)
	}
}

func TestLearnEmptyNoPanic(t *testing.T) {
	Learn("")
}

// Check reports byte ranges, while NSSpellChecker reports UTF-16
// ranges. Words after two-byte, three-byte and four-byte characters
// (a surrogate pair in UTF-16) must still land on the right bytes.
func TestCheckByteOffsetsAfterMultibyte(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"ascii", "the wrold", "wrold"},
		{"two-byte", "café wrold", "wrold"},
		{"three-byte", "日本 wrold", "wrold"},
		{"surrogate pair", "😀 wrold", "wrold"},
		{"bom", "\uFEFFthe wrold", "wrold"},
		{"two boms", "\uFEFF\uFEFFthe wrold", "wrold"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Check(tc.text)
			var words []string
			for _, r := range got {
				words = append(words, tc.text[r.StartByte:r.StartByte+r.LenBytes])
			}
			if !slices.Contains(words, tc.want) {
				t.Errorf("Check(%q) words = %q, want %q among them", tc.text, words, tc.want)
			}
		})
	}
}

func TestCheckInvalidUTF8IsNil(t *testing.T) {
	if got := Check("wrold \xff"); got != nil {
		t.Errorf("Check invalid UTF-8 = %v, want nil", got)
	}
}

func TestSuggestReturnsGuesses(t *testing.T) {
	const text = "😀 wrold"
	got := Suggest(text, len("😀 "), len("wrold"))
	if !slices.Contains(got, "world") {
		t.Errorf("Suggest(%q) = %q, want it to hold %q", text, got, "world")
	}
}

// A span that starts inside a character names no word.
func TestSuggestSplitCharacterIsNil(t *testing.T) {
	if got := Suggest("é wrold", 1, 3); got != nil {
		t.Errorf("Suggest split character = %q, want nil", got)
	}
}

// After a BOM, Suggest must look at the word the span names, not the
// one 1 UTF-16 unit later.
func TestSuggestAfterBOM(t *testing.T) {
	for _, prefix := range []string{"\uFEFF", "\uFEFF\uFEFF"} {
		text := prefix + "wrold"
		got := Suggest(text, len(prefix), len("wrold"))
		if !slices.Contains(got, "world") {
			t.Errorf("Suggest(%q) = %q, want it to hold %q", text, got, "world")
		}
	}
}
