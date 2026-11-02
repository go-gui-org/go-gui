package gui

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/go-gui-org/go-glyph"
)

// Chunked text layout (issues #1187, #1190).
//
// go-glyph shapes one paragraph-sized layout at a time. A layout holds a
// char rect and an attribute entry per byte, so its cost grows with the
// text it covers (a 10 KB paragraph shapes in ~5 ms and ~7 MB on a
// desktop), and LayoutText refuses anything over glyph.MaxTextLength
// (10,240 bytes). A text shape used to shape its whole text as one
// layout on every rebuild: past the budget it drew nothing, and below
// it every keystroke re-shaped the whole field.
//
// A textBlock cuts the text into chunks — whole paragraphs packed up to
// textChunkBytes, or pieces of a longer paragraph — shapes each chunk
// when something asks for it, keeps a bounded number of shaped chunks,
// and answers the layout queries the text widgets use (text_block_query.go)
// in whole-text byte offsets and block coordinates. A block persists
// across frames under the widget's identity (nsTextBlock), so an edit
// re-shapes the chunk it touched and a rebuild re-shapes nothing.
//
// Chunk seams. A chunk ends before a '\n' it does not hold (the next
// chunk starts after it), at the end of the text, or — for a piece of a
// paragraph longer than the budget — at a piece seam. In wrap mode a
// piece seam is reflowed: the budgeted text is shaped once to find its
// last line break, the piece ends where the line before that break
// ends, and the next piece begins where the broken line began. The next
// piece so starts where a whole-paragraph layout would also have started
// a line, and no short line appears at the seam. The space the wrapper
// consumed at the break belongs to no chunk, as a '\n' does; the caret
// offset after it is the next piece's start, which is where go-glyph
// puts a caret at a soft wrap. Greedy wrapping is prefix-stable, so the
// lines before the seam are the lines the whole paragraph would have
// had. In no-wrap mode the pieces of a paragraph sit side by side on one
// row. An empty paragraph is its own chunk with no layout and one
// synthetic line box: go-glyph gives a text ending in '\n' an empty last
// line, and so does the block.

// textChunkBytes bounds the text one glyph layout is asked to shape. It
// is a var so tests can shrink it and exercise multi-chunk blocks on
// short texts against a whole-text layout of the same shaper.
var textChunkBytes = 2048

// textBlockEagerBytes is the text size up to which a new block shapes
// every chunk once when built, so its extent is exact from the first
// frame. Above it the unseen chunks are estimated and measured as they
// come into view, with the scroll view anchored meanwhile
// (text_block_anchor.go). A var so tests can force estimates.
var textBlockEagerBytes = 64 << 10

const (
	// textBlockLiveLayouts bounds the shaped chunks a block keeps. The
	// chunks beyond it keep their measured geometry and are shaped again
	// when a query or the renderer reaches them.
	textBlockLiveLayouts = 48
	// textBlockSample is the prose the estimate's average advance is
	// measured on.
	textBlockSample = "the quick brown fox jumps over the lazy dog 0123456789"
)

// textChunk is one shaped unit of a textBlock: text[start:end] of the
// block's text, placed at (x, y) in block coordinates.
type textChunk struct {
	layout *glyph.Layout
	start  int
	end    int
	x, y   float32
	width  float32
	height float32
	// lastLineH is the last line box's height, for the trailing-leading
	// trim in boxHeight.
	lastLineH float32
	// used is the block tick of the last query that touched the chunk;
	// the least recently used shaped chunk is evicted first.
	used uint32
	// measured reports that width and height come from a layout (or
	// from the empty-paragraph rule), not from the estimate.
	measured bool
	// piece marks a chunk that continues the previous chunk's paragraph
	// with no '\n' between them.
	piece bool
	// open marks a piece whose end is provisional: it covers the rest of
	// a paragraph longer than the budget and is cut when first shaped.
	open bool
	// failed marks a chunk the shaper refused (bad UTF-8, a NUL byte).
	// It keeps its estimate and draws nothing, like the whole text did.
	failed bool
}

func (c *textChunk) needsLayout() bool {
	return c.layout == nil && !c.failed && c.start < c.end
}

