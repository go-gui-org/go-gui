package gui

import (
	"sort"
	"unicode"
	"unicode/utf8"

	"github.com/go-gui-org/go-glyph"
)

// Layout queries over a textBlock, in whole-text byte offsets and block
// coordinates. Each mirrors the glyph.Layout method the text widgets
// used on a whole-text layout, delegates to the chunk owning the offset
// or point — shaping it first when needed, with the block's window —
// and crosses chunk seams where the single layout had no seam to cross.

// cursorPos returns the caret geometry at byte offset idx. Unlike
// glyph.Layout.GetCursorPos it answers for every offset in [0, len]:
// the end of a chunk is the end of its last line.
func (b *textBlock) cursorPos(idx int) (glyph.CursorPosition, bool) {
	if len(b.chunks) == 0 || idx < 0 || idx > len(b.text) {
		return glyph.CursorPosition{}, false
	}
	i := b.shapedChunkAt(idx)
	b.settle()
	c := &b.chunks[i]
	cp := b.chunkCursorPos(c, idx-c.start)
	cp.X += c.x
	cp.Y += c.y
	return cp, true
}

// chunkCursorPos is cursorPos within one chunk, in chunk coordinates.
func (b *textBlock) chunkCursorPos(c *textChunk, local int) glyph.CursorPosition {
	if c.layout == nil {
		// An empty paragraph, or a chunk the shaper refused or has not
		// shaped: the caret sits at the chunk's left edge.
		return glyph.CursorPosition{Height: c.lastLineH}
	}
	l := c.layout
	if cp, ok := l.GetCursorPos(local); ok {
		return cp
	}
	n := len(l.Lines)
	if n == 0 {
		return glyph.CursorPosition{Height: c.lastLineH}
	}
	last := l.Lines[n-1]
	return glyph.CursorPosition{
		X: last.Rect.X + last.Rect.Width, Y: last.Rect.Y, Height: last.Rect.Height,
	}
}

// cursorPosTrailing is cursorPos with the caret at a soft-wrap boundary
// drawn at the end of the earlier line when trailing is set (the End
// key's position; see inputState.cursorTrailing).
func (b *textBlock) cursorPosTrailing(
	idx int, trailing bool,
) (glyph.CursorPosition, bool) {
	cp, ok := b.cursorPos(idx)
	if !ok || !trailing {
		return cp, ok
	}
	if _, _, r, found := b.lineBefore(idx); found {
		cp.X = r.X + r.Width
		cp.Y = r.Y
		cp.Height = r.Height
	}
	return cp, true
}

// lineBefore reports the line that ends where the line holding idx
// begins, when idx starts a line other than the first: its offsets and
// its rect in block coordinates. A paragraph start is not such a line
// start — the '\n' before it ends the earlier line — so only soft wraps
// and wrap-mode piece seams answer.
func (b *textBlock) lineBefore(idx int) (start, end int, r glyph.Rect, ok bool) {
	if len(b.chunks) == 0 || idx < 0 || idx > len(b.text) {
		return 0, 0, glyph.Rect{}, false
	}
	i := b.shapedChunkAt(idx)
	b.settle()
	c := &b.chunks[i]
	local := idx - c.start
	if c.layout != nil {
		lines := c.layout.Lines
		for k := 1; k < len(lines); k++ {
			if local == lines[k].StartIndex {
				prev := lines[k-1]
				return c.start + prev.StartIndex,
					c.start + prev.StartIndex + prev.Length,
					offsetRect(prev.Rect, c), true
			}
		}
	}
	if local != 0 || !c.piece || i == 0 || !b.wraps() {
		return 0, 0, glyph.Rect{}, false
	}
	b.ensure(i-1, b.w)
	b.settle()
	p := &b.chunks[i-1]
	if p.layout == nil || len(p.layout.Lines) == 0 {
		return 0, 0, glyph.Rect{}, false
	}
	last := p.layout.Lines[len(p.layout.Lines)-1]
	return p.start + last.StartIndex, p.end, offsetRect(last.Rect, p), true
}

