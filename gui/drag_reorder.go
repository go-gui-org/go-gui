package gui

import "time"

// drag_reorder.go provides shared drag-to-reorder infrastructure
// for ListBox, TabControl, and Tree widgets. One active drag at
// a time (mouse_lock exclusivity). Uses existing FLIP animation
// (AnimateLayout), floating layers, and MouseLock.
//
// # Lifecycle
//
// The drag has three phases: start, track, drop, managed through
// dragReorderState stored in a keyed state map (one per widget).
//
// ## 1. Start (dragReorderStart)
//
// Triggered from a row's OnClick handler. Captures a snapshot:
//   - Mouse position and item geometry (x, y, width, height)
//   - Parent position (for float offset math later)
//   - Source index within the sibling list
//   - Item midpoints: resolves each sibling's layout ID from the
//     current layout tree and records axis midpoints for fast
//     binary search during tracking
//   - Scroll position at start (for compensating auto-scroll drift)
//   - ID signature: FNV-1a hash of the sibling IDs, used to detect
//     if the backing list mutates mid-drag
//
// Then calls MouseLock which captures all subsequent mouse events
// until release.
//
// ## 2. Track (dragReorderOnMouseMove)
//
// Called on every mouse move while locked. Two-stage process:
//
// Threshold gate: Until the cursor moves 5px along the drag axis,
// nothing activates. This prevents accidental drags from clicks.
//
// Index calculation (dual strategy):
//  1. Midpoint binary search (preferred): Uses the precomputed
//     midpoints array. Binary search finds which gap the cursor
//     falls in. O(log n). Only used when scroll hasn't changed
//     since start (midpoints are absolute coordinates).
//  2. Uniform fallback: If midpoints are invalid (scrolled, or
//     layouts unavailable), estimates the index from
//     (cursor - list_start) / item_size, assuming uniform heights.
//
// Auto-scroll: If the cursor is within 40px of the scroll
// container's edge, scrolls proportionally (closer = faster).
// A repeating 16ms animation timer keeps scrolling even when the
// mouse is stationary.
//
// Mutation detection: Each move checks the ID signature against
// the latest idsMeta. If the backing list changed (items
// added/removed externally), the drag is cancelled.
//
// When currentIndex changes, a FLIP layout animation is triggered
// so siblings animate into their new positions.
//
// ## 3. Drop (dragReorderOnMouseUp)
//
// On mouse release:
//  1. Checks ID signature one more time; cancels if list mutated
//  2. Computes (movedID, beforeID) from source and gap indices.
//     beforeID is "" when dropping at the end.
//  3. Skips the callback if the gap is at sourceIndex or
//     sourceIndex + 1 (no-op: item didn't move)
//  4. Fires onReorder(movedID, beforeID, w) and triggers
//     a FLIP animation
//
// # Visual rendering (per frame)
//
// During Tree / ListBox / TabControl rebuild:
//   - The source item is excluded from normal content and its view
//     is captured as ghostContent
//   - A transparent gap spacer (same size as the item) is inserted
//     at currentIndex
//   - A floating ghost follows the cursor, offset from the parent
//     by the delta between current and start mouse positions.
//     It has 85% opacity and a drop shadow.
//
// # Keyboard path (dragReorderKeyboardMove)
//
// Alt+Arrow directly computes (movedID, beforeID) from the
// current focus index and fires onReorder immediately. No drag
// state, no ghost, just a FLIP animation. Moving down uses
// currentIndex + 2 as the before-target because the gap model
// counts slots between items.
//
// # Cancel
//
// Escape key sets cancelled = true, unlocks the mouse, triggers a
// rebuild (which sees cancelled and hides ghost/gap), then clears
// state.

const (
	dragReorderThreshold    = float32(5.0)
	dragReorderScrollZone   = float32(40.0)
	dragReorderScrollSpeed  = float32(4.0)
	dragReorderScrollAnimID = "gui.drag_reorder.scroll"
	dragGhostOpacity        = float32(0.85)
	dragGhostShadowBlur     = float32(8.0)
	dragGhostShadowOffY     = float32(2.0)
)

// DragReorderAxis selects the primary drag axis.
type DragReorderAxis uint8

