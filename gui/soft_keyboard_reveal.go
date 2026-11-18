package gui

// Keeping the focused field above the soft keyboard (issue #1109).
//
// layoutRect already lays the view out in the part of the window above
// the keyboard. That keeps a FillFill view and the dialogs on screen, but
// a field inside a scroll container can still sit below the visible part
// of it. This pass scrolls each scroll ancestor of the focused field just
// far enough to show the field.
//
// It runs once per trigger, not every frame, so the user can still scroll
// away from the field while typing. The triggers are:
//
//   - the keyboard grows while a field has focus (it opens, or a
//     suggestion bar appears): handleSoftKeyboardEvent sets
//     reveal.pending;
//   - focus moves while the keyboard is up: the focus ID differs from
//     the one the last frame saw (reveal.focusID).
//
// A shrinking keyboard reveals nothing: the field can only gain room.

// layoutRevealFocused scrolls the focused field into view above the soft
// keyboard when a trigger is pending. It runs inside layoutPipeline after
// the position passes and before the clips are set, and moves the
// already-positioned children by the same amount it changes the offset.
// So the frame that first lays out the smaller area also shows the
// field: there is no frame with the field covered, and no visible jump
// for the eye to follow. The keyboard's own slide animates the motion.
func layoutRevealFocused(layout *Layout, w *Window) {
	if w.Config.SoftKeyboardOverlays || w.softKeyboardInset <= 0 {
		return
	}
	focusID := w.FocusID()
	if focusID == "" {
		return
	}
	if !w.reveal.pending && focusID == w.reveal.focusID {
		return
	}
	// findLayoutByFocusID matches only a shape that can take focus, and
	// this runs after layoutDisables: a field disabled this frame is not
	// found, and the field the focus repair moves to is revealed then.
	ly, ok := findLayoutByFocusID(layout, focusID)
	if !ok {
		// Not in this layer. A dialog holds its field in its own layer,
		// and the pipeline runs this again for that one.
		return
	}
	if ly == layout {
		// The focused shape is the layer root itself (a focusable
		// float). Nothing in this layer contains it, and its Parent
		// leads into another layer, which the walk below must not
		// enter: it would never meet the p == layout stop.
		return
	}
	visBot := w.layoutRect().Height
	// Walk out from the field. The inner scroll container moves first;
	// each move carries the field with it, so the next container out
	// tests where the field now is. An outer move also carries the inner
	// containers, so one the walk skipped (its viewport wholly under the
	// keyboard) can now be in view with the field outside it: walk again
	// until nothing moves. Each walk only moves a container toward
	// showing the field, so it settles in a few walks; the cap stops a
	// degenerate tree from looping.
	for range revealMaxWalks {
		if !revealWalk(ly, layout, visBot, w) {
			break
		}
	}
	revealStopHidingEases(ly, layout, visBot, w)
}

// revealStopHidingEases stops an ease, on a scroll ancestor the walks
// left alone, whose target would carry the shown field out of view: an
// older ScrollVerticalToSmooth easing back to the top, for one. An ease
// that keeps the field in view is the app's own request and runs on,
// and a pending index scroll is never dropped here (scrollSmoothStop,
// not scrollSmoothCancel): this pass writes no offset to that
// container. A container a walk moved was already cancelled.
func revealStopHidingEases(ly, layout *Layout, visBot float32, w *Window) {
	top := ly.Shape.Y
	bot := ly.Shape.Y + ly.Shape.Height
	sy := w.scrollY()
	for p := ly.Parent; p != nil; p = p.Parent {
		if s := p.Shape; s != nil && s.Scrollable &&
			s.ScrollMode != ScrollHorizontalOnly && s.idKey() != "" {
			id := s.idKey()
			if target, ok := scrollSmoothTarget(w, id, scrollAxisY); ok {
				// A fling aims past the range and the apply pass stops it
				// at the edge (scrollFlingEdge): project the field to the
				// offset the fling can reach, not to its raw target.
				target = f32Clamp(target, scrollMaxOffsetY(p), 0)
				// Where the field lands when the ease arrives, by the
				// distance the ease still has to go.
				d := target - sy.GetOr(id, 0) // Default 0: never scrolled.
				// It hides the field when a reveal from the target would
				// move: the clamp is against the target, not the current
				// offset, which can sit at a range limit the target does
				// not (an ease from the end of the content back to 0).
				if f32IsFinite(d) {
					if dy := revealNeedY(p, top+d, bot+d, visBot); dy != 0 &&
						f32Clamp(target+dy, scrollMaxOffsetY(p), 0) != target {
						scrollSmoothStop(w, id, scrollAxisY)
					}
				}
			}
		}
		if p == layout {
			break
		}
	}
}

