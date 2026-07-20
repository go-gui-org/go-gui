package gui

// scrollAnchor is a pending one-shot scroll-anchoring request. See
// Window.ScrollAnchor.
type scrollAnchor struct {
	scrollID string
	anchorID string
	relY     float32 // anchor top relative to viewport top at request time
	frame    uint64  // frameCount at request time; stale entries are dropped
	reveal   bool    // ease to the top after the correction lands
}

// scrollAnchorMaxAge is how many frames a pending anchor may wait
// for a layout pass containing its scrollable (passes also run on
// floating subtrees that don't contain it) before it is dropped as
// stale — e.g. the scrollable was removed from the view.
const scrollAnchorMaxAge = 2

// ScrollAnchor requests one-shot vertical scroll anchoring for the
// scrollable with the given scroll id: on the next layout pass the
// scroll offset is corrected so the view anchorID keeps the
// viewport-relative position it has now. Call it just before
// mutating state that inserts or removes content above the reader's
// position (typically right before UpdateWindow). The correction
// happens inside the layout pipeline, before the frame renders, so
// no intermediate position is ever shown.
//
// The request is consumed by the next layout pass containing the
// scrollable. It is dropped without effect — the view jumps as if
// no anchor was requested — when either id cannot be resolved, the
// content fits the viewport, or the corrected offset would fall
// outside the scrollable range. Last write wins per scrollable.
// Main goroutine only.
func (w *Window) ScrollAnchor(scrollID, anchorID string) {
	w.scrollAnchorRequest(scrollID, anchorID, false)
}

// ScrollAnchorReveal anchors like [Window.ScrollAnchor], then eases
// the scrollable to the top with the same smoothing as
// [Window.ScrollVerticalToSmooth], so content prepended above the
// anchor glides into view instead of appearing suddenly. The ease
// must arm after the anchoring correction is in place — an ordering
// only the layout pipeline can provide, which is why the reveal is
// part of the request rather than a separate call (an ease armed
// before the correction would no-op when the reader is already at
// the top). Main goroutine only.
func (w *Window) ScrollAnchorReveal(scrollID, anchorID string) {
	w.scrollAnchorRequest(scrollID, anchorID, true)
}

// scrollAnchorRequest records a pending anchor, capturing the
// anchor's viewport-relative position from the last laid-out frame
// so the next pass can restore it.
func (w *Window) scrollAnchorRequest(scrollID, anchorID string, reveal bool) {
	if scrollID == "" || anchorID == "" {
		return
	}
	sc, ok := findScrollLayout(w, scrollID)
	if !ok {
		return
	}
	target, ok := sc.FindByID(anchorID)
	if !ok {
		return
	}
	relY := target.Shape.Y - (sc.Shape.Y + sc.Shape.PaddingTop())
	if !f32IsFinite(relY) {
		return
	}
	a := scrollAnchor{
		scrollID: scrollID,
		anchorID: anchorID,
		relY:     relY,
		frame:    w.frameCount,
		reveal:   reveal,
	}
	for i := range w.scrollAnchors {
		if w.scrollAnchors[i].scrollID == scrollID {
			w.scrollAnchors[i] = a
			return
		}
	}
	w.scrollAnchors = append(w.scrollAnchors, a)
}

// layoutApplyScrollAnchors consumes pending scroll anchors after the
// position pass: for each request whose scrollable is in this tree,
// the scroll offset is corrected so the anchor view keeps the
// viewport-relative position captured at request time, and the
// already positioned children are shifted to match (later passes lay
// out identically from the stored offset). Requests whose scrollable
// is not in this tree — e.g. a floating-subtree pass — stay pending
// briefly for the pass that has it.
func layoutApplyScrollAnchors(layout *Layout, w *Window) {
	if len(w.scrollAnchors) == 0 {
		return
	}
	kept := w.scrollAnchors[:0]
	for _, a := range w.scrollAnchors {
		sc, ok := FindLayoutByScrollID(layout, a.scrollID)
		if !ok {
			if w.frameCount-a.frame <= scrollAnchorMaxAge {
				kept = append(kept, a)
			}
			continue
		}
		applyScrollAnchor(a, sc, w)
	}
	w.scrollAnchors = kept
}

// applyScrollAnchor corrects one scrollable's offset so the anchor
// view keeps its captured viewport-relative position, then shifts
// the positioned subtree for the current frame and, for reveal
// requests, arms the ease back to the top.
func applyScrollAnchor(a scrollAnchor, sc *Layout, w *Window) {
	target, ok := sc.FindByID(a.anchorID)
	if !ok {
		return // anchor left the view; jump
	}
	viewTop := sc.Shape.Y + sc.Shape.PaddingTop()
	delta := target.Shape.Y - (viewTop + a.relY)
	if delta == 0 {
		return // anchor did not move; nothing to correct
	}
	maxOffset := scrollMaxOffsetY(sc)
	if maxOffset >= 0 {
		return // content fits the viewport; nothing to anchor
	}
	sy := w.scrollY()
	old := sy.GetOr(a.scrollID, 0)
	newOffset := old - delta
	if !f32IsFinite(newOffset) || newOffset > 0 || newOffset < maxOffset {
		return // correction would leave the scroll range; jump
	}

	// Store the corrected offset for later passes and move the
	// already positioned children to match for this frame (positions
	// are absolute; moving a parent does not move its children).
	sy.Set(a.scrollID, newOffset)
	for i := range sc.Children {
		scrollAnchorShiftY(&sc.Children[i], -delta)
	}
	// Keep any in-flight ease continuous from the corrected offset.
	scrollSmoothShiftY(w, a.scrollID, -delta)
	fireOnScroll(sc, w)
	if a.reveal {
		scrollSmoothTo(w, sc, scrollAxisY, 0)
	}
}

// scrollAnchorShiftY moves a positioned subtree vertically. Every
// descendant shifts because positions are absolute after the
// position pass.
func scrollAnchorShiftY(layout *Layout, dy float32) {
	layout.Shape.Y += dy
	for i := range layout.Children {
		scrollAnchorShiftY(&layout.Children[i], dy)
	}
}
