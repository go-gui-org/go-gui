package gui

import "slices"

// TabItemCfg configures one tab in a TabControl.
type TabItemCfg struct {
	ID       string
	Label    string
	Content  []View
	Disabled bool
}

// NewTabItem creates a TabItemCfg.
func NewTabItem(id, label string, content []View) TabItemCfg {
	return TabItemCfg{ID: id, Label: label, Content: content}
}

// TabControlCfg configures a tab control. Controlled component:
// Selected is owned by app state and updated through OnSelect.
type TabControlCfg struct {
	TextStyle         TextStyle
	textStyleSelected TextStyle
	textStyleDisabled TextStyle
	OnSelect          func(string, EventCtx)
	OnReorder         func(string, string, EventCtx)

	ID       string
	Selected string

	A11YLabel           string
	A11YDescription     string
	Items               []TabItemCfg
	Padding             Padding
	PaddingHeader       Padding
	paddingContent      Padding
	paddingTab          Padding
	SizeBorder          Opt[float32]
	sizeHeaderBorder    Opt[float32]
	sizeContentBorder   Opt[float32]
	sizeTabBorder       Opt[float32]
	Radius              Opt[float32]
	radiusHeader        Opt[float32]
	radiusContent       Opt[float32]
	radiusTab           Opt[float32]
	Spacing             Opt[float32]
	spacingHeader       Opt[float32]
	Focusable           bool
	Color               Color
	ColorBorder         Color
	ColorHeader         Color
	colorHeaderBorder   Color
	colorContent        Color
	colorContentBorder  Color
	colorTab            Color
	colorTabHover       Color
	colorTabFocus       Color
	colorTabClick       Color
	colorTabSelected    Color
	colorTabDisabled    Color
	colorTabBorder      Color
	colorTabBorderFocus Color
	Sizing              Sizing
	Disabled            bool
	Invisible           bool
	Reorderable         bool
}

type tabControlView struct {
	cfg TabControlCfg
}

func (tv *tabControlView) Content() []View { return nil }

