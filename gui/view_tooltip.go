package gui

import "time"

// tooltipState tracks active tooltip bounds and ID.
type tooltipState struct {
	hoverStart time.Time // when hover began
	id         string
	popupID    string   // cached popup ID string
	hoverID    string   // trigger currently hovered
	text       string   // RTF tooltip content
	blockKey   uint64   // FNV hash of owning RichText
	bounds     drawClip // absolute run bounds (mouse check)
	// The run's box, relative to the RTF block, that the float popup
	// hangs from: its offset from the block and its size.
	floatOffsetX, floatOffsetY float32
	floatWidth, floatHeight    float32
	// anchors lists the WithTooltip wrappers whose box held the pointer
	// in this arrange pass. resolveTooltipHover reads and empties it at
	// the end of the pass.
	anchors []tooltipAnchor
}

// tooltipAnchor is a WithTooltip wrapper that had the pointer in its
// box during an arrange pass. The shape is valid for that pass only.
type tooltipAnchor struct {
	shape   *Shape
	tipID   string
	popupID string
	delay   time.Duration
}

// setRun records the hovered run r, in the frame of the RTF block ly:
// its bounds and the box its float popup hangs from.
func (ts *tooltipState) setRun(ly *Layout, r drawClip) {
	// Readers test the window pointer against the bounds, so they are
	// kept in the window: the box of the run's corners mapped out
	// through any view transforms.
	local := drawClip{
		X:      ly.Shape.X + r.X,
		Y:      ly.Shape.Y + r.Y,
		Width:  r.Width,
		Height: r.Height,
	}
	drawn, mapped := xfBoxToWindowOK(ly, local)
	if !mapped {
		drawn = local
	}
	ts.bounds = drawn
	// The popup's anchor hangs in the window, outside any transform, so
	// it takes the run's box as drawn: the bounds, with their origin
	// mapped back into the block's frame, where float offsets are
	// given. A box that did not map is still in the block's frame, and
	// the run's own box stands in.
	ts.floatOffsetX, ts.floatOffsetY = r.X, r.Y
	ts.floatWidth, ts.floatHeight = r.Width, r.Height
	if x, y, ok := xfWindowToLocal(ly, drawn.X, drawn.Y, false); mapped && ok {
		ts.floatOffsetX, ts.floatOffsetY = x-ly.Shape.X, y-ly.Shape.Y
		ts.floatWidth, ts.floatHeight = drawn.Width, drawn.Height
	}
}

// clearText resets RTF-sourced tooltip state. No-op when
// text is empty so WithTooltip-managed state is preserved.
func (ts *tooltipState) clearText() {
	if ts.text == "" {
		return
	}
	ts.hoverID = ""
	ts.hoverStart = time.Time{}
	ts.id = ""
	ts.popupID = ""
	ts.text = ""
	ts.bounds = drawClip{}
	ts.floatOffsetX = 0
	ts.floatOffsetY = 0
	ts.floatWidth = 0
	ts.floatHeight = 0
	ts.blockKey = 0
}

// TooltipCfg configures a tooltip popup.
type TooltipCfg struct {
	TextStyle   TextStyle
	ID          string
	Content     []View
	Delay       time.Duration
	FloatZIndex int
	Padding     Padding
	Radius      Radius
	SizeBorder  Border
	OffsetX     Opt[float32]
	OffsetY     Opt[float32]
	Color       Color
	ColorBorder Color
	Anchor      Opt[FloatAttach]
	TieOff      Opt[FloatAttach]
}

// Tooltip creates a floating tooltip view.
// exportaudit:keep — collides with the tooltipState var
func Tooltip(cfg TooltipCfg) View {
	applyTooltipDefaults(&cfg)
	d := &defaultTooltipStyle
	return Column(ContainerCfg{
		ID:            cfg.ID,
		Float:         true,
		FloatAutoFlip: true,
		FloatAnchor:   cfg.Anchor.Get(FloatBottomCenter),
		FloatTieOff:   cfg.TieOff.Get(FloatTopCenter),
		FloatOffsetX:  cfg.OffsetX.Get(-3),
		FloatOffsetY:  cfg.OffsetY.Get(-3),
		FloatZIndex:   cfg.FloatZIndex,
		Shadow:        d.Shadow,
		Color:         cfg.Color,
		ColorBorder:   cfg.ColorBorder,
		SizeBorder:    BorderPx(cfg.SizeBorder.Or(d.SizeBorder)),
		Radius:        RadiusPx(cfg.Radius.Or(d.Radius)),
		Padding:       cfg.Padding,
		MaxWidth:      300,
		Content:       cfg.Content,
	})
}

// AnimationTooltip returns an Animate that checks mouse position
// after a delay and activates the tooltip if the mouse is still
// inside the trigger bounds.
func animationTooltip(cfg TooltipCfg) *Animate {
	delay := cfg.Delay
	if delay == 0 {
		delay = defaultTooltipStyle.Delay
	}
	id := cfg.ID
	return &Animate{
		AnimID: "___tooltip___",
		Delay:  delay,
		Callback: func(_ *Animate, w *Window) {
			ts := &w.viewState.tooltip
			b := ts.bounds
			mx := w.viewState.mousePosX
			my := w.viewState.mousePosY
			if ts.hoverID == id &&
				mx >= b.X && my >= b.Y &&
				mx < b.X+b.Width && my < b.Y+b.Height {
				ts.id = id
				ts.popupID = ScopeID(id, "popup")
			}
		},
	}
}