// revealMaxWalks caps the walks of one reveal. Two cover an inner
// container that only an outer move brings into view; more cover deeper
// nests of the same kind.
const revealMaxWalks = 4

// revealWalk runs one walk from the field ly up to the layer root
// layout, scrolling each vertical scroll ancestor that hides the field.
// It reports whether any container moved.
func revealWalk(ly, layout *Layout, visBot float32, w *Window) bool {
	// Positions are shifted in place, so the field's shape holds where
	// it is now, after any earlier walk.
	top := ly.Shape.Y
	bot := ly.Shape.Y + ly.Shape.Height
	sy := w.scrollY()
	moved := false
	// The walk stops at the layer root: a float's Parent still points at
	// the container it was declared in, in another layer, whose
	// positions this pass must not touch.
	for p := ly.Parent; p != nil; p = p.Parent {
		dy := revealScrollBy(p, top, bot, visBot, w)
		if dy != 0 {
			moved = true
			id := p.Shape.idKey()
			old := sy.GetOr(id, 0) // Default 0: never scrolled.
			sy.Set(id, old+dy)
			// The reveal is a jump to a place. An ease still running
			// toward its old target would pull the field back under the
			// keyboard on its next tick.
			scrollSmoothCancel(w, id, scrollAxisY)
			// Positions are absolute after the position pass, so moving
			// the offset does not move the children (see scrollAnchor).
			for i := range p.Children {
				scrollAnchorShiftY(&p.Children[i], dy)
			}
			fireOnScroll(p, w)
			top += dy
			bot += dy
		}
		if p == layout {
			break
		}
	}
	return moved
}

// revealScrollBy returns how far the vertical scroll container p must
// move its content so the span [top, bot] shows in its viewport, or 0
// when p is not a vertical scroll container or the span already shows.
// The viewport is cut at visBot, the top of the soft keyboard: a
// container that runs under the keyboard (a fixed height, a non-Fill
// root) shows nothing below it. The result keeps the offset in the
// container's range, so a field at the end of short content moves only
// as far as the content can.
func revealScrollBy(p *Layout, top, bot, visBot float32, w *Window) float32 {
	dy := revealNeedY(p, top, bot, visBot)
	if dy == 0 {
		return 0
	}
	// Default 0: never scrolled.
	old := w.scrollY().GetOr(p.Shape.idKey(), 0)
	target := f32Clamp(old+dy, scrollMaxOffsetY(p), 0)
	if !f32IsFinite(target) {
		return 0
	}
	return target - old
}

// revealNeedY returns how far the vertical scroll container p would move
// its content to show the span [top, bot], before any clamp to its scroll
// range, or 0 when p is not a vertical scroll container with an id or the
// span already shows.
func revealNeedY(p *Layout, top, bot, visBot float32) float32 {
	s := p.Shape
	if s == nil || !s.Scrollable || s.ScrollMode == ScrollHorizontalOnly {
		return 0
	}
	if s.idKey() == "" {
		return 0
	}
	viewTop := s.Y + s.PaddingTop()
	viewBot := min(s.Y+s.Height-s.Padding.Bottom-s.SizeBorder, visBot)
	if viewBot <= viewTop || !f32IsFinite(viewTop) || !f32IsFinite(viewBot) {
		// Degenerate: no room to show anything, or the container is
		// wholly under the keyboard. Scrolling cannot help.
		return 0
	}
	var dy float32
	switch {
	case bot > viewBot:
		// Below: bring the lower edge up to the viewport bottom. A field
		// taller than the viewport would lose its top that way, and the
		// top holds the caret's first line, so show the top instead.
		dy = max(viewBot-bot, viewTop-top)
	case top < viewTop:
		dy = viewTop - top
	default:
		return 0
	}
	return dy
}

