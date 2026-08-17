package gui

// view_virtual_list.go — a virtualized list whose rows may differ in
// height. The Cfg, the factory and GenerateLayout live here; the
// range arithmetic, the spacers and the measurement write-back are in
// view_virtual_list_build.go.
//
// The existing virtualized widgets (ListBox, Table, Tree, Combobox)
// own their row shape, so one scalar row height describes them
// exactly and they keep the O(1) path. VirtualList is for the case
// that scalar rules out: rows the caller builds, of heights only the
// layout engine knows — a chat transcript, a feed, a log view, a card
// list.

// VirtualListCfg configures a [VirtualList].
//
// # Heights
//
// Supply ItemHeight when the height is cheap to compute: it is then
// exact from the first frame and nothing is measured. Without it the
// list measures the rows it builds and refines a prefix-sum model, so
// positions converge over the first frames a region is visited and
// the scrollbar tightens as the reader travels.
//
// # Identity
//
// Supply ItemKey when the items can be inserted, deleted or
// reordered. Measured heights are stored under the key, so they
// follow their item; without it the model is index-keyed and an
// insert at the front leaves every row rendering at its neighbour's
// height until re-measured (call [Window.InvalidateListHeights] to
// force that). Content edited under a *stable* key needs an explicit
// invalidate either way — nothing detects it.
//
// # Scrolling
//
// The list is scrollable by construction and its scroll state is
// keyed by ID. Address rows by index with [Window.ScrollToIndex],
// [Window.ScrollToIndexAt], [Window.ScrollIndexIntoView] and
// [Window.ScrollToEnd]; ScrollVerticalToPct is the wrong tool here,
// because it takes a percentage of a content height that virtualized
// rows only estimate.
type VirtualListCfg struct {
	A11YCfg

	// ItemView builds row i. width is the list's inner content width,
	// as recorded by the previous frame's arrange — it is 0 on the
	// first frame and one frame stale after a resize.
	//
	// Use it for decisions, never for a minimum. A row that sets
	// MinWidth from it asks the list for at least the width the list
	// just reported; the row's own border and padding push that a few
	// pixels further, and the list widens every frame without bound —
	// re-wrapping and re-measuring each time, so it never settles.
	// Rows fill the width they are given through Sizing. [Debug]
	// reports the ratchet if it happens anyway.
	ItemView func(i int, width float32) View

	// ItemHeight returns row i's height without building it. Optional;
	// when set the model never measures and never re-anchors.
	// exportaudit:keep — the cheap path, for apps whose rows compute
	ItemHeight func(i int, width float32) float32

	// ItemKey returns a stable identity for row i. Optional; see the
	// Identity section above.
	ItemKey func(i int) string

	// OnKeyDown fires on the list container. It runs after the built-in
	// arrow/Home/End handling, which consumes the keys it acts on.
	OnKeyDown func(EventCtx)

	ID string `gui:"required"`

	Padding    Padding
	Radius     Opt[float32]
	SizeBorder Opt[float32]

	// ItemCount is the number of rows. Rows outside the viewport are
	// not built; their space is held by two spacer rectangles.
	ItemCount int

	// OverscanPx is how far beyond each viewport edge rows are built,
	// in pixels. Pixels rather than rows because a row count means
	// nothing when rows differ in height. Zero takes half a viewport.
	// exportaudit:keep — tuning knob for apps with unusual row sizes
	OverscanPx float32

	// Height sets the list's height directly. Virtualization needs a
	// resolved height: with Height or MaxHeight the list virtualizes
	// from the first frame; under Fill sizing the height is read back
	// from the previous frame's arrange, and the first frame builds a
	// bounded probe range instead of the whole corpus.
	Height    float32
	MinWidth  float32
	MaxWidth  float32
	MinHeight float32
	MaxHeight float32

	Color       Color
	ColorBorder Color
	// ColorBorderFocus is the border while the list holds focus.
	// Unset takes the theme's.
	ColorBorderFocus Color
	// Colors sets the per-state colors. Color above is the shorthand
	// for Colors.Base and wins over it, as do the other flat Color*
	// fields over their slots.
	Colors ColorSet

	Sizing Sizing

	// FocusDisabled opts out of the default-on focus. Focus also
	// requires a non-empty ID; without one the control is inert.
	FocusDisabled bool
	Disabled      bool
	Invisible     bool
}

type virtualListView struct {
	cfg VirtualListCfg
}

// VirtualList creates a virtualized list whose rows may differ in
// height. It requires an ID (scroll state, the height model and the
// focused index are all keyed by it) and an ItemView.
func VirtualList(cfg VirtualListCfg) View {
	RequireID("VirtualList", cfg.ID)
	applyVirtualListDefaults(&cfg)
	return &virtualListView{cfg: cfg}
}

