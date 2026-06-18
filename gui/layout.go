package gui

// Layout is a node in the UI tree. Each frame, the active [View] function
// returns a Layout tree via [GenerateViewLayout]. The layout engine then
// sizes and positions nodes ([layoutArrange]), and the renderer walks the
// tree to produce a flat []RenderCmd ([renderLayout]).
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
// The Layout type itself is the concrete View implementation — every
// widget factory ultimately produces a Layout tree.
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

// layoutParents sets the parent pointer of all nodes.
func layoutParents(layout *Layout, parent *Layout) {
	layout.Parent = parent
	for i := range layout.Children {
		layoutParents(&layout.Children[i], layout)
	}
}

// layoutDisables walks the Layout and disables children that
// have a disabled ancestor.
func layoutDisables(layout *Layout, disabled bool) {
	isDisabled := disabled || layout.Shape.Disabled
	layout.Shape.Disabled = isDisabled
	for i := range layout.Children {
		layoutDisables(&layout.Children[i], isDisabled)
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

// computeContentWidth iterates children to calculate total content
// width. Called during the fill pass to populate the cache and as a
// fallback when no cached value exists.
func computeContentWidth(layout *Layout) float32 {
	var width float32
	if layout.Shape.Axis == AxisLeftToRight {
		width += layout.spacing()
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			width += c.Shape.Width
		}
	} else {
		for i := range layout.Children {
			c := &layout.Children[i]
			if skipLayoutChild(c.Shape) {
				continue
			}
			width = f32Max(width, c.Shape.Width)
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
	if layout.Shape.Axis == AxisTopToBottom {
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