func applyTooltipDefaults(cfg *TooltipCfg) {
	d := &defaultTooltipStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Colors.Base
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.Colors.Border
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.Delay == 0 {
		cfg.Delay = d.Delay
	}
}

// WithTooltipCfg configures a tooltip wrapper.
type WithTooltipCfg struct {
	ID      string
	Text    string
	Content []View
	Delay   time.Duration
	Anchor  Opt[FloatAttach]
	TieOff  Opt[FloatAttach]
}

// WithTooltip wraps content and shows a tooltip on hover after
// a delay. Tooltip state is managed via AmendLayout.
//
// The build is deferred to layout generation. It is load-bearing
// rather than a style choice: withTooltipBuild resolves the tip ID
// with [Window.EffID] and keys the hover state, the popup's own ID and
// the AmendLayout closure on the result, and the generation-time ID
// scope is only live while the framework descends the View tree.
// Building here would resolve against the enclosing scope, so the same
// label in two ID-bearing panels would share one hover entry. See
// issue #528.
func WithTooltip(_ *Window, cfg WithTooltipCfg) View {
	return ViewFunc(func(w *Window) View {
		return withTooltipBuild(w, cfg)
	})
}

// withTooltipBuild assembles the wrapper under the ID scope of the
// panel it sits in.
func withTooltipBuild(w *Window, cfg WithTooltipCfg) View {
	tipID := cfg.ID
	if tipID == "" {
		tipID = cfg.Text
	}
	// Scope the tooltip's identity like any other widget key, so the
	// same label used in two ID-bearing panels tracks hover separately.
	// Both the state writes and the popup's shape ID derive from this
	// one string, so they cannot disagree.
	tipID = w.EffID(tipID)

	delay := cfg.Delay
	if delay == 0 {
		delay = defaultTooltipStyle.Delay
	}

	anchor := cfg.Anchor.Get(FloatBottomCenter)
	tieOff := cfg.TieOff.Get(FloatTopCenter)

	content := make([]View, 0, len(cfg.Content)+1)
	content = append(content, cfg.Content...)

	ts := &w.viewState.tooltip
	if ts.id == tipID {
		content = append(content, Tooltip(TooltipCfg{
			ID:     ts.popupID,
			Anchor: Some(anchor),
			TieOff: Some(tieOff),
			Content: []View{
				Text(TextCfg{
					Text:      cfg.Text,
					TextStyle: defaultTooltipStyle.TextStyle,
					Mode:      TextModeWrap,
				}),
			},
		}))
	}

	return Column(ContainerCfg{
		A11YRole:    AccessRoleGroup,
		A11YCfg:     A11YCfg{A11YDescription: cfg.Text},
		SizeBorder:  NoBorder,
		Content:     content,
		AmendLayout: withTooltipAmend(tipID, delay),
		// The hook tests the pointer against the wrapper's bounds, so
		// a move into them must rebuild (#973).
		pointerAmend: true,
	})
}

// withTooltipAmend returns the AmendLayout callback for a
// WithTooltip wrapper. It only records that the pointer is in the
// wrapper's box. Whether the wrapper is what the pointer is on is
// decided after the whole pass, in resolveTooltipHover: this hook runs
// inside its own layer's pipeline, before the layers above it are laid
// out, so it cannot see a dialog or a popup drawn over the wrapper
// (#1243).
func withTooltipAmend(
	tipID string, delay time.Duration,
) func(EventCtx) {
	popupID := ScopeID(tipID, "popup")
	return func(ctx EventCtx) {
		// The pointer is a window position; the shape's box is in its own
		// frame, which a view transform moves. No clip test here: layout
		// sets the clips after the AmendLayout hooks. The hit test in
		// resolveTooltipHover runs with them.
		mx, my, onView := xfWindowToLocal(ctx.Layout,
			ctx.Window.viewState.mousePosX, ctx.Window.viewState.mousePosY, false)
		// With no clip test the mapping does not refuse an inert view
		// (the turned-away face of a FlipPanel), so check it here: a
		// view that takes no pointer opens no tooltip.
		inside := onView && !layoutInert(ctx.Layout) &&
			mx >= ctx.Layout.Shape.X && my >= ctx.Layout.Shape.Y &&
			mx < ctx.Layout.Shape.X+ctx.Layout.Shape.Width &&
			my < ctx.Layout.Shape.Y+ctx.Layout.Shape.Height
		if !inside {
			return
		}
		ts := &ctx.Window.viewState.tooltip
		ts.anchors = append(ts.anchors, tooltipAnchor{
			shape:   ctx.Layout.Shape,
			tipID:   tipID,
			popupID: popupID,
			delay:   delay,
		})
	}
}

