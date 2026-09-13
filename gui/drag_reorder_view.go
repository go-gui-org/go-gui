package gui

import "slices"

var dragGhostShadowColor = RGBA(0, 0, 0, 60)

// --- index calculation ---

// dragReorderCalcIndex estimates the drop target index from
// cursor position, using the source item's origin and size to
// infer uniform item spacing.
func dragReorderCalcIndex(
	mouseMain, itemStart, itemSize float32,
	sourceIndex, itemCount int,
) int {
	if itemCount <= 1 || itemSize <= 0 {
		return 0
	}
	listStart := itemStart - float32(sourceIndex)*itemSize
	rel := mouseMain - listStart
	idx := int(rel / itemSize)
	return max(0, min(itemCount, idx))
}

// dragReorderCalcIndexFromMids estimates the drop target index
// from precomputed item midpoint coordinates using binary search.
func dragReorderCalcIndexFromMids(
	mouseMain float32, itemMids []float32,
) (int, bool) {
	if len(itemMids) == 0 {
		return 0, false
	}
	lo, hi := 0, len(itemMids)
	for lo < hi {
		mid := (lo + hi) / 2
		if itemMids[mid] <= mouseMain {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo, true
}

// dragReorderItemMidsFromLayouts resolves draggable layout IDs
// and stores axis midpoints for fast per-move hit testing. One walk
// of the tree serves every ID: per-item FindByID was O(n·tree) at
// drag start, which is the large-list wall. First match wins per ID,
// mirroring findByIDDepth; an empty ID never matches, as in FindByID.
func dragReorderItemMidsFromLayouts(
	axis dragReorderAxis,
	itemLayoutIDs []string,
	w *Window,
) ([]float32, bool) {
	if len(itemLayoutIDs) == 0 || slices.Contains(itemLayoutIDs, "") {
		return nil, false
	}
	want := make(map[string]struct{}, len(itemLayoutIDs))
	for _, id := range itemLayoutIDs {
		want[id] = struct{}{}
	}
	found := make(map[string]float32, len(itemLayoutIDs))
	dragReorderCollectMids(&w.layout, want, found, axis, 0)
	mids := make([]float32, 0, len(itemLayoutIDs))
	for _, id := range itemLayoutIDs {
		m, ok := found[id]
		if !ok {
			return nil, false
		}
		mids = append(mids, m)
	}
	return mids, true
}

// dragReorderCollectMids records the axis midpoint of every shape
// whose effective ID is wanted. Stops descending once all are found.
func dragReorderCollectMids(
	layout *Layout,
	want map[string]struct{},
	found map[string]float32,
	axis dragReorderAxis,
	depth int,
) {
	if layout == nil || overMaxDepth(depth) || len(found) == len(want) {
		return
	}
	if s := layout.Shape; s != nil {
		if id := s.idKey(); id != "" {
			if _, ok := want[id]; ok {
				if _, done := found[id]; !done {
					switch axis {
					case dragReorderVertical:
						found[id] = s.Y + (s.Height / 2)
					case dragReorderHorizontal:
						found[id] = s.X + (s.Width / 2)
					}
				}
			}
		}
	}
	for i := range layout.Children {
		if len(found) == len(want) {
			return
		}
		dragReorderCollectMids(
			&layout.Children[i], want, found, axis, depth+1)
	}
}

// --- view helpers ---

// dragReorderGhostView returns a floating container at the
// cursor position containing the dragged item content.
func dragReorderGhostView(state dragReorderState, content View) View {
	ghostX := state.mouseX - (state.startMouseX - state.itemX)
	ghostY := state.mouseY - (state.startMouseY - state.itemY)

	return Column(ContainerCfg{
		ID:           "drag_reorder_ghost",
		Float:        true,
		FloatOffsetX: ghostX - state.parentX,
		FloatOffsetY: ghostY - state.parentY,
		Width:        state.itemWidth,
		Height:       state.itemHeight,
		Opacity:      SomeF(dragGhostOpacity),
		Sizing:       FixedFixed,
		Clip:         true,
		Padding:      NoPadding,
		SizeBorder:   SomeF(0),
		VAlign:       VAlignMiddle,
		Color:        guiTheme.ColorBackground,
		Shadow: &BoxShadow{
			Color:      dragGhostShadowColor,
			OffsetY:    dragGhostShadowOffY,
			BlurRadius: dragGhostShadowBlur,
		},
		Content: []View{content},
	})
}

// dragReorderGapView returns a transparent spacer the same
// size as the dragged item.
func dragReorderGapView(
	state dragReorderState, axis dragReorderAxis,
) View {
	sizing := FillFixed
	if axis == dragReorderHorizontal {
		sizing = FixedFit
	}
	return Rectangle(RectangleCfg{
		ID:     "drag_reorder_gap",
		Color:  ColorTransparent,
		Width:  state.itemWidth,
		Height: state.itemHeight,
		Sizing: sizing,
	})
}

// --- exported utility ---

// ReorderIndices computes (from, to) indices for a
// delete(from) + insert(to, item) reorder operation.
// movedID is the ID of the moved item. beforeID is the ID of
// the item it should appear before, or "" for end of list.
// Returns (-1, -1) on no-op or not-found.
func ReorderIndices(
	ids []string, movedID, beforeID string,
) (int, int) {
	from := -1
	bi := len(ids)
	beforeFound := false
	for i, id := range ids {
		if id == movedID {
			from = i
		}
		if len(beforeID) > 0 && id == beforeID {
			bi = i
			beforeFound = true
		}
	}
	if from < 0 {
		return -1, -1
	}
	if len(beforeID) > 0 && !beforeFound {
		return -1, -1
	}
	to := bi
	if from < bi {
		to = bi - 1
	}
	if from == to {
		return -1, -1
	}
	return from, to
}
