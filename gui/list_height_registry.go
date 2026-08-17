package gui

// list_height_registry.go — per-window storage for list height
// models, plus the invalidation rules.
//
// Models are keyed by the **effective** scroll ID, byte-identical to
// the key w.scrollY() uses, because every index↔offset conversion
// pairs a model lookup with a scroll-offset read. A key mismatch
// fails silently — the scroll lands somewhere plausible and wrong —
// so listHeightKeyMatchesScrollKey asserts it in the tests.

// listHeightModels returns the per-window model store. Values are
// pointers so an AmendLayout write-back mutates in place, the same
// arrangement listBoxCache uses.
func listHeightModels(w *Window) *BoundedMap[string, *listHeightModel] {
	return StateMap[string, *listHeightModel](w, nsListHeights, capModerate)
}

// listHeightLookup returns the model registered for a scroll ID
// without creating the store. Cold path: the index-addressed scroll
// API calls it before a list has necessarily been generated.
func listHeightLookup(w *Window, scrollID string) (*listHeightModel, bool) {
	if w == nil || scrollID == "" {
		return nil, false
	}
	store := StateMapRead[string, *listHeightModel](w, nsListHeights)
	if store == nil {
		return nil, false
	}
	m, ok := store.Get(scrollID)
	if !ok || m == nil || m.Count() == 0 {
		return nil, false
	}
	return m, true
}

// listHeightRegisterUniform records the O(1) model for a list whose
// rows share one height. This is the retrofit seam: a widget that
// already virtualizes by dividing by a scalar registers exactly the
// rowHeight its own spacers use, so the index API stays consistent
// with the arithmetic already on screen.
//
// scrollID must be the effective ID (w.EffID(cfg.ID)). staticTop is
// content inside the scrollable that sits above item 0; indexBase is
// the caller index of model item 0, which is how a frozen table
// header keeps data index 0 outside the scrollable range.
func listHeightRegisterUniform(
	w *Window, scrollID string, n int, rowH, staticTop float32, indexBase int,
) {
	if w == nil || scrollID == "" || n <= 0 || rowH <= 0 || !f32IsFinite(rowH) {
		return
	}
	store := listHeightModels(w)
	cur, ok := store.Get(scrollID)
	if !ok || cur == nil || !cur.uniform {
		store.Set(scrollID,
			newUniformHeightModel(n, rowH, staticTop, indexBase))
		return
	}
	// Reuse in place: a uniform model has no per-item storage, so
	// re-registering every frame must not allocate.
	cur.rowH = f32Max(rowH, listHeightMinRow)
	cur.estimate = cur.rowH
	cur.staticTop = staticTop
	cur.indexBase = indexBase
	cur.n = n
}

// listHeightAnchor captures where the viewport currently sits, as an
// item index plus an offset into that item, so a change that moves
// every prefix below it can be undone in scroll-offset terms.
type listHeightAnchor struct {
	within float32
	index  int
	valid  bool
}

// listHeightCaptureAnchor records the item under the viewport top.
// scrollY is the stored (negative-down) offset.
func listHeightCaptureAnchor(m *listHeightModel, scrollY float32) listHeightAnchor {
	if m == nil || m.Count() == 0 || !f32IsFinite(scrollY) {
		return listHeightAnchor{}
	}
	y := -scrollY
	idx := m.IndexAt(y)
	return listHeightAnchor{index: idx, within: y - m.Prefix(idx), valid: true}
}

// listHeightRestoreAnchor returns the scroll offset that puts the
// captured anchor back under the viewport top. The result is clamped
// only at the top; the bottom clamp needs the arranged viewport
// height, and layoutAdjustScrollOffsets applies it every frame.
func listHeightRestoreAnchor(m *listHeightModel, a listHeightAnchor) (float32, bool) {
	if m == nil || !a.valid || m.Count() == 0 {
		return 0, false
	}
	off := -(m.Prefix(min(a.index, m.Count()-1)) + a.within)
	if !f32IsFinite(off) {
		return 0, false
	}
	return f32Min(off, 0), true
}

// listHeightSpec is what a variable-height list declares about
// itself each frame. Bundled rather than passed as six parameters
// because every field is read together and the caller builds it in
// one place.
type listHeightSpec struct {
	keyAt     func(int) string
	heightFn  func(i int, width float32) float32
	estimate  float32
	staticTop float32
	width     float32
	n         int
}

