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
	OnAction    func(string, EventCtx)
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
	// state is keyed by ScopeID(Cfg.ID, "scroll").
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
	// Palette state is keyed by the effective ID. The public helpers
	// (CommandPaletteShow and friends) take effective IDs too, so an app
	// that nests a palette under an ID-bearing panel passes the full
	// path — the same string this resolves to.
	id := w.EffID(cfg.ID)
	visible := StateReadOr(w, nsCmdPalette, id, false)
	if !visible {
		return generateViewLayout(Row(ContainerCfg{Padding: NoPadding}), w)
	}

	query := StateReadOr(w, nsCmdPaletteQuery, id, "")
	highlighted := StateReadOr(w, nsCmdPaletteHighlight, id, 0)

	cacheMap := StateMap[string, *cmdPaletteItemsCache](
		w, nsCmdPaletteItems, capModerate)
	cache, ok := cacheMap.Get(id)
	if !ok || cache == nil {
		cache = &cmdPaletteItemsCache{}
		cacheMap.Set(id, cache)
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
	scrollID := ScopeID(id, "scroll")
	var scrollY float32
	if cfg.Scrollable {
		// Default 0: unscrolled list before first scroll event.
		scrollY = w.scrollY().GetOr(scrollID, 0)
	}
	first, last := listCoreVisibleRange(len(filtered), rowH, cfg.MaxHeight, scrollY)

	onAction := cfg.OnAction
	paletteID := id
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
		OnItemClick: func(itemID string, _ int, ctx EventCtx) {
			if onAction != nil {
				onAction(itemID, EventCtx{nil, ctx.Event, ctx.Window})
			}
			CommandPaletteDismiss(paletteID, ctx.Window)
			if onDismiss != nil {
				onDismiss(ctx.Window)
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
		OnClick: func(ctx EventCtx) {
			CommandPaletteDismiss(paletteID, ctx.Window)
			if onDismiss != nil {
				onDismiss(ctx.Window)
			}
			// A click on the backdrop is the dismissal, not something
			// for whatever the palette is floating over.
			ctx.Consume()
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
				OnClick: func(ctx EventCtx) {
					// Absorb the click so it never reaches the
					// backdrop, which would dismiss the palette the
					// user is clicking into. An empty body used to be
					// enough, back when dispatch marked consume-class
					// events handled on the callback's behalf; now the
					// consume has to be said.
					ctx.Consume()
				},
				Content: []View{
					Row(ContainerCfg{
						Padding:    Some(PaddingSmall),
						Sizing:     FillFit,
						SizeBorder: NoBorder,
						Content: []View{
							Input(InputCfg{
								ID:            ScopeID(id, "input"),
								Text:          query,
								Placeholder:   cfg.Placeholder,
								TextStyle:     cfg.TextStyle,
								Sizing:        FillFit,
								OnTextChanged: makePaletteOnTextChanged(id),
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
// It always resets the results scroll (keyed ScopeID(id, "scroll")) to the top.
func CommandPaletteShow(id string, w *Window) {
	ss := StateMap[string, bool](w, nsCmdPalette, capModerate)
	ss.Set(id, true)
	sq := StateMap[string, string](w, nsCmdPaletteQuery, capModerate)
	sq.Set(id, "")
	sh := StateMap[string, int](w, nsCmdPaletteHighlight, capModerate)
	sh.Set(id, 0)
	w.scrollY().Set(ScopeID(id, "scroll"), 0)
	w.SetFocus(ScopeID(id, "input"))
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

func makePaletteOnEnter(paletteID string, onAction func(string, EventCtx), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string) func(EventCtx) {
	return func(ctx EventCtx) {
		sh := StateMap[string, int](ctx.Window, nsCmdPaletteHighlight, capModerate)
		// Default 0: first item highlighted; bounds-checked before use.
		cur := sh.GetOr(paletteID, 0)
		itemCount := len(filteredIDs)
		if cur >= 0 && cur < itemCount && onAction != nil &&
			!filtered[cur].Disabled {
			onAction(filteredIDs[cur], EventCtx{nil, ctx.Event, ctx.Window})
			CommandPaletteDismiss(paletteID, ctx.Window)
			if onDismiss != nil {
				onDismiss(ctx.Window)
			}
		}
	}
}

func makePaletteOnTextChanged(paletteID string) func(string, EventCtx) {
	return func(newText string, ctx EventCtx) {
		sq := StateMap[string, string](ctx.Window, nsCmdPaletteQuery, capModerate)
		sq.Set(paletteID, newText)
		sh := StateMap[string, int](ctx.Window, nsCmdPaletteHighlight, capModerate)
		sh.Set(paletteID, 0)
		ctx.Window.UpdateWindow()
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

func makePaletteOnKeyDown(paletteID string, onAction func(string, EventCtx), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string) func(EventCtx) {
	return func(ctx EventCtx) {
		paletteOnKeyDown(paletteID, onAction, onDismiss, filtered, filteredIDs, ctx.Event, ctx.Window)
	}
}

func paletteOnKeyDown(paletteID string, onAction func(string, EventCtx), onDismiss func(*Window), filtered []listCoreItem, filteredIDs []string, e *Event, w *Window) {
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
	// Default 0: first item highlighted; bounds-checked before use.
	cur := sh.GetOr(paletteID, 0)
	action := listCoreNavigate(e.KeyCode, itemCount)

	if action == listCoreSelectItem {
		if cur >= 0 && cur < itemCount && onAction != nil &&
			!filtered[cur].Disabled {
			onAction(filteredIDs[cur], EventCtx{nil, e, w})
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