// revealState is the main-thread frame state behind the reveal pass.
type revealState struct {
	// focusID is the focus ID the last frame ended with. A different
	// one while the keyboard is up asks for a reveal.
	focusID string
	// pending is set when the inset grows, and asks the next layout
	// for a reveal.
	pending bool
}

// endRevealFrame records what the reveal pass has seen, once the whole
// frame is arranged. focusID is the focus the layout ran with, read
// before the focus repair: a focus the repair moved is then unseen, and
// the next frame reveals it. A trigger is spent whether or not a scroll moved: a
// field with no scroll container to move cannot be revealed later
// either, and a pending trigger must not fire on a frame long after the
// event that raised it.
func (w *Window) endRevealFrame(focusID string) {
	w.reveal.pending = false
	w.reveal.focusID = focusID
	// The repair runs with no event behind it, so nothing else asks for
	// the frame that reveals the field it moved focus to. Without this
	// the field stays under the keyboard until the next input event.
	// The update loop already took its refresh bits, so this one carries
	// over to the next frame (as refreshOverflow does).
	if now := w.FocusID(); now != focusID && now != "" &&
		w.softKeyboardInset > 0 && !w.Config.SoftKeyboardOverlays {
		w.markLayoutRefresh(refreshReveal)
	}
}

// revealKeepFloatTop returns the y a top-level float is placed at, given
// fy from floatAttachLayout. A centred float taller than the area above the
// keyboard would be centred with its top, where a dialog keeps its title
// and first field, off the window. It keeps the top at the top of the
// area instead and lets the bottom run under the keyboard. Only while
// the keyboard shrinks the area: desktop and SoftKeyboardOverlays keep
// the centring they had before #1109.
func revealKeepFloatTop(layout *Layout, fy float32, w *Window) float32 {
	place := w.layoutRect()
	// Only a vertically centred placement (a dialog): a float tied to a
	// top or bottom edge sits where its anchor and offset put it.
	if s := layout.Shape; layout.Parent == nil && s != nil && s.Float &&
		floatAttachMiddleRow(s.FloatAnchor) && floatAttachMiddleRow(s.FloatTieOff) &&
		s.Height > place.Height && fy < place.Y &&
		place.Height < w.windowRect().Height {
		return place.Y
	}
	return fy
}

// floatAttachMiddleRow reports whether a is in the vertical middle row
// (FloatMiddleLeft, FloatMiddleCenter, FloatMiddleRight).
func floatAttachMiddleRow(a FloatAttach) bool {
	return a >= FloatMiddleLeft && a <= FloatMiddleRight
}

// layoutRect returns the part of the window that layout places content
// in: the window less the height the soft keyboard covers at the bottom
// (issue #1109). The FillFill root is pinned to it and top-level floats
// (dialogs, toasts) are placed against it, so nothing the user needs
// lands under the keyboard. With the keyboard down, or on desktop, it
// equals windowRect.
//
// Only placement uses it. The root clip, pointer tests and the frame
// clear keep windowRect: the strip under the keyboard is still part of
// the window, and while the keyboard slides in it must show the
// background, not stale pixels.
func (w *Window) layoutRect() drawClip {
	r := w.windowRect()
	if !w.Config.SoftKeyboardOverlays {
		// handleSoftKeyboardEvent clamps the inset to [0, window
		// height], so the height stays >= 0. A window resized smaller
		// after the report can leave an inset above the new height;
		// max keeps the rect from going negative until the next report.
		r.Height = max(0, r.Height-w.softKeyboardInset)
	}
	return r
}
