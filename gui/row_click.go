package gui

// Row click targets.
//
// A list-like widget (ListBox, Tree, the listCore rows of Combobox and
// CommandPalette) builds one row container per visible row on every
// frame. A per-row OnClick closure costs one heap allocation per row per
// frame, plus whatever row data the closure copies out. Instead every
// row's OnClick is the one plain func rowOnClick, and the row's shape
// points at a rowClick through eventHandlers.rowClick. The rowClick
// names its owner — the click state the whole list shares — and the
// little that differs per row.
//
// A widget makes one owner and one []rowClick per list per frame, with
// the slice's capacity fixed before the first append, so the pointers
// rows take into it stay valid. An owner is a pointer, and storing a
// pointer in an interface allocates nothing.

// rowClickOwner is the click state one list shares across its rows.
type rowClickOwner interface {
	rowClick(rc *rowClick, ctx EventCtx)
}

// rowClick is the click target of one row.
type rowClick struct {
	owner rowClickOwner
	// id is the row's item ID.
	id string
	// index is the row's position in whatever index space the owner
	// uses: the drag index of a reorderable row, the filtered index of
	// a listCore row. Unused by owners that need none.
	index int
	// tree is the tree row, for tree owners only. It points into the
	// frame's flatRows, which nothing changes after the rows are built.
	tree *treeFlatRow
	// cue is a per-row sound the owner needs at click time, resolved
	// at generation: a handler must not read the frame-scoped theme.
	// A tree drag row's drop cue depends on the row, so it lives here.
	cue SoundCue
}

// rowOnClick is the OnClick of every row that carries a rowClick. It is
// a plain func, not a closure, so a row allocates no handler.
func rowOnClick(ctx EventCtx) {
	if ctx.Layout == nil || ctx.Layout.Shape == nil ||
		ctx.Layout.Shape.events == nil {
		return
	}
	rc := ctx.Layout.Shape.events.rowClick
	if rc == nil || rc.owner == nil {
		return
	}
	rc.owner.rowClick(rc, ctx)
}

// rowClicks makes the click-target slice for a list of up to n rows.
func rowClicks(n int) []rowClick {
	return make([]rowClick, 0, n)
}

// addRowClick appends a target to clicks and returns a pointer to it.
// The caller must have sized clicks for every row (rowClicks), or a
// grow would move the targets earlier rows point at.
func addRowClick(clicks *[]rowClick, rc rowClick) *rowClick {
	*clicks = append(*clicks, rc)
	return &(*clicks)[len(*clicks)-1]
}

// rowContent returns an empty Content slice with capacity n for one
// row. It comes from the frame's view arena, not the heap: the row's
// container reads it during this frame's generation and keeps nothing.
// A nil window (some tests) gets a heap slice.
func rowContent(w *Window, n int) []View {
	if w == nil {
		return make([]View, 0, n)
	}
	return w.scratch.takeViews(n)
}