// textBlock is the chunked layout of one text at one width and style.
type textBlock struct {
	// w is the window whose measurer shapes the chunks. A block lives in
	// that window's state, so the queries need no window handed in.
	w      *Window
	style  TextStyle
	text   string
	chunks []textChunk
	// widthArg has plainTextLayoutWidthArg's meaning: > 0 wraps at that
	// width, < 0 aligns within -widthArg without wrapping, 0 is natural.
	widthArg float32
	width    float32
	height   float32
	// lineH is one line box of the style's face, for empty chunks and
	// estimates; fontH is the face height the box trim compares against.
	lineH   float32
	fontH   float32
	spacing float32
	// avgAdv is the estimated advance per byte for unshaped chunks.
	avgAdv float32
	live   int
	tick   uint32
	// dirty is the first chunk whose position settle must recompute, or
	// -1 when every position is current.
	dirty int
	mode  TextMode
	// probed reports that lineH and avgAdv were measured. They are read
	// only for an empty paragraph or an estimate, so a short text that
	// shapes whole never pays for them.
	probed bool
	// failErr is the shaper's last refusal of a chunk, with that chunk's
	// size, for the debug finding; nil when every chunk shaped.
	failErr   error
	failBytes int
}

func (b *textBlock) wraps() bool { return b.widthArg > 0 }

// newTextBlock builds the block for text. w may have no measurer, in
// which case every chunk keeps its estimate.
func newTextBlock(
	text string, widthArg float32, style TextStyle, mode TextMode, w *Window,
) *textBlock {
	b := &textBlock{dirty: -1, w: w}
	b.reset(text, widthArg, style, mode, w)
	return b
}

// newTextBlockFromLayout wraps a layout the caller shaped — a rich
// text's — as a one-chunk block, so the shared caret and key paths can
// query it. The block never re-shapes it: the chunk is live and the
// live cap is never reached.
func newTextBlockFromLayout(
	text string, l *glyph.Layout, widthArg float32, style TextStyle,
	mode TextMode, w *Window,
) *textBlock {
	b := &textBlock{
		w: w, text: text, widthArg: widthArg, style: style, mode: mode,
		dirty: -1, fontH: fontHeight(style, w), spacing: style.LineSpacing,
	}
	b.chunks = []textChunk{{start: 0, end: len(text)}}
	b.adopt(0, *l)
	b.settle()
	return b
}

// sameShaping reports whether the block was shaped for these inputs.
// Paint colours do not shape, so a recolour keeps every chunk.
func (b *textBlock) sameShaping(
	widthArg float32, style TextStyle, mode TextMode,
) bool {
	return f32AreClose(b.widthArg, widthArg) &&
		textLayoutStyleEqual(b.style, style) && b.mode == mode
}

// update brings the block to text at these shaping inputs: a no-op for
// the same text, a localized re-chunk for an edited one, and a rebuild
// when the width, mode or shaping style changed.
func (b *textBlock) update(
	text string, widthArg float32, style TextStyle, mode TextMode, w *Window,
) {
	if !b.sameShaping(widthArg, style, mode) {
		b.reset(text, widthArg, style, mode, w)
		return
	}
	if text == b.text {
		return
	}
	b.patch(text, w)
}

func (b *textBlock) reset(
	text string, widthArg float32, style TextStyle, mode TextMode, w *Window,
) {
	b.text, b.widthArg, b.style, b.mode = text, widthArg, style, mode
	b.w = w
	b.live = 0
	b.probed = false
	b.fontH = fontHeight(style, w)
	b.spacing = style.LineSpacing
	// The slice is reused: clear it to its capacity first, or the slots
	// past the new length keep the old chunks' layouts alive.
	clear(b.chunks[:cap(b.chunks)])
	b.chunks = appendTextChunks(b.chunks[:0], text, 0, len(text), false)
	b.dirty = 0
	if len(text) <= textBlockEagerBytes {
		b.measureAll(w)
	}
	for i := range b.chunks {
		if !b.chunks[i].measured {
			b.estimate(&b.chunks[i], w)
		}
	}
	b.settle()
}

