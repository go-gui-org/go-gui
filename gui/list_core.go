package gui

import (
	"cmp"
	"slices"
)

// list_core.go provides pure functions shared by ListBox, Select,
// and other list widgets. No state, no Window dependency.

const listCoreVirtualBufferRows = 2

// listCoreItem is the normalized item for the shared list engine.
// Widgets map their domain types to this before calling core fns.
type listCoreItem struct {
	ID           string
	Label        string
	Detail       string
	Icon         string
	Group        string
	Disabled     bool
	IsSubheading bool
}

// listCoreCfg configures listCoreViews rendering.
type listCoreCfg struct {
	TextStyle       TextStyle
	DetailStyle     TextStyle
	SubheadingStyle TextStyle
	OnItemClick     func(string, int, *Event, *Window)
	OnItemHover     func(int, *Event, *Window)
	PaddingItem     Padding
	ColorHighlight  Color
	ColorHover      Color
	ColorSelected   Color
	ShowDetails     bool
	ShowIcons       bool
}

// listCorePrepared holds pre-computed filter results for a frame.
type listCorePrepared struct {
	Items []listCoreItem
	IDs   []string
	HL    int
}

// listCoreScored pairs an index with its fuzzy score.
type listCoreScored struct {
	index int
	score int
}

// listCoreAction represents a keyboard navigation action.
type listCoreAction uint8

// listCoreAction constants.
const (
	listCoreNone listCoreAction = iota
	listCoreMoveUp
	listCoreMoveDown
	listCoreSelectItem
	listCoreDismiss
	listCoreFirst
	listCoreLast
)

// listCoreNavigate maps a key code to a list navigation action.
func listCoreNavigate(key KeyCode, itemCount int) listCoreAction {
	if itemCount == 0 {
		return listCoreNone
	}
	switch key {
	case KeyUp:
		return listCoreMoveUp
	case KeyDown:
		return listCoreMoveDown
	case KeyEnter:
		return listCoreSelectItem
	case KeyEscape:
		return listCoreDismiss
	case KeyHome:
		return listCoreFirst
	case KeyEnd:
		return listCoreLast
	default:
		return listCoreNone
	}
}

// listCoreApplyNav applies a navigation action to a highlight
// index. Returns new index and whether it changed.
func listCoreApplyNav(action listCoreAction, cur, itemCount int) (int, bool) {
	switch action {
	case listCoreMoveUp:
		next := max(cur-1, 0)
		return next, next != cur
	case listCoreMoveDown:
		next := min(cur+1, itemCount-1)
		return next, next != cur
	case listCoreFirst:
		return 0, cur != 0
	case listCoreLast:
		last := max(itemCount-1, 0)
		return last, cur != last
	default:
		return cur, false
	}
}

// listCoreVisibleRange computes the visible index range from
// scroll offset. Pure arithmetic.
func listCoreVisibleRange(itemCount int, rowHeight, listHeight, scrollY float32) (int, int) {
	if itemCount == 0 || rowHeight <= 0 || listHeight <= 0 {
		return 0, -1
	}
	maxIdx := itemCount - 1
	absScroll := scrollY
	if absScroll < 0 {
		absScroll = -absScroll
	}
	first := max(0, min(maxIdx, int(absScroll/rowHeight)))
	visibleRows := int(listHeight/rowHeight) + 1
	buf := listCoreVirtualBufferRows
	firstVisible := max(0, first-buf)
	lastVisible := min(maxIdx, first+visibleRows+buf)
	lastVisible = max(lastVisible, firstVisible)
	return firstVisible, lastVisible
}

// listCoreRowHeightEstimate estimates row height from text
// style + padding. No Window needed. Uses the same 1.4×
// line-height factor as text shape layout.
func listCoreRowHeightEstimate(style TextStyle, pad Padding) float32 {
	return style.Size*1.4 + pad.Height()
}