// applyVirtualListDefaults fills unset styling from the theme's list
// box style. VirtualList deliberately shares it rather than
// introducing a second list appearance — the two widgets differ in
// how rows are sized, not in how a list looks.
func applyVirtualListDefaults(cfg *VirtualListCfg) {
	d := &defaultListBoxStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, Color{}, Color{},
		Color{}, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, nil, nil, nil,
		&cfg.ColorBorder, &cfg.ColorBorderFocus)
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
}

func (lv *virtualListView) GenerateLayout(w *Window) Layout {
	cfg := &lv.cfg
	// One resolved identity for the scroll offset, the height model
	// and the focused index; see (*Window).EffID.
	cfg.ID = w.EffID(cfg.ID)

	m := virtualListModel(cfg, w)
	first, last, lead, trail, probe := virtualListRange(cfg, m, w)
	m.builtFirst = first
	m.builtCount = max(last-first+1, 0)
	m.leadOffset = 0
	if lead > 0 {
		m.leadOffset = 1
	}
	if probe && DebugEnabled() && m.hSeen && cfg.ItemCount > 0 {
		// Arrange has run and still gave the list no height, so every
		// frame rebuilds a probe window instead of the real viewport.
		w.debugWarn(debugCheckListBoxNoHeight, cfg.ID,
			"VirtualList %q resolved to height 0, so it builds a "+
				"%d-row probe every frame instead of virtualizing; "+
				"set Height/MaxHeight or give it sizing that "+
				"allocates height",
			cfg.ID, virtualListProbeRows)
	}

	dn := &defaultListBoxStyle
	layout := Layout{Shape: w.allocShape(buildContainerShape(
		&ContainerCfg{
			ID:       cfg.ID,
			axis:     axisTopToBottom,
			A11YRole: AccessRoleList,
			A11YCfg: A11YCfg{
				A11YLabel:       a11yLabel(cfg.A11YLabel, cfg.ID),
				A11YDescription: cfg.A11YDescription,
			},
			Focusable:   !cfg.FocusDisabled,
			Scrollable:  true,
			AmendLayout: virtualListAmend(m, cfg.ColorBorderFocus),
			OnKeyDown:   virtualListKeyDown(cfg),
			Width:       cfg.MaxWidth,
			Height:      cfg.Height,
			MinWidth:    cfg.MinWidth,
			MaxWidth:    cfg.MaxWidth,
			MinHeight:   cfg.MinHeight,
			MaxHeight:   cfg.MaxHeight,
			Color:       cfg.Color,
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  Some(cfg.SizeBorder.Get(dn.SizeBorder)),
			Radius:      Some(cfg.Radius.Get(dn.Radius)),
			Padding:     cfg.Padding,
			Sizing:      cfg.Sizing,
			// Fixed at 0: a gap between rows is height the model does
			// not know about, and every row position would drift by
			// one gap per row. Space rows from inside ItemView.
			Spacing:   SomeF(0),
			Disabled:  cfg.Disabled,
			Invisible: cfg.Invisible,
		}, w))}

	// The parent shape must exist before the children generate: their
	// IDs resolve under its scope (see appendChildViews).
	appendChildViews(w, &layout,
		virtualListChildren(cfg, m, w, first, last, lead, trail))
	return layout
}

// virtualListModel registers this frame's height model, applying the
// count/width/theme invalidation rules.
func virtualListModel(cfg *VirtualListCfg, w *Window) *listHeightModel {
	// The width the heights were measured at is what the previous
	// frame's arrange recorded; on frame 1 there is none, and rows are
	// built at width 0 (documented on ItemView).
	var width float32
	if prev, ok := listHeightLookup(w, cfg.ID); ok {
		width = prev.measuredW
	}
	return listHeightEnsureVariable(w, cfg.ID, listHeightSpec{
		n:        max(cfg.ItemCount, 0),
		estimate: virtualListEstimate(cfg, width),
		width:    width,
		keyAt:    cfg.ItemKey,
		heightFn: cfg.ItemHeight,
	})
}

// virtualListEstimate seeds unmeasured rows. The caller's own
// function wins; otherwise the shared list row estimate, which is
// what the uniform widgets already assume.
func virtualListEstimate(cfg *VirtualListCfg, width float32) float32 {
	if cfg.ItemHeight != nil && cfg.ItemCount > 0 {
		if h := cfg.ItemHeight(0, width); h > 0 && f32IsFinite(h) {
			return h
		}
	}
	return listCoreRowHeightEstimate(
		defaultListBoxStyle.textStyleNormal, listBoxItemPad)
}
