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
	TextStyle TextStyle
	// TextStyleSelected styles the selected tab label. Zero takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleSelected TextStyle
	// TextStyleDisabled styles disabled tab labels. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleDisabled TextStyle
	OnSelect          func(string, EventCtx)
	OnReorder         func(string, string, EventCtx)

	ID       string
	Selected string

	A11YCfg
	Items         []TabItemCfg
	Padding       Padding
	PaddingHeader Padding
	// PaddingContent insets the tab content area. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingContent Padding
	// PaddingTab insets each tab. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingTab Padding
	SizeBorder Opt[float32]
	// SizeHeaderBorder widths the header strip border. Unset takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeHeaderBorder Opt[float32]
	// SizeContentBorder widths the content area border. Unset takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeContentBorder Opt[float32]
	// SizeTabBorder widths each tab's border. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeTabBorder Opt[float32]
	Radius        Opt[float32]
	// RadiusHeader rounds the header strip. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusHeader Opt[float32]
	// RadiusContent rounds the content area. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusContent Opt[float32]
	// RadiusTab rounds each tab. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusTab Opt[float32]
	Spacing   Opt[float32]
	// SpacingHeader gaps the header tabs. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	SpacingHeader Opt[float32]
	Focusable     bool
	Color         Color
	ColorBorder   Color
	ColorHeader   Color
	// ColorHeaderBorder colors the header strip border. Unset takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorHeaderBorder Color
	// ColorContent colors the content area. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorContent Color
	// ColorContentBorder colors the content area border. Unset
	// takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorContentBorder Color
	// ColorTab/ColorTabHover/ColorTabFocus/ColorTabClick/
	// ColorTabSelected/ColorTabDisabled/ColorTabBorder/
	// ColorTabBorderFocus theme the tabs. Unset takes the theme
	// defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTab Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabHover Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabFocus Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabClick Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabSelected Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabDisabled Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabBorder Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTabBorderFocus Color
	Sizing              Sizing
	Disabled            bool
	Invisible           bool
	Reorderable         bool

	// Sound overrides the theme's selection cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses every tab's sound regardless of the theme
	// and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
}

type tabControlView struct {
	cfg TabControlCfg
}

func applyTabControlDefaults(cfg *TabControlCfg) {
	s := &defaultTabControlStyle
	cfg.Sizing = cfg.Sizing.Or(FillFill)
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
		cfg.ColorHeaderBorder = s.colorHeaderBorder
	}
	if !cfg.ColorContent.IsSet() {
		cfg.ColorContent = s.colorContent
	}
	if !cfg.ColorContentBorder.IsSet() {
		cfg.ColorContentBorder = s.colorContentBorder
	}
	if !cfg.ColorTab.IsSet() {
		cfg.ColorTab = s.colorTab
	}
	if !cfg.ColorTabHover.IsSet() {
		cfg.ColorTabHover = s.colorTabHover
	}
	if !cfg.ColorTabFocus.IsSet() {
		cfg.ColorTabFocus = s.colorTabFocus
	}
	if !cfg.ColorTabClick.IsSet() {
		cfg.ColorTabClick = s.colorTabClick
	}
	if !cfg.ColorTabSelected.IsSet() {
		cfg.ColorTabSelected = s.colorTabSelected
	}
	if !cfg.ColorTabDisabled.IsSet() {
		cfg.ColorTabDisabled = s.colorTabDisabled
	}
	if !cfg.ColorTabBorder.IsSet() {
		cfg.ColorTabBorder = s.colorTabBorder
	}
	if !cfg.ColorTabBorderFocus.IsSet() {
		cfg.ColorTabBorderFocus = s.colorTabBorderFocus
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = s.Padding
	}
	if !cfg.PaddingHeader.IsSet() {
		cfg.PaddingHeader = s.PaddingHeader
	}
	if !cfg.PaddingContent.IsSet() {
		cfg.PaddingContent = s.paddingContent
	}
	if !cfg.PaddingTab.IsSet() {
		cfg.PaddingTab = s.paddingTab
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = s.TextStyle
	}
	if cfg.TextStyleSelected == (TextStyle{}) {
		cfg.TextStyleSelected = s.textStyleSelected
	}
	if cfg.TextStyleDisabled == (TextStyle{}) {
		cfg.TextStyleDisabled = s.textStyleDisabled
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
	sizeHeaderBorder := cfg.SizeHeaderBorder.Get(s.sizeHeaderBorder)
	sizeContentBorder := cfg.SizeContentBorder.Get(s.sizeContentBorder)
	sizeTabBorder := cfg.SizeTabBorder.Get(s.sizeTabBorder)
	radius := cfg.Radius.Get(s.Radius)
	radiusHeader := cfg.RadiusHeader.Get(s.radiusHeader)
	radiusContent := cfg.RadiusContent.Get(s.radiusContent)
	radiusTab := cfg.RadiusTab.Get(s.radiusTab)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingHeader := cfg.SpacingHeader.Get(s.spacingHeader)

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

		// A disabled tab gets no handler, so it gets no cue either;
		// picking a tab is choosing one of several, which is the
		// selection role (issue #467).
		tabSound := SoundNone
		if !isDisabled {
			tabSound = resolveSoundCue(
				guiTheme.Sounds.Selection, cfg.Sound, cfg.SoundDisabled)
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
			A11YCfg:    A11YCfg{A11YLabel: item.Label},
			Color:      tabColor,
			Colors:     ColorSet{Hover: hoverColor, Click: clickColor, Focus: focusColor, Border: borderColor, BorderFocus: cfg.ColorTabBorderFocus}.resolved(tabColor, themeButtonSet()),
			Padding:    cfg.PaddingTab,
			SizeBorder: SomeF(sizeTabBorder),
			Radius:     SomeF(radiusTab),
			Disabled:   isDisabled,
			OnClick:    onClick,
			// SoundDisabled as well as Sound: ButtonCfg resolves its
			// own precedence, and a resolved SoundNone reads there as
			// "unset" and falls back to the theme. Saying both makes
			// silence stick.
			Sound:         tabSound,
			SoundDisabled: tabSound == SoundNone,
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
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleTab,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Sizing:      cfg.Sizing,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  SomeF(sizeBorder),
		Radius:      SomeF(radius),
		Padding:     cfg.Padding,
		Spacing:     SomeF(spacing),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
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
	// Space reads from KeyCode: backends set CharCode only on
	// EventChar, so testing it from a keydown handler never matched.
	// The space and Enter bodies were already identical, so they group.
	case KeyEnter, KeySpace:
		if selectedIdx >= 0 {
			targetIdx = selectedIdx
		} else {
			targetIdx = tabFirstEnabledIndex(tabNavDisabled)
		}
	default:
		return
	}

	if targetIdx < 0 || targetIdx >= len(tabNavIDs) {
		return
	}
	targetID := tabNavIDs[targetIdx]
	if len(targetID) == 0 {
		return
	}

	refire := e.KeyCode == KeyEnter || e.KeyCode == KeySpace
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