// probeMetrics reads the face metrics the estimates and the empty
// chunks use: one line box and an average advance. Called once per
// block, from the first estimate.
func (b *textBlock) probeMetrics(w *Window) {
	b.probed = true
	b.lineH = b.fontH
	b.avgAdv = b.style.Size * 0.5
	if w == nil || w.textMeasurer == nil {
		return
	}
	if l, err := w.textMeasurer.LayoutText("x", b.style, 0); err == nil &&
		l.Height > 0 && f32IsFinite(l.Height) {
		b.lineH = l.Height
	}
	if wd := w.textMeasurer.TextWidth(textBlockSample, b.style); wd > 0 &&
		f32IsFinite(wd) {
		b.avgAdv = wd / float32(len(textBlockSample))
	}
}

// appendTextChunks appends the chunks covering text[from:to] to dst. to
// is len(text) or the index of a '\n' separator, so the region holds
// whole paragraphs. Consecutive non-empty paragraphs pack into one chunk
// while it stays within the budget; an empty paragraph is its own chunk;
// a paragraph over the budget is one open chunk, cut when shaped. piece
// marks the first chunk as the continuation of an earlier paragraph.
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

// estimate sizes an unshaped chunk from the average advance: one line
// per paragraph, or as many as its width needs at the wrap width. An
// empty paragraph's one line box is final, so it counts as measured.
func (b *textBlock) estimate(c *textChunk, w *Window) {
	if !b.probed {
		b.probeMetrics(w)
	}
	if c.start == c.end {
		c.measured = true
	}
	lines := 0
	var width float32
	seg := b.text[c.start:c.end]
	for {
		para, rest, more := strings.Cut(seg, "\n")
		n := 1
		pw := float32(len(para)) * b.avgAdv
		if b.wraps() {
			n = int(pw / b.widthArg)
			if pw > float32(n)*b.widthArg {
				n++
			}
			n = max(n, 1)
			pw = f32Min(pw, b.widthArg)
		}
		lines += n
		width = f32Max(width, pw)
		if !more {
			break
		}
		seg = rest
	}
	c.width = width
	c.height = float32(lines)*b.lineH + float32(lines-1)*b.spacing
	c.lastLineH = b.lineH
}

// measureAll shapes every chunk once for its geometry. The layouts stay
// live up to the cap, so the first frame of a text within the eager
// budget draws from them without shaping again; past the cap the least
// recently shaped go, and a chunk the renderer or a query needs is
// shaped again on demand.
func (b *textBlock) measureAll(w *Window) {
	if w == nil || w.textMeasurer == nil {
		return
	}
	for i := 0; i < len(b.chunks); i++ {
		if b.chunks[i].needsLayout() {
			b.ensure(i, w)
		}
	}
}

// ensure shapes chunk i when it has no layout and returns it. An open
// chunk is cut first, which inserts the rest of its paragraph as the
// next chunk; the returned pointer is valid until the chunks change.
func (b *textBlock) ensure(i int, w *Window) *textChunk {
	b.tick++
	c := &b.chunks[i]
	c.used = b.tick
	if !c.needsLayout() || w == nil || w.textMeasurer == nil {
		return c
	}
	if c.open {
		b.splitOpen(i, w)
		c = &b.chunks[i]
	}
	// The layout keeps the text it shaped (glyph.Layout.Text). A slice
	// of b.text would keep all of b.text alive, and a chunk an edit
	// does not touch keeps its layout across edits: each would pin the
	// generation of the text it was shaped in. A copy pins one chunk.
	l, err := w.textMeasurer.LayoutText(
		strings.Clone(b.text[c.start:c.end]), b.style, b.widthArg)
	if err != nil {
		// Shown as nothing, sized by the estimate: the same outcome the
		// whole text had, now confined to one chunk. plainTextBlock
		// reports it (DebugGlyphLayoutFallback).
		c.failed = true
		c.measured = true
		b.failErr = err
		b.failBytes = c.end - c.start
		return c
	}
	b.adopt(i, l)
	return &b.chunks[i]
}

