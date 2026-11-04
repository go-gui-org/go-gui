package gui

import (
	"slices"
	"strings"
	"testing"
)

// graphemeBoundaries walks text with nextGraphemeBoundary from 0 and
// returns every boundary, 0 and the end included.
func graphemeBoundaries(text string) []int {
	out := []int{0}
	for i := 0; i < len(text); {
		i = nextGraphemeBoundary(text, i)
		out = append(out, i)
	}
	return out
}

// graphemeCases are byte offsets of the cluster boundaries of each text.
var graphemeCases = []struct {
	name string
	text string
	want []int
}{
	{"ascii", "ab", []int{0, 1, 2}},
	{"empty", "", []int{0}},
	// Skin-tone modifier joins its base emoji.
	{"skin tone", "\U0001F44D\U0001F3FD", []int{0, 8}},
	// Four emoji joined by three ZWJ form one cluster.
	{"zwj family",
		"\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466",
		[]int{0, 25}},
	// Combining mark stays with its base.
	{"combining", "áb", []int{0, 3, 4}},
	// CR LF is one cluster (UAX #29 GB3) — the multiline case.
	{"crlf", "a\r\nb", []int{0, 1, 3, 4}},
	// Regional indicators pair into one flag cluster.
	{"flag", "\U0001F1FA\U0001F1F8", []int{0, 8}},
	// An invalid byte is a cluster of its own.
	{"invalid byte", "a\xffb", []int{0, 1, 2, 3}},
	{"mixed", "ab́c\U0001F1E9\U0001F1EAd", []int{0, 1, 4, 5, 13, 14}},
}

func TestNextGraphemeBoundary(t *testing.T) {
	for _, c := range graphemeCases {
		t.Run(c.name, func(t *testing.T) {
			if got := graphemeBoundaries(c.text); !slices.Equal(got, c.want) {
				t.Fatalf("boundaries(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

// prevGraphemeBoundary answers for every offset, a caret stop or not:
// the last boundary at or before it.
func TestPrevGraphemeBoundary(t *testing.T) {
	for _, c := range graphemeCases {
		t.Run(c.name, func(t *testing.T) {
			for i := -1; i <= len(c.text)+1; i++ {
				want := 0
				for _, s := range c.want {
					if s <= i {
						want = s
					}
				}
				if got := prevGraphemeBoundary(c.text, i); got != want {
					t.Errorf("prevGraphemeBoundary(%q, %d) = %d, want %d",
						c.text, i, got, want)
				}
			}
		})
	}
}

func TestNearestGraphemeBoundary(t *testing.T) {
	cases := []struct {
		text string
		pos  int
		want int
	}{
		// "a" + U+0301 is one cluster of 3 bytes, then "b".
		{"áb", 1, 0}, // nearer the cluster start
		{"áb", 2, 3}, // nearer its end
		{"áb", 3, 3}, // on a boundary: itself
		{"áb", 9, 4}, // past the end: clamp to the end
		{"áb", -2, 0},
		// An 8-byte flag, queried at its middle: prefer the left.
		{"\U0001F1FA\U0001F1F8", 4, 0},
	}
	for _, c := range cases {
		if got := nearestGraphemeBoundary(c.text, c.pos); got != c.want {
			t.Errorf("nearestGraphemeBoundary(%q, %d) = %d, want %d",
				c.text, c.pos, got, c.want)
		}
	}
}

// A flag is a pair of regional indicators, and the pair a letter belongs
// to depends on every letter before it, however far back the run starts.
// The answer must match a walk from the start of the text at any length.
func TestPrevGraphemeBoundaryFlagRun(t *testing.T) {
	const letter = "\U0001F1E9"
	for _, prefix := range []string{"", "x", letter} {
		for n := 1; n <= 40; n++ {
			text := prefix + strings.Repeat(letter+"\U0001F1EA", n) + "y"
			stops := graphemeBoundaries(text)
			for i := 0; i <= len(text); i++ {
				want := 0
				for _, s := range stops {
					if s <= i {
						want = s
					}
				}
				if got := prevGraphemeBoundary(text, i); got != want {
					t.Fatalf("%q + %d flags, offset %d: got %d, want %d",
						prefix, n, i, got, want)
				}
			}
		}
	}
}

// Backspace on the string path, which a password field and a chunk seam
// take, removes a whole flag at the end of a run longer than the window.
func TestEditDeleteRemovesWholeFlagInLongRun(t *testing.T) {
	text := strings.Repeat("\U0001F1E9\U0001F1EA", 20)
	got, _, ok := editDelete(text, inputState{CursorPos: len(text)}, false)
	if !ok || got != text[:len(text)-8] {
		t.Fatalf("backspace left %d of %d bytes, want %d",
			len(got), len(text), len(text)-8)
	}
}

// A run of combining marks longer than the search window is cut at the
// window: the caret may then rest inside the run, but the search never
// scans text before the window, so a step costs the same in any field.
func TestPrevGraphemeBoundaryWindow(t *testing.T) {
	text := "xa"
	var textSb103 strings.Builder
	for range graphemeWindow {
		textSb103.WriteString("́")
	}
	text += textSb103.String()
	got := prevGraphemeBoundary(text, len(text)-1)
	if got <= 1 || got > len(text) {
		t.Fatalf("prevGraphemeBoundary in a long mark run = %d, want a cut "+
			"inside the run", got)
	}
}
