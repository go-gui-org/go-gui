package gui

// view_virtual_list_build.go — the range arithmetic, the spacers, the
// measurement write-back and the keyboard handling for VirtualList.
// Split from view_virtual_list.go to keep both files inside the
// 800-line gate (./scripts/large-files.sh).

const (
	// virtualListProbeRows bounds the first frame under Fill sizing,
	// where no height is known yet. The ListBox precedent builds every
	// row in that situation; at a million rows that is a hang, so a
	// probe window is built instead — enough to let arrange record a
	// height, after which the real range takes over.
	virtualListProbeRows = 64

	// virtualListMaxBuiltRows caps how many rows one frame may build,
	// as a backstop against a corpus of one-pixel rows or a viewport
	// that resolved absurdly tall. The pixel overscan is the primary
	// control; this only bounds the pathological case.
	virtualListMaxBuiltRows = 512

	// virtualListOverscanFrac is the default overscan, as a fraction
	// of the viewport height on each edge.
	virtualListOverscanFrac = 0.5
)

// virtualListRange computes the row window to build plus the two
// spacer heights that stand in for everything outside it. probe
// reports the no-height-yet case, which the caller warns about under
// Debug.
func virtualListRange(
	cfg *VirtualListCfg, m *listHeightModel, w *Window,
) (first, last int, lead, trail float32, probe bool) {
	n := m.Count()
	if n == 0 || cfg.ItemView == nil {
		return 0, -1, 0, 0, false
	}
	viewH := virtualListViewportH(cfg, m)
	if viewH <= 0 {
		// No height yet: build a bounded probe so arrange has
		// something to measure, and hold the rest in the trailing
		// spacer so the scrollbar is not wildly wrong meanwhile.
		last = min(n-1, virtualListProbeRows-1)
		return 0, last, 0, m.Total() - m.Prefix(last+1), true
	}

	// Default 0: absent entry means the list has not been scrolled.
	scrollY := w.scrollY().GetOr(cfg.ID, 0)
	over := cfg.OverscanPx
	// NaN escapes the <= 0 default check and would poison the range
	// arithmetic below; treat any non-finite overscan as unset.
	if over <= 0 || !f32IsFinite(over) {
		over = viewH * virtualListOverscanFrac
	}
	first = m.IndexAt(-scrollY - over)
	last = m.IndexAt(-scrollY + viewH + over)
	if last-first+1 > virtualListMaxBuiltRows {
		last = first + virtualListMaxBuiltRows - 1
	}
	last = clampInt(last, first, n-1)
	return first, last, m.Prefix(first), m.Total() - m.Prefix(last+1), false
}

// virtualListViewportH resolves the height virtualization measures
// against: Cfg.Height, then MaxHeight, then the inner height arrange
// recorded last frame. The first two are outer heights, so padding
// *and border* come off — a container reserves border space whether
// or not it paints one; the recorded one is already inner.
func virtualListViewportH(cfg *VirtualListCfg, m *listHeightModel) float32 {
	inset := cfg.Padding.Height() +
		2*cfg.SizeBorder.Get(defaultListBoxStyle.SizeBorder)
	if h := cfg.Height; h > 0 {
		return h - inset
	}
	if h := cfg.MaxHeight; h > 0 {
		return h - inset
	}
	return m.viewportH
}

// virtualListChildren builds the frame's children: a leading spacer
// for everything above the window, the rows themselves, a trailing
// spacer for everything below. The []View comes from the frame-scoped
// arena — appendChildViews copies out of it and retains nothing.
func virtualListChildren(
	cfg *VirtualListCfg, m *listHeightModel, w *Window,
	first, last int, lead, trail float32,
) []View {
	if last < first || cfg.ItemView == nil {
		return nil
	}
	n := last - first + 1
	if lead > 0 {
		n++
	}
	if trail > 0 {
		n++
	}
	views := w.scratch.takeViews(n)
	if lead > 0 {
		views = append(views, virtualListSpacer(lead))
	}
	for i := first; i <= last; i++ {
		v := cfg.ItemView(i, m.width)
		if v == nil {
			// Keep the child positions aligned with the row indices:
			// appendChildViews skips a nil, which would shift every
			// later row's measurement onto the wrong item.
			v = virtualListSpacer(0)
		}
		views = append(views, v)
	}
	if trail > 0 {
		views = append(views, virtualListSpacer(trail))
	}
	return views
}

