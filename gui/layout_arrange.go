package gui

import (
	"cmp"
	"slices"
)

// layoutArrange is the top-level layout orchestrator. It sets
// parents, extracts floats, injects toast/dialog overlays, runs
// the pipeline on each layer, and processes hover in reverse
// layer order.
func layoutArrange(layout *Layout, w *Window) []Layout {
	ensureLayoutShape(layout)

	// Set parent pointers.
	layoutParents(layout, nil)

	// Identities were stamped at generation. What is left is the
	// focusOwner references, which name an ancestor by leaf and so need
	// the ancestor stack a walk has and a stamp does not. It runs before
	// the floats are split out because focusKey is read downstream by
	// every pass — render, hover, focus, IME, spell check — and a float
	// lifted out of the tree has lost the ancestors the reference names.
	resolveFocusOwners(layout, w)

	// Extract floating layouts from main tree.
	floatingLayouts := w.scratch.takeFloatingLayouts(len(layout.Children))
	defer func() {
		w.scratch.putFloatingLayouts(floatingLayouts)
	}()
	// Extraction is pre-order, so a float that hosts a nested float comes
	// before it. The pipelines below run in this order, and the layers are
	// sorted by Z only after that: floatAttachLayout reads the host's
	// final position, so a nested float with a lower Z than its host must
	// not run first.
	layoutRemoveFloatingLayouts(layout, w, &floatingLayouts)
	extracted := len(floatingLayouts)

	// Inject inspector overlay as a floating layer.
	if inspectorSupported && w.inspectorEnabled {
		injectFloatingLayer(inspectorFloatingPanel(w), w, &floatingLayouts)
	}

	// Inject toast container as floating layer.
	if len(w.toasts) > 0 {
		injectFloatingLayer(toastContainerView(w), w, &floatingLayouts)
	}

	// Inject dialog as last floating layer (always on top).
	if w.dialogCfg.visible {
		injectFloatingLayer(dialogViewGenerator(w.dialogCfg), w, &floatingLayouts)
		if n := len(floatingLayouts); n > 0 {
			w.retainDialogFocus(floatingLayouts[n-1])
		}
	}

	// Run pipeline on main layout.
	layoutPipeline(layout, w)
	layouts := w.scratch.layerLayouts.take(1 + len(floatingLayouts))
	layouts = append(layouts, *layout)

	// Run pipeline on each floating layout.
	for _, fl := range floatingLayouts {
		// Note: clips haven't been set yet at this point, so
		// invisible floats still need the pipeline.
		layoutPipeline(fl, w)
		layouts = append(layouts, *fl)
	}

	// Layer order: extracted floats by Z (stable, so equal Z keeps
	// extraction order), then the injected overlays in the order they were
	// added, dialog last. cmp.Compare, not subtraction, which overflows
	// for extreme Z values.
	slices.SortStableFunc(layouts[1:1+extracted],
		func(a, b Layout) int {
			return cmp.Compare(a.Shape.FloatZIndex, b.Shape.FloatZIndex)
		})

	// Hover processing: topmost first. Only a handled OnHover stops the
	// walk, so OnHover still fires under a floating layer that handles
	// none. This differs from interactionTargetAt, where any shape under
	// the pointer blocks lower layers, and the split is deliberate:
	// OnHover is dispatch, IsHovered is visible state (#587,
	// TestInteractionStateFloatBlocksButOnHoverFires).
	for i := range slices.Backward(layouts) {
		if layoutHover(&layouts[i], w) {
			break
		}
	}
	// Mouse-leave processing: all layers, all shapes.
	for i := range layouts {
		layoutMouseLeave(&layouts[i], w)
	}
	// Build-time hover state: record what the pointer is over so the
	// next generation can read it through IsHovered.
	w.recordHoverTarget(layouts)

	return layouts
}

// injectFloatingLayer generates a view layout and appends it as a
// floating layer. No-op if v is nil.
func injectFloatingLayer(v View, w *Window, floatingLayouts *[]*Layout) {
	if v == nil {
		return
	}
	l := generateViewLayout(v, w)
	heap := w.scratch.allocFloatingLayout(l)
	layoutParents(heap, nil)
	// An injected overlay is generated outside the main tree, so it is
	// its own scope root: it picks up prefixes only from ID-bearing
	// shapes inside itself. generateViewLayout above cleared the scope
	// for exactly that reason, so the stamps inside it already agree.
	resolveFocusOwners(heap, w)
	*floatingLayouts = append(*floatingLayouts, heap)
}
