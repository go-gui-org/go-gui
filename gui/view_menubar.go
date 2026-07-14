package gui

import "slices"

// MenubarCfg configures a horizontal menubar or standalone
// menu.
type MenubarCfg struct {
	TextStyle         TextStyle
	TextStyleSubtitle TextStyle
	Action            func(string, *Event, *Window)
	ID                string
	Items             []MenuItemCfg
	FloatZIndex       int
	Padding           Opt[Padding]
	PaddingMenuItem   Opt[Padding]
	PaddingSubmenu    Opt[Padding]
	PaddingSubtitle   Opt[Padding]
	SizeBorder        Opt[float32]
	WidthSubmenuMin   Opt[float32]
	WidthSubmenuMax   Opt[float32]
	Radius            Opt[float32]
	RadiusBorder      Opt[float32]
	RadiusSubmenu     Opt[float32]
	RadiusMenuItem    Opt[float32]
	Spacing           Opt[float32]
	SpacingSubmenu    Opt[float32]
	FloatOffsetX      float32
	FloatOffsetY      float32
	Color             Color
	ColorBorder       Color
	ColorSelect       Color
	Sizing            Sizing
	FloatAnchor       FloatAttach
	FloatTieOff       FloatAttach
	Disabled          bool
	Invisible         bool
	Float             bool
	FloatAutoFlip     bool
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
	d := &DefaultMenubarStyle
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
	if cfg.TextStyleSubtitle == (TextStyle{}) {
		cfg.TextStyleSubtitle = d.TextStyleSubtitle
	}
	if cfg.Sizing == (Sizing{}) {
		cfg.Sizing = FillFit
	}
	if !cfg.Padding.IsSet() {
		cfg.Padding = Some(d.Padding)
	}
	if !cfg.PaddingMenuItem.IsSet() {
		cfg.PaddingMenuItem = Some(d.PaddingMenuItem)
	}
	if !cfg.PaddingSubmenu.IsSet() {
		cfg.PaddingSubmenu = Some(d.PaddingSubmenu)
	}
	if !cfg.PaddingSubtitle.IsSet() {
		cfg.PaddingSubtitle = Some(d.PaddingSubtitle)
	}
	if !cfg.SpacingSubmenu.IsSet() {
		cfg.SpacingSubmenu = Some(d.SpacingSubmenu)
	}
	if cfg.Action == nil {
		cfg.Action = func(_ string, e *Event, _ *Window) {
			e.IsHandled = true
		}
	}
}

// MenuIDMap maps menu item IDs to directional nav nodes.
type MenuIDMap map[string]MenuIDNode

// MenuIDNode stores directional navigation targets.
type MenuIDNode struct {
	Left  string
	Right string
	Up    string
	Down  string
}

func makeMenubarOnKeyDown(cfg MenubarCfg) func(*Layout, *Event, *Window) {
	return func(_ *Layout, e *Event, w *Window) {
		menuOnKeyDown(cfg, menuMapper, e, w)
	}
}

// menuMapperVertical builds a directional navigation graph
// for a vertical standalone menu (context menu). Top-level
// items use Up/Down for siblings, Right to enter submenus.
func menuMapperVertical(items []MenuItemCfg) MenuIDMap {
	m := make(MenuIDMap)
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
		node := MenuIDNode{
			Up:    menuItemUp(i, selectables),
			Down:  menuItemDown(i, selectables),
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
func menuMapper(items []MenuItemCfg) MenuIDMap {
	m := make(MenuIDMap)
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

		node := MenuIDNode{
			Left:  selectables[leftIdx].ID,
			Right: selectables[rightIdx].ID,
			Up:    item.ID,
			Down:  item.ID,
		}

		// Down goes to first submenu child.
		if len(item.Submenu) > 0 {
			if first, ok := firstSelectable(item.Submenu); ok {
				node.Down = first.ID
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
	rootNode MenuIDNode, rootRight string,
	m MenuIDMap) {

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
		node := MenuIDNode{
			Left:  parentID,
			Right: menuItemRight(item, rootRight),
			Up:    menuItemUp(i, selectables),
			Down:  menuItemDown(i, selectables),
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
