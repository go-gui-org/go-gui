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
	TextStyleSelected  TextStyle
	TextStyleDisabled  TextStyle
	TextStyleSeparator TextStyle
	OnSelect           func(string, *Event, *Window)
	ID                 string
	Selected           string
	Separator          string

	A11YLabel          string
	A11YDescription    string
	Items              []BreadcrumbItemCfg
	Padding            Opt[Padding]
	PaddingTrail       Opt[Padding]
	PaddingCrumb       Opt[Padding]
	PaddingContent     Opt[Padding]
	Radius             Opt[float32]
	RadiusCrumb        Opt[float32]
	RadiusContent      Opt[float32]
	Spacing            Opt[float32]
	SpacingTrail       Opt[float32]
	SizeBorder         Opt[float32]
	SizeContentBorder  Opt[float32]
	IDFocus            uint32
	Color              Color
	ColorBorder        Color
	ColorTrail         Color
	ColorCrumb         Color
	ColorCrumbHover    Color
	ColorCrumbClick    Color
	ColorCrumbSelected Color
	ColorCrumbDisabled Color
	ColorContent       Color
	ColorContentBorder Color
	Sizing             Sizing
	Disabled           bool
	Invisible          bool
}

func applyBreadcrumbDefaults(cfg *BreadcrumbCfg) {
	s := &DefaultBreadcrumbStyle
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
	if !cfg.ColorTrail.IsSet() {
		cfg.ColorTrail = s.ColorTrail
	}
	if !cfg.ColorCrumb.IsSet() {
		cfg.ColorCrumb = s.ColorCrumb
	}
	if !cfg.ColorCrumbHover.IsSet() {
		cfg.ColorCrumbHover = s.ColorCrumbHover
	}
	if !cfg.ColorCrumbClick.IsSet() {
		cfg.ColorCrumbClick = s.ColorCrumbClick
	}
	if !cfg.ColorCrumbSelected.IsSet() {
		cfg.ColorCrumbSelected = s.ColorCrumbSelected
	}
	if !cfg.ColorCrumbDisabled.IsSet() {
		cfg.ColorCrumbDisabled = s.ColorCrumbDisabled
	}
	if !cfg.ColorContent.IsSet() {
		cfg.ColorContent = s.ColorContent
	}
	if !cfg.ColorContentBorder.IsSet() {
		cfg.ColorContentBorder = s.ColorContentBorder
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(s.Padding)
	}
	if !cfg.PaddingTrail.IsSet() {
		cfg.PaddingTrail = Some(s.PaddingTrail)
	}
	if !cfg.PaddingCrumb.IsSet() {
		cfg.PaddingCrumb = Some(s.PaddingCrumb)
	}
	if !cfg.PaddingContent.IsSet() {
		cfg.PaddingContent = Some(s.PaddingContent)
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
	if cfg.TextStyleSeparator == (TextStyle{}) {
		cfg.TextStyleSeparator = s.TextStyleSeparator
	}
}

// Breadcrumb creates a breadcrumb navigation control.
func Breadcrumb(cfg BreadcrumbCfg) View {
	applyBreadcrumbDefaults(&cfg)

	s := &DefaultBreadcrumbStyle
	radius := cfg.Radius.Get(s.Radius)
	radiusCrumb := cfg.RadiusCrumb.Get(s.RadiusCrumb)
	radiusContent := cfg.RadiusContent.Get(s.RadiusContent)
	spacing := cfg.Spacing.Get(s.Spacing)
	spacingTrail := cfg.SpacingTrail.Get(s.SpacingTrail)
	sizeBorder := cfg.SizeBorder.Get(s.SizeBorder)
	sizeContentBorder := cfg.SizeContentBorder.Get(s.SizeContentBorder)

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

		var onClick func(*Layout, *Event, *Window)
		var onHover func(*Layout, *Event, *Window)
		if !isDisabled {
			onClick = makeBcOnClick(cfg.OnSelect, item.ID, cfg.IDFocus)
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
		ID:              cfg.ID,
		IDFocus:         cfg.IDFocus,
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
		OnKeyDown: func(_ *Layout, e *Event, w *Window) {
			bcOnKeydown(cfg.Disabled, cfg.Items, cfg.Selected,
				cfg.OnSelect, cfg.IDFocus, e, w)
		},
		Content: outerContent,
	})
}

func makeBcOnClick(
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

func makeBcOnHover(
	hoverColor, clickColor Color,
) func(*Layout, *Event, *Window) {
	return func(layout *Layout, e *Event, w *Window) {
		if layout.Shape.Disabled || !layout.Shape.HasEvents() ||
			layout.Shape.Events.OnClick == nil {
			return
		}
		w.SetMouseCursorPointingHand()
		layout.Shape.Color = hoverColor
		if e.MouseButton == MouseLeft {
			layout.Shape.Color = clickColor
		}
	}
}

func bcOnKeydown(
	disabled bool,
	items []BreadcrumbItemCfg,
	selected string,
	onSelect func(string, *Event, *Window),
	idFocus uint32,
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
		if e.CharCode == CharSpace {
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
	return controlID + ":crumb:" + itemID
}
