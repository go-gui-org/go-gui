package gui

// layoutSizer is a container that sizes its children itself instead of
// leaving them to the fill passes: a Splitter's panes take their sizes
// from its ratio. The sizing passes stop at such a container
// (axisNoneMeasuresChildren), so its subtree has no sizes until
// sizeChildren gives them.
//
// sizeChildren pins each child's size and its X/Y relative to the
// container, then runs layoutSizes on it. It runs after the sizing
// passes and before every pass that positions the tree or reads its
// sizes into window state — the scroll clamp, scroll anchors and index
// scrolls. Run any later and those passes act on the unsized subtree,
// and what they store outlives the sizes given afterwards (#1214).
type layoutSizer interface {
	sizeChildren(layout *Layout, w *Window)
}

// setLayoutSizer makes s the sizer of a generated container. The
// container must have event handlers to hold it (both sizers have an
// AmendLayout); an invisible one has none, and nothing to size. It is
// set on the generated shape rather than through a ContainerCfg field,
// which would grow every container's view into the next size class.
func setLayoutSizer(layout *Layout, s layoutSizer, w *Window) {
	if layout.Shape == nil || layout.Shape.events == nil {
		return
	}
	layout.Shape.events.sizer = s
	w.scratch.viewSizers++
}

// layoutSizeChildren gives each layoutSizer in the tree its turn,
// parents first, so a sizer nested inside another's child has its own
// size before it sizes its children. A frame that generated no sizer
// skips the walk.
func layoutSizeChildren(layout *Layout, w *Window) {
	if w.scratch.viewSizers == 0 {
		return
	}
	layoutSizeChildrenDepth(layout, w, 0)
}

func layoutSizeChildrenDepth(layout *Layout, w *Window, depth int) {
	if overMaxDepth(depth) {
		return
	}
	if ev := layout.Shape.events; ev != nil && ev.sizer != nil {
		ev.sizer.sizeChildren(layout, w)
	}
	for i := range layout.Children {
		layoutSizeChildrenDepth(&layout.Children[i], w, depth+1)
	}
}