// listCoreFuzzyScore scores a candidate against a query.
// Returns -1 (no match) or 0+ (lower = better).
func listCoreFuzzyScore(candidate, query string) int {
	if len(query) == 0 {
		return 0
	}
	if len(candidate) == 0 {
		return -1
	}
	qi := 0
	score := 0
	prevMatch := -1
	for ci := range len(candidate) {
		if qi >= len(query) {
			break
		}
		cb := asciiLower(candidate[ci])
		qb := asciiLower(query[qi])
		if cb == qb {
			if prevMatch >= 0 {
				gap := ci - prevMatch - 1
				score += gap
			}
			prevMatch = ci
			qi++
		}
	}
	if qi < len(query) {
		return -1
	}
	return score
}

// listCoreFilter filters + ranks items by query. Returns indices
// sorted by score. Empty query returns all in order.
func listCoreFilter(items []listCoreItem, query string) []int {
	if len(query) == 0 {
		all := make([]int, len(items))
		for i := range items {
			all[i] = i
		}
		return all
	}
	scored := make([]listCoreScored, 0, len(items))
	for i, item := range items {
		if item.IsSubheading {
			continue
		}
		s := listCoreFuzzyScore(item.Label, query)
		if s >= 0 {
			scored = append(scored, listCoreScored{index: i, score: s})
		}
	}
	slices.SortFunc(scored, func(a, b listCoreScored) int {
		return cmp.Compare(a.score, b.score)
	})
	result := make([]int, len(scored))
	for i, sc := range scored {
		result[i] = sc.index
	}
	return result
}

// listCorePrepareInto filters items by query into caller-provided
// buffers to reduce per-frame allocations.
func listCorePrepareInto(
	items []listCoreItem,
	query string,
	rawHighlight int,
	filteredDst []listCoreItem,
	idsDst []string,
	scoredDst []listCoreScored,
) (listCorePrepared, []listCoreScored) {
	filtered := filteredDst[:0]
	if len(query) == 0 {
		filtered = append(filtered, items...)
	} else {
		scored := scoredDst[:0]
		for i := range items {
			item := items[i]
			if item.IsSubheading {
				continue
			}
			s := listCoreFuzzyScore(item.Label, query)
			if s >= 0 {
				scored = append(scored, listCoreScored{
					index: i,
					score: s,
				})
			}
		}
		slices.SortFunc(scored, func(a, b listCoreScored) int {
			return cmp.Compare(a.score, b.score)
		})
		for i := range scored {
			idx := scored[i].index
			if idx >= 0 && idx < len(items) {
				filtered = append(filtered, items[idx])
			}
		}
		scoredDst = scored
	}

	hl := 0
	if len(filtered) > 0 {
		hl = max(0, min(len(filtered)-1, rawHighlight))
	}

	ids := idsDst[:0]
	for i := range filtered {
		if !filtered[i].IsSubheading {
			ids = append(ids, filtered[i].ID)
		}
	}
	return listCorePrepared{Items: filtered, IDs: ids, HL: hl}, scoredDst
}

// listCorePrepare filters items by query, clamps highlight, and
// collects selectable IDs. Skips subheadings from IDs.
func listCorePrepare(items []listCoreItem, query string, rawHighlight int) listCorePrepared {
	prepared, _ := listCorePrepareInto(items, query, rawHighlight, nil, nil, nil)
	return prepared
}

func listCoreSelectedSet(selectedIDs []string) map[string]struct{} {
	if len(selectedIDs) < 2 {
		return nil
	}
	out := make(map[string]struct{}, len(selectedIDs))
	for i := range selectedIDs {
		out[selectedIDs[i]] = struct{}{}
	}
	return out
}

func listCoreContainsSelected(selectedSet map[string]struct{}, selectedIDs []string, id string) bool {
	if selectedSet != nil {
		_, ok := selectedSet[id]
		return ok
	}
	return slices.Contains(selectedIDs, id)
}

