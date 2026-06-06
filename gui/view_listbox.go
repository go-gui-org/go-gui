package gui

var listBoxItemPad = PaddingTwoFive

type listBoxView struct {
	cfg ListBoxCfg
}

type listBoxCache struct {
	itemIDs         []string
	itemDataIndices []int
	dataHash        uint64
}

// ListBoxOption represents one row in a ListBox.
type ListBoxOption struct {
	ID           string
	Name         string
	Value        string
	IsSubheading bool
}

// ListBoxCfg configures a list box view.
type ListBoxCfg struct {
	TextStyle       TextStyle
	SubheadingStyle TextStyle
	OnSelect        func(ids []string, e *Event, w *Window)
	OnReorder       func(movedID, beforeID string, w *Window)

	ID string `gui:"required"`

	A11YLabel       string
	A11YDescription string
	SelectedIDs     []string
	// Items is a convenience field for simple string lists. Each
	// string becomes a ListBoxOption with ID==Name==Value. When
	// set, Items takes precedence over Data.
	Items       []string
	Data        []ListBoxOption
	Padding     Opt[Padding]
	Radius      Opt[float32]
	SizeBorder  Opt[float32]
	Height      float32
	MinWidth    float32
	MaxWidth    float32
	MinHeight   float32
	MaxHeight   float32
	IDScroll    uint32
	IDFocus     uint32
	Color       Color
	ColorHover  Color
	ColorBorder Color
	ColorSelect Color
	Sizing      Sizing
	Multiple    bool
	Disabled    bool
	Invisible   bool
	Reorderable bool
}

// ListBoxOption helpers.

// NewListBoxOption constructs a ListBoxOption.
func NewListBoxOption(id, name, value string) ListBoxOption {
	return ListBoxOption{ID: id, Name: name, Value: value}
}

// NewListBoxSubheading constructs a subheading row.
func NewListBoxSubheading(id, title string) ListBoxOption {
	return ListBoxOption{ID: id, Name: title, IsSubheading: true}
}

// ListBox creates a list box view.
func ListBox(cfg ListBoxCfg) View {
	requireID("ListBox", cfg.ID)
	applyListBoxDefaults(&cfg)
	if len(cfg.Items) > 0 {
		n := min(len(cfg.Items), maxDataConvLen)
		cfg.Data = make([]ListBoxOption, n)
		for i := range n {
			cfg.Data[i] = ListBoxOption{
				ID: cfg.Items[i], Name: cfg.Items[i],
				Value: cfg.Items[i]}
		}
	}
	if listBoxCanVirtualize(&cfg) ||
		(cfg.Reorderable && cfg.OnReorder != nil) ||
		cfg.IDFocus > 0 {
		return &listBoxView{cfg: cfg}
	}

	dn := &DefaultListBoxStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)

	selectedSet := listCoreSelectedSet(cfg.SelectedIDs)
	list := make([]View, 0, len(cfg.Data))
	for i := range cfg.Data {
		list = append(list, listBoxItemView(cfg.Data[i], cfg, selectedSet, ""))
	}

	listBoxID := cfg.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	selectedIDs := cfg.SelectedIDs
	itemIDs := make([]string, 0, len(cfg.Data))
	for i := range cfg.Data {
		if !cfg.Data[i].IsSubheading {
			itemIDs = append(itemIDs, cfg.Data[i].ID)
		}
	}

	return Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleList,
		A11YLabel: a11yLabel(cfg.A11YLabel, cfg.ID),
		IDFocus:   cfg.IDFocus,
		IDScroll:  cfg.IDScroll,
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			listBoxOnKeyDown(listBoxID, itemIDs,
				isMultiple, onSelect, selectedIDs,
				0, 0, 0, nil, e, w)
		},
		Width:       cfg.MaxWidth,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Sizing:      cfg.Sizing,
		Spacing:     SomeF(0),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     list,
	})
}

func listBoxCanVirtualize(cfg *ListBoxCfg) bool {
	if cfg == nil || cfg.IDScroll == 0 {
		return false
	}
	return cfg.Height > 0 || cfg.MaxHeight > 0
}

