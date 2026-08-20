package gui

import "unicode/utf8"

type comboboxItemsCache struct {
	viewKey     comboboxViewKey
	items       []listCoreItem
	filtered    []listCoreItem
	ids         []string
	scored      []listCoreScored
	views       []View
	optionsHash uint64
}

type comboboxViewKey struct {
	query       string
	theme       string
	optionsHash uint64
	first       int
	last        int
	hl          int
	filteredN   int
	rowH        float32
}

// ComboboxCfg configures a combobox view with typeahead filtering.
type ComboboxCfg struct {
	// Label names this field. Empty renders exactly as before: no
	// wrapper and no extra shape. Set, it stacks above the field in
	// the theme's label role, and fills A11YLabel when that is unset.
	// See gui/field_label.go for the convention and why it is one.
	Label            string
	TextStyle        TextStyle
	PlaceholderStyle TextStyle
	OnSelect         func(string, EventCtx)
	ID               string `gui:"required"`
	Value            string
	Placeholder      string

	A11YCfg
	Options     []string
	FloatZIndex int
	Padding     Padding
	SizeBorder  Opt[float32]
	Radius      Opt[float32]
	MinWidth    float32
	MaxWidth    float32
	// MaxDropdownHeight caps the dropdown list's height. Zero takes
	// the theme default.
	// exportaudit:keep — caller-facing config (issue #372)
	MaxDropdownHeight float32
	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool

	// Scrollable opts the dropdown into the scroll system. Scroll
	// state is keyed by ScopeID(Cfg.ID, "dropdown").
	Scrollable       bool
	Color            Color
	ColorBorder      Color
	ColorBorderFocus Color
	ColorFocus       Color
	ColorHighlight   Color
	// ColorTextOnSelect is the text color drawn over the
	// highlighted row's fill. Unset takes the theme's.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTextOnSelect Color
	ColorHover        Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors   ColorSet
	Sizing   Sizing
	Disabled bool
}

// comboboxView implements View for combobox.
type comboboxView struct {
	cfg ComboboxCfg
}

// Combobox creates a combobox view.
func Combobox(cfg ComboboxCfg) View {
	RequireID("Combobox", cfg.ID)
	applyComboboxDefaults(&cfg)
	cfg.A11YLabel = a11yLabel(cfg.A11YLabel, cfg.Label)
	return labelledField(
		cfg.Label, cfg.TextStyle, HAlignLeft, cfg.Sizing,
		&comboboxView{cfg: cfg})
}

