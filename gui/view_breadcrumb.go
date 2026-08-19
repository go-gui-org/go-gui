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
	TextStyle TextStyle
	// TextStyleSelected styles the current crumb. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleSelected TextStyle
	// TextStyleDisabled styles disabled crumbs. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleDisabled TextStyle
	// TextStyleSeparator styles the separator glyph. Zero takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleSeparator TextStyle
	OnSelect           func(string, EventCtx)
	ID                 string
	Selected           string
	Separator          string

	A11YCfg
	Items   []BreadcrumbItemCfg
	Padding Padding
	// PaddingTrail insets the trail. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingTrail Padding
	// PaddingCrumb insets each crumb. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingCrumb Padding
	// PaddingContent insets the content area. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingContent Padding
	Radius         Opt[float32]
	// RadiusCrumb rounds each crumb. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusCrumb Opt[float32]
	// RadiusContent rounds the content area. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusContent Opt[float32]
	Spacing       Opt[float32]
	// SpacingTrail gaps the crumbs. Unset takes the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	SpacingTrail Opt[float32]
	SizeBorder   Opt[float32]
	// SizeContentBorder widths the content border. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	SizeContentBorder Opt[float32]
	Focusable         bool
	Color             Color
	ColorBorder       Color
	// ColorTrail colors the trail background. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTrail Color
	// ColorCrumb/ColorCrumbHover/ColorCrumbClick/
	// ColorCrumbSelected/ColorCrumbDisabled theme the crumbs. Unset
	// takes the theme defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorCrumb Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorCrumbHover Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorCrumbClick Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorCrumbSelected Color
	// exportaudit:keep — caller-facing config (issue #372)
	ColorCrumbDisabled Color
	// ColorContent colors the content area. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorContent Color
	// ColorContentBorder colors the content border. Unset takes the
	// theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorContentBorder Color
	Sizing             Sizing
	Disabled           bool
	Invisible          bool
}