func applyTabControlDefaults(cfg *TabControlCfg) {
	s := &defaultTabControlStyle
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FillFill
	}
	if !cfg.Color.IsSet() {
		cfg.Color = s.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.ColorHeader.IsSet() {
		cfg.ColorHeader = s.ColorHeader
	}
	if !cfg.colorHeaderBorder.IsSet() {
		cfg.colorHeaderBorder = s.colorHeaderBorder
	}
	if !cfg.colorContent.IsSet() {
		cfg.colorContent = s.colorContent
	}
	if !cfg.colorContentBorder.IsSet() {
		cfg.colorContentBorder = s.colorContentBorder
	}
	if !cfg.colorTab.IsSet() {
		cfg.colorTab = s.colorTab
	}
	if !cfg.colorTabHover.IsSet() {
		cfg.colorTabHover = s.colorTabHover
	}
	if !cfg.colorTabFocus.IsSet() {
		cfg.colorTabFocus = s.colorTabFocus
	}
	if !cfg.colorTabClick.IsSet() {
		cfg.colorTabClick = s.colorTabClick
	}
	if !cfg.colorTabSelected.IsSet() {
		cfg.colorTabSelected = s.colorTabSelected
	}
	if !cfg.colorTabDisabled.IsSet() {
		cfg.colorTabDisabled = s.colorTabDisabled
	}
	if !cfg.colorTabBorder.IsSet() {
		cfg.colorTabBorder = s.colorTabBorder
	}
	if !cfg.colorTabBorderFocus.IsSet() {
		cfg.colorTabBorderFocus = s.colorTabBorderFocus
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = s.Padding
	}
	if !cfg.PaddingHeader.IsSet() {
		cfg.PaddingHeader = s.PaddingHeader
	}
	if !cfg.paddingContent.IsSet() {
		cfg.paddingContent = s.paddingContent
	}
	if !cfg.paddingTab.IsSet() {
		cfg.paddingTab = s.paddingTab
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.textStyleSelected == (TextStyle{}) {
		cfg.textStyleSelected = s.textStyleSelected
	}
	if cfg.textStyleDisabled == (TextStyle{}) {
		cfg.textStyleDisabled = s.textStyleDisabled
	}
}

// TabControl creates a tab control with header row and content.
func TabControl(cfg TabControlCfg) View {
	applyTabControlDefaults(&cfg)
	return &tabControlView{cfg: cfg}
}

func makeTabOnClick(
	onSelect func(string, EventCtx),
	id string, focusID string,
) func(EventCtx) {
	return func(ctx EventCtx) {
		if onSelect != nil {
			onSelect(id, EventCtx{nil, ctx.Event, ctx.Window})
		}
		if focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		ctx.Consume()
	}
}

func makeTabDragClick(
	controlID string,
	dragIdx int,
	itemID string,
	tabIDs []string,
	onReorder func(string, string, EventCtx),
	tabLayoutIDs []string,
	onSelect func(string, EventCtx),
	focusID string,
) func(EventCtx) {
	return func(ctx EventCtx) {
		dragReorderStart(dragReorderStartCfg{
			DragKey:       controlID,
			Index:         dragIdx,
			ItemID:        itemID,
			Axis:          dragReorderHorizontal,
			ItemIDs:       tabIDs,
			OnReorder:     onReorder,
			ItemLayoutIDs: tabLayoutIDs,
			Layout:        ctx.Layout,
			Event:         ctx.Event,
		}, ctx.Window)
		if onSelect != nil {
			onSelect(itemID, EventCtx{nil, ctx.Event, ctx.Window})
		}
		if focusID != "" {
			ctx.Window.SetFocus(focusID)
		}
		ctx.Consume()
	}
}

//nolint:gocyclo // complex widget layout
func (tv *tabControlView) GenerateLayout(w *Window) Layout {
	cfg := &tv.cfg

	// One resolved identity for every key below; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)
	s := &defaultTabControlStyle

	// Resolve Opt fields.
	sizeBorder := cfg.SizeBorder.Get(s.SizeBorder)
	sizeHeaderBorder := cfg.sizeHeaderBorder.Get(s.sizeHeaderBorder)
	sizeContentBorder := cfg.sizeContentBorder.Get(s.sizeContentBorder)
	sizeTabBorder := cfg.sizeTabBorder.Get(s.sizeTabBorder)
	radius := cfg.Radius.Get(s.Radius)
	radiusHeader := cfg.radiusHeader.Get(s.radiusHeader)
	radiusContent := cfg.radiusContent.Get(s.radiusContent)
	radiusTab := cfg.radiusTab.Get(s.radiusTab)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingHeader := cfg.spacingHeader.Get(s.spacingHeader)

	// Build tab navigation arrays.
	tabNavIDs := make([]string, len(cfg.Items))
	tabNavDisabled := make([]bool, len(cfg.Items))
	for i, item := range cfg.Items {
		tabNavIDs[i] = item.ID
		tabNavDisabled[i] = item.Disabled
	}
	selectedIdx := tabSelectedIndex(tabNavIDs, tabNavDisabled, cfg.Selected)

	// Reorderable-specific state.
	var tabIDs, tabLayoutIDs []string
	var tabDragIdx map[int]int
	var drag dragReorderState
	var dragging bool

	if cfg.Reorderable {
		tabIDs = make([]string, 0, len(cfg.Items))
		tabLayoutIDs = make([]string, 0, len(cfg.Items))
		tabDragIdx = make(map[int]int)
		di := 0
		for i, item := range cfg.Items {
			if !item.Disabled && !cfg.Disabled {
				tabIDs = append(tabIDs, item.ID)
				tabLayoutIDs = append(tabLayoutIDs,
					tabButtonID(cfg.ID, item.ID))
				tabDragIdx[i] = di
				di++
			}
		}
		drag = dragReorderGet(w, cfg.ID)
		dragging = drag.active && !drag.cancelled
		if drag.started || drag.active {
			dragReorderIDsMetaSet(w, cfg.ID, tabIDs)
		}
	}

	headerCap := len(cfg.Items)
	if cfg.Reorderable {
		headerCap += 3
	}
	headerItems := make([]View, 0, headerCap)
	var ghostView View
	onReorder := cfg.OnReorder

	for i, item := range cfg.Items {
		isSelected := i == selectedIdx
		isDisabled := cfg.Disabled || item.Disabled

		// Reorderable: insert gap before current drag position.
		if cfg.Reorderable && dragging && !isDisabled &&
			tabDragIdx[i] == drag.currentIndex {
			headerItems = append(headerItems,
				dragReorderGapView(drag, dragReorderHorizontal))
		}

		tabColor := cfg.colorTab
		hoverColor := cfg.colorTabHover
		focusColor := cfg.colorTabFocus
		clickColor := cfg.colorTabClick
		borderColor := cfg.colorTabBorder

		if isDisabled {
			tabColor = cfg.colorTabDisabled
			hoverColor = cfg.colorTabDisabled
			focusColor = cfg.colorTabDisabled
			clickColor = cfg.colorTabDisabled
		} else if isSelected {
			tabColor = cfg.colorTabSelected
			hoverColor = cfg.colorTabSelected
			focusColor = cfg.colorTabSelected
			clickColor = cfg.colorTabSelected
		}

		ts := cfg.TextStyle
		if isDisabled {
			ts = cfg.textStyleDisabled
		} else if isSelected {
			ts = cfg.textStyleSelected
		}

		a11yState := AccessStateNone
		if isSelected {
			a11yState = AccessStateSelected
		}

		var onClick func(EventCtx)
		if cfg.Reorderable && !isDisabled {
			onClick = makeTabDragClick(cfg.ID, tabDragIdx[i],
				item.ID, tabIDs, onReorder, tabLayoutIDs,
				cfg.OnSelect, cfg.ID)
		} else if !isDisabled {
			onClick = makeTabOnClick(
				cfg.OnSelect, item.ID, cfg.ID)
		}

		tabBtn := Button(ButtonCfg{
			ID:         tabButtonID(cfg.ID, item.ID),
			A11YRole:   AccessRoleTabItem,
			A11YState:  a11yState,
			A11YLabel:  item.Label,
			Color:      tabColor,
			Colors:     ColorSet{Hover: hoverColor, Click: clickColor, Focus: focusColor, Border: borderColor, BorderFocus: cfg.colorTabBorderFocus},
			Padding:    cfg.paddingTab,
			SizeBorder: SomeF(sizeTabBorder),
			Radius:     SomeF(radiusTab),
			Disabled:   isDisabled,
			OnClick:    onClick,
			Content: []View{
				Text(TextCfg{Text: item.Label, TextStyle: ts}),
			},
		})

		// Reorderable: skip source item (becomes ghost).
		if cfg.Reorderable && dragging && !isDisabled &&
			tabDragIdx[i] == drag.sourceIndex {
			ghostView = tabBtn
			continue
		}

		headerItems = append(headerItems, tabBtn)
	}

	// Trailing reorderable elements.
	if cfg.Reorderable && dragging {
		if drag.currentIndex >= len(tabIDs) {
			headerItems = append(headerItems,
				dragReorderGapView(drag, dragReorderHorizontal))
		}
		if ghostView != nil {
			headerItems = append(headerItems,
				dragReorderGhostView(drag, ghostView))
		}
	}

	// Active content — direct assignment, no copy needed.
	var activeContent []View
	if selectedIdx >= 0 && selectedIdx < len(cfg.Items) {
		activeContent = cfg.Items[selectedIdx].Content
	}

	// Closure captures.
	disabled := cfg.Disabled
	selected := cfg.Selected
	onSelect := cfg.OnSelect
	focusID := cfg.ID
	reorderable := cfg.Reorderable
	controlID := cfg.ID

	return generateViewLayout(Column(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       cfg.Focusable,
		A11YRole:        AccessRoleTab,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
		A11YDescription: cfg.A11YDescription,
		Sizing:          cfg.Sizing,
		Color:           cfg.Color,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      SomeF(sizeBorder),
		Radius:          SomeF(radius),
		Padding:         cfg.Padding,
		Spacing:         SomeF(spacing),
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		OnKeyDown: func(ctx EventCtx) {
			if reorderable {
				if dragReorderEscape(
					controlID, ctx.Event.KeyCode, ctx.Window) {
					ctx.Consume()
					return
				}
				for idx, id := range tabIDs {
					if id == selected {
						if dragReorderKeyboardMove(
							ctx.Event.KeyCode, ctx.Event.Modifiers,
							dragReorderHorizontal,
							idx, tabIDs, onReorder, ctx.Window) {
							ctx.Consume()
							return
						}
						break
					}
				}
			}
			tabControlOnKeydown(disabled, tabNavIDs,
				tabNavDisabled, selected, onSelect,
				focusID, ctx.Event, ctx.Window)
		},
		Content: []View{
			Row(ContainerCfg{
				Color:       cfg.ColorHeader,
				ColorBorder: cfg.colorHeaderBorder,
				SizeBorder:  SomeF(sizeHeaderBorder),
				Radius:      SomeF(radiusHeader),
				Padding:     cfg.PaddingHeader,
				Spacing:     SomeF(spacingHeader),
				Sizing:      FillFit,
				Content:     headerItems,
			}),
			Column(ContainerCfg{
				Color:       cfg.colorContent,
				ColorBorder: cfg.colorContentBorder,
				SizeBorder:  SomeF(sizeContentBorder),
				Radius:      SomeF(radiusContent),
				Padding:     cfg.paddingContent,
				Sizing:      FillFill,
				Content:     activeContent,
			}),
		},
	}), w)
}