func offsetRect(r glyph.Rect, c *textChunk) glyph.Rect {
	r.X += c.x
	r.Y += c.y
	return r
}

// trailingLineStart returns the start of the line before a soft-wrap
// boundary at idx, or fallback when idx is not one.
func (b *textBlock) trailingLineStart(idx, fallback int) int {
	if s, _, _, ok := b.lineBefore(idx); ok {
		return s
	}
	return fallback
}

// trailingLineEnd returns the end of the line before a soft-wrap
// boundary at idx, or fallback when idx is not one.
func (b *textBlock) trailingLineEnd(idx, fallback int) int {
	if _, e, _, ok := b.lineBefore(idx); ok {
		return e
	}
	return fallback
}

// band returns the vertical extent of the block's lines, for the drag
// edge rule: from the first line's top to the last line's bottom.
func (b *textBlock) band() (top, bottom float32) {
	b.settle()
	n := len(b.chunks)
	if n == 0 {
		return 0, 0
	}
	first, last := &b.chunks[0], &b.chunks[n-1]
	top = first.y
	if first.layout != nil && len(first.layout.Lines) > 0 {
		top += first.layout.Lines[0].Rect.Y
	}
	bottom = last.y + last.height
	if last.layout != nil && len(last.layout.Lines) > 0 {
		r := last.layout.Lines[len(last.layout.Lines)-1].Rect
		bottom = last.y + r.Y + r.Height
	}
	return top, bottom
}

// chunkAtPoint returns the index of the chunk under (x, y): the last
// row starting at or above y, and within a no-wrap row the piece whose
// span holds x, or the row's last piece past its end.
func (b *textBlock) chunkAtPoint(x, y float32) int {
	n := len(b.chunks)
	if n == 0 {
		return 0
	}
	// A point on the edge between two rows belongs to the upper one, as
	// glyph.Layout.GetClosestOffset has it for the edge between lines.
	i := sort.Search(n, func(i int) bool { return b.chunks[i].y >= y })
	i = max(i-1, 0)
	rowY := b.chunks[i].y
	first, last := i, i
	for first > 0 && b.chunks[first-1].y == rowY {
		first--
	}
	for last+1 < n && b.chunks[last+1].y == rowY {
		last++
	}
	for k := first; k < last; k++ {
		if x < b.chunks[k+1].x {
			return k
		}
	}
	return last
}

// closestOffset returns the byte offset nearest to (x, y) in block
// coordinates; a point outside the text snaps to the nearest line.
func (b *textBlock) closestOffset(x, y float32) int {
	if len(b.chunks) == 0 {
		return 0
	}
	i := b.shapedChunkAtPoint(x, y)
	b.settle()
	c := &b.chunks[i]
	if c.layout == nil {
		return c.start
	}
	return c.start + c.layout.GetClosestOffset(x-c.x, y-c.y)
}

// selectionRects returns the rectangles covering [beg, end) that lie
// within the vertical band [yMin, yMax], in block coordinates. The band
// keeps a selection over a long text from shaping its unseen chunks.
func (b *textBlock) selectionRects(
	beg, end int, yMin, yMax float32,
) []glyph.Rect {
	return b.selectionRectsVisible(beg, end, -f32Inf, f32Inf, yMin, yMax)
}

