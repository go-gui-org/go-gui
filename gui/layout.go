package gui

// Layout is a node in the UI tree. Each frame, the active [View] function
// returns a [View] whose GenerateLayout produces its Layout tree; the
// layout engine then sizes and positions nodes ([layoutArrange]), and the
// renderer walks the tree to produce a flat []RenderCmd ([renderLayout]).
//
// Parents are pointers; children are values. This avoids reference cycles
// while allowing upward traversal during event dispatch and layout queries.
// The [Shape] pointer holds all visual/render state (position, size, color,
// events). Layout provides the tree structure (parent, children) plus
// animation offsets (opacity, translation) that the renderer blends with
// Shape values.
//
// Users construct Layouts indirectly via widget factory functions
// ([Button], [Text], [Column], etc.), which return [View] interfaces.
// Widget factories build the Layout tree for their subtree at
// generation time.
type Layout struct {
	// Shape holds the visual state for this node: position, size, color,
	// event handlers, text, and effects. Set by widget factories;
	// mutated by the layout engine (X, Y, Width, Height) and renderer.
	Shape *Shape

	// Parent is the containing Layout, or nil for the root. Set by
	// GenerateViewLayout during tree construction.
	Parent *Layout

	// Children holds the child Layouts in display order. The layout
	// engine iterates children to measure content and arrange positions.
	Children []Layout
}

// Depth policy for the layout and render walks, stated once here and
// referred to from each guarded walk.
//
// The event and focus walks have capped their recursion since
// maxEventDepth was added (event_handlers.go): the breadth guard cannot
// see a chain of single-child layouts, which recurses until the stack
// gives out. The layout and render passes walk the same tree and were
// left uncapped, so a tree deep enough to matter was refused input while
// still being measured, positioned and drawn.
//
// Each walk below therefore takes the same budget. Past it the walk stops
// descending rather than panicking: a node beyond the cap is left at its
// zero state — unsized, unpositioned, unpainted — which degrades the
// frame instead of the process. The cap is deliberately maxEventDepth
// rather than a second constant, so the depth at which input stops and
// the depth at which layout stops cannot drift apart.
//
// This is consistency with an existing decision, not a fix for a live
// crash: the layout tree comes from application code, and real trees nest
// dozens deep at most.

// layoutParents sets the parent pointer of all nodes.
//
// A node past the depth cap keeps a nil Parent, the same state a detached
// subtree already has.
func layoutParents(layout *Layout, parent *Layout) {
	layoutParentsDepth(layout, parent, 0)
}

func layoutParentsDepth(layout *Layout, parent *Layout, depth int) {
	if overMaxDepth(depth) {
		return
	}
	layout.Parent = parent
	for i := range layout.Children {
		layoutParentsDepth(&layout.Children[i], layout, depth+1)
	}
}

// layoutDisables walks the Layout and disables children that
// have a disabled ancestor.
func layoutDisables(layout *Layout, disabled bool) {
	layoutDisablesDepth(layout, disabled, 0)
}

func layoutDisablesDepth(layout *Layout, disabled bool, depth int) {
	if overMaxDepth(depth) {
		return
	}
	isDisabled := disabled || layout.Shape.Disabled
	layout.Shape.Disabled = isDisabled
	for i := range layout.Children {
		layoutDisablesDepth(&layout.Children[i], isDisabled, depth+1)
	}
}

// layoutPlaceholder returns an empty placeholder Layout.
func layoutPlaceholder() Layout {
	return Layout{
		Shape: &Shape{shapeType: shapeNone},
	}
}

// skipLayoutChild reports whether a child should be excluded
// from spacing, content-size, and overflow calculations.
func skipLayoutChild(s *Shape) bool {
	if s == nil {
		return true
	}
	return s.Float || s.shapeType == shapeNone || s.OverDraw
}

// spacing does the fence-post calculation for spacings.
func (layout *Layout) spacing() float32 {
	count := 0
	for i := range layout.Children {
		c := &layout.Children[i]
		if skipLayoutChild(c.Shape) {
			continue
		}
		count++
	}
	return float32(max(0, count-1)) * layout.Shape.Spacing
}

// contentWidth returns total content width. Uses the fill-pass cache
// when available to avoid redundant child-tree summation.
func contentWidth(layout *Layout) float32 {
	if layout.Shape.fillGen != 0 {
		return layout.Shape.contentW
	}
	return computeContentWidth(layout)
}

// childExtentW is how far one child reaches on the X axis: its Width,
// or Shape.inkOverflowW where painted ink reaches further. Only text
// that cannot wrap sets that field (see Shape.inkOverflowW), so for
// every other child this is just Width. A non-finite overflow is
// ignored rather than poisoning the sum.
func childExtentW(s *Shape) float32 {
	ink := s.inkOverflowW
	if !f32IsFinite(ink) {
		return s.Width
	}
	return f32Max(s.Width, ink)
}

// computeContentWidth iterates children to calculate total content
// width. Called during the fill pass to populate the cache and as a
// fallback when no cached value exists. Each child's extent comes
// from childExtentW.
func computeContentWidth(layout *Layout) float32 {
	var width float32
	if layout == nil || layout.Shape == nil {
		return 0
	}
	// Both axes measure a child the same way; only the combining
	// operator differs. See childExtentW.

	if layout.Shape.Axis == axisLeftToRight {
		width += layout.spacing()
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			width += childExtentW(c.Shape)
		}
	} else {
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			width = f32Max(width, childExtentW(c.Shape))
		}
	}
	return width
}

// contentHeight returns total content height. Uses the fill-pass cache
// when available to avoid redundant child-tree summation.
func contentHeight(layout *Layout) float32 {
	if layout.Shape.fillGen != 0 {
		return layout.Shape.contentH
	}
	return computeContentHeight(layout)
}

// computeContentHeight iterates children to calculate total content
// height. Called during the fill pass to populate the cache and as a
// fallback when no cached value exists.
func computeContentHeight(layout *Layout) float32 {
	var height float32
	if layout.Shape.Axis == axisTopToBottom {
		height += layout.spacing()
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			height += c.Shape.Height
		}
	} else {
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			height = f32Max(height, c.Shape.Height)
		}
	}
	return height
}
