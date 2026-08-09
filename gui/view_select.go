package gui

import (
	"slices"
	"strings"
)

const selectDropdownMaxH float32 = 200

// selectSubheaderPrefix marks options as section subheaders.
const selectSubheaderPrefix = "---"

func isSelectSubheader(option string) bool {
	return strings.HasPrefix(option, selectSubheaderPrefix)
}

// SelectCfg configures a select (dropdown) view.
type SelectCfg struct {
	TextStyle        TextStyle
	subheadingStyle  TextStyle
	PlaceholderStyle TextStyle
	OnSelect         func([]string, EventCtx)
	ID               string `gui:"required,focus"`
	Placeholder      string

	A11YLabel       string
	A11YDescription string
	Selected        []string // currently selected option text(s)
	Options         []string
	FloatZIndex     int
	Padding         Opt[Padding]
	SizeBorder      Opt[float32]
	Radius          Opt[float32]
	MinWidth        float32
	MaxWidth        float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled    bool
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorFocus       Color
	ColorSelect      Color
	Sizing           Sizing
	SelectMultiple   bool
	noWrap           bool
	Disabled         bool
	Invisible        bool
}

// selectView implements View for select (dropdown).
type selectView struct {
	cfg SelectCfg
}

// Select creates a select (dropdown) view.
func Select(cfg SelectCfg) View {
	applySelectDefaults(&cfg)
	requireFocusID("Select", cfg.FocusDisabled, cfg.ID)
	return &selectView{cfg: cfg}
}

func (sv *selectView) Content() []View { return nil }

func (sv *selectView) GenerateLayout(w *Window) Layout {
	cfg := &sv.cfg
	dn := &defaultSelectStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	// Open/highlight state is keyed by the widget's effective ID, so two
	// selects written with the same leaf under different ID-bearing
	// panels do not share one dropdown. The dropdown itself is a child
	// of this widget's container, so its shape carries the plain
	// "dropdown" leaf and the framework joins it to the same string.
	id := w.EffID(cfg.ID)
	isOpen := StateReadOr(w, nsSelect, id, false)
	dropdownScrollID := ScopeID(id, "dropdown")

	empty := len(cfg.Selected) == 0 || len(cfg.Selected[0]) == 0
	clip := cfg.SelectMultiple && cfg.noWrap

	txt := cfg.Placeholder
	if !empty {
		txt = strings.Join(cfg.Selected, ", ")
	}
	txtStyle := cfg.PlaceholderStyle
	if !empty {
		txtStyle = cfg.TextStyle
	}
	wrapMode := TextModeSingleLine
	if cfg.SelectMultiple && !cfg.noWrap {
		wrapMode = TextModeWrap
	}

	arrowText := "▼"
	if isOpen {
		arrowText = "▲"
	}

	spacerSizing := FillFill
	if wrapMode != TextModeSingleLine {
		spacerSizing = FitFill
	}

	content := make([]View, 0, 4)
	content = append(content,
		Text(TextCfg{
			Text:      txt,
			TextStyle: txtStyle,
			Mode:      wrapMode,
		}),
		Row(ContainerCfg{
			Sizing:  spacerSizing,
			Padding: NoPadding,
		}),
		Text(TextCfg{
			Text:      arrowText,
			TextStyle: cfg.TextStyle,
		}),
	)

	if isOpen {
		highlightedIdx := StateReadOr(
			w, nsSelectHL, id, 0)
		options := make([]View, 0, len(cfg.Options))
		for i, option := range cfg.Options {
			if isSelectSubheader(option) {
				options = append(options,
					selectSubHeaderView(cfg, option))
			} else {
				options = append(options,
					selectOptionView(cfg, id, option, i,
						i == highlightedIdx))
			}
		}
		content = append(content, Column(ContainerCfg{
			ID:            "dropdown",
			SizeBorder:    Some(sizeBorder),
			Radius:        Some(radius),
			ColorBorder:   cfg.ColorBorder,
			Color:         cfg.Color,
			MinHeight:     50,
			MaxHeight:     selectDropdownMaxH,
			MinWidth:      cfg.MinWidth,
			MaxWidth:      cfg.MaxWidth,
			Float:         true,
			floatAutoFlip: true,
			FloatAnchor:   FloatBottomLeft,
			FloatTieOff:   FloatTopLeft,
			FloatOffsetY:  -sizeBorder,
			FloatZIndex:   cfg.FloatZIndex,
			Scrollable:    true,
			Padding: SomeP(
				PadSmall, PadMedium, PadSmall, PadSmall),
			Spacing: NoSpacing,
			Content: options,
		}))
	}

	colorFocus := cfg.ColorFocus
	colorBorderFocus := cfg.ColorBorderFocus

	// Build the outer row layout directly.
	ccfg := ContainerCfg{
		ID:          cfg.ID,
		Focusable:   !cfg.FocusDisabled,
		Clip:        clip,
		A11YRole:    AccessRoleComboBox,
		A11YLabel:   a11yLabel(cfg.A11YLabel, cfg.Placeholder),
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Sizing:      cfg.Sizing,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		Disabled:    cfg.Disabled,
		Invisible:   cfg.Invisible,
		axis:        axisLeftToRight,
		AmendLayout: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled {
				return
			}
			if ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
				ctx.Layout.Shape.Color = colorFocus
				ctx.Layout.Shape.ColorBorder = colorBorderFocus
			}
		},
		OnKeyDown: makeSelectOnKeyDown(&sv.cfg, id, dropdownScrollID),
		OnClick: func(ctx EventCtx) {
			ss := StateMap[string, bool](
				ctx.Window, nsSelect, capModerate)
			// Default false: absent entry means "not selected", toggle correct.
			cur := ss.GetOr(id, false)
			ss.Clear()
			if !cur {
				ss.Set(id, true)
				sh := StateMap[string, int](
					ctx.Window, nsSelectHL, capModerate)
				sh.Set(id, selectInitialHighlight(
					cfg.Selected, cfg.Options))
			}
		},
	}
	ccfg.clickButton = MouseLeft
	return generateViewLayout(&containerView{
		cfg:     ccfg,
		content: content,
	}, w)
}

