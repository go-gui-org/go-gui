package gui

// layoutOverflow hides children that don't fit in an overflow container.
func layoutOverflow(layout *Layout, w *Window) {
	layoutOverflowDepth(layout, w, 0)
}

func layoutOverflowDepth(layout *Layout, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	for i := range layout.Children {
		layoutOverflowDepth(&layout.Children[i], w, depth+1)
	}

	if !layout.Shape.Overflow {
		return
	}

	// Wrap and Overflow are contradictory strategies for the same
	// condition — wrap breaks content onto a new row, overflow hides the
	// tail behind a trigger — and wrap wins (issue #380). Without the
	// gate, a wrap whose content fits one row keeps its LTR axis and
	// layoutOverflow hides a child even when everything fits, because it
	// reserves room for the trigger button.
	//
	// Checked before the axis bail so the note fires for any wrap+overflow
	// container: a wrap that broke rows flips to a TTB axis, and the
	// report must not depend on whether the content happened to fit. The
	// mask guard keeps the off-state cost at one atomic load, matching
	// debugUnconsumed.
	if layout.Shape.Wrap {
		if DebugCategory(debugMask.Load())&DebugWrapOverflow != 0 {
			key := layout.Shape.idKey()
			w.debugWarn(debugCheckWrapOverflow, key,
				"container %q sets both Wrap and Overflow; Overflow is "+
					"ignored (issue #380)", key)
		}
		return
	}

	if layout.Shape.Axis != axisLeftToRight {
		return
	}

	// A Fit-width overflow container resolves against its nearest
	// definite-width ancestor (issue #379): its fit width is the full
	// content sum, so nothing below it ever overflows.
	if changed, path, pathLen := constrainFitContainerWidth(layout); changed {
		refitFitAncestors(layout, path[:pathLen])
		layout.Shape.contentW = computeContentWidth(layout)
	}

	if len(layout.Children) < 2 || layout.Shape.Scrollable {
		return
	}

	available := layout.Shape.Width - layout.Shape.paddingWidth()
	spacing := layout.Shape.Spacing

	// Find the trigger button (last non-float, non-placeholder child).
	triggerIdx := len(layout.Children) - 1
	for triggerIdx > 0 && skipLayoutChild(layout.Children[triggerIdx].Shape) {
		triggerIdx--
	}
	triggerW := layout.Children[triggerIdx].Shape.Width

	// The last in-flow item needs no room for the trigger: if it fits,
	// every item fits and the trigger is hidden.
	lastItemIdx := -1
	for i := triggerIdx - 1; i >= 0; i-- {
		if !skipLayoutChild(layout.Children[i].Shape) {
			lastItemIdx = i
			break
		}
	}

	var used float32
	firstHideIdx := triggerIdx // child index where hiding starts
	// A gap follows every in-flow child that came before, which is not
	// the same question as whether they took any width: a zero-width
	// in-flow child still holds a slot, and layoutPositions still
	// advances past it by width + spacing. Testing `used > 0` lost that
	// gap and let an item stay in a row it overflows. Same flag, same
	// reason, as the wrap pass (layout_wrap.go).
	rowHasFlowChild := false

	for i := range triggerIdx {
		child := &layout.Children[i]
		if skipLayoutChild(child.Shape) {
			continue
		}
		var gap float32
		if rowHasFlowChild {
			gap = spacing
		}
		needed := used + gap + child.Shape.Width
		reserve := spacing + triggerW
		if i == lastItemIdx {
			reserve = 0
		}
		if needed+reserve > available {
			firstHideIdx = i
			break
		}
		used = needed
		rowHasFlowChild = true
	}

	// The stored count is an index into the container's children, not a
	// count of laid-out children: OverflowPanel slices cfg.Items with it.
	// Skipped placeholders (empty, Float, OverDraw) before the trigger take
	// an index too, so counting only laid-out children came up short. Then
	// the all-fit case was never seen (trigger stayed visible) and the menu
	// listed items still in the row. firstHideIdx is already an index, and
	// everything before it stays in the row.
	visibleCount := firstHideIdx
	if firstHideIdx == triggerIdx {
		hideOverflowChild(&layout.Children[triggerIdx])
	} else {
		for i := firstHideIdx; i < triggerIdx; i++ {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			hideOverflowChild(&layout.Children[i])
		}
	}
	// Hidden children take no width now. The fill pass cached contentW
	// before they were hidden, and applyContainerAlignment reads it, so
	// a centered or end-aligned row would be placed against the old sum.
	layout.Shape.contentW = computeContentWidth(layout)

	om := w.overflow()
	id := layout.Shape.idKey()
	old, ok := om.Get(id)
	if !ok {
		old = -1
	}
	if old != visibleCount {
		om.Set(id, visibleCount)
		ss := StateMap[string, bool](w, nsSelect, capModerate)
		ss.Delete(id)
		w.refreshLayout = true
	}
}

// hideOverflowChild takes a child out of the row: it stops drawing
// (shapeNone), stops taking width, and clips so its own descendants —
// which still render and still carry their sizes — collapse with it.
//
// Focusability is cleared across the whole subtree, not just the child.
// Tab order comes from collectFocusCandidates, which walks every node and
// gates only on canTakeFocus; it reads neither shapeType nor the clip, so
// a hidden child and everything under it would otherwise stay reachable by
// Tab while invisible. The walk covers descendants because an overflowed
// item is often a container whose focusable widget sits inside it.
//
// Clearing Focusable in place is safe for the same reason the shapeType
// write above is: these shapes are frame-scoped, rebuilt from the view
// tree on the next generation pass.
func hideOverflowChild(child *Layout) {
	child.Shape.shapeType = shapeNone
	child.Shape.Width = 0
	child.Shape.Clip = true
	clearSubtreeFocusable(child, 0)
}

// clearSubtreeFocusable drops Focusable from layout and every descendant.
// Depth-capped like the other tree walks (see maxEventDepth).
func clearSubtreeFocusable(layout *Layout, depth int) {
	if layout == nil || layout.Shape == nil || overMaxDepth(depth) {
		return
	}
	layout.Shape.Focusable = false
	for i := range layout.Children {
		clearSubtreeFocusable(&layout.Children[i], depth+1)
	}
}