// resolveTooltipHover starts, keeps or ends the WithTooltip hover once
// every layer of the pass is placed and clipped. Of the wrappers that
// recorded the pointer in their box, the one that counts is the
// topmost widget under the pointer that has a tooltip (pointedAnchor).
// A dialog, an open dropdown or any other layer drawn over a wrapper,
// or a clip that hides that part of it, keeps its tooltip shut (#1243).
//
// Runs under w.mu from layoutArrange. A change to the shown tooltip
// asks for another layout pass, which FrameFn runs in the same frame,
// so a tooltip under a dialog that just opened goes in that frame.
func (w *Window) resolveTooltipHover(layers []Layout) {
	ts := &w.viewState.tooltip
	if len(ts.anchors) == 0 && ts.hoverID == "" {
		return
	}
	a, ok := ts.pointedAnchor(layers, w)
	clear(ts.anchors) // drop the pass's shape pointers
	ts.anchors = ts.anchors[:0]
	if ts.text != "" {
		// A RichText run holds the tooltip (view_rtf.go); it ends its
		// own hover.
		return
	}
	shown := ts.id != ""
	switch {
	case !ok:
		ts.hoverID = ""
		ts.hoverStart = time.Time{}
		ts.id = ""
		ts.popupID = ""

	case ts.hoverID != a.tipID:
		// A new hover, or the pointer went straight from one wrapper
		// to another: the old one's tooltip ends here.
		ts.hoverID = a.tipID
		ts.hoverStart = time.Now()
		if ts.id != a.tipID {
			ts.id = ""
			ts.popupID = ""
		}
		tipID, popupID := a.tipID, a.popupID
		w.AnimationAdd(&Animate{
			AnimID: "___tooltip___",
			Delay:  a.delay,
			Callback: func(_ *Animate, w *Window) {
				if w.viewState.tooltip.hoverID == tipID {
					w.viewState.tooltip.id = tipID
					w.viewState.tooltip.popupID = popupID
				}
			},
		})

	case ts.id == "" && time.Since(ts.hoverStart) >= a.delay:
		ts.id = a.tipID
		ts.popupID = a.popupID
	}
	if shown != (ts.id != "") {
		w.InvalidateLayout()
	}
}

// pointedAnchor returns the recorded wrapper the pointer is on: the
// first one met climbing from the topmost shape under the pointer
// through its ancestors, so the topmost widget there that has a
// tooltip wins. A nested wrapper's content is drawn over the wrapper
// around it, so on a button with a tooltip inside a card with one,
// the button's opens.
//
// At a float the climb leaves that float's layer for the tree it was
// lifted from only when nothing in the layer has an ID. Such a float
// is part of the widget it hangs from, like a ColorSwatch's color or a
// slider's track. A layer with an ID anywhere, such as a dropdown, a
// menu panel or a dialog, is drawn over the wrapper and ends the
// climb, also where the pointer is on its padding or a gap between
// its items. The wrapper's own tooltip popup has an ID but counts as
// the wrapper: it overlaps the wrapper's edge (TooltipCfg.OffsetY).
func (ts *tooltipState) pointedAnchor(layers []Layout, w *Window) (tooltipAnchor, bool) {
	if len(ts.anchors) == 0 {
		return tooltipAnchor{}, false
	}
	hit := layoutAtPoint(layers,
		w.viewState.mousePosX, w.viewState.mousePosY, w)
	// The depth cap also ends the climb on a malformed Parent cycle.
	for p, depth := hit, 0; p != nil && !overMaxDepth(depth); p, depth = p.Parent, depth+1 {
		if p.Shape == nil {
			continue
		}
		if i := ts.anchorIndex(p.Shape); i >= 0 {
			return ts.anchors[i], true
		}
		if p.Shape.Float && layerHasID(p, 0) && !ts.isAnchorPopup(p) {
			return tooltipAnchor{}, false
		}
	}
	return tooltipAnchor{}, false
}

// layerHasID reports whether l or a shape inside it has an ID, enabled
// or not. A float lifted out of l left an ID-less placeholder behind,
// so the walk stays in l's layer.
func layerHasID(l *Layout, depth int) bool {
	if overMaxDepth(depth) || l.Shape == nil {
		return false
	}
	if l.Shape.idKey() != "" {
		return true
	}
	for i := range l.Children {
		if layerHasID(&l.Children[i], depth+1) {
			return true
		}
	}
	return false
}

// anchorIndex returns the index of the recorded wrapper with shape s,
// or -1.
func (ts *tooltipState) anchorIndex(s *Shape) int {
	for i := range ts.anchors {
		if ts.anchors[i].shape == s {
			return i
		}
	}
	return -1
}

// isAnchorPopup reports whether the float layer root p is the tooltip
// popup of the recorded wrapper it was lifted from.
func (ts *tooltipState) isAnchorPopup(p *Layout) bool {
	if p.Parent == nil || p.Parent.Shape == nil {
		return false
	}
	i := ts.anchorIndex(p.Parent.Shape)
	return i >= 0 && p.Shape.idKey() == ts.anchors[i].popupID
}
