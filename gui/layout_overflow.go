package gui

// layoutOverflow hides children that don't fit in an overflow container.
func layoutOverflow(layout *Layout, w *Window) {
	for i := range layout.Children {
		layoutOverflow(&layout.Children[i], w)
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
	if changed, path, depth := constrainFitContainerWidth(layout); changed {
		refitFitAncestors(layout, path[:depth])
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

	var used float32
	visibleCount := 0
	firstHideIdx := triggerIdx // child index where hiding starts

	for i := range triggerIdx {
		child := &layout.Children[i]
		if skipLayoutChild(child.Shape) {
			continue
		}
		var gap float32
		if used > 0 {
			gap = spacing
		}
		needed := used + gap + child.Shape.Width
		if needed+spacing+triggerW > available {
			firstHideIdx = i
			break
		}
		used = needed
		visibleCount++
	}

	if visibleCount >= triggerIdx {
		hideOverflowChild(&layout.Children[triggerIdx])
		visibleCount = triggerIdx
	} else {
		for i := firstHideIdx; i < triggerIdx; i++ {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			hideOverflowChild(&layout.Children[i])
		}
	}

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

func hideOverflowChild(child *Layout) {
	child.Shape.shapeType = shapeNone
	child.Shape.Width = 0
	child.Shape.Clip = true
}