// Axis values for DragReorderAxis.
const (
	DragReorderVertical DragReorderAxis = iota
	DragReorderHorizontal
)

// dragReorderState tracks an in-progress drag-reorder operation.
type dragReorderState struct {
	itemID            string
	itemLayoutIDs     []string
	itemMids          []float32
	sourceIndex       int
	currentIndex      int
	itemCount         int
	idsLen            int
	idsHash           uint64
	midsOffset        int
	startMouseX       float32
	startMouseY       float32
	mouseX            float32
	mouseY            float32
	itemX             float32
	itemY             float32
	itemWidth         float32
	itemHeight        float32
	parentX           float32
	parentY           float32
	scrollID          string
	containerStart    float32
	containerEnd      float32
	startScrollX      float32
	startScrollY      float32
	started           bool
	active            bool
	cancelled         bool
	layoutsValid      bool
	scrollTimerActive bool
}

// dragReorderIDsMeta stores a snapshot of the item IDs' length
// and hash for mutation detection.
type dragReorderIDsMeta struct {
	idsLen  int
	idsHash uint64
}

// --- state accessors ---

func dragReorderGet(w *Window, key string) dragReorderState {
	sm := StateMap[string, dragReorderState](w, nsDragReorder, capFew)
	v, ok := sm.Get(key)
	if !ok {
		return dragReorderState{}
	}
	return v
}

func dragReorderSet(w *Window, key string, state dragReorderState) {
	sm := StateMap[string, dragReorderState](w, nsDragReorder, capFew)
	sm.Set(key, state)
}

func dragReorderClear(w *Window, key string) {
	sm := StateMap[string, dragReorderState](w, nsDragReorder, capFew)
	sm.Delete(key)
}

func dragReorderIDsMetaSet(w *Window, key string, ids []string) {
	sm := StateMap[string, dragReorderIDsMeta](
		w, nsDragReorderIDsMeta, capFew)
	sm.Set(key, dragReorderIDsMeta{
		idsLen:  len(ids),
		idsHash: dragReorderIDsSignature(ids),
	})
}

func dragReorderIDsMetaGet(w *Window, key string) (dragReorderIDsMeta, bool) {
	sm := StateMapRead[string, dragReorderIDsMeta](
		w, nsDragReorderIDsMeta)
	if sm == nil {
		return dragReorderIDsMeta{}, false
	}
	return sm.Get(key)
}

func dragReorderIDsChanged(state dragReorderState, meta dragReorderIDsMeta) bool {
	return state.idsLen != meta.idsLen || state.idsHash != meta.idsHash
}

// --- lifecycle functions ---

// dragReorderStartCfg groups parameters for dragReorderStart.
type dragReorderStartCfg struct {
	OnReorder     func(string, string, *Window)
	Layout        *Layout
	Event         *Event
	DragKey       string
	ItemID        string
	ItemIDs       []string
	ItemLayoutIDs []string
	Index         int
	MidsOffset    int
	ScrollID      string
	Axis          DragReorderAxis
}

