package gui

type cmdPaletteItemsCache struct {
	viewKey    cmdPaletteViewKey
	items      []listCoreItem
	filtered   []listCoreItem
	ids        []string
	scored     []listCoreScored
	views      []View
	sourceHash uint64
}

type cmdPaletteViewKey struct {
	query      string
	theme      string
	sourceHash uint64
	first      int
	last       int
	hl         int
	filteredN  int
	rowH       float32
}

// CommandPaletteItem represents one action in the palette.
type CommandPaletteItem struct {
	ID       string
	Label    string
	Detail   string
	Icon     string
	Group    string
	Disabled bool
}

// CommandPaletteCfg configures a command palette view.
type CommandPaletteCfg struct {
	TextStyle   TextStyle
	DetailStyle TextStyle
	OnAction    func(string, *Event, *Window)
	OnDismiss   func(*Window)
	ID          string `gui:"required"`
	Placeholder string
	Items       []CommandPaletteItem
	FloatZIndex int
	SizeBorder  Opt[float32]
	Radius      Opt[float32]
	Width       float32
	MaxHeight   float32

	// Scrollable opts the results list into the scroll system. Scroll
	// state is keyed by Cfg.ID + ":scroll".
	Scrollable     bool
	Color          Color
	ColorBorder    Color
	ColorHighlight Color
	BackdropColor  Color
}

// commandPaletteView implements View for command palette.
type commandPaletteView struct {
	cfg CommandPaletteCfg
}

// CommandPalette creates the palette view. Include in view tree;
// hidden unless CommandPaletteShow was called.
func CommandPalette(cfg CommandPaletteCfg) View {
	RequireID("CommandPalette", cfg.ID)
	applyCommandPaletteDefaults(&cfg)
	return &commandPaletteView{cfg: cfg}
}

func (cp *commandPaletteView) Content() []View { return nil }