// selectOptionView builds a single option row.
func selectOptionView(
	cfg *SelectCfg, id, option string, index int, highlighted bool,
) View {
	selectMultiple := cfg.SelectMultiple
	onSelect := cfg.OnSelect
	selectArray := cfg.Selected
	colorSelect := cfg.ColorSelect
	optColor := ColorTransparent
	if highlighted {
		optColor = cfg.ColorSelect
	}

	checkColor := ColorTransparent
	if slices.Contains(cfg.Selected, option) {
		checkColor = cfg.TextStyle.Color
	}

	return Row(ContainerCfg{
		Color:   optColor,
		Padding: SomeP(0, PadSmall, 0, 1),
		Sizing:  FillFit,
		Content: []View{
			Row(ContainerCfg{
				Padding: Some(padTBLR(2, 0)),
				Content: []View{
					Text(TextCfg{
						Text: "✓",
						TextStyle: TextStyle{
							Color: checkColor,
							Size:  cfg.TextStyle.Size,
						},
					}),
					Text(TextCfg{
						Text:      option,
						TextStyle: cfg.TextStyle,
					}),
				},
			}),
		},
		OnClick: func(ctx EventCtx) {
			ctx.Consume()
			if onSelect == nil {
				return
			}
			ss := StateMap[string, bool](
				ctx.Window, nsSelect, capModerate)
			var s []string
			if selectMultiple {
				s = listBoxNextSelectedIDs(
					selectArray, option, true)
			} else {
				ss.Clear()
				s = []string{option}
			}
			onSelect(s, EventCtx{nil, ctx.Event, ctx.Window})
		},
		OnHover: func(ctx EventCtx) {
			ctx.Window.setMouseCursor(CursorPointingHand)
			ctx.Layout.Shape.Color = colorSelect
			sh := StateMap[string, int](
				ctx.Window, nsSelectHL, capModerate)
			// Default 0: absent entry gets zero index, checked immediately.
			cur := sh.GetOr(id, 0)
			if cur != index {
				sh.Set(id, index)
			}
		},
	})
}

// selectSubHeaderView builds a section header row.
func selectSubHeaderView(cfg *SelectCfg, option string) View {
	label := option
	if len(option) > len(selectSubheaderPrefix) {
		label = option[len(selectSubheaderPrefix):]
	}
	return Column(ContainerCfg{
		Padding: SomeP(guiTheme.PaddingMedium.Top, 0, 0, 0),
		Sizing:  FillFit,
		Content: []View{
			Row(ContainerCfg{
				Padding: NoPadding,
				Sizing:  FillFit,
				Spacing: SomeF(PadXSmall),
				Content: []View{
					Text(TextCfg{
						Text: "✓",
						TextStyle: TextStyle{
							Color: ColorTransparent,
							Size:  cfg.subheadingStyle.Size,
						},
					}),
					Text(TextCfg{
						Text:      label,
						TextStyle: cfg.subheadingStyle,
					}),
				},
			}),
			Row(ContainerCfg{
				Padding: Some(padTBLR(0, PadMedium)),
				Sizing:  FillFit,
				Content: []View{
					Rectangle(RectangleCfg{
						Width:  1,
						Height: 1,
						Sizing: FillFit,
						Color:  cfg.subheadingStyle.Color,
					}),
				},
			}),
		},
	})
}