// dragReorderStart initiates a drag-reorder from an OnClick
// handler. Captures initial mouse/item positions and locks
// the mouse.
func dragReorderStart(cfg dragReorderStartCfg, w *Window) {
	dragKey := cfg.DragKey
	index := cfg.Index
	itemID := cfg.ItemID
	axis := cfg.Axis
	itemIDs := cfg.ItemIDs
	onReorder := cfg.OnReorder
	itemLayoutIDs := cfg.ItemLayoutIDs
	midsOffset := cfg.MidsOffset
	scrollID := cfg.ScrollID
	layout := cfg.Layout
	e := cfg.Event
	var parentX, parentY float32
	if layout.Parent != nil {
		parentX = layout.Parent.Shape.X
		parentY = layout.Parent.Shape.Y
	}

	var containerStart, containerEnd float32
	if scrollID != "" && layout.Parent != nil {
		switch axis {
		case DragReorderVertical:
			containerStart = layout.Parent.Shape.Y
			containerEnd = layout.Parent.Shape.Y +
				layout.Parent.Shape.Height
		case DragReorderHorizontal:
			containerStart = layout.Parent.Shape.X
			containerEnd = layout.Parent.Shape.X +
				layout.Parent.Shape.Width
		}
	}

	var startScrollX, startScrollY float32
	if scrollID != "" {
		// Default 0: unscrolled position when no offset recorded yet.
		if smx := w.scrollXRead(); smx != nil {
			startScrollX = smx.GetOr(scrollID, 0)
		}
		if smy := w.scrollYRead(); smy != nil {
			startScrollY = smy.GetOr(scrollID, 0)
		}
	}

	itemMids, midsOK := dragReorderItemMidsFromLayouts(
		axis, itemLayoutIDs, w)
	if !midsOK {
		itemMids = nil
	}
	layoutsValid := len(itemMids) > 0 &&
		len(itemMids) == len(itemLayoutIDs)

	layoutIDs := make([]string, len(itemLayoutIDs))
	copy(layoutIDs, itemLayoutIDs)

	state := dragReorderState{
		started:        true,
		sourceIndex:    index,
		currentIndex:   index,
		itemCount:      len(itemIDs),
		idsLen:         len(itemIDs),
		idsHash:        dragReorderIDsSignature(itemIDs),
		itemLayoutIDs:  layoutIDs,
		itemMids:       itemMids,
		startMouseX:    e.MouseX + layout.Shape.X,
		startMouseY:    e.MouseY + layout.Shape.Y,
		mouseX:         e.MouseX + layout.Shape.X,
		mouseY:         e.MouseY + layout.Shape.Y,
		itemX:          layout.Shape.X,
		itemY:          layout.Shape.Y,
		itemWidth:      layout.Shape.Width,
		itemHeight:     layout.Shape.Height,
		parentX:        parentX,
		parentY:        parentY,
		itemID:         itemID,
		scrollID:       scrollID,
		containerStart: containerStart,
		containerEnd:   containerEnd,
		startScrollX:   startScrollX,
		startScrollY:   startScrollY,
		layoutsValid:   layoutsValid,
		midsOffset:     midsOffset,
	}
	dragReorderSet(w, dragKey, state)
	w.MouseLock(dragReorderMakeLock(
		dragKey, axis, itemIDs, onReorder))
}

// dragReorderMakeLock builds a MouseLockCfg that implements
// the full drag lifecycle.
func dragReorderMakeLock(
	dragKey string,
	axis DragReorderAxis,
	itemIDs []string,
	onReorder func(string, string, *Window),
) MouseLockCfg {
	return MouseLockCfg{
		MouseMove: func(_ *Layout, e *Event, w *Window) {
			dragReorderOnMouseMove(
				dragKey, axis, e.MouseX, e.MouseY, w)
		},
		MouseUp: func(_ *Layout, _ *Event, w *Window) {
			dragReorderOnMouseUp(
				dragKey, itemIDs, onReorder, w)
		},
	}
}

