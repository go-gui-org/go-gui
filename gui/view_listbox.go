package gui

var listBoxItemPad = PaddingTwoFive

type listBoxView struct {
	cfg ListBoxCfg
}

type listBoxCache struct {
	itemIDs         []string
	itemDataIndices []int
	dataHash        uint64
	// resolvedH is the list's height as resolved by the layout
	// engine, captured after arrange (AmendLayout) each frame. The
	// view phase runs before sizing, so virtualization under Fill
	// sizing reads this from the previous frame. 0 until the first
	// frame has been arranged.
	resolvedH float32
	// hSeen records that AmendLayout has run at least once, so a
	// persistent 0 can be distinguished from "not arranged yet".
	hSeen bool
}

// ListBoxOption represents one row in a ListBox.
type ListBoxOption struct {
	ID           string
	Name         string
	Value        string
	isSubheading bool
}

// ListBoxCfg configures a list box view.
type ListBoxCfg struct {
	TextStyle       TextStyle
	subheadingStyle TextStyle
	OnSelect        func([]string, EventCtx)
	OnReorder       func(string, string, EventCtx)

	ID string `gui:"required"`

	A11YCfg
	SelectedIDs []string
	// Items is a convenience field for simple string lists. Each
	// string becomes a ListBoxOption with ID==Name==Value. When
	// set, Items takes precedence over Data.
	Items      []string
	Data       []ListBoxOption
	Padding    Padding
	Radius     Opt[float32]
	SizeBorder Opt[float32]
	// Height sets the list's height directly. Row virtualization
	// needs a resolved height: with Height or MaxHeight the list
	// virtualizes from the first frame; under Fill sizing the
	// resolved height is read back from the previous frame's
	// arrange, so the first frame builds every row once.
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	// MaxHeight caps the list's height. Like Height, it resolves the
	// height virtualization needs.
	MaxHeight float32
	// Scrollable opts the list into the scroll system. Scroll state
	// is keyed by Cfg.ID — pass that same id to Window.ScrollVerticalTo.
	// Virtualization is automatic; it requires a resolved height, so
	// pair Scrollable with Height or MaxHeight, or rely on Fill
	// sizing (which virtualizes from the second frame).
	Scrollable bool
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Color         Color
	ColorHover    Color
	ColorBorder   Color
	ColorSelect   Color
	Sizing        Sizing
	Multiple      bool
	Disabled      bool
	Invisible     bool
	Reorderable   bool
}

// ListBoxOption helpers.

// NewListBoxOption constructs a ListBoxOption.
func NewListBoxOption(id, name, value string) ListBoxOption {
	return ListBoxOption{ID: id, Name: name, Value: value}
}

// NewListBoxSubheading constructs a subheading row.
func NewListBoxSubheading(id, title string) ListBoxOption {
	return ListBoxOption{ID: id, Name: title, isSubheading: true}
}

