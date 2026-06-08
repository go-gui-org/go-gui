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
	TextStyleSelected TextStyle
	TextStyleDisabled TextStyle
	OnSelect          func(string, *Event, *Window)
	OnReorder         func(movedID, beforeID string, w *Window)

	ID       string
	Selected string

	A11YLabel           string
	A11YDescription     string
	Items               []TabItemCfg
	Padding             Opt[Padding]
	PaddingHeader       Opt[Padding]
	PaddingContent      Opt[Padding]
	PaddingTab          Opt[Padding]
	SizeBorder          Opt[float32]
	SizeHeaderBorder    Opt[float32]
	SizeContentBorder   Opt[float32]
	SizeTabBorder       Opt[float32]
	Radius              Opt[float32]
	RadiusHeader        Opt[float32]
	RadiusContent       Opt[float32]
	RadiusTab           Opt[float32]
	Spacing             Opt[float32]
	SpacingHeader       Opt[float32]
	IDFocus             uint32
	Color               Color
	ColorBorder         Color
	ColorHeader         Color
	ColorHeaderBorder   Color
	ColorContent        Color
	ColorContentBorder  Color
	ColorTab            Color
	ColorTabHover       Color
	ColorTabFocus       Color
	ColorTabClick       Color
	ColorTabSelected    Color
	ColorTabDisabled    Color
	ColorTabBorder      Color
	ColorTabBorderFocus Color
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
	s := &DefaultTabControlStyle
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
	if !cfg.ColorHeaderBorder.IsSet() {
		cfg.ColorHeaderBorder = s.ColorHeaderBorder
	}
	if !cfg.ColorContent.IsSet() {
		cfg.ColorContent = s.ColorContent
	}
	if !cfg.ColorContentBorder.IsSet() {
		cfg.ColorContentBorder = s.ColorContentBorder
	}
	if !cfg.ColorTab.IsSet() {
		cfg.ColorTab = s.ColorTab
	}
	if !cfg.ColorTabHover.IsSet() {
		cfg.ColorTabHover = s.ColorTabHover
	}
	if !cfg.ColorTabFocus.IsSet() {
		cfg.ColorTabFocus = s.ColorTabFocus
	}
	if !cfg.ColorTabClick.IsSet() {
		cfg.ColorTabClick = s.ColorTabClick
	}
	if !cfg.ColorTabSelected.IsSet() {
		cfg.ColorTabSelected = s.ColorTabSelected
	}
	if !cfg.ColorTabDisabled.IsSet() {
		cfg.ColorTabDisabled = s.ColorTabDisabled
	}
	if !cfg.ColorTabBorder.IsSet() {
		cfg.ColorTabBorder = s.ColorTabBorder
	}
	if !cfg.ColorTabBorderFocus.IsSet() {
		cfg.ColorTabBorderFocus = s.ColorTabBorderFocus
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(s.Padding)
	}
	if !cfg.PaddingHeader.IsSet() {
		cfg.PaddingHeader = Some(s.PaddingHeader)
	}
	if !cfg.PaddingContent.IsSet() {
		cfg.PaddingContent = Some(s.PaddingContent)
	}
	if !cfg.PaddingTab.IsSet() {
		cfg.PaddingTab = Some(s.PaddingTab)
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleSelected == (TextStyle{}) {
		cfg.TextStyleSelected = s.TextStyleSelected
	}
	if cfg.TextStyleDisabled == (TextStyle{}) {
		cfg.TextStyleDisabled = s.TextStyleDisabled
	}
}

// TabControl creates a tab control with header row and content.
func TabControl(cfg TabControlCfg) View {
	applyTabControlDefaults(&cfg)
	return &tabControlView{cfg: cfg}
}

func makeTabOnClick(
	onSelect func(string, *Event, *Window),
	id string, idFocus uint32,
) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		if onSelect != nil {
			onSelect(id, e, w)
		}
		if idFocus > 0 {
			w.SetIDFocus(idFocus)
		}
		e.IsHandled = true
	}
}

func makeTabDragClick(
	controlID string,
	dragIdx int,
	itemID string,
	tabIDs []string,
	onReorder func(string, string, *Window),
	tabLayoutIDs []string,
	onSelect func(string, *Event, *Window),
	idFocus uint32,
) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		dragReorderStart(dragReorderStartCfg{
			DragKey:       controlID,
			Index:         dragIdx,
			ItemID:        itemID,
			Axis:          DragReorderHorizontal,
			ItemIDs:       tabIDs,
			OnReorder:     onReorder,
			ItemLayoutIDs: tabLayoutIDs,
			Layout:        layout,
			Event:         e,
		}, w)
		if onSelect != nil {
			onSelect(itemID, e, w)
		}
		if idFocus > 0 {
			w.SetIDFocus(idFocus)
		}
		e.IsHandled = true
	}
}