// virtualListSpacer is the transparent rectangle standing in for rows
// that were not built.
func virtualListSpacer(h float32) View {
	return Rectangle(RectangleCfg{
		Color:  ColorTransparent,
		Height: h,
		Sizing: FillFixed,
	})
}

// virtualListWidthRatchetRuns is how many consecutive frames of
// unexplained widening it takes to report one. A window resize widens
// the list too, but it stops; a row that demands the width it was
// handed never does.
const virtualListWidthRatchetRuns = 4

// virtualListNoteWidth records the inner width arrange resolved and
// watches for the one mistake that makes a virtual list misbehave
// without any error: a row turning the width ItemView is handed into a
// width it demands. MinWidth from that argument asks the list for at
// least the width it just reported, the row's own border pushes that a
// few pixels further, and the list widens every frame — which, because
// a width change invalidates the wrap, re-wraps and re-measures every
// frame too. Nothing about it looks like a failure from inside; the
// list just never settles.
func virtualListNoteWidth(w *Window, m *listHeightModel, id string, iw float32) {
	// Only growth with the window standing still counts: that is what
	// separates the ratchet from a resize the user asked for.
	if iw > m.measuredW+listHeightEps && m.lastWinW == w.windowWidth {
		m.widthRun++
	} else {
		m.widthRun = 0
	}
	m.measuredW, m.lastWinW = iw, w.windowWidth
	if m.widthRun >= virtualListWidthRatchetRuns && DebugEnabled() {
		w.debugWarn(debugCheckListWidthRatchet, id,
			"list %q has widened on %d consecutive frames with the "+
				"window unchanged (now %.0f px): a row is demanding the "+
				"width ItemView handed it — MinWidth taken from that "+
				"argument ratchets, and each width change re-wraps every "+
				"row", id, m.widthRun, iw)
	}
}

// virtualListAmend is the list's single AmendLayout: one hook on the
// scroll container rather than one per row. layoutAmend fires
// children-first, so every row's Shape.Height is final by the time
// this runs.
func virtualListAmend(m *listHeightModel, colorBorderFocus Color) func(EventCtx) {
	return amendAll(
		virtualListMeasure(m),
		focusRingAmend(Color{}, colorBorderFocus),
	)
}

