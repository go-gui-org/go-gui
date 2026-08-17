package gui

import (
	"slices"
	"testing"
)

func TestGraphemeStops(t *testing.T) {
	cases := []struct {
		name string
		text string
		want []int
	}{
		{"ascii", "ab", []int{0, 1, 2}},
		{"empty", "", []int{0}},
		// Skin-tone modifier joins its base emoji.
		{"skin tone", "\U0001F44D\U0001F3FD", []int{0, 2}},
		// Four emoji joined by three ZWJ form one cluster.
		{"zwj family",
			"\U0001F468\u200D\U0001F469\u200D\U0001F467\u200D\U0001F466",
			[]int{0, 7}},
		// Combining mark stays with its base.
		{"combining", "a\u0301b", []int{0, 2, 3}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := graphemeStops(c.text)
			if !slices.Equal(got, c.want) {
				t.Fatalf("graphemeStops(%q) = %v, want %v", c.text, got, c.want)
			}
		})
	}
}

func TestPrevNextGraphemeStop(t *testing.T) {
	stops := []int{0, 1, 8}
	cases := []struct {
		pos  int
		prev int
		next int
	}{
		{0, 0, 1}, // at start: prev clamps to 0
		{1, 0, 8}, // on a boundary
		{5, 1, 8}, // mid-cluster: cluster start and end
		{8, 1, 8}, // at end: next stays
	}
	for _, c := range cases {
		if got := prevGraphemeStop(stops, c.pos); got != c.prev {
			t.Errorf("prevGraphemeStop(stops, %d) = %d, want %d", c.pos, got, c.prev)
		}
		if got := nextGraphemeStop(stops, c.pos); got != c.next {
			t.Errorf("nextGraphemeStop(stops, %d) = %d, want %d", c.pos, got, c.next)
		}
	}
}

func TestClosestGraphemeStop(t *testing.T) {
	stops := []int{0, 1, 8}
	cases := []struct {
		pos  int
		want int
	}{
		{4, 1},  // nearer to 1 than 8
		{5, 8},  // nearer to 8 than 1
		{10, 8}, // past the end: clamp to last stop
		{1, 1},  // on a boundary: itself
	}
	for _, c := range cases {
		if got := closestGraphemeStop(stops, c.pos); got != c.want {
			t.Errorf("closestGraphemeStop(stops, %d) = %d, want %d", c.pos, got, c.want)
		}
	}

	// Equidistant: prefer the left stop.
	tie := []int{0, 2, 6}
	if got := closestGraphemeStop(tie, 4); got != 2 {
		t.Errorf("closestGraphemeStop(tie, 4) = %d, want 2", got)
	}
}
