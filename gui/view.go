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

// ViewFunc adapts a function to a View, deferring construction to
// layout-generation time so the function can read window state. It is
// useful inside Content slices when a subtree needs State(*Window)
// but the enclosing function does not receive w.
type ViewFunc func(*Window) View

// Content satisfies the View interface. ViewFunc always
// returns empty content; its GenerateLayout defers to the
// wrapped function to build the subtree.
func (f ViewFunc) Content() []View { return nil }

// GenerateLayout calls the wrapped function and recursively
// builds the full Layout tree from the returned View.
func (f ViewFunc) GenerateLayout(w *Window) Layout {
	v := f(w)
	if v == nil {
		return Layout{}
	}
	return generateViewLayout(v, w)
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
// This is the supported entry point for composite View widgets.
// View.GenerateLayout produces a single node and does not recurse;
// GenerateViewLayout walks Content() to build the whole subtree. A
// widget package assembling a tree from child Views wants this one.
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
	// Pre-size to final length so append never reallocates. Child
	// slices come from a frame-scoped arena (reset each frame in
	// resetViewPools) to avoid a per-node heap allocation; the
	// reservation is pinned to wantCap so appends stay in-region.
	wantCap := len(layout.Children) + len(children)
	if cap(layout.Children) < wantCap {
		var grown []Layout
		if w != nil {
			grown = w.scratch.takeLayoutChildren(wantCap)
		} else {
			grown = make([]Layout, 0, wantCap)
		}
		grown = grown[:len(layout.Children)]
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