func (cv *comboboxView) GenerateLayout(w *Window) Layout {
	cfg := &cv.cfg
	dn := &defaultComboboxStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	// Per-widget state is keyed by the widget's *effective* ID, so two
	// comboboxes written with the same leaf under different ID-bearing
	// panels keep separate open/query/highlight state. Handlers close
	// over the resolved key; it stays valid while those ancestors do.
	id := w.EffID(cfg.ID)
	isOpen := StateReadOr(w, nsCombobox, id, false)
	query := StateReadOr(w, nsComboboxQuery, id, "")
	highlighted := StateReadOr(w, nsComboboxHighlight, id, 0)

	cacheMap := StateMap[string, *comboboxItemsCache](
		w, nsComboboxItems, capModerate)
	cache, ok := cacheMap.Get(id)
	if !ok || cache == nil {
		cache = &comboboxItemsCache{}
		cacheMap.Set(id, cache)
	}

	// Convert options to core items only when options changed.
	optionsHash := comboboxOptionsHash(cfg.Options)
	if cache.optionsHash != optionsHash || len(cache.items) != len(cfg.Options) {
		if cap(cache.items) < len(cfg.Options) {
			cache.items = make([]listCoreItem, len(cfg.Options))
		} else {
			cache.items = cache.items[:len(cfg.Options)]
		}
		for i := range cfg.Options {
			opt := cfg.Options[i]
			cache.items[i] = listCoreItem{ID: opt, Label: opt}
		}
		cache.optionsHash = optionsHash
	}

	// Filter when query is present.
	filterQuery := query
	prepared, scored := listCorePrepareInto(
		cache.items, filterQuery, highlighted,
		cache.filtered, cache.ids, cache.scored,
	)
	cache.filtered = prepared.Items
	cache.ids = prepared.IDs
	cache.scored = scored
	filtered := prepared.Items
	filteredIDs := prepared.IDs
	hl := prepared.HL

	// Virtualization.
	rowH := listCoreRowHeightEstimate(
		cfg.TextStyle, cfg.Padding.Or(PaddingNone), w)
	pad := cfg.Padding.Or(PaddingNone)
	listH := cfg.MaxDropdownHeight - 2*sizeBorder - pad.Top - pad.Bottom
	var scrollY float32
	// The dropdown is a child of the container that claims cfg.ID, so
	// its shape carries the plain leaf below and the framework joins it.
	// The key here is that same join, spelled out because this read
	// happens during generation.
	dropdownScrollID := ScopeID(id, "dropdown")
	if cfg.Scrollable {
		// Default 0: unscrolled dropdown before first scroll event.
		scrollY = w.scrollY().GetOr(dropdownScrollID, 0)
	}
	first, last := listCoreVisibleRange(len(filtered), rowH, listH, scrollY)
	// Index space: the *filtered* items, so it moves with the query.
	listHeightRegisterUniform(w, dropdownScrollID, len(filtered),
		rowH, 0, 0)

	// Build dropdown content.
	onSelect := cfg.OnSelect
	coreCfg := listCoreCfg{
		TextStyle:         cfg.TextStyle,
		ColorHighlight:    cfg.ColorHighlight,
		ColorHover:        cfg.ColorHover,
		ColorSelected:     cfg.ColorHighlight,
		colorTextOnSelect: cfg.ColorTextOnSelect,
		PaddingItem:       cfg.Padding.Or(PaddingNone),
		OnItemClick: func(itemID string, _ int, ctx EventCtx) {
			if onSelect != nil {
				onSelect(itemID, EventCtx{nil, ctx.Event, ctx.Window})
			}
			comboboxClose(id, ctx.Window)
		},
	}

	content := make([]View, 0, 4)

	if isOpen {
		txt := query
		ts := cfg.TextStyle
		if len(txt) == 0 {
			txt = cfg.Placeholder
			ts = cfg.PlaceholderStyle
		}
		content = append(content, Text(TextCfg{
			Text:      txt,
			TextStyle: ts,
			Mode:      TextModeSingleLine,
		}))
	} else {
		empty := len(cfg.Value) == 0
		txt := cfg.Value
		ts := cfg.TextStyle
		if empty {
			txt = cfg.Placeholder
			ts = cfg.PlaceholderStyle
		}
		content = append(content, Text(TextCfg{
			Text:      txt,
			TextStyle: ts,
			Mode:      TextModeSingleLine,
		}))
	}

	content = append(content,
		Row(ContainerCfg{
			Sizing:  FillFill,
			Padding: NoPadding,
		}),
	)

	content = append(content, disclosureArrow(isOpen, cfg.TextStyle))

	if isOpen {
		viewKey := comboboxViewKey{
			optionsHash: cache.optionsHash,
			query:       filterQuery,
			first:       first,
			last:        last,
			hl:          hl,
			filteredN:   len(filtered),
			rowH:        rowH,
			theme:       guiTheme.Name,
		}
		dropdownContent := cache.views
		if cache.viewKey != viewKey || dropdownContent == nil {
			dropdownContent = listCoreViews(filtered, coreCfg,
				first, last, hl, nil, rowH)
			cache.views = dropdownContent
			cache.viewKey = viewKey
		}
		content = append(content, Column(ContainerCfg{
			ID:           "dropdown",
			Shadow:       dn.Shadow,
			SizeBorder:   Some(sizeBorder),
			Radius:       Some(radius),
			ColorBorder:  cfg.ColorBorder,
			Color:        cfg.Color,
			MinHeight:    50,
			MaxHeight:    cfg.MaxDropdownHeight,
			Float:        true,
			FloatAnchor:  FloatBottomLeft,
			FloatTieOff:  FloatTopLeft,
			FloatOffsetY: -sizeBorder,
			FloatZIndex:  cfg.FloatZIndex,
			Scrollable:   cfg.Scrollable,
			Padding:      cfg.Padding,
			Spacing:      SomeF(0),
			Content:      dropdownContent,
			AmendLayout: func(ctx EventCtx) {
				if ctx.Layout.Parent == nil {
					return
				}
				ctx.Layout.Shape.Width = ctx.Layout.Parent.Shape.Width
				// Re-run OverDraw children's AmendLayout so scrollbars
				// reposition to the updated width.
				for i := range ctx.Layout.Children {
					c := &ctx.Layout.Children[i]
					if c.Shape.OverDraw &&
						c.Shape.hasEvents() &&
						c.Shape.events.AmendLayout != nil {
						c.Shape.events.AmendLayout(EventCtx{c, nil, ctx.Window})
					}
				}
			},
		}))
	}

	colorFocus := cfg.ColorFocus
	colorBorderFocus := cfg.ColorBorderFocus

	ccfg := ContainerCfg{
		ID:        cfg.ID,
		Focusable: !cfg.FocusDisabled,
		A11YRole:  AccessRoleComboBox,
		A11YCfg: A11YCfg{
			A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.Placeholder),
			A11YDescription: cfg.A11YDescription,
		},
		Color:       cfg.Color,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  Some(sizeBorder),
		Radius:      Some(radius),
		Padding:     cfg.Padding,
		Sizing:      cfg.Sizing,
		MinWidth:    cfg.MinWidth,
		MaxWidth:    cfg.MaxWidth,
		Disabled:    cfg.Disabled,
		axis:        axisLeftToRight,
		VAlign:      VAlignMiddle,
		AmendLayout: func(ctx EventCtx) {
			if ctx.Layout.Shape.Disabled {
				return
			}
			if ctx.Window.IsFocus(ctx.Layout.Shape.idKey()) {
				ctx.Layout.Shape.Color = colorFocus
				ctx.Layout.Shape.ColorBorder = colorBorderFocus
			}
		},
		OnKeyDown: makeComboboxOnKeyDown(id, onSelect, id, filteredIDs,
			dropdownScrollID, rowH, listH),
		OnChar: makeComboboxOnChar(id),
		OnClick: func(ctx EventCtx) {
			if isOpen {
				comboboxClose(id, ctx.Window)
			} else {
				comboboxOpen(id, id, ctx.Window)
			}
		},
	}
	ccfg.clickButton = MouseLeft
	return generateViewLayout(&containerView{
		cfg:     ccfg,
		content: content,
	}, w)
}