// listCoreViews builds visible item views with virtualization
// spacers. first/last are indices from listCoreVisibleRange.
func listCoreViews(items []listCoreItem, cfg listCoreCfg, first, last, highlighted int, selectedIDs []string, rowHeight float32) []View {
	total := len(items)
	capacity := 2
	if last >= first {
		capacity = last - first + 3
	}
	views := make([]View, 0, capacity)

	if first > 0 && rowHeight > 0 {
		views = append(views, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first) * rowHeight,
			Sizing: FillFixed,
		}))
	}

	selectedSet := listCoreSelectedSet(selectedIDs)
	for idx := first; idx <= last; idx++ {
		if idx < 0 || idx >= total {
			continue
		}
		isHL := idx == highlighted
		isSel := listCoreContainsSelected(selectedSet, selectedIDs, items[idx].ID)
		views = append(views,
			listCoreItemView(items[idx], idx, isHL, isSel, cfg))
	}

	if last < total-1 && rowHeight > 0 {
		remaining := total - 1 - last
		views = append(views, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rowHeight,
			Sizing: FillFixed,
		}))
	}
	return views
}

// listCoreItemView renders a single item row.
func listCoreItemView(item listCoreItem, index int, isHighlighted, isSelected bool, cfg listCoreCfg) View {
	bg := ColorTransparent
	if isHighlighted {
		bg = cfg.ColorHighlight
	} else if isSelected {
		bg = cfg.ColorSelected
	}

	if item.IsSubheading {
		return listCoreSubheadingView(item, cfg)
	}

	content := make([]View, 0, 4)

	if cfg.ShowIcons && len(item.Icon) > 0 {
		content = append(content, Text(TextCfg{
			Text:      item.Icon,
			TextStyle: cfg.TextStyle,
		}))
	}

	content = append(content, Text(TextCfg{
		Text:      item.Label,
		TextStyle: cfg.TextStyle,
		Mode:      TextModeSingleLine,
	}))

	if cfg.ShowDetails && len(item.Detail) > 0 {
		content = append(content,
			Row(ContainerCfg{
				Sizing:  FillFill,
				Padding: NoPadding,
			}),
			Text(TextCfg{
				Text:      item.Detail,
				TextStyle: cfg.DetailStyle,
				Mode:      TextModeSingleLine,
			}),
		)
	}

	onItemClick := cfg.OnItemClick
	onItemHover := cfg.OnItemHover
	hasClick := onItemClick != nil
	hasHover := onItemHover != nil
	colorHover := cfg.ColorHover
	isDisabled := item.Disabled
	itemID := item.ID

	return Row(ContainerCfg{
		Color:      bg,
		Padding:    Some(cfg.PaddingItem),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content:    content,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if hasClick && !isDisabled {
				onItemClick(itemID, index, e, w)
			}
		},
		OnHover: func(layout *Layout, e *Event, w *Window) {
			if !isDisabled {
				w.SetMouseCursor(CursorPointingHand)
				if layout.Shape.Color == ColorTransparent {
					layout.Shape.Color = colorHover
				}
			}
			if hasHover {
				onItemHover(index, e, w)
			}
		},
	})
}

// listCoreSubheadingView renders a subheading row.
func listCoreSubheadingView(item listCoreItem, cfg listCoreCfg) View {
	return Column(ContainerCfg{
		Spacing: SomeF(1),
		Padding: NoPadding,
		Sizing:  FillFit,
		Content: []View{
			Text(TextCfg{
				Text:      item.Label,
				TextStyle: cfg.SubheadingStyle,
			}),
			Row(ContainerCfg{
				Padding: NoPadding,
				Sizing:  FillFit,
				Content: []View{
					Rectangle(RectangleCfg{
						Width:  1,
						Height: 1,
						Sizing: FillFit,
						Color:  cfg.SubheadingStyle.Color,
					}),
				},
			}),
		},
	})
}

// listBoxNextSelectedIDs computes the next selection set after
// toggling datID.
func listBoxNextSelectedIDs(selectedIDs []string, datID string, isMultiple bool) []string {
	if !isMultiple {
		return []string{datID}
	}
	if slices.Contains(selectedIDs, datID) {
		// Remove it.
		next := make([]string, 0, len(selectedIDs)-1)
		for _, id2 := range selectedIDs {
			if id2 != datID {
				next = append(next, id2)
			}
		}
		return next
	}
	// Add it.
	next := make([]string, 0, len(selectedIDs)+1)
	next = append(next, selectedIDs...)
	next = append(next, datID)
	return next
}
