package gui

import "math"

// Scroll windows keep part of a row in place while the rest scrolls
// sideways (#1363). A Table with frozen columns and a DataGrid with
// pinned ones (#1364) use them.
//
// The scroll body states how wide its content is (ScrollExtentX) and
// keeps its rows in place. Each row is as wide as the viewport and holds
// the frozen cells next to a scroll window: a clipping container with
// the other cells, linked to the body by ScrollLinkX. The window moves
// its cells by the body's X offset. The scrolled cells are cut off at
// the window's edge, not covered, so the frozen part needs no opaque
// fill and a row stays one container with one fill and one click
// handler.
//
// In a viewport too narrow for the frozen cells and the narrowest window
// (ScrollExtentMin), the rows are wider than the viewport. The first
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
//
// The sides snap to whole device pixels (snapClipX). Inside a transform
// the rect is not in window pixels, so it stays as it is.
func scrollWindowClip(s *Shape, box, inherited drawClip, w *Window) drawClip {
	if s.events == nil || !s.events.scrollWindow ||
		box.Width <= 0 || box.Height <= 0 {
		return box
	}
	box.Y, box.Height = inherited.Y, inherited.Height
	if len(w.xfStack) == 0 {
		box = snapClipX(box, w.BackingScale)
	}
	return box
}

// snapClipX moves the sides of a clip to the device pixels that the
// backends draw a box's sides at under that scale: each rounds
// (gpu.SnapRect). A backend rounds a clip outward (gpu.ClipRect). Under a
// fractional scale that let a filled cell draw up to one device pixel
// past the window, over the border of the frozen cell next to it.
func snapClipX(c drawClip, scale float32) drawClip {
	if scale <= 0 || !f32IsFinite(scale) {
		return c
	}
	s := float64(scale)
	// In float32, as SnapRect does. The conversions keep arm64 from
	// fusing the product into something wider.
	left := math.Round(float64(float32(c.X * scale)))
	right := math.Round(float64(float32(float32(c.X+c.Width) * scale)))
	if right <= left {
		return drawClip{}
	}
	// A float32 just beside the device pixel would round outward again.
	x := float32(left / s)
	for math.Floor(float64(x)*s) < left {
		x = math.Nextafter32(x, math.MaxFloat32)
	}
	width := float32(right/s - float64(x))
	for math.Ceil((float64(x)+float64(width))*s) > right {
		width = math.Nextafter32(width, 0)
	}
	c.X, c.Width = x, width
	return c
}

// ScrollWindowCfg configures [Window.ScrollWindow].
// exportaudit:keep — caller-facing config (issue #1364)
type ScrollWindowCfg struct {
	// ScrollID is the effective ID of the Scrollable container the window
	// follows. That container has a ScrollExtentX.
	ScrollID string
	// Content is what scrolls: the cells of a row between its frozen
	// ones, left to right.
	Content []View
	// MinWidth is the room the window keeps in a narrow container.
	MinWidth float32
	// MaxWidth is the width of Content, so that the window ends where its
	// content does in a wide container. 0 sets no limit.
	MaxWidth float32
	// Border is the width of the cell border when the cells of a row
	// overlap by it, each with its neighbour, as in a full grid. The
	// container of the row sets the same negative Spacing. The window
	// keeps its content off the border of the cell before it.
	Border float32
	// ClipEnd keeps the content off the border of the cell after the
	// window too. Set it when frozen cells follow.
	ClipEnd bool
}

// ScrollWindow builds the part of a row that scrolls sideways while the
// cells before and after it stay in place. Put it in a Row with those
// cells, and that Row in the scroll container that ScrollID names. The
// window fills the width the frozen cells leave and moves its content by
// the container's horizontal offset. It clips sideways only, so a focus
// ring in a cell is not cut above and below.
//
// The view comes from a pool that lasts one frame. Build it while the
// frame's views are built, and do not keep it.
func (w *Window) ScrollWindow(cfg ScrollWindowCfg) View {
	border := cfg.Border
	if border < 0 || !f32IsFinite(border) {
		border = 0
	}
	// The window overlaps the cell before it by the border, as any cell
	// does. The padding keeps the clip off that border, or cells would
	// draw over it while they scroll; the content itself starts at the
	// window's edge (scrollWindowInset).
	var end float32
	if cfg.ClipEnd {
		end = border
	}
	cv := containerView{
		cfg: ContainerCfg{
			axis:         axisLeftToRight,
			Sizing:       FillFill,
			MinWidth:     cfg.MinWidth,
			MaxWidth:     cfg.MaxWidth,
			Padding:      NewPadding(0, end, 0, border),
			Spacing:      SpacingPx(-border),
			SizeBorder:   NoBorder,
			Radius:       NoRadius,
			Clip:         true,
			ScrollLinkX:  cfg.ScrollID,
			scrollWindow: true,
			clickButton:  clickLeftOnly,
		},
		content: cfg.Content,
	}
	// Pooled: a window per row is the only view a split row adds.
	if w == nil {
		// A copy: the address of cv itself would move it to the heap on
		// the pooled path too.
		heapCV := cv
		return &heapCV
	}
	return w.scratch.viewWindows.alloc(cv)
}

// scrollWindowInset is how far a scroll window's content starts before
// its content box: its start padding, which is there for the clip only.
func scrollWindowInset(s *Shape) float32 {
	if s.events == nil || !s.events.scrollWindow {
		return 0
	}
	return s.Padding.Left
}