func (lv *listBoxView) Content() []View { return nil }

func (lv *listBoxView) GenerateLayout(w *Window) Layout {
	cfg := &lv.cfg

	dn := &DefaultListBoxStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)

	cache := listBoxEnsureCache(cfg, w)
	selectedSet := listCoreSelectedSet(cfg.SelectedIDs)

	first, last, virtualize, listH, rowH :=
		listBoxVisibleRange(cfg, w)

	listBoxID := cfg.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	selectedIDs := cfg.SelectedIDs
	itemIDs := cache.itemIDs
	itemDataIndices := cache.itemDataIndices

	// Keyboard focus highlight.
	lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
	focusIdx, _ := lbf.Get(cfg.ID)
	var focusedID string
	if focusIdx >= 0 && focusIdx < len(itemIDs) {
		focusedID = itemIDs[focusIdx]
	}

	canReorder := cfg.Reorderable && cfg.OnReorder != nil
	var drag dragReorderState
	if canReorder {
		drag = dragReorderGet(w, cfg.ID)
	}
	dragging := canReorder && drag.active && !drag.cancelled
	onReorder := cfg.OnReorder
	idScroll := cfg.IDScroll

	dragIdxByRow := listBoxDragIndexByRow(cfg, canReorder)
	itemLayoutIDs, midsOffset := listBoxItemLayoutIDs(
		cfg, canReorder, first, last)

	if canReorder && (drag.started || drag.active) {
		dragReorderIDsMetaSet(w, cfg.ID, itemIDs)
	}

	list, ghostContent := listBoxBuildItems(
		cfg, selectedSet, focusedID, dragIdxByRow,
		itemIDs, itemLayoutIDs, midsOffset, idScroll,
		canReorder, dragging, drag,
		virtualize, first, last)

	if dragging && drag.currentIndex >= len(itemIDs) {
		list = append(list,
			dragReorderGapView(drag, DragReorderVertical))
	}
	if dragging && ghostContent != nil {
		list = append(list,
			dragReorderGhostView(drag, ghostContent))
	}

	return GenerateViewLayout(Column(ContainerCfg{
		ID:        cfg.ID,
		A11YRole:  AccessRoleList,
		A11YLabel: a11yLabel(cfg.A11YLabel, cfg.ID),
		IDFocus:   cfg.IDFocus,
		IDScroll:  cfg.IDScroll,
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			if canReorder {
				if dragReorderEscape(
					listBoxID, e.KeyCode, w) {
					e.IsHandled = true
					return
				}
				lbf = StateMap[string, int](
					w, nsListBoxFocus, capModerate)
				curIdx, _ := lbf.Get(listBoxID)
				if curIdx >= 0 && curIdx < len(itemIDs) &&
					dragReorderKeyboardMove(e.KeyCode,
						e.Modifiers, DragReorderVertical,
						curIdx, itemIDs, onReorder, w) {
					e.IsHandled = true
					return
				}
			}
			listBoxOnKeyDown(listBoxID, itemIDs,
				isMultiple, onSelect, selectedIDs,
				idScroll, rowH, listH, itemDataIndices, e, w)
		},
		Width:       cfg.MaxWidth,
		Height:      cfg.Height,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		MinHeight:   cfg.MinHeight,
		MaxHeight:   cfg.MaxHeight,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Sizing:      cfg.Sizing,
		Spacing:     SomeF(0),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		Content:     list,
	}), w)
}