func comboboxOpen(id string, focusID string, w *Window) {
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	ss.Set(id, true)
	sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
	sh.Set(id, 0)
	if focusID != "" {
		w.SetFocus(focusID)
	}
	w.UpdateWindow()
}

func comboboxClose(id string, w *Window) {
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	ss.Set(id, false)
	sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
	sh.Set(id, 0)
	w.UpdateWindow()
}

func makeComboboxOnChar(cfgID string) func(EventCtx) {
	return func(ctx EventCtx) {
		ss := StateMap[string, bool](ctx.Window, nsCombobox, capModerate)
		// Default false: absent entry means "not open".
		isOpen := ss.GetOr(cfgID, false)
		if !isOpen {
			// Closed: typing is not ours to swallow
			return
		}
		ch := rune(ctx.Event.CharCode)
		if ch < charSpace {
			// Control characters belong to OnKeyDown
			return
		}
		sq := StateMap[string, string](ctx.Window, nsComboboxQuery, capModerate)
		// Default "": absent entry means empty initial query.
		query := sq.GetOr(cfgID, "")
		query += string(ch)
		sq.Set(cfgID, query)
		sh := StateMap[string, int](ctx.Window, nsComboboxHighlight, capModerate)
		sh.Set(cfgID, 0)
		ctx.Window.UpdateWindow()
		ctx.Consume()
	}
}

