package gui

// View is a user-defined view. Views are never displayed
// directly. A Layout is generated from the View. Window does
// not hold a reference to a View. Views should be stateless.
//
// Pipeline: View -> generateViewLayout -> Layout ->
// layoutArrange -> renderLayout -> []RenderCmd
type View interface {
	Content() []View
	GenerateLayout(w *Window) Layout
}

// ensureLayoutShape normalizes layout nodes so pipeline passes can
// safely dereference Shape fields.
func ensureLayoutShape(layout *Layout) {
	if layout == nil {
		return
	}
	if layout.Shape == nil {
		layout.Shape = &Shape{shapeType: shapeNone}
	}
}

// GenerateViewLayout recursively builds a Layout tree from a
// View tree. Each View produces its own layout, then child
// Views are appended.
//
// Deprecated: Internal pipeline function. Not part of the stable
// public API. Will be removed in a future version. Use
// View.GenerateLayout for individual layout generation.
func GenerateViewLayout(view View, w *Window) Layout {
	return generateViewLayout(view, w)
}

// generateViewLayout is the internal recursive layout builder.
func generateViewLayout(view View, w *Window) Layout {
	layout := view.GenerateLayout(w)
	ensureLayoutShape(&layout)
	children := view.Content()
	if len(children) > maxEventChildren {
		children = children[:maxEventChildren]
	}
	// Pre-size to final length so append never reallocates.
	wantCap := len(layout.Children) + len(children)
	if cap(layout.Children) < wantCap {
		grown := make([]Layout, len(layout.Children), wantCap)
		copy(grown, layout.Children)
		layout.Children = grown
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		layout.Children = append(
			layout.Children,
			generateViewLayout(child, w),
		)
	}
	return layout
}