// selectionRectsVisible returns the rectangles covering [beg, end) whose
// chunks reach into the visible block-coordinate box. The renderer clips the
// rectangles themselves; culling here prevents a selection over a long
// single-line value from shaping every off-screen horizontal chunk.
func (b *textBlock) selectionRectsVisible(
	beg, end int, xMin, xMax, yMin, yMax float32,
) []glyph.Rect {
	if beg >= end || len(b.chunks) == 0 {
		return nil
	}
	b.settle()
	var out []glyph.Rect
	for i := b.chunkAt(beg); i < len(b.chunks); i++ {
		c := &b.chunks[i]
		if c.start >= end {
			break
		}
		if !chunkInSelectionBox(c, xMin, xMax, yMin, yMax) {
			continue
		}
		b.ensure(i, b.w)
		b.settle()
		c = &b.chunks[i]
		// An open chunk's estimate covers its unsplit remainder. Once
		// shaping cuts off the first piece, that piece may be outside the
		// box that made the estimate look visible, so test it again.
		if c.layout == nil ||
			!chunkInSelectionBox(c, xMin, xMax, yMin, yMax) {
			continue
		}
		lb := max(beg, c.start) - c.start
		le := min(end, c.end) - c.start
		for _, r := range c.layout.GetSelectionRects(lb, le) {
			r = offsetRect(r, c)
			// Pieces of a no-wrap row abut: one rectangle per row, as a
			// single layout gives, not one per piece.
			if k := len(out) - 1; k >= 0 && out[k].Y == r.Y &&
				out[k].Height == r.Height &&
				f32AreClose(out[k].X+out[k].Width, r.X) {
				out[k].Width = r.X + r.Width - out[k].X
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

func chunkInSelectionBox(
	c *textChunk, xMin, xMax, yMin, yMax float32,
) bool {
	return c.start < c.end &&
		c.x <= xMax && c.x+c.width >= xMin &&
		c.y <= yMax && c.y+c.height >= yMin
}

// shapedChunkAt returns the index of the chunk owning idx once that
// chunk is shaped. Shaping an open chunk cuts it, so the owner is
// resolved again until it is a chunk that needs no shaping.
func (b *textBlock) shapedChunkAt(idx int) int {
	for {
		i := b.chunkAt(idx)
		if !b.chunks[i].needsLayout() || b.w == nil || b.w.textMeasurer == nil {
			return i
		}
		// A no-wrap open chunk can be cut without shaping: unlike a wrapped
		// seam, its split does not depend on line breaking. Walk the metadata
		// toward a distant caret first and leave the intervening pieces at
		// their estimated widths. Their cumulative X is used both to place
		// the destination chunk and to scroll that caret into view, so it
		// cancels at the viewport; only the destination needs exact cursor
		// geometry. This keeps End on a megabyte line from shaping hundreds
		// of off-screen chunks merely to discover their seams.
		if b.chunks[i].open && !b.wraps() {
			b.splitOpen(i, b.w)
			if b.chunkAt(idx) != i {
				continue
			}
		}
		b.ensure(i, b.w)
		if !b.chunks[i].needsLayout() && b.chunkAt(idx) == i {
			return i
		}
	}
}

// shapedChunkAtPoint returns the chunk under (x, y) once that chunk is
// shaped. An open chunk provisionally covers the unsplit remainder of a
// long paragraph; shaping it can put the point in the remainder instead,
// so the point is resolved again until it lands in a real piece.
func (b *textBlock) shapedChunkAtPoint(x, y float32) int {
	for {
		b.settle()
		i := b.chunkAtPoint(x, y)
		if !b.chunks[i].needsLayout() || b.w == nil || b.w.textMeasurer == nil {
			return i
		}
		b.ensure(i, b.w)
		b.settle()
		if !b.chunks[i].needsLayout() && b.chunkAtPoint(x, y) == i {
			return i
		}
	}
}

// chunkStopAt0 reports whether a chunk's first byte is a caret stop.
// glyph.Layout.MoveCursorLeft answers 0 both for a stop at 0 and for no
// stop at all, so a 0 needs this to be read; a layout without attributes
// treats every byte as a stop.
func chunkStopAt0(c *textChunk) bool {
	l := c.layout
	if l == nil || len(l.LogAttrs) == 0 {
		return true
	}
	a, ok := l.LogAttrByIndex[0]
	return ok && a >= 0 && a < len(l.LogAttrs) && l.LogAttrs[a].IsCursorPosition
}

// moveLeft returns the caret stop before idx, or 0.
func (b *textBlock) moveLeft(idx int) int {
	if idx <= 0 || len(b.chunks) == 0 {
		return 0
	}
	idx = min(idx, len(b.text))
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	if idx > c.start {
		if c.layout == nil {
			return prevGraphemeBoundary(b.text, idx-1)
		}
		local := idx - c.start
		if r := c.layout.MoveCursorLeft(local); r > 0 || chunkStopAt0(c) {
			return c.start + r
		}
	}
	// No stop before idx in this chunk: the last stop of the chunk
	// before it, past the seam byte when there is one.
	at := c.start
	if at <= 0 {
		return 0
	}
	j := b.shapedChunkAt(at - 1)
	p := &b.chunks[j]
	if p.start == p.end {
		return p.start
	}
	if at == p.end+1 && b.text[p.end] == '\n' {
		// The position before a newline is a stop.
		return p.end
	}
	if p.layout == nil {
		return prevGraphemeBoundary(b.text, p.end-1)
	}
	if r := p.layout.MoveCursorLeft(p.end - p.start); r > 0 || chunkStopAt0(p) {
		return p.start + r
	}
	return b.moveLeft(p.start)
}

// moveRight returns the caret stop after idx, or the text end.
func (b *textBlock) moveRight(idx int) int {
	if idx >= len(b.text) || len(b.chunks) == 0 {
		return len(b.text)
	}
	idx = max(idx, 0)
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	local := idx - c.start
	n := c.end - c.start
	if local < n {
		if c.layout == nil {
			return nextGraphemeBoundary(b.text, idx)
		}
		// glyph answers the last stop both for a stop there and for no
		// stop above local; the chunk's end is a stop of the text only
		// before a newline or at the text end.
		if r := c.layout.MoveCursorRight(local); r > local && r < n {
			return c.start + r
		}
		if i+1 >= len(b.chunks) {
			return c.end
		}
		if next := &b.chunks[i+1]; next.start == c.end+1 && b.text[c.end] == '\n' {
			return c.end
		}
	}
	if i+1 >= len(b.chunks) {
		return len(b.text)
	}
	return b.firstStopOf(i + 1)
}

// firstStopOf returns the first caret stop of chunk j, which is its
// start unless a wrap-consumed space opens it.
func (b *textBlock) firstStopOf(j int) int {
	c := &b.chunks[j]
	if c.start == c.end {
		return c.start
	}
	j = b.shapedChunkAt(c.start)
	c = &b.chunks[j]
	if c.layout == nil || chunkStopAt0(c) {
		return c.start
	}
	if r := c.layout.MoveCursorRight(0); r > 0 && r <= c.end-c.start {
		return c.start + r
	}
	return c.start
}

// moveWordLeft returns the word start before idx, or 0.
func (b *textBlock) moveWordLeft(idx int) int {
	if idx <= 0 || len(b.chunks) == 0 {
		return 0
	}
	idx = min(idx, len(b.text))
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	if idx > c.start {
		if c.layout != nil {
			// glyph answers 0 both for a word starting the chunk and for
			// no word start at all; only a word at 0 that no earlier
			// piece continues is an answer.
			if r := c.layout.MoveCursorWordLeft(idx - c.start); r > 0 ||
				(!b.seamBefore(i) && chunkWordStartsAt0(c)) {
				return c.start + r
			}
		}
	}
	return b.wordStartLeftText(idx)
}

// chunkWordStartsAt0 reports whether the chunk's layout marks its first
// byte as a word start.
func chunkWordStartsAt0(c *textChunk) bool {
	l := c.layout
	if l == nil {
		return false
	}
	a, ok := l.LogAttrByIndex[0]
	return ok && a >= 0 && a < len(l.LogAttrs) && l.LogAttrs[a].IsWordStart
}

// wordStartLeftText finds the word start before idx in the text, one
// paragraph at a time from the one holding idx, with the class rules
// the layout applies (glyph.WordStartLeft). A paragraph is scanned
// from its start, so the cost is bounded by paragraph length, not by
// the text before it.
func (b *textBlock) wordStartLeftText(idx int) int {
	for idx > 0 {
		from := b.paragraphStart(idx)
		s := b.text[from:idx]
		if r := glyph.WordStartLeft(s, len(s)); r > 0 {
			return from + r
		}
		if first, _ := utf8.DecodeRuneInString(s); len(s) > 0 &&
			!unicode.IsSpace(first) {
			return from
		}
		if from == 0 {
			return 0
		}
		idx = from - 1
	}
	return 0
}

// moveWordRight returns the word start after idx, or the text end.
func (b *textBlock) moveWordRight(idx int) int {
	if idx >= len(b.text) || len(b.chunks) == 0 {
		return len(b.text)
	}
	idx = max(idx, 0)
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	local := idx - c.start
	n := c.end - c.start
	if c.layout != nil && local < n {
		// A result at the chunk's end means no word start follows in
		// this chunk; glyph answers the last caret stop then.
		if r := c.layout.MoveCursorWordRight(local); r > local && r < n {
			return c.start + r
		}
	} else if c.layout == nil && local < n {
		return glyph.WordStartRight(b.text, idx)
	}
	// Past the chunk, the text answers: a word may straddle the seam,
	// and a run boundary depends only on its neighbours, so scanning
	// from the paragraph start is exact and bounded by the distance to
	// the next word start.
	from := b.paragraphStart(idx)
	return from + glyph.WordStartRight(b.text[from:], idx-from)
}

// moveLineStart returns the start of the line holding idx. At a soft
// wrap the later line wins, as in glyph.Layout.MoveCursorLineStart.
func (b *textBlock) moveLineStart(idx int) int {
	if len(b.chunks) == 0 || idx <= 0 {
		return 0
	}
	idx = min(idx, len(b.text))
	i := b.shapedChunkAt(idx)
	if first, last := b.rowBounds(i); first != last {
		return b.chunks[first].start
	}
	c := &b.chunks[i]
	if c.layout == nil {
		return b.paragraphStart(idx)
	}
	return c.start + c.layout.MoveCursorLineStart(idx-c.start)
}

// moveLineEnd returns the end of the line holding idx.
func (b *textBlock) moveLineEnd(idx int) int {
	if len(b.chunks) == 0 {
		return 0
	}
	idx = max(0, min(idx, len(b.text)))
	i := b.shapedChunkAt(idx)
	if first, last := b.rowBounds(i); first != last {
		return b.chunks[last].end
	}
	c := &b.chunks[i]
	if c.layout == nil {
		return b.paragraphEnd(idx)
	}
	return c.start + c.layout.MoveCursorLineEnd(idx-c.start)
}

// paragraphStart returns the byte offset after the '\n' before idx.
func (b *textBlock) paragraphStart(idx int) int {
	for i := min(idx, len(b.text)) - 1; i >= 0; i-- {
		if b.text[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// paragraphEnd returns the byte offset of the '\n' at or after idx.
func (b *textBlock) paragraphEnd(idx int) int {
	for i := max(idx, 0); i < len(b.text); i++ {
		if b.text[i] == '\n' {
			return i
		}
	}
	return len(b.text)
}

// moveUp returns the offset on the line above idx nearest to prefX in
// block coordinates, or idx on the first line. prefX < 0 means the
// caret's own x.
func (b *textBlock) moveUp(idx int, prefX float32) int {
	return b.moveVertical(idx, prefX, true)
}

// moveDown is moveUp for the line below.
func (b *textBlock) moveDown(idx int, prefX float32) int {
	return b.moveVertical(idx, prefX, false)
}

func (b *textBlock) moveVertical(idx int, prefX float32, up bool) int {
	if len(b.chunks) == 0 {
		return idx
	}
	idx = max(0, min(idx, len(b.text)))
	if prefX < 0 {
		if cp, ok := b.cursorPos(idx); ok {
			prefX = cp.X
		}
	}
	i := b.shapedChunkAt(idx)
	b.settle()
	c := &b.chunks[i]
	local := idx - c.start
	if c.layout != nil {
		var r int
		if up {
			r = c.layout.MoveCursorUp(local, prefX-c.x)
		} else {
			r = c.layout.MoveCursorDown(local, prefX-c.x)
		}
		if r != local {
			return c.start + r
		}
	}
	// First (or last) line of the chunk: the neighbouring row.
	var j int
	if up {
		j = b.rowAbove(i, prefX)
	} else {
		j = b.rowBelow(i, prefX)
	}
	if j < 0 {
		return idx
	}
	b.ensure(j, b.w)
	b.settle()
	p := &b.chunks[j]
	if p.layout == nil || len(p.layout.Lines) == 0 {
		return p.start
	}
	var line glyph.Line
	if up {
		line = p.layout.Lines[len(p.layout.Lines)-1]
	} else {
		line = p.layout.Lines[0]
	}
	return p.start + p.layout.GetClosestOffset(
		prefX-p.x, line.Rect.Y+line.Rect.Height/2)
}

// rowAbove returns the chunk on the row above chunk i under prefX, or
// -1 on the first row. Pieces of a no-wrap paragraph share a row, so
// the row is found by y and the piece by x.
func (b *textBlock) rowAbove(i int, prefX float32) int {
	y := b.chunks[i].y
	j := i - 1
	for j >= 0 && b.chunks[j].y >= y {
		j--
	}
	if j < 0 {
		return -1
	}
	rowY := b.chunks[j].y
	for j > 0 && b.chunks[j-1].y == rowY && prefX < b.chunks[j].x {
		j--
	}
	return j
}

// rowBelow is rowAbove for the row below, or -1 on the last row.
func (b *textBlock) rowBelow(i int, prefX float32) int {
	n := len(b.chunks)
	y := b.chunks[i].y
	k := i + 1
	for k < n && b.chunks[k].y <= y {
		k++
	}
	if k >= n {
		return -1
	}
	rowY := b.chunks[k].y
	for k+1 < n && b.chunks[k+1].y == rowY && prefX >= b.chunks[k+1].x {
		k++
	}
	return k
}

// wordAt returns the [start, end) byte range of the word holding idx.
func (b *textBlock) wordAt(idx int) (int, int) {
	if len(b.chunks) == 0 {
		return 0, 0
	}
	idx = max(0, min(idx, len(b.text)))
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	local := idx - c.start
	n := c.end - c.start
	// A run that may reach past the chunk — the gap at its end, a word
	// at either edge, an empty or unshaped chunk — is read off the text
	// with the same class rules the layout applies (glyph.WordBounds-
	// InString), so the answer is the whole text's.
	if c.layout == nil || local >= n {
		return glyph.WordBoundsInString(b.text, idx)
	}
	s, e := c.layout.GetWordAtIndex(local)
	// An empty answer is a chunk with no word in it, whose whitespace
	// run continues past it.
	if s == e || (s == 0 && i > 0) || (e == n && i+1 < len(b.chunks)) {
		return glyph.WordBoundsInString(b.text, idx)
	}
	return c.start + s, c.start + e
}

// deleteGrapheme removes the grapheme cluster after (forward) or before
// idx within its chunk, with the cluster boundaries of the chunk's
// layout. It declines at a chunk edge, where the deletion would cross a
// '\n' or a piece seam, and when the chunk has no layout; the caller
// then falls back to the string-based delete.
func (b *textBlock) deleteGrapheme(idx int, forward bool) (string, int, bool) {
	if len(b.chunks) == 0 || idx < 0 || idx > len(b.text) {
		return "", 0, false
	}
	i := b.shapedChunkAt(idx)
	c := &b.chunks[i]
	if c.layout == nil {
		return "", 0, false
	}
	local := idx - c.start
	chunkText := b.text[c.start:c.end]
	if (forward && local >= len(chunkText)) || (!forward && local <= 0) {
		return "", 0, false
	}
	var res glyph.MutationResult
	if forward {
		res = glyph.DeleteForward(chunkText, *c.layout, local)
	} else {
		res = glyph.DeleteBackward(chunkText, *c.layout, local)
	}
	if res.RangeStart >= res.RangeEnd {
		return "", 0, false
	}
	beg := c.start + res.RangeStart
	end := c.start + res.RangeEnd
	return b.text[:beg] + b.text[end:], c.start + res.CursorPos, true
}
