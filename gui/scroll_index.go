package gui

import "slices"

// scroll_index.go — index-addressed scrolling.
//
// The existing scroll surface addresses a *view*: scrollToView
// resolves an ID through FindByID, so it structurally cannot reach a
// row virtualization has not built. Percent scrolling is no answer
// either, because under virtualization the content height is a
// fiction assembled from spacers. Index addressing goes through the
// height model instead, which knows where item i sits whether or not
// the row exists this frame.
//
// A request is applied immediately when the list has already been
// generated, and queued otherwise. The queued form mirrors
// scrollAnchor (gui/scroll_anchor.go), including its "not in this
// layer's tree, stay pending" branch, because layoutPipeline runs per
// layer and a floating subtree's pass does not contain the list.

// virtualScrollMode selects how the target index is aligned in the
// viewport.
type virtualScrollMode uint8

const (
	// virtualScrollAt aligns the item by frac: 0 puts its top at the
	// viewport top, 0.5 centres it, 1 puts its bottom at the bottom.
	virtualScrollAt virtualScrollMode = iota
	// virtualScrollIntoView moves by the least amount that makes the
	// item visible, and does nothing when it already is.
	virtualScrollIntoView
	// virtualScrollEnd pins the list to the bottom of its content.
	virtualScrollEnd
)

// virtualScrollReq is a pending index-addressed scroll.
type virtualScrollReq struct {
	scrollID string
	frac     float32
	index    int
	frame    uint64 // frameCount at request time
	mode     virtualScrollMode
}

// virtualScrollMaxAge is how many frames a request stays live. It is
// longer than scrollAnchorMaxAge on purpose: a far jump into rows
// that have never been measured lands estimate-accurate, and re-
// applying while the newly built rows are measured is what converges
// it onto the real position.
const virtualScrollMaxAge = 4

// ScrollToIndex scrolls the given list so item index sits at the top
// of the viewport. id is the list's effective scroll ID; index is in
// the widget's own index space (see the retrofit table in
// docs/specs/virtualized-variable-height-lists.md — a frozen table
// header, for instance, is data index 0 but sits outside the
// scrollable). Main goroutine only.
func (w *Window) ScrollToIndex(id string, index int) {
	w.scrollIndexRequest(id, index, virtualScrollAt, 0)
}

// ScrollToIndexAt scrolls the given list so item index lands at frac
// of the viewport: 0 top, 0.5 middle, 1 bottom. Main goroutine only.
func (w *Window) ScrollToIndexAt(id string, index int, frac float32) {
	w.scrollIndexRequest(id, index, virtualScrollAt,
		f32Clamp(frac, 0, 1))
}

// ScrollIndexIntoView scrolls the given list by the least amount that
// brings item index fully into the viewport, and does nothing when it
// is already visible. This is the one to call when moving a selection
// with the arrow keys. Main goroutine only.
// exportaudit:keep — the selection-following form of the index API
func (w *Window) ScrollIndexIntoView(id string, index int) {
	w.scrollIndexRequest(id, index, virtualScrollIntoView, 0)
}

// ScrollToEnd pins the given list to the bottom of its content. It
// differs from ScrollVerticalToPct(id, 1) under virtualization, where
// the content height is assembled from spacers over an estimated row
// height and a percentage therefore drifts. Main goroutine only.
func (w *Window) ScrollToEnd(id string) {
	w.scrollIndexRequest(id, 0, virtualScrollEnd, 0)
}

// scrollIndexRequest applies the request against the current layout
// when it can, and queues it either way so later passes converge it
// as rows are measured.
func (w *Window) scrollIndexRequest(
	id string, index int, mode virtualScrollMode, frac float32,
) {
	if w == nil || id == "" {
		return
	}
	r := virtualScrollReq{
		scrollID: id,
		index:    index,
		frac:     frac,
		frame:    w.frameCount,
		mode:     mode,
	}
	// A jump and a pending anchor both write the same offset; the
	// explicit jump is the later intent, so drop the anchor.
	w.dropScrollAnchor(id)
	scrollSmoothCancel(w, id, scrollAxisY)

	if sc, ok := findScrollLayout(w, id); ok {
		applyVirtualScroll(r, sc, w, false)
	}
	w.queueVirtualScroll(r)
	w.UpdateWindow()
}

// queueVirtualScroll records the request, replacing any pending one
// for the same scrollable — last write wins, as with scroll anchors.
func (w *Window) queueVirtualScroll(r virtualScrollReq) {
	for i := range w.virtualScrolls {
		if w.virtualScrolls[i].scrollID == r.scrollID {
			w.virtualScrolls[i] = r
			return
		}
	}
	w.virtualScrolls = append(w.virtualScrolls, r)
}