func (cp *commandPaletteView) GenerateLayout(w *Window) Layout {
	cfg := &cp.cfg
	dn := &DefaultCommandPaletteStyle
	sizeBorder := cfg.SizeBorder.Get(dn.SizeBorder)
	radius := cfg.Radius.Get(dn.Radius)
	visible := StateReadOr(w, nsCmdPalette, cfg.ID, false)
	if !visible {
		return generateViewLayout(Row(ContainerCfg{Padding: NoPadding}), w)
	}

	query := StateReadOr(w, nsCmdPaletteQuery, cfg.ID, "")
	highlighted := StateReadOr(w, nsCmdPaletteHighlight, cfg.ID, 0)

	cacheMap := StateMap[string, *cmdPaletteItemsCache](
		w, nsCmdPaletteItems, capModerate)
	cache, ok := cacheMap.Get(cfg.ID)
	if !ok || cache == nil {
		cache = &cmdPaletteItemsCache{}
		cacheMap.Set(cfg.ID, cache)
	}

	// Convert to core items only when source items changed.
	itemsHash := commandPaletteItemsHash(cfg.Items)
	if cache.sourceHash != itemsHash || len(cache.items) != len(cfg.Items) {
		if cap(cache.items) < len(cfg.Items) {
			cache.items = make([]listCoreItem, len(cfg.Items))
		} else {
			cache.items = cache.items[:len(cfg.Items)]
		}
		for i := range cfg.Items {
			cache.items[i] = cmdPaletteItemToCore(cfg.Items[i])
		}
		cache.sourceHash = itemsHash
	}

	// Filter + rank.
	prepared, scored := listCorePrepareInto(
		cache.items, query, highlighted,
		cache.filtered, cache.ids, cache.scored,
	)
	cache.filtered = prepared.Items
	cache.ids = prepared.IDs
	cache.scored = scored
	filtered := prepared.Items
	filteredIDs := prepared.IDs
	hl := prepared.HL

	// Virtualization.
	rowH := listCoreRowHeightEstimate(cfg.TextStyle, PaddingTwoFive)
	scrollID := cfg.ID + ":scroll"
	var scrollY float32
	if cfg.Scrollable {
		scrollY, _ = w.scrollY().Get(scrollID)
	}
	first, last := listCoreVisibleRange(len(filtered), rowH, cfg.MaxHeight, scrollY)

	onAction := cfg.OnAction
	paletteID := cfg.ID
	onDismiss := cfg.OnDismiss

	coreCfg := listCoreCfg{
		TextStyle:      cfg.TextStyle,
		DetailStyle:    cfg.DetailStyle,
		ColorHighlight: cfg.ColorHighlight,
		ColorHover:     cfg.ColorHighlight,
		ColorSelected:  cfg.ColorHighlight,
		PaddingItem:    PaddingTwoFive,
		ShowDetails:    true,
		ShowIcons:      true,
		OnItemClick: func(itemID string, _ int, e *Event, w *Window) {
			if onAction != nil {
				onAction(itemID, e, w)
			}
			CommandPaletteDismiss(paletteID, w)
			if onDismiss != nil {
				onDismiss(w)
			}
		},
	}

	viewKey := cmdPaletteViewKey{
		sourceHash: cache.sourceHash,
		query:      query,
		first:      first,
		last:       last,
		hl:         hl,
		filteredN:  len(filtered),
		rowH:       rowH,
		theme:      guiTheme.Name,
	}
	resultViews := cache.views
	if cache.viewKey != viewKey || resultViews == nil {
		resultViews = listCoreViews(filtered, coreCfg, first, last, hl, nil, rowH)
		cache.views = resultViews
		cache.viewKey = viewKey
	}

	// Build layout: backdrop column with centered card.
	return generateViewLayout(Column(ContainerCfg{
		Color:       cfg.BackdropColor,
		Sizing:      FillFill,
		Float:       true,
		FloatZIndex: cfg.FloatZIndex,
		VAlign:      VAlignTop,
		HAlign:      HAlignCenter,
		Padding:     NoPadding,
		OnClick: func(_ *Layout, e *Event, w *Window) {
			CommandPaletteDismiss(paletteID, w)
			if onDismiss != nil {
				onDismiss(w)
			}
			e.IsHandled = true
		},
		Content: []View{
			Column(ContainerCfg{
				ID:          cfg.ID,
				A11YRole:    AccessRoleDialog,
				Color:       cfg.Color,
				ColorBorder: cfg.ColorBorder,
				SizeBorder:  Some(sizeBorder),
				Radius:      Some(radius),
				Width:       cfg.Width,
				Padding:     NoPadding,
				Spacing:     SomeF(0),
				Sizing:      FixedFit,
				OnClick: func(_ *Layout, e *Event, _ *Window) {
					// Prevent backdrop click when clicking card.
					e.IsHandled = true
				},
				Content: []View{
					Row(ContainerCfg{
						Padding:    Some(PaddingSmall),
						Sizing:     FillFit,
						SizeBorder: NoBorder,
						Content: []View{
							Input(InputCfg{
								ID:            cfg.ID + ".input",
								Text:          query,
								Placeholder:   cfg.Placeholder,
								TextStyle:     cfg.TextStyle,
								Focusable:     true,
								Sizing:        FillFit,
								OnTextChanged: makePaletteOnTextChanged(cfg.ID),
								OnKeyDown:     makePaletteOnKeyDown(paletteID, onAction, onDismiss, filtered, filteredIDs),
								OnEnter:       makePaletteOnEnter(paletteID, onAction, onDismiss, filtered, filteredIDs),
							}),
						},
					}),
					Column(ContainerCfg{
						ID:         scrollID,
						Scrollable: true,
						MaxHeight:  cfg.MaxHeight,
						Sizing:     FillFit,
						Padding:    NoPadding,
						SizeBorder: NoBorder,
						Spacing:    SomeF(0),
						Clip:       true,
						Content:    resultViews,
					}),
				},
			}),
		},
	}), w)
}

