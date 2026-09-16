package gui

import "slices"

// window_interaction_state.go — hover and press state a view reads
// while it is built (issue #587, docs/specs/build-time-interaction-state.md).
//
// Generation has no geometry for the frame it is building, so the
// answer comes from the last arranged frame: layoutArrange records the
// shape under the pointer, a left press records the shape it landed
// on, and IsHovered / IsPressed compare against those records. When the
// recorded hover target changes, the arrange pass asks for one more
// layout pass, which FrameFn runs in the same frame, so the new look is
// on screen in the frame the pointer moved.

// pointerOffWindow parks the pointer position after it leaves the
// window, far outside any shape, so the hover and mouse-leave passes
// see the exit too.
const pointerOffWindow float32 = -1 << 20

// IsHovered reports whether the pointer was over the widget with this
// effective ID, or over one of its ID-bearing descendants, in the last
// arranged frame. Read it while building a view to pick a look.
//
// effectiveID is the widget's effective ID: a leaf under an ID-bearing
// ancestor is addressed by its full path ("detail:nav"), not by the
// leaf its Cfg was written with. Inside GenerateLayout use
// w.EffID(cfg.ID); read one back with [Window.ResolveID].
//
// Only the topmost shape counts: a float, dialog or other shape drawn
// over the widget blocks it, even one with no handlers. That differs
// from OnHover, which is event dispatch and reaches the deepest
// OnHover handler under covering shapes. Disabled shapes never hover.
// While the mouse is locked (a drag), the value stays as it was.
//
// Change only what sits inside the widget's bounds (inner padding,
// colors, gradients, children). A look that moves or resizes the
// hovered shape itself can pull it out from under a still pointer,
// and the look then flips every frame.
//
// Known limits: a child with an absolute ID (one containing ":") does
// not make its parent hovered, and a window that is not focused
// receives no mouse moves, so it records no new hover.
//
// Main-thread only, like [Window.IsFocus]: no lock is taken.
// exportaudit:keep — public seam for views with their own GenerateLayout
func (w *Window) IsHovered(effectiveID string) bool {
	return targetWithin(w.viewState.hoverTargetID, effectiveID)
}

// IsPressed reports whether a left-button press (or touch) that
// started on the widget with this effective ID, or on one of its
// ID-bearing descendants, is still held. The press stays recorded
// when the pointer moves off the widget until the button is released;
// a look for "pressed and still over it" uses IsPressed(id) &&
// IsHovered(id).
//
// effectiveID follows the same rules as [Window.IsHovered]. Keyboard
// activation (ClickOnSpace, ClickOnEnter) does not set it: no key-held
// state exists.
//
// Main-thread only, like [Window.IsFocus]: no lock is taken.
// exportaudit:keep — public seam for views with their own GenerateLayout
func (w *Window) IsPressed(effectiveID string) bool {
	return targetWithin(w.viewState.pressTargetID, effectiveID)
}

// targetWithin reports whether target is id or lies under it: equal,
// or id followed by a ":" segment boundary. Compares in place, so it
// allocates nothing.
func targetWithin(target, id string) bool {
	if id == "" || len(target) < len(id) || target[:len(id)] != id {
		return false
	}
	return len(target) == len(id) || target[len(id)] == ':'
}

// recordHoverTarget stores the shape under the pointer after the
// arrange pass and asks for another layout pass when it changed, so a
// view reading IsHovered is rebuilt with the new answer. Runs under
// w.mu from layoutArrange; InvalidateLayout only sets a flag and posts
// a wake, so it is safe there.
func (w *Window) recordHoverTarget(layers []Layout) {
	if w.mouseIsLocked() {
		// Frozen, not cleared: a drag keeps the look it started with.
		return
	}
	target := ""
	if w.viewState.pointerInWindow {
		target = interactionTargetAt(layers,
			w.viewState.pointerX, w.viewState.pointerY, w)
	}
	if target == w.viewState.hoverTargetID {
		return
	}
	w.viewState.hoverTargetID = target
	w.InvalidateLayout()
}