func applyBreadcrumbDefaults(cfg *BreadcrumbCfg) {
	s := &defaultBreadcrumbStyle
	if cfg.Separator == "" {
		cfg.Separator = s.Separator
	}
	cfg.Sizing = cfg.Sizing.Or(FillFit)
	if !cfg.Color.IsSet() {
		cfg.Color = s.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = s.ColorBorder
	}
	if !cfg.ColorTrail.IsSet() {
		cfg.ColorTrail = s.colorTrail
	}
	if !cfg.ColorCrumb.IsSet() {
		cfg.ColorCrumb = s.colorCrumb
	}
	if !cfg.ColorCrumbHover.IsSet() {
		cfg.ColorCrumbHover = s.colorCrumbHover
	}
	if !cfg.ColorCrumbClick.IsSet() {
		cfg.ColorCrumbClick = s.colorCrumbClick
	}
	if !cfg.ColorCrumbSelected.IsSet() {
		cfg.ColorCrumbSelected = s.colorCrumbSelected
	}
	if !cfg.ColorCrumbDisabled.IsSet() {
		cfg.ColorCrumbDisabled = s.colorCrumbDisabled
	}
	if !cfg.ColorContent.IsSet() {
		cfg.ColorContent = s.colorContent
	}
	if !cfg.ColorContentBorder.IsSet() {
		cfg.ColorContentBorder = s.colorContentBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = s.Padding
	}
	if !cfg.PaddingTrail.IsSet() {
		cfg.PaddingTrail = s.paddingTrail
	}
	if !cfg.PaddingCrumb.IsSet() {
		cfg.PaddingCrumb = s.paddingCrumb
	}
	if !cfg.PaddingContent.IsSet() {
		cfg.PaddingContent = s.paddingContent
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
	if cfg.TextStyleSeparator == (TextStyle{}) {
		cfg.TextStyleSeparator = s.textStyleSeparator
	}
}

// Breadcrumb creates a breadcrumb navigation control.
func Breadcrumb(cfg BreadcrumbCfg) View {
	applyBreadcrumbDefaults(&cfg)

	s := &defaultBreadcrumbStyle
	radius := cfg.Radius.Get(s.Radius)
	radiusCrumb := cfg.RadiusCrumb.Get(s.radiusCrumb)
	radiusContent := cfg.RadiusContent.Get(s.radiusContent)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingTrail := cfg.SpacingTrail.Get(s.spacingTrail)
	sizeBorder := cfg.SizeBorder.Get(s.SizeBorder)
	sizeContentBorder := cfg.SizeContentBorder.Get(s.sizeContentBorder)

	selectedIdx := bcSelectedIndex(cfg.Items, cfg.Selected)

	trailItems := make([]View, 0, len(cfg.Items)*2)
	hasContent := bcHasAnyContent(cfg.Items)

	for i, item := range cfg.Items {
		if i > 0 {
			trailItems = append(trailItems, Text(TextCfg{
				Text:      cfg.Separator,
				TextStyle: cfg.TextStyleSeparator,
			}))
		}

		isSelected := i == selectedIdx
		isDisabled := cfg.Disabled || item.Disabled

		ts := cfg.TextStyle
		if isDisabled {
			ts = cfg.TextStyleDisabled
		} else if isSelected {
			ts = cfg.TextStyleSelected
		}

		crumbColor := cfg.ColorCrumb
		if isDisabled {
			crumbColor = cfg.ColorCrumbDisabled
		} else if isSelected {
			crumbColor = cfg.ColorCrumbSelected
		}

		hoverColor := cfg.ColorCrumbHover
		clickColor := cfg.ColorCrumbClick
		if isDisabled {
			hoverColor = cfg.ColorCrumbDisabled
			clickColor = cfg.ColorCrumbDisabled
		} else if isSelected {
			hoverColor = cfg.ColorCrumbSelected
			clickColor = cfg.ColorCrumbSelected
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
			Padding: cfg.PaddingCrumb,
			Radius:  Some(radiusCrumb),
			Spacing: Some(spacingTrail),
			OnClick: onClick,
			OnHover: onHover,
			Content: crumbContent,
		}))
	}

	outerContent := make([]View, 0, 2)
	outerContent = append(outerContent, Row(ContainerCfg{
		Color:   cfg.ColorTrail,
		Padding: cfg.PaddingTrail,
		Spacing: Some(spacingTrail),
		Sizing:  FillFit,
		VAlign:  VAlignMiddle,
		Content: trailItems,
	}))

	if hasContent && selectedIdx >= 0 && selectedIdx < len(cfg.Items) {
		outerContent = append(outerContent, Column(ContainerCfg{
			Color:       cfg.ColorContent,
			ColorBorder: cfg.ColorContentBorder,
			SizeBorder:  Some(sizeContentBorder),
			Radius:      Some(radiusContent),
			Padding:     cfg.PaddingContent,
			Sizing:      FillFill,
			Content:     cfg.Items[selectedIdx].Content,
		}))
	}

	return Column(ContainerCfg{
		ID:        cfg.ID,
		Focusable: cfg.Focusable,
		A11YRole:  AccessRoleToolbar,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
			A11YDescription: cfg.A11YDescription,
		},
		Sizing:      cfg.Sizing,
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Spacing:     Some(spacing),
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
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
	// Space reads from KeyCode: backends set CharCode only on
	// EventChar, so testing it from a keydown handler never matched.
	// The space and Enter bodies were already identical, so they group.
	case KeyEnter, KeySpace:
		if selectedIdx >= 0 {
			targetIdx = selectedIdx
		} else {
			targetIdx = bcFirstEnabledIndex(items)
		}
	default:
		return
	}

	if targetIdx < 0 || targetIdx >= len(items) {
		return
	}
	targetID := items[targetIdx].ID
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