func tabControlOnKeydown(
	disabled bool,
	tabNavIDs []string,
	tabNavDisabled []bool,
	selected string,
	onSelect func(string, EventCtx),
	focusID string,
	e *Event,
	w *Window,
) {
	if disabled || len(tabNavIDs) == 0 || e.Modifiers != ModNone {
		return
	}

	selectedIdx := tabSelectedIndex(tabNavIDs, tabNavDisabled, selected)
	var targetIdx int

	switch e.KeyCode {
	case KeyLeft, KeyUp:
		if selectedIdx >= 0 {
			targetIdx = tabPrevEnabledIndex(tabNavDisabled, selectedIdx)
		} else {
			targetIdx = tabLastEnabledIndex(tabNavDisabled)
		}
	case KeyRight, KeyDown:
		if selectedIdx >= 0 {
			targetIdx = tabNextEnabledIndex(tabNavDisabled, selectedIdx)
		} else {
			targetIdx = tabFirstEnabledIndex(tabNavDisabled)
		}
	case KeyHome:
		targetIdx = tabFirstEnabledIndex(tabNavDisabled)
	case KeyEnd:
		targetIdx = tabLastEnabledIndex(tabNavDisabled)
	case KeyEnter:
		if selectedIdx >= 0 {
			targetIdx = selectedIdx
		} else {
			targetIdx = tabFirstEnabledIndex(tabNavDisabled)
		}
	default:
		if e.CharCode == charSpace {
			if selectedIdx >= 0 {
				targetIdx = selectedIdx
			} else {
				targetIdx = tabFirstEnabledIndex(tabNavDisabled)
			}
		} else {
			return
		}
	}

	if targetIdx < 0 || targetIdx >= len(tabNavIDs) {
		return
	}
	targetID := tabNavIDs[targetIdx]
	if len(targetID) == 0 {
		return
	}

	refire := e.KeyCode == KeyEnter || e.CharCode == charSpace
	if targetID != selected || refire {
		if onSelect != nil {
			onSelect(targetID, EventCtx{nil, e, w})
		}
	}
	if focusID != "" {
		w.SetFocus(focusID)
	}
	e.IsHandled = true
}

