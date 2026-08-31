package gui

import "slices"

// MenubarCfg configures a horizontal menubar or standalone
// menu.
type MenubarCfg struct {
	TextStyle TextStyle
	// TextStyleSubtitle styles subtitle items. Zero takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	TextStyleSubtitle TextStyle
	Action            func(string, EventCtx)
	ID                string
	Items             []MenuItemCfg
	FloatZIndex       int
	Padding           Padding
	// PaddingMenuItem insets each item. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingMenuItem Padding
	// PaddingSubmenu insets submenu panes. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingSubmenu Padding
	// PaddingSubtitle insets subtitle items. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	PaddingSubtitle Padding
	SizeBorder      Opt[float32]
	// WidthSubmenuMin/Max bound submenu panes. Unset takes the
	// theme defaults.
	// exportaudit:keep — caller-facing config (issue #372)
	WidthSubmenuMin Opt[float32]
	// exportaudit:keep — caller-facing config (issue #372)
	WidthSubmenuMax Opt[float32]
	Radius          Opt[float32]
	// RadiusBorder rounds the bar frame. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusBorder Opt[float32]
	// RadiusSubmenu rounds submenu panes. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusSubmenu Opt[float32]
	// RadiusMenuItem rounds each item. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	RadiusMenuItem Opt[float32]
	Spacing        Opt[float32]
	// SpacingSubmenu gaps submenu items. Unset takes the theme
	// default.
	// exportaudit:keep — caller-facing config (issue #372)
	SpacingSubmenu Opt[float32]
	FloatOffsetX   float32
	FloatOffsetY   float32
	Color          Color
	ColorBorder    Color
	ColorSelect    Color
	// ColorTextOnSelect is the text color drawn over the selected
	// item's fill. Unset takes the theme's.
	// exportaudit:keep — caller-facing config (issue #372)
	ColorTextOnSelect Color
	// Colors sets the per-state colors. Color above is the
	// shorthand for Colors.Base and wins over it; the other flat
	// Color* fields win over their Colors slots the same way.
	Colors      ColorSet
	Sizing      Sizing
	FloatAnchor floatAttach
	FloatTieOff floatAttach
	Disabled    bool
	Invisible   bool
	Float       bool
	// FloatAutoFlip mirrors the float to the opposite side when it
	// would cross the window edge.
	// exportaudit:keep — caller-facing config (issue #372)
	FloatAutoFlip bool

	// Sound overrides the theme's click cue for this instance.
	// SoundNone (the zero value) takes the theme's cue for that role,
	// which is itself silent unless the app opted in (issue #446).
	// exportaudit:keep — caller-facing config (issue #467)
	Sound SoundCue

	// SoundDisabled suppresses every item's sound regardless of the theme
	// and of Sound above.
	// exportaudit:keep — caller-facing config (issue #467)
	SoundDisabled bool
}

// Menubar creates a horizontal menubar with keyboard
// navigation.
func Menubar(w *Window, cfg MenubarCfg) View {
	applyMenubarDefaults(&cfg)
	RequireID("Menubar", cfg.ID)
	checkForDuplicateMenuIDs(cfg.Items)

	// On focus with no selection, select first item.
	if w.IsFocus(cfg.ID) {
		sel := StateReadOr(
			w, nsMenu, cfg.ID, "")
		if sel == "" {
			if first, ok := firstSelectable(cfg.Items); ok {
				sm := StateMap[string, string](
					w, nsMenu, capModerate)
				sm.Set(cfg.ID, first.ID)
			}
		}
	}

	return Row(ContainerCfg{
		ID:            cfg.ID,
		Focusable:     true,
		Color:         cfg.Color,
		ColorBorder:   cfg.ColorBorder,
		SizeBorder:    cfg.SizeBorder,
		Radius:        cfg.RadiusBorder,
		Spacing:       cfg.Spacing,
		Padding:       cfg.Padding,
		Sizing:        cfg.Sizing,
		Float:         cfg.Float,
		FloatAutoFlip: cfg.FloatAutoFlip,
		FloatAnchor:   cfg.FloatAnchor,
		FloatTieOff:   cfg.FloatTieOff,
		FloatOffsetX:  cfg.FloatOffsetX,
		FloatOffsetY:  cfg.FloatOffsetY,
		FloatZIndex:   cfg.FloatZIndex,
		Disabled:      cfg.Disabled,
		Invisible:     cfg.Invisible,
		A11YRole:      AccessRoleMenuBar,
		OnKeyDown:     makeMenubarOnKeyDown(cfg),
		AmendLayout:   makeMenuAmendLayout(cfg.ID),
		Content:       menuBuild(cfg, 0, cfg.Items, w),
	})
}