// listBoxEnsureCache returns or creates the list box cache for
// the given config, refreshing item IDs when data changes.
func listBoxEnsureCache(cfg *ListBoxCfg, w *Window) *listBoxCache {
	cacheMap := StateMap[string, *listBoxCache](
		w, nsListBoxCache, capModerate)
	cache, ok := cacheMap.Get(cfg.ID)
	if !ok || cache == nil {
		cache = &listBoxCache{}
		cacheMap.Set(cfg.ID, cache)
	}
	dataHash := listBoxDataHash(cfg.Data)
	if cache.dataHash != dataHash || len(cache.itemIDs) == 0 {
		itemIDs := make([]string, 0, len(cfg.Data))
		indices := make([]int, 0, len(cfg.Data))
		for i := range cfg.Data {
			if !cfg.Data[i].IsSubheading {
				itemIDs = append(itemIDs, cfg.Data[i].ID)
				indices = append(indices, i)
			}
		}
		cache.itemIDs = itemIDs
		cache.itemDataIndices = indices
		cache.dataHash = dataHash
	}
	return cache
}

// listBoxVisibleRange computes the visible row range and
// virtualization parameters for the list box.
func listBoxVisibleRange(
	cfg *ListBoxCfg, w *Window,
) (first, last int, virtualize bool, listH, rowH float32) {
	first = 0
	last = len(cfg.Data) - 1
	virtualize = cfg.IDScroll > 0
	listH = cfg.Height
	if listH <= 0 {
		listH = cfg.MaxHeight
	}
	rowH = listCoreRowHeightEstimate(cfg.TextStyle, listBoxItemPad)
	if virtualize && listH > 0 && len(cfg.Data) > 0 {
		scrollY := StateReadOr(
			w, nsScrollY, cfg.IDScroll, float32(0))
		first, last = listCoreVisibleRange(
			len(cfg.Data), rowH, listH, scrollY)
	} else {
		virtualize = false
	}
	return first, last, virtualize, listH, rowH
}

// listBoxDragIndexByRow builds a mapping from data row index to
// draggable item index (-1 for subheadings).
func listBoxDragIndexByRow(
	cfg *ListBoxCfg, canReorder bool,
) []int {
	if !canReorder {
		return nil
	}
	dragIdxByRow := make([]int, len(cfg.Data))
	di := 0
	for i := range cfg.Data {
		if !cfg.Data[i].IsSubheading {
			dragIdxByRow[i] = di
			di++
		} else {
			dragIdxByRow[i] = -1
		}
	}
	return dragIdxByRow
}

// listBoxItemLayoutIDs builds layout IDs and midsOffset for
// drag-reorder tracking.
func listBoxItemLayoutIDs(
	cfg *ListBoxCfg, canReorder bool, first, last int,
) ([]string, int) {
	if !canReorder {
		return nil, 0
	}
	itemLayoutIDs := make([]string, 0, last-first+1)
	midsOffset := 0
	for idx := range first {
		if idx < len(cfg.Data) &&
			!cfg.Data[idx].IsSubheading {
			midsOffset++
		}
	}
	for idx := first; idx <= last; idx++ {
		if idx >= 0 && idx < len(cfg.Data) &&
			!cfg.Data[idx].IsSubheading {
			itemLayoutIDs = append(itemLayoutIDs,
				"lb_"+cfg.ID+"_"+cfg.Data[idx].ID)
		}
	}
	return itemLayoutIDs, midsOffset
}

