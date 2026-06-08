package gui

// wrapRowRange describes a contiguous range of children in one wrap row.
type wrapRowRange struct {
	start, end int
}

// layoutWrapContainers restructures wrap containers into column-of-rows.
func layoutWrapContainers(layout *Layout, w *Window) {
	for i := range layout.Children {
		layoutWrapContainers(&layout.Children[i], w)
	}

	if !layout.Shape.Wrap || layout.Shape.Axis != AxisLeftToRight || len(layout.Children) == 0 {
		return
	}

	available := layout.Shape.Width - layout.Shape.paddingWidth()
	if available <= 0 {
		return
	}

	spacing := layout.Shape.Spacing

	var rows []wrapRowRange
	if w != nil {
		rows = w.scratch.wrapRows.take(len(layout.Children))
		defer func() {
			w.scratch.wrapRows.put(rows)
		}()
	}
	rowStart := 0
	var rowWidth float32
	rowHasFlowChild := false

	for idx := range layout.Children {
		child := &layout.Children[idx]
		if skipLayoutChild(child.Shape) {
			continue
		}
		childW := child.Shape.Width
		var gap float32
		if rowHasFlowChild {
			gap = spacing
		}
		if rowHasFlowChild && rowWidth+gap+childW > available && idx > rowStart {
			rows = append(rows, wrapRowRange{start: rowStart, end: idx})
			rowStart = idx
			rowWidth = 0
			rowHasFlowChild = false
		}
		if rowHasFlowChild {
			rowWidth += spacing
		}
		rowWidth += childW
		rowHasFlowChild = true
	}
	if rowHasFlowChild && rowStart < len(layout.Children) {
		rows = append(rows, wrapRowRange{start: rowStart, end: len(layout.Children)})
	}

	if len(rows) <= 1 {
		return
	}

	layout.Shape.Axis = AxisTopToBottom

	newChildren := make([]Layout, 0, len(rows))
	for i := range rows {
		row := rows[i]
		rowChildren := layout.Children[row.start:row.end:row.end]
		rowShape := Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			Sizing:    FixedFit,
			Width:     available,
			Spacing:   spacing,
			Color:     Color{},
			HAlign:    layout.Shape.HAlign,
			VAlign:    layout.Shape.VAlign,
			TextDir:   layout.Shape.TextDir,
		}
		var sp *Shape
		if w != nil {
			sp = w.scratch.viewShapes.alloc(rowShape)
		} else {
			sp = &rowShape
		}
		newChildren = append(newChildren, Layout{
			Shape:    sp,
			Children: rowChildren,
		})
	}

	layout.Children = newChildren
	for i := range layout.Children {
		layout.Children[i].Parent = layout
		for j := range layout.Children[i].Children {
			layout.Children[i].Children[j].Parent = &layout.Children[i]
		}
	}
}
