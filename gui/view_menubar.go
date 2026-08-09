package gui

import "slices"

// MenubarCfg configures a horizontal menubar or standalone
// menu.
type MenubarCfg struct {
	TextStyle         TextStyle
	textStyleSubtitle TextStyle
	Action            func(string, EventCtx)
	ID                string
	Items             []MenuItemCfg
	FloatZIndex       int
	Padding           Opt[Padding]
	paddingMenuItem   Opt[Padding]
	paddingSubmenu    Opt[Padding]
	paddingSubtitle   Opt[Padding]
	SizeBorder        Opt[float32]
	widthSubmenuMin   Opt[float32]
	widthSubmenuMax   Opt[float32]
	Radius            Opt[float32]
	radiusBorder      Opt[float32]
	radiusSubmenu     Opt[float32]
	radiusMenuItem    Opt[float32]
	Spacing           Opt[float32]
	spacingSubmenu    Opt[float32]
	FloatOffsetX      float32
	FloatOffsetY      float32
	Color             Color
	ColorBorder       Color
	ColorSelect       Color
	Sizing            Sizing
	FloatAnchor       floatAttach
	FloatTieOff       floatAttach
	Disabled          bool
	Invisible         bool
	Float             bool
	floatAutoFlip     bool
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
		Radius:        cfg.radiusBorder,
		Spacing:       cfg.Spacing,
		Padding:       cfg.Padding,
		Sizing:        cfg.Sizing,
		Float:         cfg.Float,
		floatAutoFlip: cfg.floatAutoFlip,
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
	if !cfg.Color.IsSet() {
		cfg.Color = d.Color
	}
	if !cfg.ColorBorder.IsSet() {
		cfg.ColorBorder = d.ColorBorder
	}
	if !cfg.ColorSelect.IsSet() {
		cfg.ColorSelect = d.ColorSelect
	}
	if cfg.TextStyle == (TextStyle{}) {
		cfg.TextStyle = d.TextStyle
	}
	if cfg.textStyleSubtitle == (TextStyle{}) {
		cfg.textStyleSubtitle = d.textStyleSubtitle
	}
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FillFit
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if !cfg.paddingMenuItem.IsSet() {
		cfg.paddingMenuItem = Some(d.paddingMenuItem)
	}
	if !cfg.paddingSubmenu.IsSet() {
		cfg.paddingSubmenu = Some(d.paddingSubmenu)
	}
	if !cfg.paddingSubtitle.IsSet() {
		cfg.paddingSubtitle = Some(d.paddingSubtitle)
	}
	if !cfg.spacingSubmenu.IsSet() {
		cfg.spacingSubmenu = Some(d.spacingSubmenu)
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
