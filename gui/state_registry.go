package gui

// StateRegistry stores per-widget BoundedMap instances keyed by
// namespace string.
//
// Concurrency: StateMap reads and writes registry.maps without
// holding w.mu. This is safe because all callers execute on the
// main goroutine — during GenerateLayout (under w.mu), AmendLayout
// (under w.mu), and OnValue animation callbacks (dispatched via
// flushCommands on the main goroutine, before FrameFn acquires
// w.mu). The animation loop goroutine only enqueues OnValue
// callbacks; it never calls StateMap directly.
type StateRegistry struct {
	maps map[string]any
}

// StateMap returns (or lazily creates) a *BoundedMap[K, V] for the
// given namespace.
// SAFETY: main-goroutine only (see StateRegistry doc).
func StateMap[K comparable, V any](w *Window, ns string, maxSize int) *BoundedMap[K, V] {
	if ptr, ok := w.viewState.registry.maps[ns]; ok {
		return ptr.(*BoundedMap[K, V])
	}
	m := NewBoundedMap[K, V](maxSize)
	if w.viewState.registry.maps == nil {
		w.viewState.registry.maps = make(map[string]any)
	}
	w.viewState.registry.maps[ns] = m
	return m
}

// StateMapRead returns a *BoundedMap[K, V] for read-only access.
// Returns nil if namespace not initialized.
func StateMapRead[K comparable, V any](w *Window, ns string) *BoundedMap[K, V] {
	if ptr, ok := w.viewState.registry.maps[ns]; ok {
		return ptr.(*BoundedMap[K, V])
	}
	return nil
}

// StateReadOr returns the value for key in namespace, or defaultVal
// if not found.
func StateReadOr[K comparable, V any](w *Window, ns string, key K, defaultVal V) V {
	sm := StateMapRead[K, V](w, ns)
	if sm == nil {
		return defaultVal
	}
	v, ok := sm.Get(key)
	if !ok {
		return defaultVal
	}
	return v
}

// RequireID panics if id is empty. Use in stateful widget factories
// whose internal state is keyed by cfg.ID in StateMap.
func RequireID(widget, id string) {
	if id == "" {
		panic("gui: " + widget + " requires a non-empty Cfg.ID")
	}
}

// Clear drops all registry references.
func (r *StateRegistry) Clear() {
	clear(r.maps)
}

// ClearNamespace drops all entries in a single namespace.
func (r *StateRegistry) ClearNamespace(ns string) {
	delete(r.maps, ns)
}

// entryCount returns the number of entries in the BoundedMap
// for the given namespace, or 0 if not found.
func (r *StateRegistry) entryCount(ns string) int {
	type lenner interface{ Len() int }
	if l, ok := r.maps[ns].(lenner); ok {
		return l.Len()
	}
	return 0
}

// Namespace constants for internal gui state maps.
const (
	nsOverflow            = "gui.overflow"
	nsScrollX             = "gui.scroll.x"
	nsScrollY             = "gui.scroll.y"
	nsSelect              = "gui.select"
	nsInput               = "gui.input"
	nsInputFocus          = "gui.input.focus"
	nsSelectHL            = "gui.select.highlight"
	nsListBoxFocus        = "gui.listbox.focus"
	nsListBoxCache        = "gui.listbox.cache"
	nsProgress            = "gui.progress"
	nsSidebar             = "gui.sidebar"
	nsCombobox            = "gui.combobox"
	nsComboboxQuery       = "gui.combobox.query"
	nsComboboxHighlight   = "gui.combobox.highlight"
	nsComboboxItems       = "gui.combobox.items"
	nsCmdPalette          = "gui.cmd_palette"
	nsCmdPaletteQuery     = "gui.cmd_palette.query"
	nsCmdPaletteHighlight = "gui.cmd_palette.highlight"
	nsCmdPaletteItems     = "gui.cmd_palette.items"
	nsTreeExpanded        = "gui.tree.expanded"
	nsTreeFocus           = "gui.tree.focus"
	nsTreeLazy            = "gui.tree.lazy"
	nsInspector           = "gui.inspector"
	nsInspectorWidth      = "gui.inspector.w"
	nsDrawCanvas          = "gui.draw_canvas"
	nsMenu                = "gui.menu"
	nsDatePicker          = "gui.date_picker"
	nsColorPicker         = "gui.color_picker"
	nsSliderPress         = "gui.slider.press"
	nsInputDate           = "gui.input_date"
	nsInputDateText       = "gui.input_date.text"
	nsDgColWidths         = "gui.dg.col_widths"
	nsDgPresentation      = "gui.dg.presentation"
	nsDgResize            = "gui.dg.resize"
	nsDgHeaderHover       = "gui.dg.header_hover"
	nsDgRange             = "gui.dg.range"
	nsDgChooserOpen       = "gui.dg.chooser_open"
	nsDgEdit              = "gui.dg.edit"
	nsDgCrud              = "gui.dg.crud"
	nsDgJump              = "gui.dg.jump"
	nsDgPendingJump       = "gui.dg.pending_jump"
	nsDgSource            = "gui.dg.source"
	nsActiveDownloads     = "gui.active_downloads"
	nsImageResolved       = "gui.image.resolved"
	nsSvgCache            = "gui.svg_cache"
	nsSvgDimCache         = "gui.svg_dim_cache"
	nsSvgAnimSeen         = "gui.svg_anim_seen"
	nsDragReorder         = "gui.drag_reorder"
	nsDragReorderIDsMeta  = "gui.drag_reorder.ids_meta"
	nsTableColWidths      = "gui.table.col_widths"
	nsDockDrag            = "gui.dock_drag"
	nsContextMenu         = "gui.context_menu"
	nsContextMenuFocus    = "gui.context_menu.focus"
	nsRtfLinkMenu         = "gui.rtf_link_menu"
	nsForm                = "gui.form"
	nsSpellCheck          = "gui.spell_check"
	nsSkeleton            = "gui.skeleton"
	nsMathSpinner         = "gui.math_spinner"
	nsHoverInside         = "gui.hover.inside"
	nsMdSel               = "gui.markdown.sel"
	nsMdBlocks            = "gui.markdown.blocks"
)

// Capacity tiers.
const (
	capFew        = 20
	capModerate   = 50
	capMany       = 100
	capScroll     = 200
	capImageCache = 500
)