//nolint:gocyclo // complex widget layout
func (tv *tabControlView) GenerateLayout(w *Window) Layout {
	cfg := &tv.cfg
	s := &DefaultTabControlStyle

	// Resolve Opt fields.
	sizeBorder := cfg.SizeBorder.Get(s.SizeBorder)
	sizeHeaderBorder := cfg.SizeHeaderBorder.Get(s.SizeHeaderBorder)
	sizeContentBorder := cfg.SizeContentBorder.Get(s.SizeContentBorder)
	sizeTabBorder := cfg.SizeTabBorder.Get(s.SizeTabBorder)
	radius := cfg.Radius.Get(s.Radius)
	radiusHeader := cfg.RadiusHeader.Get(s.RadiusHeader)
	radiusContent := cfg.RadiusContent.Get(s.RadiusContent)
	radiusTab := cfg.RadiusTab.Get(s.RadiusTab)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingHeader := cfg.SpacingHeader.Get(s.SpacingHeader)

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
				dragReorderGapView(drag, DragReorderHorizontal))
		}

		tabColor := cfg.ColorTab
		hoverColor := cfg.ColorTabHover
		focusColor := cfg.ColorTabFocus
		clickColor := cfg.ColorTabClick
		borderColor := cfg.ColorTabBorder

		if isDisabled {
			tabColor = cfg.ColorTabDisabled
			hoverColor = cfg.ColorTabDisabled
			focusColor = cfg.ColorTabDisabled
			clickColor = cfg.ColorTabDisabled
		} else if isSelected {
			tabColor = cfg.ColorTabSelected
			hoverColor = cfg.ColorTabSelected
			focusColor = cfg.ColorTabSelected
			clickColor = cfg.ColorTabSelected
		}

		ts := cfg.TextStyle
		if isDisabled {
			ts = cfg.TextStyleDisabled
		} else if isSelected {
			ts = cfg.TextStyleSelected
		}

		a11yState := AccessStateNone
		if isSelected {
			a11yState = AccessStateSelected
		}

		var onClick func(*Layout, *Event, *Window)
		if cfg.Reorderable && !isDisabled {
			onClick = makeTabDragClick(cfg.ID, tabDragIdx[i],
				item.ID, tabIDs, onReorder, tabLayoutIDs,
				cfg.OnSelect, cfg.IDFocus)
		} else if !isDisabled {
			onClick = makeTabOnClick(
				cfg.OnSelect, item.ID, cfg.IDFocus)
		}

		tabBtn := Button(ButtonCfg{
			ID:               tabButtonID(cfg.ID, item.ID),
			A11YRole:         AccessRoleTabItem,
			A11YState:        a11yState,
			A11YLabel:        item.Label,
			Color:            tabColor,
			ColorHover:       hoverColor,
			ColorFocus:       focusColor,
			ColorClick:       clickColor,
			ColorBorder:      borderColor,
			ColorBorderFocus: cfg.ColorTabBorderFocus,
			Padding:          cfg.PaddingTab,
			SizeBorder:       SomeF(sizeTabBorder),
			Radius:           SomeF(radiusTab),
			Disabled:         isDisabled,
			OnClick:          onClick,
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
				dragReorderGapView(drag, DragReorderHorizontal))
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
	idFocus := cfg.IDFocus
	reorderable := cfg.Reorderable
	controlID := cfg.ID

	return generateViewLayout(Column(ContainerCfg{
		ID:              cfg.ID,
		IDFocus:         cfg.IDFocus,
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
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			if reorderable {
				if dragReorderEscape(
					controlID, e.KeyCode, w) {
					e.IsHandled = true
					return
				}
				for idx, id := range tabIDs {
					if id == selected {
						if dragReorderKeyboardMove(
							e.KeyCode, e.Modifiers,
							DragReorderHorizontal,
							idx, tabIDs, onReorder, w) {
							e.IsHandled = true
							return
						}
						break
					}
				}
			}
			tabControlOnKeydown(disabled, tabNavIDs,
				tabNavDisabled, selected, onSelect,
				idFocus, e, w)
		},
		Content: []View{
			Row(ContainerCfg{
				Color:       cfg.ColorHeader,
				ColorBorder: cfg.ColorHeaderBorder,
				SizeBorder:  SomeF(sizeHeaderBorder),
				Radius:      SomeF(radiusHeader),
				Padding:     cfg.PaddingHeader,
				Spacing:     SomeF(spacingHeader),
				Sizing:      FillFit,
				Content:     headerItems,
			}),
			Column(ContainerCfg{
				Color:       cfg.ColorContent,
				ColorBorder: cfg.ColorContentBorder,
				SizeBorder:  SomeF(sizeContentBorder),
				Radius:      SomeF(radiusContent),
				Padding:     cfg.PaddingContent,
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
	onSelect func(string, *Event, *Window),
	idFocus uint32,
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
		if e.CharCode == CharSpace {
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

	refire := e.KeyCode == KeyEnter || e.CharCode == CharSpace
	if targetID != selected || refire {
		if onSelect != nil {
			onSelect(targetID, e, w)
		}
	}
	if idFocus > 0 {
		w.SetIDFocus(idFocus)
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
	return "tc_" + controlID + "_" + tabID
}