// virtualListMeasure records the arranged geometry and writes the
// built rows' measured heights back into the model.
//
// The re-anchor is the subtle half. Rows above the built window sit
// inside the leading spacer and are never measured, so the model's
// prefix up to `first` does not move; a measurement therefore shifts
// every position from `first` down by the accumulated delta. Keying
// the correction on the row under the viewport top — not on `first`,
// where the delta is structurally always zero — and moving the stored
// offset by that delta is what holds the reader still across the
// frame boundary.
//
// The already-positioned children are shifted to match, for the same
// reason scroll anchoring shifts them: this frame's rows were laid
// out at their *actual* heights above a leading spacer sized from the
// stale model, so they already sit off by exactly that delta. Without
// the shift the correction lands one frame late and reads as a jump.
func virtualListMeasure(m *listHeightModel) func(EventCtx) {
	return func(ctx EventCtx) {
		ly := ctx.Layout
		if ly == nil || ly.Shape == nil || ctx.Window == nil {
			return
		}
		sh := ly.Shape
		if iw := sh.Width - sh.paddingWidth(); iw > 0 && f32IsFinite(iw) {
			virtualListNoteWidth(ctx.Window, m, sh.idKey(), iw)
		}
		if ih := sh.Height - sh.paddingHeight(); ih > 0 && f32IsFinite(ih) {
			m.viewportH = ih
		}
		m.hSeen = true
		if m.heightFn != nil || m.builtCount == 0 || m.uniform {
			return // the caller computes heights; nothing to measure
		}

		scrollID := sh.idKey()
		sy := ctx.Window.scrollY()
		// Default 0: absent entry means the list has not been scrolled.
		scrollY := sy.GetOr(scrollID, 0)
		anchor := m.IndexAt(-scrollY)
		before := m.Prefix(anchor)

		for k := range m.builtCount {
			ci := m.leadOffset + k
			if ci >= len(ly.Children) {
				break
			}
			child := ly.Children[ci].Shape
			if child == nil {
				continue
			}
			idx := m.builtFirst + k
			// The keyed write is gated on the tree actually changing,
			// so a settled list stops paying keyAt (typically a string
			// allocation) per built row per frame. A first measurement
			// inside the deadband stays unkeyed; a rebuild then seeds
			// it from the estimate, off by under listHeightEps.
			if delta := m.SetHeight(idx, child.Height); delta != 0 &&
				m.keyAt != nil {
				m.recordKey(m.keyAt(idx), child.Height)
			}
		}

		delta := m.Prefix(anchor) - before
		if delta == 0 {
			return
		}
		// Negative-down offsets: growing the content above the anchor
		// by delta means scrolling delta further to stay put. The top
		// clamp is applied here; the bottom one needs next frame's
		// content height, and layoutAdjustScrollOffsets applies it.
		next := f32Min(scrollY-delta, 0)
		if !f32IsFinite(next) || next == scrollY {
			return
		}
		sy.Set(scrollID, next)
		// Positions are absolute after the position pass, so moving the
		// scrollable would not move its children (see scrollAnchor).
		dy := next - scrollY
		for i := range ly.Children {
			scrollAnchorShiftY(&ly.Children[i], dy)
		}
		scrollSmoothShiftY(ctx.Window, scrollID, dy)
	}
}

// VirtualListFocusedIndex returns the row index a virtual list's
// keyboard navigation currently sits on. ItemView reads it to render
// the focused row — an unbuilt row has no shape to hold focus, so the
// focus lives on the list and is expressed as an index.
func (w *Window) VirtualListFocusedIndex(id string) int {
	return StateReadOr(w, nsVirtualListFocus, id, 0)
}

// SetVirtualListFocusedIndex moves a virtual list's keyboard focus to
// index and scrolls it into view. Main goroutine only.
// exportaudit:keep — the write half of the focused-index pair
func (w *Window) SetVirtualListFocusedIndex(id string, index int) {
	StateMap[string, int](w, nsVirtualListFocus, capModerate).
		Set(id, max(index, 0))
	w.ScrollIndexIntoView(id, index)
}

// virtualListKeyDown moves the focused index and keeps it in view.
// Focus lands on the row one frame later, once the index-addressed
// scroll has built it — which is why the arrow keys move an index
// rather than calling SetFocus on a row that may not exist.
func virtualListKeyDown(cfg *VirtualListCfg) func(EventCtx) {
	id := cfg.ID
	count := cfg.ItemCount
	user := cfg.OnKeyDown
	return func(ctx EventCtx) {
		if ctx.Event != nil && count > 0 {
			cur := ctx.Window.VirtualListFocusedIndex(id)
			next := virtualListNavigate(ctx.Event.KeyCode, cur, count)
			if next != cur {
				ctx.Window.SetVirtualListFocusedIndex(id, next)
				ctx.Consume()
				return
			}
		}
		if user != nil {
			user(ctx)
		}
	}
}

// virtualListNavigate maps a key to a new focused index. Page moves a
// fixed row count rather than a viewport's worth: under variable
// heights "a viewport of rows" is not a number the view phase has.
func virtualListNavigate(key KeyCode, cur, count int) int {
	const pageRows = 10
	switch key {
	case KeyUp:
		return clampInt(cur-1, 0, count-1)
	case KeyDown:
		return clampInt(cur+1, 0, count-1)
	case KeyPageUp:
		return clampInt(cur-pageRows, 0, count-1)
	case KeyPageDown:
		return clampInt(cur+pageRows, 0, count-1)
	case KeyHome:
		return 0
	case KeyEnd:
		return count - 1
	}
	return cur
}
