package gui

// Scroll anchoring for chunked text (#1187, #1190).
//
// A textBlock larger than textBlockEagerBytes starts with estimated chunk
// heights, measured when a chunk first comes into view. When the view
// scrolls up into such text, the chunk entering at the top is measured by
// the frame that first shows it; if its estimate was off, every line
// below it, everything already on screen, moves by the error.
//
// This pass, run once positions are set, measures the chunks the scroll
// view shows before anything is drawn, grows the boxes that fit the text
// by what the measurement changed, and moves the scroll offset by the
// height that appeared or vanished above the first chunk that was already
// measured, so the text the reader is looking at stays where it was. It is
// the correction Window.ScrollAnchor applies to inserted content
// (scroll_anchor.go), with the anchor found in the text rather than named.

// queueTextAnchor records a text shape whose block still holds estimated
// chunks, for layoutAnchorTextBlocks to visit after the position pass.
// The wrap pass calls it, as it already walks every text shape; a tree
// with no such shape costs the anchor pass nothing.
func queueTextAnchor(layout *Layout, w *Window) {
	w.scratch.textAnchors = append(w.scratch.textAnchors, layout)
}

// resetTextAnchors empties the queue. layoutPipeline calls it first, not
// the wrap pass, which runs once per layoutSizer pane: the queue holds the
// texts of the whole tree being laid out, and nothing older even when a
// caller ran the wrap pass without the anchor pass after it.
func resetTextAnchors(w *Window) {
	clear(w.scratch.textAnchors)
	w.scratch.textAnchors = w.scratch.textAnchors[:0]
}

// layoutAnchorTextBlocks measures the visible chunks of each queued text
// and keeps its scroll view on the same lines. It consumes the queue.
func layoutAnchorTextBlocks(w *Window) {
	for _, ly := range w.scratch.textAnchors {
		anchorTextBlock(ly, w)
	}
	resetTextAnchors(w)
}

func anchorTextBlock(ly *Layout, w *Window) {
	shape := ly.Shape
	if shape == nil || shape.TC == nil || shape.TC.block == nil {
		return
	}
	blk := shape.TC.block
	sc, follows := textScrollView(ly)
	if sc == nil {
		return
	}
	viewTop := sc.Shape.Y + sc.Shape.PaddingTop()
	viewH := sc.Shape.Height - sc.Shape.paddingHeight()
	baseY := shape.Y + shape.PaddingTop()
	if viewH <= 0 || !f32IsFinite(viewH) || !f32IsFinite(viewTop) ||
		!f32IsFinite(baseY) {
		return
	}
	y0, y1 := viewTop-baseY, viewTop+viewH-baseY

	// The anchor is the first chunk in view whose height is already
	// measured: it was on screen before, so a move of it is a jump. It is
	// held by its first byte: measuring an open chunk above it cuts that
	// chunk into pieces and moves the anchor to a later index.
	blk.settle()
	anchorStart, anchorY := -1, float32(0)
	for i := range blk.chunks {
		c := &blk.chunks[i]
		if c.y > y1 {
			break
		}
		if c.y+c.height >= y0 && c.measured {
			anchorStart, anchorY = c.start, c.y
			break
		}
	}
	oldH := blk.boxHeight()
	blk.ensureRange(y0, y1)
	if !follows {
		// A box between the text and the view keeps its own height, so
		// the text's growth does not reach the scroll content and an
		// offset change would move that box, not the text. The chunks
		// are measured; the next frame lays the boxes out from them.
		return
	}
	if dh := blk.boxHeight() - oldH; dh != 0 && f32IsFinite(dh) &&
		!textAnchorGrow(ly, sc, dh) {
		return // a height limit holds a box; as for !follows above
	}
	if anchorStart < 0 {
		return // nothing in view was on screen before; nothing jumps
	}
	anchor := blk.chunkAt(anchorStart)
	if blk.chunks[anchor].start != anchorStart {
		return
	}
	delta := blk.chunks[anchor].y - anchorY
	if delta == 0 || !f32IsFinite(delta) {
		return
	}
	maxOffset := scrollMaxOffsetY(sc)
	if maxOffset >= 0 {
		return
	}
	applyScrollAnchorShift(scrollAnchor{scrollID: sc.Shape.idKey()},
		sc, w, delta, maxOffset)
}

// textScrollView returns the nearest view above ly that scrolls
// vertically, and whether the text's height reaches its content: the
// text and every box between are fit-height, and every box from the
// text's parent to the view stacks its children top to bottom, so a
// change in the text's height is the same change in the content's,
// unless a height limit holds a box (textAnchorRoom). The walk stops at
// a float: its Parent is the container it was lifted from, in another
// layer, whose boxes the text does not size.
func textScrollView(ly *Layout) (sc *Layout, follows bool) {
	if ly.Shape.Float {
		return nil, false
	}
	follows = ly.Shape.Sizing.Height == SizingFit
	depth := 0
	for p := ly.Parent; p != nil && !overMaxDepth(depth); p = p.Parent {
		s := p.Shape
		if s == nil {
			return nil, false
		}
		if s.Axis != axisTopToBottom {
			follows = false
		}
		if s.Scrollable && s.ScrollMode != ScrollHorizontalOnly {
			if s.idKey() == "" {
				return nil, false
			}
			return p, follows
		}
		if s.Float {
			return nil, false
		}
		if s.Sizing.Height != SizingFit {
			follows = false
		}
		depth++
	}
	return nil, false
}

// textAnchorGrow applies a change dh in the text's height to the
// positioned tree: the text and each box up to the scroll view grow by
// dh, the siblings after each of them move down by dh, and the cached
// content heights follow, so the scroll range and the scrollbar match
// the measured text in this frame. textScrollView has checked that every
// box on the way is a fit-height column. It reports false, and changes
// nothing, when a box's height limit would not take dh in full.
func textAnchorGrow(ly, sc *Layout, dh float32) bool {
	for p := ly; p != nil && p != sc; p = p.Parent {
		if !textAnchorRoom(p.Shape, dh) {
			return false
		}
	}
	ly.Shape.Height += dh
	child := ly
	for p := ly.Parent; p != nil; p = p.Parent {
		past := false
		for i := range p.Children {
			if &p.Children[i] == child {
				past = true
				continue
			}
			if past {
				scrollAnchorShiftY(&p.Children[i], dh)
			}
		}
		if p.Shape.fillGen != 0 {
			p.Shape.contentH += dh
		}
		if p == sc {
			return true
		}
		p.Shape.Height += dh
		child = p
	}
	return true
}

// textAnchorRoom reports whether a fit-height box takes a change of dh
// in its content's height in full. A MaxHeight caps it; a MinHeight
// holds it when the box sits at its minimum, its content possibly
// shorter, or would fall below it. Layout derives a MinHeight for most
// containers, from their padding at least, so a minimum alone does not
// stop the change.
func textAnchorRoom(s *Shape, dh float32) bool {
	h := s.Height + dh
	if s.MaxHeight > 0 && h > s.MaxHeight {
		return false
	}
	if s.MinHeight > 0 && (s.Height <= s.MinHeight || h < s.MinHeight) {
		return false
	}
	return true
}

// hasEstimates reports whether any chunk still awaits measuring.
func (b *textBlock) hasEstimates() bool {
	for i := range b.chunks {
		if !b.chunks[i].measured {
			return true
		}
	}
	return false
}
