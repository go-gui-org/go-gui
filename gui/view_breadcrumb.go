package gui

import "slices"

// BreadcrumbItemCfg configures one item in a Breadcrumb.
type BreadcrumbItemCfg struct {
	ID       string
	Label    string
	Content  []View
	Disabled bool
}

// NewBreadcrumbItem creates a BreadcrumbItemCfg.
func NewBreadcrumbItem(id, label string, content []View) BreadcrumbItemCfg {
	return BreadcrumbItemCfg{ID: id, Label: label, Content: content}
}

// BreadcrumbCfg configures a breadcrumb navigation control.
// Controlled component: Selected is owned by app state and
// updated through OnSelect.
type BreadcrumbCfg struct {
	TextStyle          TextStyle
	textStyleSelected  TextStyle
	textStyleDisabled  TextStyle
	textStyleSeparator TextStyle
	OnSelect           func(string, EventCtx)
	ID                 string
	Selected           string
	Separator          string

	A11YLabel          string
	A11YDescription    string
	Items              []BreadcrumbItemCfg
	Padding            Opt[Padding]
	paddingTrail       Opt[Padding]
	paddingCrumb       Opt[Padding]
	paddingContent     Opt[Padding]
	Radius             Opt[float32]
	radiusCrumb        Opt[float32]
	radiusContent      Opt[float32]
	Spacing            Opt[float32]
	spacingTrail       Opt[float32]
	SizeBorder         Opt[float32]
	sizeContentBorder  Opt[float32]
	Focusable          bool
	Color              Color
	ColorBorder        Color
	colorTrail         Color
	colorCrumb         Color
	colorCrumbHover    Color
	colorCrumbClick    Color
	colorCrumbSelected Color
	colorCrumbDisabled Color
	colorContent       Color
	colorContentBorder Color
	Sizing             Sizing
	Disabled           bool
	Invisible          bool
}