func makeComboboxOnKeyDown(cfgID string, onSelect func(string, EventCtx), focusID string, filteredIDs []string, scrollID string, rowH, listH float32) func(EventCtx) {
	return func(ctx EventCtx) {
		comboboxOnKeyDown(cfgID, onSelect, focusID, filteredIDs, scrollID, rowH, listH, ctx.Event, ctx.Window)
	}
}

func comboboxOnKeyDown(cfgID string, onSelect func(string, EventCtx), focusID string, filteredIDs []string, scrollID string, rowH, listH float32, e *Event, w *Window) {
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	// Default false: absent entry means "not open".
	isOpen := ss.GetOr(cfgID, false)

	if !isOpen {
		if e.KeyCode == KeySpace || e.KeyCode == KeyEnter ||
			e.KeyCode == KeyUp || e.KeyCode == KeyDown {
			comboboxOpen(cfgID, focusID, w)
			e.IsHandled = true
		}
		return
	}

	if e.KeyCode == KeyEscape || e.KeyCode == KeyTab {
		comboboxClose(cfgID, w)
		e.IsHandled = true
		return
	}

	if e.KeyCode == KeyBackspace {
		sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
		// Default "": absent entry means empty query, len check below.
		query := sq.GetOr(cfgID, "")
		if len(query) > 0 {
			_, sz := utf8.DecodeLastRuneInString(query)
			sq.Set(cfgID, query[:len(query)-sz])
			sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
			sh.Set(cfgID, 0)
			w.UpdateWindow()
		}
		e.IsHandled = true
		return
	}

	itemCount := len(filteredIDs)
	sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
	// Default 0: first item highlighted; bounds-checked before use.
	cur := sh.GetOr(cfgID, 0)
	action := listCoreNavigate(e.KeyCode, itemCount)

	if action == listCoreSelectItem {
		if cur >= 0 && cur < itemCount && onSelect != nil {
			onSelect(filteredIDs[cur], EventCtx{nil, e, w})
			comboboxClose(cfgID, w)
		}
		e.IsHandled = true
		return
	}
	next, changed := listCoreApplyNav(action, cur, itemCount)
	if changed {
		sh.Set(cfgID, next)
		if scrollID != "" && rowH > 0 {
			scrollEnsureVisible(scrollID, next, rowH, listH, w)
		}
		w.UpdateWindow()
		e.IsHandled = true
	}
}

// scrollEnsureVisible scrolls so the item at index idx is visible.
func scrollEnsureVisible(
	id string, idx int, rowH, listH float32, w *Window,
) {
	sm := w.scrollY()
	// Default 0: unscrolled list before first scroll event.
	scrollY := sm.GetOr(id, 0)
	top := float32(idx) * rowH
	bottom := top + rowH
	visible := -scrollY
	if top < visible {
		sm.Set(id, -top)
	} else if bottom > visible+listH {
		sm.Set(id, -(bottom - listH))
	}
}

func applyComboboxDefaults(cfg *ComboboxCfg) {
	d := &defaultComboboxStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, Color{},
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, &cfg.ColorHover, nil,
		&cfg.ColorFocus, &cfg.ColorBorder, &cfg.ColorBorderFocus)
	if !cfg.ColorHighlight.IsSet() {
		cfg.ColorHighlight = d.ColorHighlight
	}
	if !cfg.ColorTextOnSelect.IsSet() {
		cfg.ColorTextOnSelect = d.ColorTextOnSelect
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	if cfg.MinWidth == 0 {
		cfg.MinWidth = d.MinWidth
	}
	if cfg.MaxWidth == 0 {
		cfg.MaxWidth = d.MaxWidth
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = d.PlaceholderStyle
	}
	if cfg.MaxDropdownHeight == 0 {
		cfg.MaxDropdownHeight = d.maxDropdownHeight
	}
}

func comboboxOptionsHash(options []string) uint64 {
	const offset uint64 = 14695981039346656037
	const prime uint64 = 1099511628211
	h := offset
	for i := range options {
		s := options[i]
		for j := range len(s) {
			h ^= uint64(s[j])
			h *= prime
		}
		h ^= 0xff
		h *= prime
	}
	return h
}