// listBoxBuildItems builds the list of item views, including
// virtualization spacers and drag-reorder gap/ghost handling.
func listBoxBuildItems(
	cfg *ListBoxCfg,
	selectedSet map[string]struct{},
	focusedID string,
	dragIdxByRow []int,
	itemIDs, itemLayoutIDs []string,
	midsOffset int, idScroll uint32,
	canReorder, dragging bool,
	drag dragReorderState,
	virtualize bool, first, last int,
) ([]View, View) {
	listCap := len(cfg.Data)
	if virtualize && last >= first {
		listCap = last - first + 3
	}
	if dragging {
		listCap += 3
	}
	list := make([]View, 0, listCap)

	if virtualize && first > 0 {
		rh := listCoreRowHeightEstimate(
			cfg.TextStyle, listBoxItemPad)
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(first) * rh,
			Sizing: FillFixed,
		}))
	}

	var ghostContent View
	for idx := first; idx <= last; idx++ {
		if idx < 0 || idx >= len(cfg.Data) {
			continue
		}
		di := -1
		isDraggable := false
		if dragIdxByRow != nil && idx < len(dragIdxByRow) {
			di = dragIdxByRow[idx]
			isDraggable = di >= 0
		}

		if dragging && isDraggable && di == drag.currentIndex {
			list = append(list,
				dragReorderGapView(drag, DragReorderVertical))
		}

		if dragging && isDraggable && di == drag.sourceIndex {
			ghostContent = listBoxItemContent(
				cfg.Data[idx], *cfg)
			continue
		}

		if canReorder && isDraggable {
			list = append(list, listBoxReorderItemView(
				cfg.Data[idx], *cfg, selectedSet, di,
				itemIDs, itemLayoutIDs, midsOffset, idScroll))
		} else {
			list = append(list,
				listBoxItemView(
					cfg.Data[idx], *cfg, selectedSet, focusedID))
		}
	}

	if virtualize && last < len(cfg.Data)-1 {
		rh := listCoreRowHeightEstimate(
			cfg.TextStyle, listBoxItemPad)
		remaining := len(cfg.Data) - 1 - last
		list = append(list, Rectangle(RectangleCfg{
			Color:  ColorTransparent,
			Height: float32(remaining) * rh,
			Sizing: FillFixed,
		}))
	}

	return list, ghostContent
}

func listBoxItemView(dat ListBoxOption, cfg ListBoxCfg, selectedSet map[string]struct{}, focusedID string) View {
	color := ColorTransparent
	if dat.ID == focusedID && !dat.IsSubheading {
		color = cfg.ColorHover
	}
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		color = cfg.ColorSelect
	}
	isSub := dat.IsSubheading
	content := listBoxItemContent(dat, cfg)

	datID := dat.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	hasOnSelect := onSelect != nil
	selectedIDs := cfg.SelectedIDs
	colorHover := cfg.ColorHover

	a11yState := AccessStateNone
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		a11yState = AccessStateSelected
	}

	return Row(ContainerCfg{
		A11YRole:   AccessRoleListItem,
		A11YLabel:  dat.Name,
		A11YState:  a11yState,
		Color:      color,
		Padding:    Some(listBoxItemPad),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content:    []View{content},
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if hasOnSelect && !isSub {
				ids := listBoxNextSelectedIDs(
					selectedIDs, datID, isMultiple)
				onSelect(ids, e, w)
			}
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			if hasOnSelect && !isSub {
				w.SetMouseCursor(CursorPointingHand)
				if layout.Shape.Color == ColorTransparent {
					layout.Shape.Color = colorHover
				}
			}
		},
	})
}

func listBoxReorderItemView(
	dat ListBoxOption,
	cfg ListBoxCfg,
	selectedSet map[string]struct{},
	dragIdx int,
	itemIDs []string,
	itemLayoutIDs []string,
	midsOffset int,
	idScroll uint32,
) View {
	color := ColorTransparent
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		color = cfg.ColorSelect
	}
	content := listBoxItemContent(dat, cfg)
	layoutID := "lb_" + cfg.ID + "_" + dat.ID

	datID := dat.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	hasOnSelect := onSelect != nil
	selectedIDs := cfg.SelectedIDs
	colorHover := cfg.ColorHover
	listBoxID := cfg.ID
	onReorder := cfg.OnReorder

	a11yState := AccessStateNone
	if listCoreContainsSelected(selectedSet, cfg.SelectedIDs, dat.ID) {
		a11yState = AccessStateSelected
	}

	return Row(ContainerCfg{
		ID:         layoutID,
		A11YRole:   AccessRoleListItem,
		A11YLabel:  dat.Name,
		A11YState:  a11yState,
		Color:      color,
		Padding:    Some(listBoxItemPad),
		SizeBorder: NoBorder,
		Sizing:     FillFit,
		Content:    []View{content},
		OnClick: func(layout *Layout, e *Event, w *Window) {
			dragReorderStart(dragReorderStartCfg{
				DragKey:       listBoxID,
				Index:         dragIdx,
				ItemID:        datID,
				Axis:          DragReorderVertical,
				ItemIDs:       itemIDs,
				OnReorder:     onReorder,
				ItemLayoutIDs: itemLayoutIDs,
				MidsOffset:    midsOffset,
				IDScroll:      idScroll,
				Layout:        layout,
				Event:         e,
			}, w)
			if hasOnSelect {
				ids := listBoxNextSelectedIDs(
					selectedIDs, datID, isMultiple)
				onSelect(ids, e, w)
			}
			e.IsHandled = true
		},
		OnHover: func(layout *Layout, _ *Event, w *Window) {
			w.SetMouseCursor(CursorPointingHand)
			if layout.Shape.Color == ColorTransparent {
				layout.Shape.Color = colorHover
			}
		},
	})
}

