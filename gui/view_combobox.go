package gui

import "unicode/utf8"

type comboboxItemsCache struct {
	viewKey     comboboxViewKey
	items       []ListCoreItem
	filtered    []ListCoreItem
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
	TextStyle        TextStyle
	PlaceholderStyle TextStyle
	OnSelect         func(string, *Event, *Window)
	ID               string `gui:"required"`
	Value            string
	Placeholder      string

	A11YLabel         string
	A11YDescription   string
	Options           []string
	FloatZIndex       int
	Padding           Opt[Padding]
	SizeBorder        Opt[float32]
	Radius            Opt[float32]
	MinWidth          float32
	MaxWidth          float32
	MaxDropdownHeight float32
	IDFocus           uint32
	IDScroll          uint32
	Color             Color
	ColorBorder       Color
	ColorBorderFocus  Color
	ColorFocus        Color
	ColorHighlight    Color
	ColorHover        Color
	Sizing            Sizing
	Disabled          bool
}

// comboboxView implements View for combobox.
type comboboxView struct {
	cfg ComboboxCfg
}

// Combobox creates a combobox view.
func Combobox(cfg ComboboxCfg) View {
	requireID("Combobox", cfg.ID)
	applyComboboxDefaults(&cfg)
	return &comboboxView{cfg: cfg}
}

func (cv *comboboxView) Content() []View { return nil }

func (cv *comboboxView) GenerateLayout(w *Window) Layout {
	cfg := &cv.cfg
	dn := &DefaultComboboxStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	isOpen := StateReadOr(w, nsCombobox, cfg.ID, false)
	query := StateReadOr(w, nsComboboxQuery, cfg.ID, "")
	highlighted := StateReadOr(w, nsComboboxHighlight, cfg.ID, 0)

	cacheMap := StateMap[string, *comboboxItemsCache](
		w, nsComboboxItems, capModerate)
	cache, ok := cacheMap.Get(cfg.ID)
	if !ok || cache == nil {
		cache = &comboboxItemsCache{}
		cacheMap.Set(cfg.ID, cache)
	}

	// Convert options to core items only when options changed.
	optionsHash := comboboxOptionsHash(cfg.Options)
	if cache.optionsHash != optionsHash || len(cache.items) != len(cfg.Options) {
		if cap(cache.items) < len(cfg.Options) {
			cache.items = make([]ListCoreItem, len(cfg.Options))
		} else {
			cache.items = cache.items[:len(cfg.Options)]
		}
		for i := range cfg.Options {
			opt := cfg.Options[i]
			cache.items[i] = ListCoreItem{ID: opt, Label: opt}
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
	rowH := listCoreRowHeightEstimate(cfg.TextStyle, cfg.Padding.Get(Padding{}))
	pad := cfg.Padding.Get(Padding{})
	listH := cfg.MaxDropdownHeight - 2*sizeBorder - pad.Top - pad.Bottom
	var scrollY float32
	if cfg.IDScroll > 0 {
		scrollY = StateReadOr(w, nsScrollY, cfg.IDScroll, float32(0))
	}
	first, last := listCoreVisibleRange(len(filtered), rowH, listH, scrollY)

	// Build dropdown content.
	onSelect := cfg.OnSelect
	cfgID := cfg.ID
	coreCfg := ListCoreCfg{
		TextStyle:      cfg.TextStyle,
		ColorHighlight: cfg.ColorHighlight,
		ColorHover:     cfg.ColorHover,
		ColorSelected:  cfg.ColorHighlight,
		PaddingItem:    cfg.Padding.Get(Padding{}),
		OnItemClick: func(itemID string, _ int, e *Event, w *Window) {
			if onSelect != nil {
				onSelect(itemID, e, w)
			}
			comboboxClose(cfgID, w)
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

	arrowText := "▼"
	if isOpen {
		arrowText = "▲"
	}
	content = append(content, Text(TextCfg{
		Text:      arrowText,
		TextStyle: cfg.TextStyle,
	}))

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
			ID:           cfg.ID + ".dropdown",
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
			IDScroll:     cfg.IDScroll,
			Padding:      cfg.Padding,
			Spacing:      SomeF(0),
			Content:      dropdownContent,
			AmendLayout: func(layout *Layout, w *Window) {
				if layout.Parent == nil {
					return
				}
				layout.Shape.Width = layout.Parent.Shape.Width
				// Re-run OverDraw children's AmendLayout so scrollbars
				// reposition to the updated width.
				for i := range layout.Children {
					c := &layout.Children[i]
					if c.Shape.OverDraw &&
						c.Shape.hasEvents() &&
						c.Shape.events.AmendLayout != nil {
						c.Shape.events.AmendLayout(c, w)
					}
				}
			},
		}))
	}

	colorFocus := cfg.ColorFocus
	colorBorderFocus := cfg.ColorBorderFocus
	idFocus := cfg.IDFocus

	ccfg := ContainerCfg{
		ID:          cfg.ID,
		IDFocus:     idFocus,
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
		axis:        AxisLeftToRight,
		AmendLayout: func(layout *Layout, w *Window) {
			if layout.Shape.Disabled {
				return
			}
			if w.IsFocus(layout.Shape.IDFocus) {
				layout.Shape.Color = colorFocus
				layout.Shape.ColorBorder = colorBorderFocus
			}
		},
		OnKeyDown: makeComboboxOnKeyDown(cfg.ID, onSelect, idFocus, filteredIDs, cfg.IDScroll, rowH, listH),
		OnChar:    makeComboboxOnChar(cfg.ID),
		OnClick: func(_ *Layout, e *Event, w *Window) {
			if isOpen {
				comboboxClose(cfgID, w)
			} else {
				comboboxOpen(cfgID, idFocus, w)
			}
			e.IsHandled = true
		},
	}
	ccfg.OnClick = leftClickOnly(ccfg.OnClick)
	outerRow := &containerView{
		shape:   buildContainerShape(&ccfg),
		content: content,
	}
	return GenerateViewLayout(outerRow, w)
}