// CommandPaletteShow makes the palette visible and focuses input.
// It always resets the results scroll (keyed id+":scroll") to the top.
func CommandPaletteShow(id string, w *Window) {
	ss := StateMap[string, bool](w, nsCmdPalette, capModerate)
	ss.Set(id, true)
	sq := StateMap[string, string](w, nsCmdPaletteQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	sh.Set(id, 0)
	w.scrollY().Set(id+":scroll", 0)
	w.SetFocus(id + ".input")
	w.UpdateWindow()
}

// CommandPaletteDismiss hides the palette.
func CommandPaletteDismiss(id string, w *Window) {
	ss := StateMap[string, bool](w, nsCmdPalette, capModerate)
	ss.Set(id, false)
	sq := StateMap[string, string](w, nsCmdPaletteQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	sh.Set(id, 0)
	w.UpdateWindow()
}

// CommandPaletteToggle toggles palette visibility.
func CommandPaletteToggle(id string, w *Window) {
	visible := StateReadOr(w, nsCmdPalette, id, false)
	if visible {
		CommandPaletteDismiss(id, w)
	} else {
		CommandPaletteShow(id, w)
	}
}

// CommandPaletteIsVisible returns whether the palette is showing.
func CommandPaletteIsVisible(id string, w *Window) bool {
	return StateReadOr(w, nsCmdPalette, id, false)
}

func cmdPaletteItemToCore(item CommandPaletteItem) listCoreItem {
	return listCoreItem{
		ID:       item.ID,
		Label:    item.Label,
		Detail:   item.Detail,
		Icon:     item.Icon,
		Group:    item.Group,
		Disabled: item.Disabled,
	}
}

func makePaletteOnEnter(paletteID string, onAction func(string, *Event, *Window), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
		cur, _ := sh.Get(paletteID) // ok ignored: zero index is valid, bounds-checked below
		itemCount := len(filteredIDs)
		if cur >= 0 && cur < itemCount && onAction != nil &&
			!filtered[cur].Disabled {
			onAction(filteredIDs[cur], e, w)
			CommandPaletteDismiss(paletteID, w)
			if onDismiss != nil {
				onDismiss(w)
			}
		}
	}
}

func makePaletteOnTextChanged(paletteID string) func(*Layout, string, *Window) {
	return func(_ *Layout, newText string, w *Window) {
		sq := StateMap[string, string](w, nsCmdPaletteQuery, capModerate)
		sq.Set(paletteID, newText)
		sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
		sh.Set(paletteID, 0)
		w.UpdateWindow()
	}
}

func commandPaletteItemsHash(items []CommandPaletteItem) uint64 {
	const offset uint64 = 14695981039346656037
	const prime uint64 = 1099511628211
	h := offset
	for i := range items {
		it := &items[i]
		h = hashString64(h, it.ID)
		h = hashString64(h, it.Label)
		h = hashString64(h, it.Detail)
		h = hashString64(h, it.Icon)
		h = hashString64(h, it.Group)
		if it.Disabled {
			h ^= 1
		}
		h *= prime
	}
	return h
}

func hashString64(h uint64, s string) uint64 {
	const prime uint64 = 1099511628211
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= prime
	}
	h ^= 0xff
	h *= prime
	return h
}

func makePaletteOnKeyDown(paletteID string, onAction func(string, *Event, *Window), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		paletteOnKeyDown(paletteID, onAction, onDismiss, filtered, filteredIDs, e, w)
	}
}

func paletteOnKeyDown(paletteID string, onAction func(string, *Event, *Window), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string, e *Event, w *Window) {
	if e.KeyCode == KeyEscape {
		CommandPaletteDismiss(paletteID, w)
		if onDismiss != nil {
			onDismiss(w)
		}
		e.IsHandled = true
		return
	}

	itemCount := len(filteredIDs)
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	cur, _ := sh.Get(paletteID) // ok ignored: zero index is valid, bounds-checked below
	action := listCoreNavigate(e.KeyCode, itemCount)

	if action == listCoreSelectItem {
		if cur >= 0 && cur < itemCount && onAction != nil &&
			!filtered[cur].Disabled {
			onAction(filteredIDs[cur], e, w)
			CommandPaletteDismiss(paletteID, w)
			if onDismiss != nil {
				onDismiss(w)
			}
		}
		e.IsHandled = true
		return
	}
	next, changed := listCoreApplyNav(action, cur, itemCount)
	if changed {
		sh.Set(paletteID, next)
		w.UpdateWindow()
		e.IsHandled = true
	}
}

func applyCommandPaletteDefaults(cfg *CommandPaletteCfg) {
	d := &DefaultCommandPaletteStyle
	if cfg.ID == "" {
		cfg.ID = "__cmd_palette__"
	}
	if cfg.Placeholder == "" {
		cfg.Placeholder = "Type a command..."
	}
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorHighlight.IsSet() {
		cfg.ColorHighlight = d.ColorHighlight
	}
	if cfg.Width == 0 {
		cfg.Width = d.Width
	}
	if cfg.MaxHeight == 0 {
		cfg.MaxHeight = d.MaxHeight
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.DetailStyle == (TextStyle{}) {
		cfg.DetailStyle = d.DetailStyle
	}
	if !cfg.BackdropColor.IsSet() {
		cfg.BackdropColor = d.BackdropColor
	}
	if cfg.FloatZIndex == 0 {
		cfg.FloatZIndex = 1000
	}
}