// recordPressTarget stores the shape a left press lands on. Called
// where mouseButtonHeld is set, for backend presses and touch-synthesized
// ones alike, and hit-tests the last arranged frame the event was
// aimed at. A press while the mouse is locked belongs to the drag.
func (w *Window) recordPressTarget(e *Event) {
	if e.MouseButton != MouseLeft || w.mouseIsLocked() {
		return
	}
	w.viewState.pressTargetID = interactionTargetAt(w.layout.Children,
		e.MouseX, e.MouseY, w)
}

// pointerAt records where the pointer is for the next hover record: a
// mouse move, or a touch press or drag. mousePosX/Y stays the mouse's
// own, so a touch does not change OnHover dispatch.
func (w *Window) pointerAt(x, y float32) {
	w.viewState.pointerX = x
	w.viewState.pointerY = y
	w.viewState.pointerInWindow = true
}

// pointerLifted handles the last finger lifting: nothing stays
// hovered. mousePosX/Y is left alone, so OnHover dispatch is
// unchanged; the press is cleared by the synthesized release.
func (w *Window) pointerLifted() {
	w.viewState.pointerInWindow = false
	w.viewState.hoverTargetID = ""
}

// pointerLeftWindow handles a mouse exit from the window. Nothing stays
// hovered, and the position moves off-window so OnHover stops firing
// and OnMouseLeave fires on the next arrange pass. The press, if any,
// is left to the button release, which still arrives.
func (w *Window) pointerLeftWindow() {
	w.pointerLifted()
	w.viewState.mousePosX = pointerOffWindow
	w.viewState.mousePosY = pointerOffWindow
}

// interactionTargetAt returns the effective ID of the enabled,
// ID-bearing shape the point (x, y) is over, or "".
//
// Layers are tried topmost first, and the first layer holding any
// shape under the point decides, even when that shape carries no ID:
// what is drawn on top blocks what is below. While a dialog is visible
// only the dialog layer (always the last) is tried, the same rule
// event dispatch applies.
func interactionTargetAt(layers []Layout, x, y float32, w *Window) string {
	if len(layers) == 0 {
		return ""
	}
	if w.dialogCfg.visible {
		layers = layers[len(layers)-1:]
	}
	for i := range slices.Backward(layers) {
		if id, hit := interactionTargetDepth(&layers[i], x, y, 0); hit {
			return id
		}
	}
	return ""
}

// interactionTargetDepth finds the deepest shape under (x, y) in
// layout, children topmost first, then climbs back out of the recursion
// to the nearest enabled ID-bearing shape on that path. hit reports
// whether any shape was under the point; id is "" when the path held no
// usable ID. Climbing the recursion rather than Parent pointers keeps
// the walk inside the layer: a float's Parent still points at the tree
// it was lifted from.
func interactionTargetDepth(layout *Layout, x, y float32, depth int) (string, bool) {
	if overMaxDepth(depth) {
		return "", false
	}
	shape := layout.Shape
	if shape == nil {
		return "", false
	}
	// Children of a rotated container are hit-tested in its unrotated
	// frame, as layoutHoverDepth does.
	cx, cy := x, y
	if shape.QuarterTurns > 0 {
		cx, cy = rotateCoordsInverse(shape, x, y)
	}
	for i := range slices.Backward(layout.Children) {
		id, hit := interactionTargetDepth(&layout.Children[i], cx, cy, depth+1)
		if !hit {
			continue
		}
		if id != "" {
			return id, true
		}
		// A shape below this one was hit but carried no usable ID:
		// this shape is the nearest candidate on the path.
		return enabledIDKey(shape), true
	}
	if !shape.PointInShape(x, y) {
		return "", false
	}
	return enabledIDKey(shape), true
}

// enabledIDKey returns the shape's effective ID when it can be a hover
// or press target, otherwise "".
func enabledIDKey(s *Shape) string {
	if s.Disabled {
		return ""
	}
	return s.idKey()
}