// adopt stores a shaped layout for chunk i and takes its geometry.
func (b *textBlock) adopt(i int, l glyph.Layout) {
	c := &b.chunks[i]
	lp := new(glyph.Layout)
	*lp = l
	c.layout = lp
	c.width = l.Width
	c.height = l.Height
	c.lastLineH = b.lineH
	if n := len(l.Lines); n > 0 {
		last := l.Lines[n-1].Rect
		if c.height <= 0 && last.Y+last.Height > 0 {
			// A host-built layout may carry lines but no height.
			c.height = last.Y + last.Height
		}
		// Read the last line's own box, not an average: glyph adds
		// LineSpacing after every line but the last, and a fallback face
		// can make one line taller than the rest. A layout without line
		// rects falls back to the average, exact when the lines are even.
		switch {
		case last.Height > 0 && f32IsFinite(last.Height):
			c.lastLineH = last.Height
		case c.height > 0 && f32IsFinite(c.height):
			c.lastLineH = c.height / float32(n)
		}
	}
	c.measured = true
	b.live++
	b.markDirty(i)
	b.evict(i)
}

// evict drops the least recently used shaped chunks until the live
// count is within the cap, never the chunk at keep.
func (b *textBlock) evict(keep int) {
	for b.live > textBlockLiveLayouts {
		victim := -1
		oldest := ^uint32(0)
		for j := range b.chunks {
			c := &b.chunks[j]
			if j != keep && c.layout != nil && c.used < oldest {
				oldest = c.used
				victim = j
			}
		}
		if victim < 0 {
			return
		}
		b.chunks[victim].layout = nil
		b.live--
	}
}