func applyMenubarDefaults(cfg *MenubarCfg) {
	d := &defaultMenubarStyle
	cfg.Colors = cfg.Colors.resolved(cfg.Color, themeColorSet(
		d.Color, d.ColorHover, Color{},
		d.ColorFocus, d.ColorBorder, d.ColorBorderFocus,
	))
	cfg.Colors.applyTo(&cfg.Color, nil, nil, nil,
		&cfg.ColorBorder, nil)
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if !cfg.ColorTextOnSelect.IsSet() {
		cfg.ColorTextOnSelect = d.ColorTextOnSelect
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.TextStyleSubtitle == (TextStyle{}) {
		cfg.TextStyleSubtitle = d.textStyleSubtitle
	}
	cfg.Sizing = cfg.Sizing.Or(FillFit)
	if !cfg.Padding.IsSet() {
		cfg.Padding = d.Padding
	}
	if !cfg.PaddingMenuItem.IsSet() {
		cfg.PaddingMenuItem = d.paddingMenuItem
	}
	if !cfg.PaddingSubmenu.IsSet() {
		cfg.PaddingSubmenu = d.paddingSubmenu
	}
	if !cfg.PaddingSubtitle.IsSet() {
		cfg.PaddingSubtitle = d.paddingSubtitle
	}
	if !cfg.SpacingSubmenu.IsSet() {
		cfg.SpacingSubmenu = Some(d.spacingSubmenu)
	}
	if cfg.Action == nil {
		cfg.Action = func(_ string, ctx EventCtx) {
			ctx.Consume()
		}
	}
}

// MenuIDMap maps menu item IDs to directional nav nodes.
type menuIDMap map[string]menuIDNode

// MenuIDNode stores directional navigation targets.
type menuIDNode struct {
	Left  string
	Right string
	up    string
	down  string
}

func makeMenubarOnKeyDown(cfg MenubarCfg) func(EventCtx) {
	return func(ctx EventCtx) {
		menuOnKeyDown(cfg, menuMapper, ctx.Event, ctx.Window)
	}
}

// menuMapperVertical builds a directional navigation graph
// for a vertical standalone menu (context menu). Top-level
// items use Up/Down for siblings, Right to enter submenus.
func menuMapperVertical(items []MenuItemCfg) menuIDMap {
	m := make(menuIDMap)
	selectables := make([]MenuItemCfg, 0, len(items))
	for _, item := range items {
		if isSelectableMenuID(item.ID) {
			selectables = append(selectables, item)
		}
	}
	if len(selectables) == 0 {
		return m
	}

	for i, item := range selectables {
		node := menuIDNode{
			up:    menuItemUp(i, selectables),
			down:  menuItemDown(i, selectables),
			Right: menuItemRight(item, ""),
		}
		m[item.ID] = node

		if len(item.Submenu) > 0 {
			submenuMapper(item.Submenu, item.ID,
				node, "", m)
		}
	}
	return m
}

// menuMapper builds a directional navigation graph for all
// menu items.
func menuMapper(items []MenuItemCfg) menuIDMap {
	m := make(menuIDMap)
	selectables := make([]MenuItemCfg, 0, len(items))
	for _, item := range items {
		if isSelectableMenuID(item.ID) {
			selectables = append(selectables, item)
		}
	}
	if len(selectables) == 0 {
		return m
	}

	for i, item := range selectables {
		leftIdx := (i - 1 + len(selectables)) % len(selectables)
		rightIdx := (i + 1) % len(selectables)

		node := menuIDNode{
			Left:  selectables[leftIdx].ID,
			Right: selectables[rightIdx].ID,
			up:    item.ID,
			down:  item.ID,
		}

		// Down goes to first submenu child.
		if len(item.Submenu) > 0 {
			if first, ok := firstSelectable(item.Submenu); ok {
				node.down = first.ID
			}
		}

		m[item.ID] = node

		// Build submenu mappings.
		if len(item.Submenu) > 0 {
			rightID := selectables[rightIdx].ID
			submenuMapper(item.Submenu, item.ID,
				node, rightID, m)
		}
	}
	return m
}

// submenuMapper recursively builds navigation for submenu
// items.
func submenuMapper(items []MenuItemCfg, parentID string,
	rootNode menuIDNode, rootRight string,
	m menuIDMap) {

	selectables := make([]MenuItemCfg, 0, len(items))
	for _, item := range items {
		if isSelectableMenuID(item.ID) {
			selectables = append(selectables, item)
		}
	}
	if len(selectables) == 0 {
		return
	}

	for i, item := range selectables {
		node := menuIDNode{
			Left:  parentID,
			Right: menuItemRight(item, rootRight),
			up:    menuItemUp(i, selectables),
			down:  menuItemDown(i, selectables),
		}
		m[item.ID] = node

		if len(item.Submenu) > 0 {
			submenuMapper(item.Submenu, item.ID,
				rootNode, rootRight, m)
		}
	}
}

// isSelectableMenuID returns true if the ID is neither a
// separator nor subtitle sentinel.
func isSelectableMenuID(id string) bool {
	return id != menuSeparatorID && id != menuSubtitleID
}

func nextSelectable(idx int, items []MenuItemCfg) (MenuItemCfg, bool) {
	for i := idx + 1; i < len(items); i++ {
		if isSelectableMenuID(items[i].ID) {
			return items[i], true
		}
	}
	return MenuItemCfg{}, false
}

func previousSelectable(idx int, items []MenuItemCfg) (MenuItemCfg, bool) {
	for i := idx - 1; i >= 0; i-- {
		if isSelectableMenuID(items[i].ID) {
			return items[i], true
		}
	}
	return MenuItemCfg{}, false
}

func firstSelectable(items []MenuItemCfg) (MenuItemCfg, bool) {
	for _, item := range items {
		if isSelectableMenuID(item.ID) {
			return item, true
		}
	}
	return MenuItemCfg{}, false
}

func lastSelectable(items []MenuItemCfg) (MenuItemCfg, bool) {
	for _, item := range slices.Backward(items) {
		if isSelectableMenuID(item.ID) {
			return item, true
		}
	}
	return MenuItemCfg{}, false
}

// menuItemRight returns the right-nav target: first submenu
// child if present, else idRight (root-level right sibling).
func menuItemRight(item MenuItemCfg, idRight string) string {
	if len(item.Submenu) > 0 {
		if first, ok := firstSelectable(item.Submenu); ok {
			return first.ID
		}
	}
	return idRight
}

// menuItemUp returns the up-nav target: previous selectable
// sibling or wrap to last.
func menuItemUp(idx int, items []MenuItemCfg) string {
	if prev, ok := previousSelectable(idx, items); ok {
		return prev.ID
	}
	if last, ok := lastSelectable(items); ok {
		return last.ID
	}
	return items[idx].ID
}

// menuItemDown returns the down-nav target: next selectable
// sibling or wrap to first.
func menuItemDown(idx int, items []MenuItemCfg) string {
	if next, ok := nextSelectable(idx, items); ok {
		return next.ID
	}
	if first, ok := firstSelectable(items); ok {
		return first.ID
	}
	return items[idx].ID
}