func makeSelectOnKeyDown(
	cfg *SelectCfg, id, scrollID string,
) func(EventCtx) {
	return func(ctx EventCtx) {
		selectOnKeyDown(cfg, id, scrollID, ctx.Event, ctx.Window)
	}
}

// selectInitialHighlight returns the index of the first selected
// option, or 0 if none match.
func selectInitialHighlight(selected, options []string) int {
	if len(selected) > 0 {
		for i, opt := range options {
			if opt == selected[0] {
				return i
			}
		}
	}
	return 0
}

func selectOnKeyDown(
	cfg *SelectCfg, id, scrollID string, e *Event, w *Window,
) {
	if len(cfg.Options) == 0 {
		return
	}

	ss := StateMap[string, bool](w, nsSelect, capModerate)
	sh := StateMap[string, int](w, nsSelectHL, capModerate)
	// Default false: absent entry means dropdown not open.
	isOpen := ss.GetOr(id, false)

	// Open on space/enter.
	if (e.KeyCode == KeySpace || e.KeyCode == KeyEnter) && !isOpen {
		ss.Set(id, true)
		sh.Set(id, selectInitialHighlight(
			cfg.Selected, cfg.Options))
		e.IsHandled = true
		return
	}

	// Close on escape or tab.
	if (e.KeyCode == KeyEscape || e.KeyCode == KeyTab) && isOpen {
		ss.Clear()
		e.IsHandled = true
		return
	}

	if !isOpen {
		return
	}

	// Default 0: first item highlighted; bounds-checked before use.
	currentIdx := sh.GetOr(id, 0)
	action := listCoreNavigate(e.KeyCode, len(cfg.Options))

	if action == listCoreSelectItem {
		if currentIdx >= 0 && currentIdx < len(cfg.Options) {
			option := cfg.Options[currentIdx]
			if !isSelectSubheader(option) {
				var s []string
				if cfg.SelectMultiple {
					s = listBoxNextSelectedIDs(
						cfg.Selected, option, true)
				} else {
					ss.Clear()
					s = []string{option}
				}
				if cfg.OnSelect != nil {
					cfg.OnSelect(s, EventCtx{nil, e, w})
				}
			}
		}
		e.IsHandled = true
		return
	}

	if action == listCoreFirst || action == listCoreLast {
		var nextIdx int
		if action == listCoreFirst {
			nextIdx = selectNextSelectable(cfg.Options, 0, 1)
		} else {
			nextIdx = selectNextSelectable(
				cfg.Options, len(cfg.Options)-1, -1)
		}
		if nextIdx >= 0 {
			sh.Set(id, nextIdx)
			selectScrollTo(cfg, scrollID, nextIdx, w)
			e.IsHandled = true
		}
		return
	}

	if action == listCoreMoveUp || action == listCoreMoveDown {
		dir := 1
		if action == listCoreMoveUp {
			dir = -1
		}
		nextIdx := selectNextSelectable(
			cfg.Options, currentIdx+dir, dir)
		if nextIdx >= 0 {
			sh.Set(id, nextIdx)
			selectScrollTo(cfg, scrollID, nextIdx, w)
			e.IsHandled = true
		}
	}
}

// selectNextSelectable finds the next non-subheader option starting
// at start, stepping by dir (+1 or -1). Returns -1 if none found.
func selectNextSelectable(options []string, start, dir int) int {
	for i := start; i >= 0 && i < len(options); i += dir {
		if !isSelectSubheader(options[i]) {
			return i
		}
	}
	return -1
}

func selectScrollTo(cfg *SelectCfg, scrollID string, idx int, w *Window) {
	rowH := cfg.TextStyle.Size + 4
	listH := selectDropdownMaxH - 2*cfg.SizeBorder.Get(
		defaultSelectStyle.SizeBorder)
	scrollEnsureVisible(scrollID, idx, rowH, listH, w)
}

func applySelectDefaults(cfg *SelectCfg) {
	d := &defaultSelectStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if cfg.MinWidth == 0 {
		cfg.MinWidth = d.MinWidth
	}
	if cfg.MaxWidth == 0 {
		cfg.MaxWidth = d.MaxWidth
	}

	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.textStyleNormal
	}
	if cfg.subheadingStyle == (TextStyle{}) {
		cfg.subheadingStyle = d.subheadingStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = d.PlaceholderStyle
	}
}
