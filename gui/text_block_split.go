package gui

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-glyph"
)

// textProbeMeasurer is an optional lean-layout capability used while finding
// wrapped chunk seams. A probe only reads line boxes, so the go-glyph-backed
// measurers can omit character rectangles and cursor/word maps that would be
// immediately discarded. Custom measurers need not implement it.
type textProbeMeasurer interface {
	LayoutTextNoHitTesting(
		text string, style TextStyle, wrapWidth float32,
	) (glyph.Layout, error)
}

// splitOpen cuts open chunk i within the budget and inserts the rest of
// its paragraph after it as the next piece.
//
// In wrap mode the seam is a line break the whole paragraph also has. A
// budget-sized prefix is shaped and every complete line except its final one
// is retained; the final line may end only because the probe did. The byte
// between pieces, when there is one, is the space the wrapper consumed; it
// belongs to no chunk. In no-wrap mode pieces share a row, so the cut keeps
// every byte and falls after a space when one is near.
func (b *textBlock) splitOpen(i int, w *Window) {
	c := &b.chunks[i]
	c.open = false
	limit := c.start + textChunkBytes
	if c.end <= limit {
		return
	}
	pieceEnd, rest := 0, 0
	cutLimit := limit
	if b.wraps() {
		cutLimit = c.start + max(textChunkBytes/2, 1)
		probeEnd := c.end
		if limit < c.end {
			probeEnd = prevGraphemeBoundary(b.text, limit)
		}
		probe := w.textMeasurer.LayoutText
		if lean, ok := w.textMeasurer.(textProbeMeasurer); ok {
			probe = lean.LayoutTextNoHitTesting
		}
		l, err := probe(b.text[c.start:probeEnd], b.style, b.widthArg)
		// A visual line can itself exceed the normal probe budget. Retry once
		// with the old two-budget ceiling before falling back to a hard cut;
		// ordinary wrapped text never pays for the larger probe.
		if err == nil && len(l.Lines) < 2 && probeEnd < c.end {
			probeEnd = c.end
			if limit+textChunkBytes < c.end {
				probeEnd = prevGraphemeBoundary(
					b.text, limit+textChunkBytes)
			}
			l, err = probe(b.text[c.start:probeEnd], b.style, b.widthArg)
		}
		if err == nil {
			// Every line before the last ended because the wrapper chose a
			// real break, not because the probe ran out of input.
			pick := len(l.Lines) - 1
			if pick > 0 {
				prev := l.Lines[pick-1]
				pieceEnd = c.start + prev.StartIndex + prev.Length
				rest = c.start + l.Lines[pick].StartIndex
				b.adoptProbeGeometry(c, l.Lines[:pick])
			}
		}
	}
	if rest <= c.start || rest >= c.end {
		// No break (a run with no opportunity, or a shaper refusal): a
		// hard cut at a grapheme boundary.
		cut := textSplitPoint(b.text, c.start, c.end, cutLimit)
		pieceEnd, rest = cut, cut
	}
	end := c.end
	c.end = pieceEnd
	next := textChunk{
		start: rest, end: end, piece: true,
		open: end-rest > textChunkBytes,
	}
	b.estimate(&next, w)
	b.chunks = slices.Insert(b.chunks, i+1, next)
	if !b.chunks[i].measured {
		b.estimate(&b.chunks[i], w)
	}
	b.markDirty(i)
}

// adoptProbeGeometry records the exact geometry of the complete lines before
// a seam. The probe has no retained layout: an on-screen piece is still
// shaped with hit-test data when drawn or queried. A far-caret walk, however,
// can place every preceding piece exactly without shaping the same prefix a
// second time.
func (b *textBlock) adoptProbeGeometry(c *textChunk, lines []glyph.Line) {
	if len(lines) == 0 {
		return
	}
	var width float32
	for _, line := range lines {
		width = f32Max(width, line.Rect.Width)
	}
	last := lines[len(lines)-1].Rect
	c.width = width
	c.height = last.Y + last.Height
	c.lastLineH = b.lineH
	switch {
	case last.Height > 0 && f32IsFinite(last.Height):
		c.lastLineH = last.Height
	case c.height > 0 && f32IsFinite(c.height):
		c.lastLineH = c.height / float32(len(lines))
	}
	c.measured = true
}

func isBlankByte(c byte) bool { return c == ' ' || c == '\t' }

// textSplitPoint picks where to cut text[start:end] so the first part
// ends by limit: after the last space in the half budget before limit,
// else at the grapheme boundary at or before limit.
func textSplitPoint(text string, start, end, limit int) int {
	lo := max(start+1, limit-textChunkBytes/2)
	for j := limit; j > lo; j-- {
		if isBlankByte(text[j-1]) {
			return j
		}
	}
	if g := prevGraphemeBoundary(text, limit); g > start {
		return g
	}
	for j := limit; j > start; j-- {
		if utf8.RuneStart(text[j]) {
			return j
		}
	}
	return end
}

// appendTextChunks appends the chunks covering text[from:to] to dst. to
// is len(text) or the index of a '\n' separator, so the region holds
// whole paragraphs. Consecutive paragraphs pack into one chunk while it
// stays within the budget, empty ones between them included; an empty
// paragraph that would end a chunk is its own chunk instead, so a chunk
// never ends on an empty line; a paragraph over the budget is one open
// chunk, cut when shaped. piece marks the first chunk as the
// continuation of an earlier paragraph.
func appendTextChunks(
	dst []textChunk, text string, from, to int, piece bool,
) []textChunk {
	ps := from
	cur := -1 // index in dst of the chunk being packed
	for {
		pe := to
		if k := strings.IndexByte(text[ps:to], '\n'); k >= 0 {
			pe = ps + k
		}
		switch {
		case pe == ps && cur >= 0 && blankRunFits(text, ps, to, dst[cur].start):
			// Blank lines between paragraphs of one chunk: take them
			// and the paragraph after them. Double-spaced text would
			// otherwise be a chunk per line, too many to stay shaped.
			pe = nextParagraphEnd(text, ps, to)
			dst[cur].end = pe
		case pe == ps:
			dst = append(dst, textChunk{start: ps, end: pe})
			cur = -1
		case pe-ps > textChunkBytes:
			dst = append(dst, textChunk{start: ps, end: pe, open: true})
			cur = -1
		case cur >= 0 && pe-dst[cur].start <= textChunkBytes:
			dst[cur].end = pe
		default:
			dst = append(dst, textChunk{start: ps, end: pe})
			cur = len(dst) - 1
		}
		if piece {
			dst[len(dst)-1].piece = true
			piece = false
		}
		if pe >= to {
			break
		}
		ps = pe + 1
	}
	return dst
}

// blankRunFits reports whether the blank lines starting at ps, an empty
// paragraph, end in a non-empty paragraph within [ps, to] that the chunk
// starting at start can take within the budget.
func blankRunFits(text string, ps, to, start int) bool {
	if ps >= to {
		return false // the region ends on the empty paragraph
	}
	j := ps
	for j < to && text[j] == '\n' {
		j++
	}
	if j >= to {
		return false // only empty paragraphs before the region's end
	}
	return nextParagraphEnd(text, ps, to)-start <= textChunkBytes
}

// nextParagraphEnd returns the end of the first non-empty paragraph at
// or after ps, the start of a run of empty ones, within [ps, to].
func nextParagraphEnd(text string, ps, to int) int {
	j := ps
	for j < to && text[j] == '\n' {
		j++
	}
	if k := strings.IndexByte(text[j:to], '\n'); k >= 0 {
		return j + k
	}
	return to
}
