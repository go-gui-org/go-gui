package gui

// A scroll link makes a container that does not scroll follow the
// horizontal offset of one that does (#1293). A frozen table header
// uses it: the header stays put vertically, but moves sideways with
// the body, so the columns stay in line.
//
// The offset is read while children are positioned, after
// layoutAdjustScrollOffsets clamped it for this frame. Reading it
// during view generation instead, the way DataGrid's frozen header did
// before ContainerCfg.ScrollLinkX existed, is one frame late whenever
// the clamp moves the offset (a window resize while scrolled to the
// end), and the header then sits out of line with the body for that
// frame.
//
// A wheel or trackpad scroll over the linked container moves the
// container it follows, so the header scrolls the table like the body
// does.

// scrollLinkX returns the key of the scroll container whose X offset
// s follows, or "" when s follows none.
func (s *Shape) scrollLinkX() string {
	if s.events == nil {
		return ""
	}
	return s.events.scrollLinkX
}

// scrollLinkedX returns the current X offset of the container s
// follows. ok is false when s follows none, when that container is not
// a Scrollable container of this frame, or when its offset is unset.
// The store keeps an offset after its container is gone, so the frame's
// set of scroll containers (layoutAdjustScrollOffsets) decides, not the
// store.
func scrollLinkedX(s *Shape, w *Window) (offset float32, ok bool) {
	key := s.scrollLinkX()
	if key == "" || w == nil {
		return 0, false
	}
	if _, live := w.scratch.scrollLive[key]; !live {
		return 0, false
	}
	return w.scrollX().Get(key)
}

// scrollWheelTarget returns the container a wheel over layout scrolls:
// layout itself when it scrolls, the container it follows when it has
// a scroll link, or nil.
func scrollWheelTarget(layout *Layout, w *Window) *Layout {
	if layout.Shape.Scrollable {
		return layout
	}
	key := layout.Shape.scrollLinkX()
	if key == "" {
		return nil
	}
	target, ok := w.layout.findByID(key)
	if !ok || target.Shape == nil || !target.Shape.Scrollable ||
		target.Shape.Disabled {
		return nil
	}
	return target
}
