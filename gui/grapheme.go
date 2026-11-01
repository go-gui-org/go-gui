package gui

// Grapheme-boundary helpers for the caret paths that have no glyph
// layout: the nil-measurer fallback (headless tests, defensive
// fallback), a chunk seam of a textBlock, and the cut of a paragraph
// longer than a chunk. Boundaries are recomputed with UAX #29 via
// rivo/uniseg — the same segmentation go-glyph uses to build its valid
// cursor positions — so fallback motion and delete agree with the shaped
// path on where the caret may rest.
//
// Every helper takes and returns byte offsets and segments only a window
// of the text around the offset, so a caret step in a large field costs
// the same as in a short one.

import (
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

// graphemeWindow bounds the bytes a boundary search segments before an
// offset. A cluster longer than it (a run of combining marks) is cut.
const graphemeWindow = 64

// prevGraphemeBoundary returns the last grapheme-cluster boundary at or
// before byte offset i. It segments only a window before i, from a rune
// start, which is exact for every cluster shorter than the window.
func prevGraphemeBoundary(text string, i int) int {
	if i <= 0 {
		return 0
	}
	if i >= len(text) {
		return len(text)
	}
	j := max(0, i-graphemeWindow)
	for j > 0 && !utf8.RuneStart(text[j]) {
		j--
	}
	last := j
	rest := text[j:]
	state := -1
	off := j
	for len(rest) > 0 {
		var tail string
		_, tail, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
		off += len(rest) - len(tail)
		if off > i {
			break
		}
		last = off
		rest = tail
	}
	return last
}

// nextGraphemeBoundary returns the first grapheme-cluster boundary after
// byte offset i, or len(text). i is expected to be a boundary itself, as
// a caret offset is; from inside a cluster the rest of that cluster is
// the answer.
func nextGraphemeBoundary(text string, i int) int {
	if i >= len(text) {
		return len(text)
	}
	if i < 0 {
		i = 0
	}
	cluster, _, _, _ := uniseg.FirstGraphemeClusterInString(text[i:], -1)
	if len(cluster) == 0 {
		return len(text)
	}
	return i + len(cluster)
}

// nearestGraphemeBoundary returns the boundary nearest to byte offset i,
// preferring the one on the left when equidistant — the snap for
// vertical motion, whose rune-column target can land inside a cluster
// on the target line.
func nearestGraphemeBoundary(text string, i int) int {
	i = max(0, min(i, len(text)))
	prev := prevGraphemeBoundary(text, i)
	if prev == i {
		return i
	}
	next := nextGraphemeBoundary(text, prev)
	if i-prev <= next-i {
		return prev
	}
	return next
}