func applyBreadcrumbDefaults(cfg *BreadcrumbCfg) {
	s := &defaultBreadcrumbStyle
	if cfg.Separator == "" {
		cfg.Separator = s.Separator
	}
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FillFit
	}
	if !cfg.Color.IsSet() {
		cfg.Color = s.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.colorTrail.IsSet() {
		cfg.colorTrail = s.colorTrail
	}
	if !cfg.colorCrumb.IsSet() {
		cfg.colorCrumb = s.colorCrumb
	}
	if !cfg.colorCrumbHover.IsSet() {
		cfg.colorCrumbHover = s.colorCrumbHover
	}
	if !cfg.colorCrumbClick.IsSet() {
		cfg.colorCrumbClick = s.colorCrumbClick
	}
	if !cfg.colorCrumbSelected.IsSet() {
		cfg.colorCrumbSelected = s.colorCrumbSelected
	}
	if !cfg.colorCrumbDisabled.IsSet() {
		cfg.colorCrumbDisabled = s.colorCrumbDisabled
	}
	if !cfg.colorContent.IsSet() {
		cfg.colorContent = s.colorContent
	}
	if !cfg.colorContentBorder.IsSet() {
		cfg.colorContentBorder = s.colorContentBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(s.Padding)
	}
	if !cfg.paddingTrail.IsSet() {
		cfg.paddingTrail = Some(s.paddingTrail)
	}
	if !cfg.paddingCrumb.IsSet() {
		cfg.paddingCrumb = Some(s.paddingCrumb)
	}
	if !cfg.paddingContent.IsSet() {
		cfg.paddingContent = Some(s.paddingContent)
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
	if cfg.textStyleSeparator == (TextStyle{}) {
		cfg.textStyleSeparator = s.textStyleSeparator
	}
}

// Breadcrumb creates a breadcrumb navigation control.
func Breadcrumb(cfg BreadcrumbCfg) View {
	applyBreadcrumbDefaults(&cfg)

	s := &defaultBreadcrumbStyle
	radius := cfg.Radius.Get(s.Radius)
	radiusCrumb := cfg.radiusCrumb.Get(s.radiusCrumb)
	radiusContent := cfg.radiusContent.Get(s.radiusContent)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingTrail := cfg.spacingTrail.Get(s.spacingTrail)
	sizeBorder := cfg.SizeBorder.Get(s.SizeBorder)
	sizeContentBorder := cfg.sizeContentBorder.Get(s.sizeContentBorder)

	selectedIdx := bcSelectedIndex(cfg.Items, cfg.Selected)

	trailItems := make([]View, 0, len(cfg.Items)*2)
	hasContent := bcHasAnyContent(cfg.Items)

	for i, item := range cfg.Items {
		if i > 0 {
			trailItems = append(trailItems, Text(TextCfg{
				Text:      cfg.Separator,
				TextStyle: cfg.textStyleSeparator,
			}))
		}

		isSelected := i == selectedIdx
		isDisabled := cfg.Disabled || item.Disabled

		ts := cfg.TextStyle
		if isDisabled {
			ts = cfg.textStyleDisabled
		} else if isSelected {
			ts = cfg.textStyleSelected
		}

		crumbColor := cfg.colorCrumb
		if isDisabled {
			crumbColor = cfg.colorCrumbDisabled
		} else if isSelected {
			crumbColor = cfg.colorCrumbSelected
		}

		hoverColor := cfg.colorCrumbHover
		clickColor := cfg.colorCrumbClick
		if isDisabled {
			hoverColor = cfg.colorCrumbDisabled
			clickColor = cfg.colorCrumbDisabled
		} else if isSelected {
			hoverColor = cfg.colorCrumbSelected
			clickColor = cfg.colorCrumbSelected
		}

		var onClick func(EventCtx)
		var onHover func(EventCtx)
		if !isDisabled {
			onClick = makeBcOnClick(cfg.OnSelect, item.ID, cfg.ID)
			onHover = makeBcOnHover(hoverColor, clickColor)
		}

		crumbContent := []View{
			Text(TextCfg{Text: item.Label, TextStyle: ts}),
		}

		trailItems = append(trailItems, Row(ContainerCfg{
			ID:      bcCrumbID(cfg.ID, item.ID),
			Color:   crumbColor,
			Padding: cfg.paddingCrumb,
			Radius:  Some(radiusCrumb),
			Spacing: Some(spacingTrail),
			OnClick: onClick,
			OnHover: onHover,
			Content: crumbContent,
		}))
	}

	outerContent := make([]View, 0, 2)
	outerContent = append(outerContent, Row(ContainerCfg{
		Color:   cfg.colorTrail,
		Padding: cfg.paddingTrail,
		Spacing: Some(spacingTrail),
		Sizing:  FillFit,
		VAlign:  VAlignMiddle,
		Content: trailItems,
	}))

	if hasContent && selectedIdx >= 0 && selectedIdx < len(cfg.Items) {
		outerContent = append(outerContent, Column(ContainerCfg{
			Color:       cfg.colorContent,
			ColorBorder: cfg.colorContentBorder,
			SizeBorder:  Some(sizeContentBorder),
			Radius:      Some(radiusContent),
			Padding:     cfg.paddingContent,
			Sizing:      FillFill,
			Content:     cfg.Items[selectedIdx].Content,
		}))
	}

	return Column(ContainerCfg{
		ID:              cfg.ID,
		Focusable:       cfg.Focusable,
		A11YRole:        AccessRoleToolbar,
		A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
		A11YDescription: cfg.A11YDescription,
		Sizing:          cfg.Sizing,
		Color:           cfg.Color,
		ColorBorder:     cfg.ColorBorder,
		SizeBorder:      Some(sizeBorder),
		Radius:          Some(radius),
		Padding:         cfg.Padding,
		Spacing:         Some(spacing),
		Disabled:        cfg.Disabled,
		Invisible:       cfg.Invisible,
		OnKeyDown: func(ctx EventCtx) {
			bcOnKeydown(cfg.Disabled, cfg.Items, cfg.Selected,
				cfg.OnSelect, cfg.ID, ctx.Event, ctx.Window)
		},
		Content: outerContent,
	})
}

func makeBcOnClick(
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

func makeBcOnHover(
	hoverColor, clickColor Color,
) func(EventCtx) {
	return func(ctx EventCtx) {
		if ctx.Layout.Shape.Disabled || !ctx.Layout.Shape.hasEvents() ||
			ctx.Layout.Shape.events.OnClick == nil {
			return
		}
		ctx.Window.SetMouseCursorPointingHand()
		ctx.Layout.Shape.Color = hoverColor
		if ctx.Event.MouseButton == MouseLeft {
			ctx.Layout.Shape.Color = clickColor
		}
	}
}

func bcOnKeydown(
	disabled bool,
	items []BreadcrumbItemCfg,
	selected string,
	onSelect func(string, EventCtx),
	focusID string,
	e *Event,
	w *Window,
) {
	if disabled || len(items) == 0 || e.Modifiers != ModNone {
		return
	}

	selectedIdx := bcSelectedIndex(items, selected)
	var targetIdx int

	switch e.KeyCode {
	case KeyLeft:
		if selectedIdx >= 0 {
			targetIdx = bcPrevEnabledIndex(items, selectedIdx)
		} else {
			targetIdx = bcLastEnabledIndex(items)
		}
	case KeyRight:
		if selectedIdx >= 0 {
			targetIdx = bcNextEnabledIndex(items, selectedIdx)
		} else {
			targetIdx = bcFirstEnabledIndex(items)
		}
	case KeyHome:
		targetIdx = bcFirstEnabledIndex(items)
	case KeyEnd:
		targetIdx = bcLastEnabledIndex(items)
	case KeyEnter:
		if selectedIdx >= 0 {
			targetIdx = selectedIdx
		} else {
			targetIdx = bcFirstEnabledIndex(items)
		}
	default:
		if e.CharCode == charSpace {
			if selectedIdx >= 0 {
				targetIdx = selectedIdx
			} else {
				targetIdx = bcFirstEnabledIndex(items)
			}
		} else {
			return
		}
	}

	if targetIdx < 0 || targetIdx >= len(items) {
		return
	}
	targetID := items[targetIdx].ID
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

// bcSelectedIndex resolves the selected index. Falls back to
// last enabled item (breadcrumb convention).
func bcSelectedIndex(items []BreadcrumbItemCfg, selected string) int {
	if len(selected) > 0 {
		for i, item := range items {
			if item.ID == selected && !item.Disabled {
				return i
			}
		}
	}
	return bcLastEnabledIndex(items)
}

func bcFirstEnabledIndex(items []BreadcrumbItemCfg) int {
	for i, item := range items {
		if !item.Disabled {
			return i
		}
	}
	return -1
}

func bcLastEnabledIndex(items []BreadcrumbItemCfg) int {
	for i, item := range slices.Backward(items) {
		if !item.Disabled {
			return i
		}
	}
	return -1
}

func bcNextEnabledIndex(items []BreadcrumbItemCfg, selectedIdx int) int {
	n := len(items)
	if n == 0 {
		return -1
	}
	idx := selectedIdx
	if idx < 0 || idx >= n {
		idx = -1
	}
	for range n {
		idx = (idx + 1 + n) % n
		if !items[idx].Disabled {
			return idx
		}
	}
	return -1
}

func bcPrevEnabledIndex(items []BreadcrumbItemCfg, selectedIdx int) int {
	n := len(items)
	if n == 0 {
		return -1
	}
	idx := selectedIdx
	if idx < 0 || idx >= n {
		idx = 0
	}
	for range n {
		idx = (idx - 1 + n) % n
		if !items[idx].Disabled {
			return idx
		}
	}
	return -1
}

func bcHasAnyContent(items []BreadcrumbItemCfg) bool {
	for _, item := range items {
		if len(item.Content) > 0 {
			return true
		}
	}
	return false
}

func bcCrumbID(controlID, itemID string) string {
	return ScopeID(controlID, "crumb", itemID)
}
