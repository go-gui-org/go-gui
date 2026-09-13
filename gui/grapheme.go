package gui

// Grapheme-boundary helpers for the caret-motion fallback path. When no
// glyph layout exists (nil textMeasurer — headless tests, defensive
// fallback), edit state has no shaped cluster information, so boundaries
// are recomputed here with UAX #29 via rivo/uniseg — the same
// segmentation go-glyph uses to build its valid cursor positions. That
// keeps fallback motion and delete agreeing with the shaped path on
// where the caret may rest.

import (
	"slices"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// graphemeStops returns the rune indices at which the caret may rest in
// text — every grapheme-cluster boundary, including 0 and the end. The
// nil-measurer fallback computes this set once per motion or delete;
// the shaped path needs no equivalent because glyph.Layout carries the
// positions already.
func graphemeStops(text string) []int {
	// Fixed guess, not a RuneCountInString pre-scan: that would cost
	// a full pass over text on every fallback keypress. Growth
	// handles long text with amortized copies.
	stops := make([]int, 0, 16)
	stops = append(stops, 0)
	runeIdx := 0
	gr := uniseg.NewGraphemes(text)
	for gr.Next() {
		runeIdx += utf8.RuneCountInString(gr.Str())
		stops = append(stops, runeIdx)
	}
	return stops
}

// prevGraphemeStop returns the largest stop strictly before pos, or 0
// when there is none — the fallback for Backspace and Left-arrow
// granularity. stops must be sorted ascending, as graphemeStops
// returns; an empty slice holds no boundary, so pos is returned
// unchanged (no motion).
func prevGraphemeStop(stops []int, pos int) int {
	if len(stops) == 0 {
		return pos
	}
	i, _ := slices.BinarySearch(stops, pos)
	if i == 0 {
		return 0
	}
	return stops[i-1]
}

// nextGraphemeStop returns the smallest stop strictly after pos, or the
// final stop (end of text) when there is none — the fallback for Delete
// and Right-arrow granularity. stops must be sorted ascending, as
// graphemeStops returns; an empty slice holds no boundary, so pos is
// returned unchanged (no motion).
func nextGraphemeStop(stops []int, pos int) int {
	if len(stops) == 0 {
		return pos
	}
	// BinarySearch finds the first stop >= pos; stops hold unique
	// values, so stepping past an exact hit lands on the first
	// stop strictly after pos.
	i, found := slices.BinarySearch(stops, pos)
	if found {
		i++
	}
	if i >= len(stops) {
		return stops[len(stops)-1]
	}
	return stops[i]
}

// closestGraphemeStop returns the stop nearest pos, preferring the one
// on the left when equidistant — the fallback snap for vertical motion,
// whose rune-column target can land inside a cluster on the target line.
// stops must be sorted ascending, as graphemeStops returns; an empty
// slice holds no boundary, so pos is returned unchanged (no motion).
func closestGraphemeStop(stops []int, pos int) int {
	if len(stops) == 0 {
		return pos
	}
	i, _ := slices.BinarySearch(stops, pos)
	if i == len(stops) {
		return stops[len(stops)-1]
	}
	if i > 0 && pos-stops[i-1] <= stops[i]-pos {
		return stops[i-1]
	}
	return stops[i]
}