// ListBox creates a list box view.
func ListBox(cfg ListBoxCfg) View {
	RequireID("ListBox", cfg.ID)
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
		!cfg.FocusDisabled {
		return &listBoxView{cfg: cfg}
	}

	dn := &defaultListBoxStyle
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
		if !cfg.Data[i].isSubheading {
			itemIDs = append(itemIDs, cfg.Data[i].ID)
		}
	}

	return Column(ContainerCfg{
		ID:       cfg.ID,
		A11YRole: AccessRoleList,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Focusable:  !cfg.FocusDisabled,
		Scrollable: cfg.Scrollable,
		OnKeyDown: func(ctx EventCtx) {
			listBoxOnKeyDown(listBoxID, itemIDs,
				isMultiple, onSelect, selectedIDs,
				"", 0, 0, nil, ctx.Event, ctx.Window)
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

// listBoxCanVirtualize reports whether the list takes the
// virtualizing path. Scrollable alone qualifies: with no configured
// height, virtualization starts on the second frame once Arrange has
// resolved one.
func listBoxCanVirtualize(cfg *ListBoxCfg) bool {
	return cfg != nil && cfg.Scrollable
}

func (lv *listBoxView) GenerateLayout(w *Window) Layout {
	cfg := &lv.cfg

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	dn := &defaultListBoxStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)

	cache := listBoxEnsureCache(cfg, w)
	selectedSet := listCoreSelectedSet(cfg.SelectedIDs)

	first, last, virtualize, listH, rowH :=
		listBoxVisibleRange(cfg, cache, w)

	listBoxID := cfg.ID
	isMultiple := cfg.Multiple
	onSelect := cfg.OnSelect
	selectedIDs := cfg.SelectedIDs
	itemIDs := cache.itemIDs
	itemDataIndices := cache.itemDataIndices

	// Keyboard focus highlight.
	lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
	// Default 0: start at first item; bounds-checked below.
	focusIdx := lbf.GetOr(cfg.ID, 0)
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
	scrollID := cfg.ID

	dragIdxByRow := listBoxDragIndexByRow(cfg, canReorder)
	itemLayoutIDs, midsOffset := listBoxItemLayoutIDs(
		cfg, canReorder, first, last)

	if canReorder && (drag.started || drag.active) {
		dragReorderIDsMetaSet(w, cfg.ID, itemIDs)
	}

	list, ghostContent := listBoxBuildItems(
		cfg, selectedSet, focusedID, dragIdxByRow,
		itemIDs, itemLayoutIDs, midsOffset, scrollID,
		canReorder, dragging, drag,
		virtualize, first, last)

	if dragging && drag.currentIndex >= len(itemIDs) {
		list = append(list,
			dragReorderGapView(drag, dragReorderVertical))
	}
	if dragging && ghostContent != nil {
		list = append(list,
			dragReorderGhostView(drag, ghostContent))
	}

	return generateViewLayout(Column(ContainerCfg{
		ID:       cfg.ID,
		A11YRole: AccessRoleList,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Focusable:   !cfg.FocusDisabled,
		Scrollable:  cfg.Scrollable,
		AmendLayout: listBoxAmendLayout(cache),
		OnKeyDown: func(ctx EventCtx) {
			if canReorder {
				if dragReorderEscape(
					listBoxID, ctx.Event.KeyCode, ctx.Window) {
					ctx.Consume()
					return
				}
				lbf = StateMap[string, int](
					ctx.Window, nsListBoxFocus, capModerate)
				// Default 0: bounds-checked before use; zero index handled.
				curIdx := lbf.GetOr(listBoxID, 0)
				if curIdx >= 0 && curIdx < len(itemIDs) &&
					dragReorderKeyboardMove(ctx.Event.KeyCode,
						ctx.Event.Modifiers, dragReorderVertical,
						curIdx, itemIDs, onReorder, ctx.Window) {
					ctx.Consume()
					return
				}
			}
			listBoxOnKeyDown(listBoxID, itemIDs,
				isMultiple, onSelect, selectedIDs,
				scrollID, rowH, listH, itemDataIndices, ctx.Event, ctx.Window)
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
			if !cfg.Data[i].isSubheading {
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
// virtualization parameters for the list box. The height comes from
// Cfg.Height, then MaxHeight, then the cache's resolvedH — the
// height the layout engine allocated last frame, which is how Fill
// sizing virtualizes.
func listBoxVisibleRange(
	cfg *ListBoxCfg, cache *listBoxCache, w *Window,
) (first, last int, virtualize bool, listH, rowH float32) {
	first = 0
	last = len(cfg.Data) - 1
	virtualize = cfg.Scrollable
	listH = cfg.Height
	if listH <= 0 {
		listH = cfg.MaxHeight
	}
	if listH <= 0 {
		listH = cache.resolvedH
	}
	rowH = listCoreRowHeightEstimate(cfg.TextStyle, listBoxItemPad)
	if virtualize && listH > 0 && len(cfg.Data) > 0 {
		// Default 0: absent entry means not scrolled.
		scrollY := w.scrollY().GetOr(cfg.ID, 0)
		first, last = listCoreVisibleRange(
			len(cfg.Data), rowH, listH, scrollY)
	} else {
		virtualize = false
		if cfg.Scrollable && cache.hSeen &&
			len(cfg.Data) > 0 && DebugEnabled() {
			// The layout has run at least once and still gave the
			// list no height, so every row builds each frame.
			w.debugWarn(debugCheckListBoxNoHeight, cfg.ID,
				"scrollable listbox %q resolved to height 0, so "+
					"virtualization is off and all %d rows build "+
					"every frame; set Height/MaxHeight or give it "+
					"sizing that allocates height",
				cfg.ID, len(cfg.Data))
		}
	}
	return first, last, virtualize, listH, rowH
}

// listBoxAmendLayout captures the arranged height into the cache so
// the next frame's view phase can virtualize against it. Runs after
// sizing, when Shape.Height holds the resolved height — including
// the Fill-allocated height the view phase cannot know.
func listBoxAmendLayout(cache *listBoxCache) func(EventCtx) {
	return func(ctx EventCtx) {
		if ctx.Layout == nil || ctx.Layout.Shape == nil {
			return
		}
		// Guard against a non-finite or degenerate height: a
		// non-finite value would flow into listCoreVisibleRange's
		// float→int division.
		if h := ctx.Layout.Shape.Height; h > 0 && f32IsFinite(h) {
			cache.resolvedH = h
		}
		cache.hSeen = true
	}
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
		if !cfg.Data[i].isSubheading {
			dragIdxByRow[i] = di
			di++
		} else {
			dragIdxByRow[i] = -1
		}
	}
	return dragIdxByRow
}

func listBoxOnKeyDown(
	listBoxID string,
	itemIDs []string,
	isMultiple bool,
	onSelect func([]string, EventCtx),
	selectedIDs []string,
	scrollID string, rowH, listH float32,
	itemDataIndices []int,
	e *Event,
	w *Window,
) {
	if len(itemIDs) == 0 || onSelect == nil {
		return
	}

	action := listCoreNavigate(e.KeyCode, len(itemIDs))
	if e.KeyCode == KeySpace {
		action = listCoreSelectItem
	}
	if action == listCoreNone {
		return
	}
	e.IsHandled = true

	lbf := StateMap[string, int](w, nsListBoxFocus, capModerate)
	// Default 0: bounds-checked before use; zero index handled.
	curIdx := lbf.GetOr(listBoxID, 0)

	if action == listCoreSelectItem {
		if curIdx >= 0 && curIdx < len(itemIDs) {
			datID := itemIDs[curIdx]
			ids := listBoxNextSelectedIDs(
				selectedIDs, datID, isMultiple)
			onSelect(ids, EventCtx{nil, e, w})
		}
		return
	}

	next, changed := listCoreApplyNav(action, curIdx, len(itemIDs))
	if changed {
		lbf.Set(listBoxID, next)
		if scrollID != "" && rowH > 0 {
			scrollEnsureVisible(scrollID,
				listBoxDataIndex(itemDataIndices, next),
				rowH, listH, w)
		}
		w.UpdateWindow()
	}
}

func applyListBoxDefaults(cfg *ListBoxCfg) {
	d := &defaultListBoxStyle
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
		cfg.Padding = d.Padding
	}

	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.textStyleNormal
	}
	if cfg.subheadingStyle == (TextStyle{}) {
		cfg.subheadingStyle = d.subheadingStyle
	}
}

// listBoxDataHash hashes the fields the list box cache derives
// from: ID and IsSubheading. Name and Value never reach the cache, so
// hashing them would burn O(data) time every frame for nothing.
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

		if it.isSubheading {
			h ^= 1
		}
		h *= prime
	}
	return h
}
