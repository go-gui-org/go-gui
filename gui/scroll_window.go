package gui

import "math"

// Scroll windows keep part of a row in place while the rest scrolls
// sideways (#1363). A Table with frozen columns uses them.
//
// The scroll body states how wide its content is (scrollExtentX) and
// keeps its rows in place. Each row is as wide as the viewport and holds
// the frozen cells next to a scroll window: a clipping container with
// the other cells, linked to the body by ScrollLinkX. The window moves
// its cells by the body's X offset. The scrolled cells are cut off at
// the window's edge, not covered, so the frozen part needs no opaque
// fill and a row stays one container with one fill and one click
// handler.
//
// In a viewport too narrow for the frozen cells and the narrowest window
// (scrollExtentMin), the rows are wider than the viewport. The first
// part of the scroll range, the lead, then moves the rows like a plain
// scroll container does, and the windows move by the rest. Without it
// the cells past the viewport's edge could not be reached.

// scrollLive is how a Scrollable container of this frame moves its
// content sideways; see scratchPools.scrollLive.
type scrollLive struct {
	// lead is the largest distance the container moves its own children
	// (scrollLead).
	lead float32
	// mirrors is scrollMirrorsX.
	mirrors bool
}

func scrollLiveOf(s *Shape) scrollLive {
	return scrollLive{lead: scrollLead(s), mirrors: scrollMirrorsX(s)}
}

// scrollExtent returns the content width s states, and whether it states
// one. Only a Scrollable container does; a non-finite width is ignored
// rather than poisoning the scroll range.
func (s *Shape) scrollExtent() (float32, bool) {
	if s.events == nil || !s.Scrollable {
		return 0, false
	}
	e := s.events.scrollExtentX
	return e, e > 0 && f32IsFinite(e)
}

// scrollLead is how far s moves its own children sideways at most.
// A plain scroll container moves them by its whole offset.
func scrollLead(s *Shape) float32 {
	if _, ok := s.scrollExtent(); !ok {
		return math.MaxFloat32
	}
	lead := s.events.scrollExtentMin - (s.Width - s.paddingWidth())
	if !f32IsFinite(lead) {
		return 0
	}
	return f32Max(0, lead)
}

// scrollOwnX is the part of the X offset that a container with that lead
// moves its own children by. Offsets run from 0 down.
func scrollOwnX(offset, lead float32) float32 {
	return f32Max(offset, -lead)
}

// scrollWindowClip returns the clip a scroll window's children are drawn
// in: box, the clip of any other container, opened up to the inherited
// clip on the Y axis. The window is only as high as its row, and a cell
// may draw past that as it does outside a window: the focus ring of a
// control in a cell is a glow around it. Any other shape's clip is
// returned as it is. Hit testing needs no such step: there every
// container, clipping or not, bounds its children.
func scrollWindowClip(s *Shape, box, inherited drawClip) drawClip {
	if s.events == nil || !s.events.scrollWindow ||
		box.Width <= 0 || box.Height <= 0 {
		return box
	}
	box.Y, box.Height = inherited.Y, inherited.Height
	return box
}