func listBoxItemContent(dat ListBoxOption, cfg ListBoxCfg) View {
	if dat.IsSubheading {
		return Column(ContainerCfg{
			Spacing: SomeF(1),
			Padding: NoPadding,
			Sizing:  FillFit,
			Content: []View{
				Text(TextCfg{
					Text:      dat.Name,
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
	return Text(TextCfg{
		Text:      dat.Name,
		Mode:      TextModeMultiline,
		TextStyle: cfg.TextStyle,
	})
}

// listBoxDataIndex maps an itemIDs index to the full data index
// (including subheading rows). Returns dataIdx unchanged if no
// mapping is provided.
func listBoxDataIndex(itemDataIndices []int, idx int) int {
	if idx >= 0 && idx < len(itemDataIndices) {
		return itemDataIndices[idx]
	}
	return idx
}

func listBoxOnKeyDown(
	listBoxID string,
	itemIDs []string,
	isMultiple bool,
	onSelect func([]string, *Event, *Window),
	selectedIDs []string,
	idScroll uint32, rowH, listH float32,
	itemDataIndices []int,
	e *Event,
	w *Window,
) {
	if len(itemIDs) == 0 || onSelect == nil {
		return
	}

	action := listCoreNavigate(e.KeyCode, len(itemIDs))
	if e.KeyCode == KeySpace {
		action = ListCoreSelectItem
	}
	if action == ListCoreNone {
		return
	}
	e.IsHandled = true

	lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
	curIdx, _ := lbf.Get(listBoxID)

	if action == ListCoreSelectItem {
		if curIdx >= 0 && curIdx < len(itemIDs) {
			datID := itemIDs[curIdx]
			ids := listBoxNextSelectedIDs(
				selectedIDs, datID, isMultiple)
			onSelect(ids, e, w)
		}
		return
	}

	next, changed := listCoreApplyNav(action, curIdx, len(itemIDs))
	if changed {
		lbf.Set(listBoxID, next)
		if idScroll > 0 && rowH > 0 {
			scrollEnsureVisible(idScroll,
				listBoxDataIndex(itemDataIndices, next),
				rowH, listH, w)
		}
		w.UpdateWindow()
	}
}

func applyListBoxDefaults(cfg *ListBoxCfg) {
	d := &DefaultListBoxStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}

	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyleNormal
	}
	if cfg.SubheadingStyle == (TextStyle{}) {
		cfg.SubheadingStyle = d.SubheadingStyle
	}
}

func listBoxDataHash(items []ListBoxOption) uint64 {
	const offset uint64 = 14695981039346656037
	const prime uint64 = 1099511628211
	h := offset
	for i := range items {
		it := items[i]
		for j := range len(it.ID) {
			h ^= uint64(it.ID[j])
			h *= prime
		}
		h ^= 0xff
		h *= prime

		for j := range len(it.Name) {
			h ^= uint64(it.Name[j])
			h *= prime
		}
		h ^= 0xff
		h *= prime

		for j := range len(it.Value) {
			h ^= uint64(it.Value[j])
			h *= prime
		}
		h ^= 0xff
		h *= prime

		if it.IsSubheading {
			h ^= 1
		}
		h *= prime
	}
	return h
}
