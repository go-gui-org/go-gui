package gui

// Layout defines a tree of layouts. Views generate Layouts.
type Layout struct {
	Shape    *Shape
	Parent   *Layout
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

// contentWidth returns total content width.
func contentWidth(layout *Layout) float32 {
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

// contentHeight returns total content height.
func contentHeight(layout *Layout) float32 {
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