// splitOpen cuts open chunk i within the budget and inserts the rest of
// its paragraph after it as the next piece.
//
// In wrap mode the seam is a line break the whole paragraph also has:
// the text up to a budget past the limit is shaped once, and the last
// line start within the limit becomes the next piece's start, with the
// piece ending where the line before it ends. The byte between, when
// there is one, is the space the wrapper consumed; it belongs to no
// chunk. Shaping past the limit is what makes the break real — a
// break chosen at the end of a probe is where the probe ran out, not
// where the paragraph wraps. In no-wrap mode the pieces share a row, so
// the cut keeps every byte and falls after a space when one is near.
func (b *textBlock) splitOpen(i int, w *Window) {
	c := &b.chunks[i]
	c.open = false
	limit := c.start + textChunkBytes
	if c.end <= limit {
		return
	}
	pieceEnd, rest := 0, 0
	if b.wraps() {
		probeEnd := c.end
		if limit+textChunkBytes < c.end {
			probeEnd = prevGraphemeBoundary(b.text, limit+textChunkBytes)
		}
		l, err := w.textMeasurer.LayoutText(
			b.text[c.start:probeEnd], b.style, b.widthArg)
		if err == nil {
			// The last line start within the limit; failing that, the
			// first one past it, which keeps a line longer than the
			// budget whole as long as the probe reaches its end. Both
			// stay within two budgets, well under the shaper's cap.
			pick := -1
			for k := 1; k < len(l.Lines); k++ {
				if c.start+l.Lines[k].StartIndex <= limit || pick < 0 {
					pick = k
				}
			}
			if pick > 0 {
				prev := l.Lines[pick-1]
				pieceEnd = c.start + prev.StartIndex + prev.Length
				rest = c.start + l.Lines[pick].StartIndex
			}
		}
	}
	if rest <= c.start || rest >= c.end {
		// No break (a run with no opportunity, or a shaper refusal): a
		// hard cut at a grapheme boundary.
		cut := textSplitPoint(b.text, c.start, c.end, limit)
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
	b.estimate(&b.chunks[i], w)
	b.markDirty(i)
}

func isBlankByte(c byte) bool { return c == ' ' || c == '\t' }

// textSplitPoint picks where to cut text[start:end] so the first part
// ends by limit: after the last space in the budget's second half, else
// at the grapheme boundary at or before limit.
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

// patch re-chunks the paragraphs an edit touched and shifts the rest.
// The edit is located as the bytes outside the common prefix and suffix
// of the old and new text, so a programmatic replacement is handled the
// same way as a keystroke. Chunks before the edit keep their layouts;
// so do the chunks after it, moved by the length difference. Within a
// long paragraph the pieces before the edited one keep theirs too,
// because a piece's lines depend only on the text before its seam; the
// edited piece and every later piece of that paragraph merge into one
// open chunk and are cut again when shaped.
//
// The re-chunked chunks that hold text before the edit are shaped at
// once rather than estimated. That text was on screen: an estimate there
// would be measured by the frame that draws it, after the caret's scroll
// position was chosen, and move the caret and every line below it by the
// estimate's error. They are one or two chunks, however large the edit.
// Inserted text after them stays estimated and is measured as it comes
// into view, with the scroll view anchored (text_block_anchor.go), so a
// large paste does not shape all of itself in the keystroke.
func (b *textBlock) patch(text string, w *Window) {
	old := b.text
	p := commonPrefixLen(old, text)
	s := commonSuffixLen(old, text, p)
	oldEnd := len(old) - s
	a := b.chunkAt(p)
	// In wrap mode an edit in a piece can change where the piece before
	// it breaks: its last line ended because the first word of this
	// piece did not fit. Re-derive that seam as well. A no-wrap cut
	// depends on its own bytes alone.
	if a > 0 && b.chunks[a].piece && b.wraps() {
		a--
	}
	z := b.chunkAt(oldEnd)
	for z+1 < len(b.chunks) && b.chunks[z+1].piece {
		z++
	}
	if z < a {
		z = a
	}
	regionStart := b.chunks[a].start
	piece := b.chunks[a].piece
	delta := len(text) - len(old)
	regionEnd := b.chunks[z].end + delta
	for i := a; i <= z; i++ {
		if b.chunks[i].layout != nil {
			b.live--
		}
	}
	for i := z + 1; i < len(b.chunks); i++ {
		b.chunks[i].start += delta
		b.chunks[i].end += delta
	}
	b.text = text
	var fresh []textChunk
	if piece {
		// The region starts inside a paragraph: its remainder is pieces
		// of that paragraph, and only the paragraphs after it pack.
		paraEnd := regionEnd
		if k := strings.IndexByte(text[regionStart:regionEnd], '\n'); k >= 0 {
			paraEnd = regionStart + k
		}
		fresh = appendTextChunks(fresh, text, regionStart, paraEnd, true)
		if paraEnd < regionEnd {
			fresh = appendTextChunks(fresh, text, paraEnd+1, regionEnd, false)
		}
	} else {
		fresh = appendTextChunks(fresh, text, regionStart, regionEnd, false)
	}
	for i := range fresh {
		b.estimate(&fresh[i], w)
	}
	b.chunks = slices.Replace(b.chunks, a, z+1, fresh...)
	b.markDirty(a)
	// Shaping an open chunk cuts it and inserts its remainder after it,
	// so the bound is an offset: the chunk holding the edit's start.
	for i := a; i < len(b.chunks) && b.chunks[i].start <= p; i++ {
		b.ensure(i, w)
	}
	b.settle()
}

// ensureRange shapes every chunk that reaches into [y0, y1], in block
// coordinates. Shaping replaces an estimated height with the measured
// one and moves every chunk below, so the range is walked again until a
// pass shapes nothing.
func (b *textBlock) ensureRange(y0, y1 float32) {
	for range 4 {
		b.settle()
		shaped := false
		for i := 0; i < len(b.chunks); i++ {
			c := &b.chunks[i]
			if c.y > y1 {
				break
			}
			if c.y+c.height < y0 || !c.needsLayout() {
				continue
			}
			b.ensure(i, b.w)
			shaped = true
		}
		if !shaped {
			break
		}
	}
	b.settle()
}

// commonPrefixLen returns the length of the longest common prefix.
func commonPrefixLen(a, b string) int {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

// commonSuffixLen returns the length of the longest common suffix that
// does not overlap the first p bytes of either string.
func commonSuffixLen(a, b string, p int) int {
	n := min(len(a), len(b)) - p
	i := 0
	for i < n && a[len(a)-1-i] == b[len(b)-1-i] {
		i++
	}
	return i
}

func (b *textBlock) markDirty(i int) {
	if b.dirty < 0 || i < b.dirty {
		b.dirty = i
	}
}

// settle recomputes chunk positions from the first dirty chunk and the
// block's extents. A piece of a no-wrap paragraph continues its row;
// every other chunk starts a new row below the previous one.
func (b *textBlock) settle() {
	if b.dirty < 0 {
		return
	}
	from := b.dirty
	b.dirty = -1
	for i := from; i < len(b.chunks); i++ {
		c := &b.chunks[i]
		if i == 0 {
			c.x, c.y = 0, 0
			continue
		}
		p := &b.chunks[i-1]
		if c.piece && !b.wraps() {
			c.x, c.y = p.x+p.width, p.y
		} else {
			c.x, c.y = 0, p.y+p.height+b.spacing
		}
	}
	b.width, b.height = 0, 0
	for i := range b.chunks {
		c := &b.chunks[i]
		b.height = f32Max(b.height, c.y+c.height)
		b.width = f32Max(b.width, c.x+c.width)
	}
}

// chunkAt returns the index of the chunk owning byte offset idx: the
// last chunk that starts at or before it. A chunk's end offset belongs
// to it, so a caret before a '\n' or a consumed seam space sits at the
// end of the line before it; the offset after the seam byte is the next
// chunk's start.
func (b *textBlock) chunkAt(idx int) int {
	n := len(b.chunks)
	i := sort.Search(n, func(i int) bool { return b.chunks[i].start > idx })
	return max(i-1, 0)
}

// seamBefore reports whether chunk i continues the previous chunk with
// no byte between them, so a word can straddle the seam.
func (b *textBlock) seamBefore(i int) bool {
	return i > 0 && b.chunks[i].piece &&
		b.chunks[i].start == b.chunks[i-1].end
}

// rowBounds returns the first and last chunk of the row holding chunk
// i. Only the pieces of a no-wrap paragraph share a row; every other
// chunk is a row of its own.
func (b *textBlock) rowBounds(i int) (first, last int) {
	first, last = i, i
	if b.wraps() {
		return first, last
	}
	for first > 0 && b.chunks[first].piece {
		first--
	}
	for last+1 < len(b.chunks) && b.chunks[last+1].piece {
		last++
	}
	return first, last
}

// boxHeight is the height go-gui sizes the shape with: the block's
// height less the leading below the last baseline.
//
// glyph reports a layout's Height as n whole line boxes, and a line box
// is the baseline-to-baseline advance: ascent + descent + leading,
// floored at 1.15em. Rendering puts the first baseline at FontAscent
// below the shape top and advances by that same line height, so the
// leading below the *last* baseline is space nothing ever paints into.
// Kept in the shape, it is padding only at the bottom: a centred or
// padded row then reads visibly top-biased. Dropping it makes a one-line
// shape exactly FontHeight, the box the single-line path assigns, so the
// two paths agree. Inter-line spacing is untouched: only the tail goes.
// A face whose line box is tighter than ascent+descent keeps glyph's
// number: growing the shape would be a regression.
func (b *textBlock) boxHeight() float32 {
	b.settle()
	n := len(b.chunks)
	if n == 0 || b.height <= 0 || !f32IsFinite(b.height) {
		return b.height
	}
	lastH := b.chunks[n-1].lastLineH
	if !f32IsFinite(b.fontH) || lastH <= 0 || !f32IsFinite(lastH) ||
		b.fontH >= lastH {
		return b.height
	}
	return b.height - (lastH - b.fontH)
}

// adoptWidth records that the shape's width became w without changing a
// line: shrinkWrapToInk pulls a wrapped box back to its longest line,
// and re-wrapping at that width yields the same breaks, so the shaped
// chunks stay valid for the next frame's lookup at the new width.
func (b *textBlock) adoptWidth(w float32) {
	if b.wraps() && w > 0 {
		b.widthArg = w
	}
}

// single returns the one layout of a block that fits in one chunk, for
// the paths that still take a glyph.Layout (content transitions).
func (b *textBlock) single() (glyph.Layout, bool) {
	if len(b.chunks) != 1 {
		return glyph.Layout{}, false
	}
	c := b.ensure(0, b.w)
	if c.layout == nil {
		return glyph.Layout{}, false
	}
	return *c.layout, true
}

// textBlockAnonKey keys a block for a text shape with no identity by
// the text's backing memory, so a large label an app keeps across
// frames keeps its layout too. The hit is content-checked by update, so
// a different text behind the same pointer only costs a re-chunk.
func textBlockAnonKey(text string) string {
	return "~text:" + strconv.Itoa(len(text)) + ":" +
		strconv.FormatUint(uint64(uintptr(unsafe.Pointer(unsafe.StringData(text)))), 16)
}