func tabSelectedIndex(ids []string, disabled []bool, selected string) int {
	if len(selected) > 0 {
		for i, id := range ids {
			if id == selected && !disabled[i] {
				return i
			}
		}
	}
	return tabFirstEnabledIndex(disabled)
}

func tabFirstEnabledIndex(disabled []bool) int {
	for i, d := range disabled {
		if !d {
			return i
		}
	}
	return -1
}

func tabLastEnabledIndex(disabled []bool) int {
	for i, d := range slices.Backward(disabled) {
		if !d {
			return i
		}
	}
	return -1
}

func tabNextEnabledIndex(disabled []bool, selectedIdx int) int {
	n := len(disabled)
	if n == 0 {
		return -1
	}
	idx := selectedIdx
	if idx < 0 || idx >= n {
		idx = -1
	}
	for range n {
		idx = (idx + 1 + n) % n
		if !disabled[idx] {
			return idx
		}
	}
	return -1
}

func tabPrevEnabledIndex(disabled []bool, selectedIdx int) int {
	n := len(disabled)
	if n == 0 {
		return -1
	}
	idx := selectedIdx
	if idx < 0 || idx >= n {
		idx = 0
	}
	for range n {
		idx = (idx - 1 + n) % n
		if !disabled[idx] {
			return idx
		}
	}
	return -1
}

func tabButtonID(controlID, tabID string) string {
	return ScopeID(controlID, "tab", tabID)
}