// dropVirtualScroll removes a pending index-addressed scroll for a
// scrollable. Called (via scrollSmoothCancel) whenever a direct
// offset write lands, so a still-converging request cannot snap the
// user's own scroll back.
func (w *Window) dropVirtualScroll(scrollID string) {
	w.virtualScrolls = slices.DeleteFunc(w.virtualScrolls,
		func(r virtualScrollReq) bool { return r.scrollID == scrollID })
}

// dropScrollAnchor removes a pending anchor for a scrollable.
func (w *Window) dropScrollAnchor(scrollID string) {
	w.scrollAnchors = slices.DeleteFunc(w.scrollAnchors,
		func(a scrollAnchor) bool { return a.scrollID == scrollID })
}

// layoutApplyVirtualScrolls consumes pending index-addressed scrolls.
//
// It must run after layoutPositions and layoutApplyScrollAnchors:
// layoutAdjustScrollOffsets is the *first* position pass and zero-
// fills any scrollable with no stored entry, so an offset written
// before it is clobbered on a list the user has never scrolled — the
// exact case ScrollToIndex exists for. By this point the arranged
// height and contentHeight are both known, so the clamp is real.
func layoutApplyVirtualScrolls(layout *Layout, w *Window) {
	if len(w.virtualScrolls) == 0 {
		return
	}
	kept := w.virtualScrolls[:0]
	for _, r := range w.virtualScrolls {
		sc, ok := findLayoutByScrollID(layout, r.scrollID)
		if !ok {
			// Not in this layer's tree; a later pass may have it.
			if w.frameCount-r.frame <= virtualScrollMaxAge {
				kept = append(kept, r)
			}
			continue
		}
		// Keep converging while the request is young: a jump into
		// never-measured rows only lands exactly once those rows have
		// been built and written back. A uniform model is exact on the
		// first application, so it settles immediately.
		if !applyVirtualScroll(r, sc, w, true) &&
			w.frameCount-r.frame <= virtualScrollMaxAge {
			kept = append(kept, r)
		}
	}
	w.virtualScrolls = kept
}

// applyVirtualScroll converts one request to a scroll offset through
// the list's height model. shift moves the already-positioned
// children so the correction never renders as a jump; the immediate
// (pre-layout) path passes false, because that frame's positions are
// recomputed from the stored offset anyway. The result reports
// whether re-applying can no longer change anything — a variable
// model keeps refining while its rows are measured.
func applyVirtualScroll(
	r virtualScrollReq, sc *Layout, w *Window, shift bool,
) (settled bool) {
	m, ok := listHeightLookup(w, r.scrollID)
	if !ok {
		return true
	}
	settled = m.uniform
	viewH := sc.Shape.Height - sc.Shape.paddingHeight()
	if viewH <= 0 || !f32IsFinite(viewH) {
		return settled
	}
	maxOffset := scrollMaxOffsetY(sc)
	sy := w.scrollY()
	// Default 0: absent entry means the list has not been scrolled.
	old := sy.GetOr(r.scrollID, 0)

	target, want := virtualScrollTarget(r, m, viewH, maxOffset, old)
	if !want {
		return
	}
	target = f32Clamp(target, maxOffset, 0)
	if !f32IsFinite(target) || target == old {
		return
	}
	sy.Set(r.scrollID, target)
	if shift {
		// Positions are absolute after the position pass, so moving the
		// scrollable does not move its children (see scrollAnchor).
		dy := target - old
		for i := range sc.Children {
			scrollAnchorShiftY(&sc.Children[i], dy)
		}
		scrollSmoothShiftY(w, r.scrollID, dy)
	}
	fireOnScroll(sc, w)
	return settled
}

// virtualScrollTarget computes the offset a request asks for. The
// second result is false when the request is satisfied already, which
// is how ScrollIndexIntoView stays a no-op on a visible row.
func virtualScrollTarget(
	r virtualScrollReq, m *listHeightModel,
	viewH, maxOffset, cur float32,
) (float32, bool) {
	if r.mode == virtualScrollEnd {
		return maxOffset, true
	}
	i := clampInt(r.index-m.indexBase, 0, m.Count()-1)
	top := m.Prefix(i)
	h := m.Height(i)
	if r.mode == virtualScrollIntoView {
		visTop := -cur
		visBot := visTop + viewH
		switch {
		case top < visTop:
			return -top, true
		case top+h > visBot:
			return -(top + h - viewH), true
		default:
			return 0, false // already visible
		}
	}
	// frac aligns the item's box inside the viewport: at frac 0 the
	// slack is 0, at 1 it is the whole viewport minus the item.
	return -(top - r.frac*(viewH-h)), true
}
