package gui

// view_scrollbar_look.go — ScrollbarCfg.Thumb and Track hooks (#664,
// docs/specs/scrollbar-look-hook.md).
//
// The stock thumb is a childless container that scrollbarAmendLayout
// sizes and moves after arrange. A hook view has children, and they are
// laid out at generation for the size the thumb had then. Running the
// sizing passes again on a laid-out subtree is not safe (they add to the
// sizes already there), so the hook view is built at the size the last
// pass gave the thumb, and the amend asks for one more pass when that
// size changed. The frame loop runs it in the same frame, as it does for
// a new hover target.

// scrollbarLookSizes is the size each part had in the last layout pass.
type scrollbarLookSizes struct {
	thumbW, thumbH float32
	trackW, trackH float32
}

// scrollbarLookView defers the hooks to generation, where the effective
// ID, and so the hover and press state, can be read.
type scrollbarLookView struct {
	cfg ScrollbarCfg
}

func (v scrollbarLookView) GenerateLayout(w *Window) Layout {
	cfg := v.cfg
	vertical := cfg.Orientation != scrollbarHorizontal
	if cfg.ID == "" {
		cfg.ID = "scrollbar-x"
		if vertical {
			cfg.ID = "scrollbar-y"
		}
	}
	eid := w.EffID(cfg.ID)
	is := w.interactionState(eid)
	var sizes scrollbarLookSizes
	if m := StateMapRead[string, scrollbarLookSizes](w, nsScrollbarLook); m != nil {
		sizes, _ = m.Get(eid)
	}
	state := ScrollbarState{Hovered: is.Hovered, Pressed: is.Pressed, Vertical: vertical}

	content := make([]View, 0, 2)
	trackAt := -1
	if cfg.Track != nil {
		trackAt = len(content)
		ts := state
		ts.Width, ts.Height = sizes.trackW, sizes.trackH
		content = append(content, scrollbarPartBox(sizes.trackW, sizes.trackH, nil, cfg.Track(ts)))
	}
	thumbAt := len(content)
	if cfg.Thumb != nil {
		ts := state
		ts.Width, ts.Height = sizes.thumbW, sizes.thumbH
		content = append(content, scrollbarPartBox(sizes.thumbW, sizes.thumbH,
			makeScrollbarOnMouseDown(cfg), cfg.Thumb(ts)))
	} else {
		content = append(content, scrollbarThumb(cfg))
	}

	hovered := is.Hovered
	bar := ContainerCfg{
		ID:                   cfg.ID,
		A11YRole:             AccessRoleScrollBar,
		Color:                cfg.ColorBackground,
		OverDraw:             true,
		Padding:              NoPadding,
		scrollbarOrientation: cfg.Orientation,
		AmendLayout: func(ctx EventCtx) {
			scrollbarAmendLayoutLook(cfg, ctx, thumbAt, trackAt, hovered)
		},
		OnHover: makeScrollbarOnHover(cfg, thumbAt),
		OnClick: makeScrollbarGutterClick(cfg),
		Content: content,
	}
	if vertical {
		return generateViewLayout(Column(bar), w)
	}
	return generateViewLayout(Row(bar), w)
}

// scrollbarPartBox holds a hook view at the size the part had last pass.
// A zero size, before the first pass, sizes to the content instead.
func scrollbarPartBox(width, height float32, onClick func(EventCtx), view View) View {
	return Column(ContainerCfg{
		Width:      width,
		Height:     height,
		Sizing:     FixedFixed,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		OnClick:    onClick,
		Content:    []View{view},
	})
}

// scrollbarAmendLayoutLook places the parts with the stock geometry, moves
// each hook view's children with its box, hides a hook thumb when the
// stock one would be hidden, and records the sizes for the next pass.
func scrollbarAmendLayoutLook(cfg ScrollbarCfg, ctx EventCtx, thumbAt, trackAt int, hovered bool) {
	layout := ctx.Layout
	if len(layout.Children) <= thumbAt {
		return
	}
	thumb := &layout.Children[thumbAt]
	tx, ty := thumb.Shape.X, thumb.Shape.Y
	scrollbarAmendLayout(cfg, ctx, layout, ctx.Window, thumbAt)
	if cfg.Thumb != nil {
		layoutShiftChildren(thumb, thumb.Shape.X-tx, thumb.Shape.Y-ty)
	}

	var sizes scrollbarLookSizes
	if trackAt >= 0 {
		track := &layout.Children[trackAt]
		dx, dy := layout.Shape.X-track.Shape.X, layout.Shape.Y-track.Shape.Y
		track.Shape.X, track.Shape.Y = layout.Shape.X, layout.Shape.Y
		track.Shape.Width, track.Shape.Height = layout.Shape.Width, layout.Shape.Height
		layoutShiftChildren(track, dx, dy)
		sizes.trackW, sizes.trackH = track.Shape.Width, track.Shape.Height
	}

	if cfg.Thumb != nil {
		// The stock geometry hides the thumb by painting it transparent.
		// A hook view paints its own colors, so hide the whole subtree
		// instead. In hover-only mode the pointer brings it back.
		hidden := thumb.Shape.Color == ColorTransparent &&
			(cfg.Overflow != scrollbarOnHover || !hovered)
		if hidden {
			thumb.Shape.shapeType = shapeNone
			thumb.Shape.Width, thumb.Shape.Height = 0, 0
			thumb.Shape.Clip = true
		} else {
			thumb.Shape.Color = Color{}
			sizes.thumbW, sizes.thumbH = thumb.Shape.Width, thumb.Shape.Height
		}
	}

	eid := layout.Shape.idKey()
	m := StateMap[string, scrollbarLookSizes](ctx.Window, nsScrollbarLook, capModerate)
	if old, ok := m.Get(eid); !ok || old != sizes {
		m.Set(eid, sizes)
		// InvalidateLayout only sets a flag and posts a wake, so it is
		// safe under the frame lock.
		ctx.Window.InvalidateLayout()
	}
}

// layoutShiftChildren moves every descendant of layout, but not layout
// itself, whose position the caller has already set.
func layoutShiftChildren(layout *Layout, dx, dy float32) {
	for i := range layout.Children {
		layoutShift(&layout.Children[i], dx, dy)
	}
}