// listHeightEnsureVariable returns the variable-height model for a
// list, creating it and applying the invalidation rules:
//
//   - width change — the heights were measured at a different inner
//     width, so every one of them is a guess again;
//   - theme change — text metrics moved under the same premise;
//   - count change — re-seat the items, taking each measured height
//     from its key when the caller supplies one, so an insert or a
//     reorder costs nothing.
//
// A reset preserves the reader's position: the anchor is captured
// before the model changes and the scroll offset restored after, so
// the correction is a stored-offset edit rather than a visible jump.
func listHeightEnsureVariable(
	w *Window, scrollID string, spec listHeightSpec,
) *listHeightModel {
	n, width := spec.n, spec.width
	if n > listHeightMaxVariable {
		// Per-item storage is ~12 B, so past the cap a variable model
		// costs tens of megabytes. Fall back to the uniform path,
		// which is exact-in-shape and wrong only in the amount no
		// estimate could get right at that size.
		if DebugEnabled() {
			w.debugWarn(debugCheckListHeightsCapped, scrollID,
				"list %q has %d items, past the %d-item cap for "+
					"per-item heights; falling back to a uniform row "+
					"height, so positions follow the estimate",
				scrollID, n, listHeightMaxVariable)
		}
		listHeightRegisterUniform(w, scrollID, n,
			spec.estimate, spec.staticTop, 0)
		m, _ := listHeightModels(w).Get(scrollID)
		return m
	}
	store := listHeightModels(w)
	m, ok := store.Get(scrollID)
	if !ok || m == nil || m.uniform {
		m = &listHeightModel{
			estimate:  f32Max(spec.estimate, listHeightMinRow),
			staticTop: spec.staticTop,
			width:     width,
			heightFn:  spec.heightFn,
			themeID:   guiTheme.id,
		}
		m.keyAt = spec.keyAt
		if spec.keyAt != nil {
			m.enableKeys(n)
		}
		m.Resize(n)
		store.Set(scrollID, m)
		return m
	}
	m.keyAt = spec.keyAt
	if spec.keyAt != nil {
		m.enableKeys(n)
	}
	m.heightFn = spec.heightFn
	m.staticTop = spec.staticTop
	m.estimate = f32Max(spec.estimate, listHeightMinRow)

	sy := w.scrollY()
	// Default 0: absent entry means the list has not been scrolled.
	scrollY := sy.GetOr(scrollID, 0)
	anchor := listHeightCaptureAnchor(m, scrollY)

	switch {
	case !f32AreClose(m.width, width) || m.themeID != guiTheme.id:
		m.width = width
		m.themeID = guiTheme.id
		m.Resize(n)
		if m.heightFn != nil {
			// The cheap path is exact at the new width, so recompute.
			m.reseed()
		}
		// Measured heights are deliberately kept. They describe a wrap
		// that no longer applies, but a height measured for *this* row
		// still predicts it better than one estimate shared by the whole
		// corpus, and the rows on screen are re-measured this frame
		// anyway. Dropping them made every frame of a window resize
		// re-seat the reader from the estimate — and turned an app that
		// widens its own list (see examples/virtual_list) from slightly
		// wrong into a list that re-wrapped on every frame.
	case n != m.Count():
		m.RebuildFromKeys(n)
	default:
		return m // nothing moved; leave the offset alone
	}

	if off, restored := listHeightRestoreAnchor(m, anchor); restored {
		sy.Set(scrollID, off)
	}
	return m
}

// InvalidateListHeights drops every measured height for the given
// list, so the next frames re-measure from the estimate. Call it when
// a row's content changed under a stable key — nothing detects that,
// because the model keys on identity, not on content. id is the
// effective scroll ID. Main goroutine only.
// exportaudit:keep — the only escape hatch for content edited under a
// stable key; nothing detects that case
func (w *Window) InvalidateListHeights(id string) {
	m, ok := listHeightLookup(w, id)
	if !ok || m.uniform {
		return
	}
	sy := w.scrollY()
	// Default 0: absent entry means the list has not been scrolled.
	anchor := listHeightCaptureAnchor(m, sy.GetOr(id, 0))
	m.ResetToEstimate(m.estimate)
	if off, restored := listHeightRestoreAnchor(m, anchor); restored {
		sy.Set(id, off)
	}
	w.UpdateWindow()
}