// dragReorderOnMouseMove handles threshold detection and
// index tracking during a drag.
//
//nolint:gocyclo // drag state machine
func dragReorderOnMouseMove(
	dragKey string,
	axis DragReorderAxis,
	mouseX, mouseY float32,
	w *Window,
) {
	state := dragReorderGet(w, dragKey)
	if state.cancelled {
		return
	}
	if meta, ok := dragReorderIDsMetaGet(w, dragKey); ok {
		if dragReorderIDsChanged(state, meta) {
			dragReorderCancel(dragKey, w)
			return
		}
	}

	mouseChanged := mouseX != state.mouseX || mouseY != state.mouseY
	state.mouseX = mouseX
	state.mouseY = mouseY
	activated := false

	if !state.active {
		dx := mouseX - state.startMouseX
		dy := mouseY - state.startMouseY
		var dist float32
		switch axis {
		case DragReorderVertical:
			dist = f32Abs(dy)
		case DragReorderHorizontal:
			dist = f32Abs(dx)
		}
		if dist < dragReorderThreshold {
			dragReorderSet(w, dragKey, state)
			return
		}
		state.active = true
		activated = true
		w.AnimateLayout(LayoutTransitionCfg{})
	}

	// Determine drop target from cursor vs item geometry.
	var mouseMain float32
	switch axis {
	case DragReorderVertical:
		mouseMain = mouseY
	case DragReorderHorizontal:
		mouseMain = mouseX
	}
	mouseOrig := mouseMain
	scrolledSinceStart := false

	if state.scrollID != "" {
		// Default 0: unscrolled position when no offset recorded yet.
		var scrollVal float32
		switch axis {
		case DragReorderVertical:
			scrollVal = w.scrollY().GetOr(state.scrollID, 0)
		case DragReorderHorizontal:
			scrollVal = w.scrollX().GetOr(state.scrollID, 0)
		}
		var startScroll float32
		switch axis {
		case DragReorderVertical:
			startScroll = state.startScrollY
		case DragReorderHorizontal:
			startScroll = state.startScrollX
		}
		scrolledSinceStart = scrollVal != startScroll
		mouseMain -= (scrollVal - startScroll)
	}

	newIndex := -1
	if !scrolledSinceStart && state.layoutsValid {
		if idx, ok := dragReorderCalcIndexFromMids(
			mouseMain, state.itemMids); ok {
			newIndex = idx + state.midsOffset
		}
	}

	if newIndex < 0 {
		var itemStart, itemSize float32
		switch axis {
		case DragReorderVertical:
			itemStart = state.itemY
			itemSize = state.itemHeight
		case DragReorderHorizontal:
			itemStart = state.itemX
			itemSize = state.itemWidth
		}
		newIndex = dragReorderCalcIndex(
			mouseMain, itemStart, itemSize,
			state.sourceIndex, state.itemCount)
	}

	didScroll := dragReorderAutoScroll(
		mouseOrig, state.containerStart, state.containerEnd,
		state.scrollID, axis, w)

	if didScroll && !state.scrollTimerActive {
		state.scrollTimerActive = true
		w.AnimationAdd(&Animate{
			AnimID: dragReorderScrollAnimID,
			Repeat: true,
			Delay:  16 * time.Millisecond,
			Callback: func(an *Animate, w *Window) {
				st := dragReorderGet(w, dragKey)
				if !st.active || st.cancelled {
					an.stopped = true
					return
				}
				dragReorderOnMouseMove(
					dragKey, axis, st.mouseX, st.mouseY, w)
			},
		})
	} else if !didScroll && state.scrollTimerActive {
		state.scrollTimerActive = false
		w.AnimationRemove(dragReorderScrollAnimID)
	}

	indexChanged := false
	if newIndex != state.currentIndex {
		w.AnimateLayout(LayoutTransitionCfg{})
		state.currentIndex = newIndex
		indexChanged = true
	}

	dragReorderSet(w, dragKey, state)
	if activated || indexChanged || didScroll ||
		(state.active && mouseChanged) {
		w.UpdateWindow()
	}
}

// dragReorderOnMouseUp finalizes the drag: fires onReorder
// with (movedID, beforeID) if the gap index differs from the
// source position. beforeID is "" when dropping at the end.
func dragReorderOnMouseUp(
	dragKey string,
	itemIDs []string,
	onReorder func(string, string, *Window),
	w *Window,
) {
	state := dragReorderGet(w, dragKey)
	wasActive := state.active
	src := state.sourceIndex
	gap := state.currentIndex

	if meta, ok := dragReorderIDsMetaGet(w, dragKey); ok {
		if dragReorderIDsChanged(state, meta) {
			dragReorderClear(w, dragKey)
			w.MouseUnlock()
			w.AnimationRemove(dragReorderScrollAnimID)
			w.UpdateWindow()
			return
		}
	}
	dragReorderClear(w, dragKey)
	w.MouseUnlock()
	w.AnimationRemove(dragReorderScrollAnimID)

	if wasActive && !state.cancelled &&
		gap != src && gap != src+1 {
		if onReorder != nil && src >= 0 && src < len(itemIDs) {
			movedID := itemIDs[src]
			beforeID := ""
			if gap < len(itemIDs) {
				beforeID = itemIDs[gap]
			}
			w.AnimateLayout(LayoutTransitionCfg{})
			onReorder(movedID, beforeID, w)
		}
	}
	w.UpdateWindow()
}

