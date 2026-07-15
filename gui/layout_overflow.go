package gui

// layoutOverflow hides children that don't fit in an overflow container.
func layoutOverflow(layout *Layout, w *Window) {
	for i := range layout.Children {
		layoutOverflow(&layout.Children[i], w)
	}

	if !layout.Shape.Overflow || layout.Shape.Axis != AxisLeftToRight ||
		len(layout.Children) < 2 || layout.Shape.Scrollable {
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
	old, ok := om.Get(layout.Shape.ID)
	if !ok {
		old = -1
	}
	if old != visibleCount {
		om.Set(layout.Shape.ID, visibleCount)
		ss := StateMap[string, bool](w, nsSelect, capModerate)
		ss.Delete(layout.Shape.ID)
		w.refreshLayout = true
	}
}

func hideOverflowChild(child *Layout) {
	child.Shape.shapeType = shapeNone
	child.Shape.Width = 0
	child.Shape.Clip = true
}