func comboboxOpen(id string, idFocus uint32, w *Window) {
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	ss.Set(id, true)
	sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
	sh.Set(id, 0)
	if idFocus > 0 {
		w.SetIDFocus(idFocus)
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

func makeComboboxOnChar(cfgID string) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		ss := StateMap[string, bool](w, nsCombobox, capModerate)
		isOpen, _ := ss.Get(cfgID) // ok ignored: false means "not open"
		if !isOpen {
			return
		}
		ch := rune(e.CharCode)
		if ch < CharSpace {
			return
		}
		sq := StateMap[string, string](w, nsComboboxQuery, capModerate)
		// ok ignored: empty string is correct initial query.
		query, _ := sq.Get(cfgID)
		query += string(ch)
		sq.Set(cfgID, query)
		sh := StateMap[string, int](w, nsComboboxHighlight, capModerate)
		sh.Set(cfgID, 0)
		w.UpdateWindow()
		e.IsHandled = true
	}
}

func makeComboboxOnKeyDown(cfgID string, onSelect func(string, *Event, *Window), idFocus uint32, filteredIDs []string, idScroll uint32, rowH, listH float32) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		comboboxOnKeyDown(cfgID, onSelect, idFocus, filteredIDs, idScroll, rowH, listH, e, w)
	}
}

func comboboxOnKeyDown(cfgID string, onSelect func(string, *Event, *Window), idFocus uint32, filteredIDs []string, idScroll uint32, rowH, listH float32, e *Event, w *Window) {
	ss := StateMap[string, bool](w, nsCombobox, capModerate)
	isOpen, _ := ss.Get(cfgID) // ok ignored: false means "not open"

	if !isOpen {
		if e.KeyCode == KeySpace || e.KeyCode == KeyEnter ||
			e.KeyCode == KeyUp || e.KeyCode == KeyDown {
			comboboxOpen(cfgID, idFocus, w)
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
		query, _ := sq.Get(cfgID) // ok ignored: empty string, len checked below
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
	cur, _ := sh.Get(cfgID)
	action := listCoreNavigate(e.KeyCode, itemCount)

	if action == ListCoreSelectItem {
		if cur >= 0 && cur < itemCount && onSelect != nil {
			onSelect(filteredIDs[cur], e, w)
			comboboxClose(cfgID, w)
		}
		e.IsHandled = true
		return
	}
	next, changed := listCoreApplyNav(action, cur, itemCount)
	if changed {
		sh.Set(cfgID, next)
		if idScroll > 0 && rowH > 0 {
			scrollEnsureVisible(idScroll, next, rowH, listH, w)
		}
		w.UpdateWindow()
		e.IsHandled = true
	}
}

// scrollEnsureVisible scrolls so the item at index idx is visible.
func scrollEnsureVisible(
	idScroll uint32, idx int, rowH, listH float32, w *Window,
) {
	sm := StateMap[uint32, float32](w, nsScrollY, capScroll)
	scrollY, _ := sm.Get(idScroll)
	top := float32(idx) * rowH
	bottom := top + rowH
	visible := -scrollY
	if top < visible {
		sm.Set(idScroll, -top)
	} else if bottom > visible+listH {
		sm.Set(idScroll, -(bottom - listH))
	}
}

func applyComboboxDefaults(cfg *ComboboxCfg) {
	d := &DefaultComboboxStyle
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorHover.IsSet() {
		cfg.ColorHover = d.ColorHover
	}
	if !cfg.ColorFocus.IsSet() {
		cfg.ColorFocus = d.ColorFocus
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorBorderFocus.IsSet() {
		cfg.ColorBorderFocus = d.ColorBorderFocus
	}
	if !cfg.ColorHighlight.IsSet() {
		cfg.ColorHighlight = d.ColorHighlight
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
		cfg.TextStyle = d.TextStyle
	}
	if cfg.PlaceholderStyle == (TextStyle{}) {
		cfg.PlaceholderStyle = d.PlaceholderStyle
	}
	if cfg.MaxDropdownHeight == 0 {
		cfg.MaxDropdownHeight = d.MaxDropdownHeight
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