// dragReorderCancel cancels an active drag without firing
// the callback. Called from escape-key handlers.
func dragReorderCancel(dragKey string, w *Window) {
	state := dragReorderGet(w, dragKey)
	if !state.active && !state.cancelled {
		dragReorderClear(w, dragKey)
		w.MouseUnlock()
		return
	}
	state.cancelled = true
	dragReorderSet(w, dragKey, state)
	w.MouseUnlock()
	w.AnimationRemove(dragReorderScrollAnimID)
	// UpdateWindow before Clear: the rebuild sees cancelled=true,
	// hides ghost/gap, then Clear removes state for next frame.
	w.UpdateWindow()
	dragReorderClear(w, dragKey)
}

// --- auto-scroll ---

// dragReorderAutoScroll checks if the cursor is near the edge
// of a scrollable container and scrolls accordingly.
func dragReorderAutoScroll(
	mouseMain, containerStart, containerEnd float32,
	scrollID string,
	axis DragReorderAxis,
	w *Window,
) bool {
	if scrollID == "" {
		return false
	}
	nearStart := mouseMain - containerStart
	nearEnd := containerEnd - mouseMain

	if nearStart < dragReorderScrollZone && nearStart >= 0 {
		ratio := 1.0 - (nearStart / dragReorderScrollZone)
		delta := dragReorderScrollSpeed * ratio
		if delta != 0 {
			switch axis {
			case DragReorderVertical:
				w.ScrollVerticalBy(scrollID, delta)
			case DragReorderHorizontal:
				w.ScrollHorizontalBy(scrollID, delta)
			}
			return true
		}
	} else if nearEnd < dragReorderScrollZone && nearEnd >= 0 {
		ratio := 1.0 - (nearEnd / dragReorderScrollZone)
		delta := -dragReorderScrollSpeed * ratio
		if delta != 0 {
			switch axis {
			case DragReorderVertical:
				w.ScrollVerticalBy(scrollID, delta)
			case DragReorderHorizontal:
				w.ScrollHorizontalBy(scrollID, delta)
			}
			return true
		}
	}
	return false
}

// --- keyboard reorder ---

// dragReorderKeyboardMove handles Alt+Arrow keyboard reorder.
// Converts gap indices to (movedID, beforeID) and calls
// onReorder directly. Returns true if the event was handled.
func dragReorderKeyboardMove(
	keyCode KeyCode,
	modifiers Modifier,
	axis DragReorderAxis,
	currentIndex int,
	itemIDs []string,
	onReorder func(string, string, *Window),
	w *Window,
) bool {
	itemCount := len(itemIDs)
	if onReorder == nil || itemCount <= 1 {
		return false
	}
	if !modifiers.Has(ModAlt) {
		return false
	}

	newIndex := -1
	switch axis {
	case DragReorderVertical:
		switch keyCode {
		case KeyUp:
			if currentIndex > 0 {
				newIndex = currentIndex - 1
			}
		case KeyDown:
			if currentIndex < itemCount-1 {
				newIndex = min(currentIndex+2, itemCount)
			}
		}
	case DragReorderHorizontal:
		switch keyCode {
		case KeyLeft:
			if currentIndex > 0 {
				newIndex = currentIndex - 1
			}
		case KeyRight:
			if currentIndex < itemCount-1 {
				newIndex = min(currentIndex+2, itemCount)
			}
		}
	}

	if newIndex < 0 {
		return false
	}

	movedID := itemIDs[currentIndex]
	beforeID := ""
	if newIndex < itemCount {
		beforeID = itemIDs[newIndex]
	}
	w.AnimateLayout(LayoutTransitionCfg{})
	onReorder(movedID, beforeID, w)
	return true
}

// dragReorderEscape checks for escape key during an active drag
// and cancels it. Returns true if handled.
func dragReorderEscape(
	dragKey string, keyCode KeyCode, w *Window,
) bool {
	if keyCode != KeyEscape {
		return false
	}
	state := dragReorderGet(w, dragKey)
	if !state.started && !state.active {
		return false
	}
	dragReorderCancel(dragKey, w)
	return true
}

// --- ID signature ---

// dragReorderIDsSignature computes a stable FNV-1a signature
// of the item IDs to detect mid-drag list mutations.
func dragReorderIDsSignature(ids []string) uint64 {
	h := Fnv64Offset
	for _, id := range ids {
		h = Fnv64Str(h, id)
		h = Fnv64Byte(h, 0x1f)
	}
	return h
}
