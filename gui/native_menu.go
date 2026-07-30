package gui

// NativeMenuItemCfg defines a native OS menu item.
type NativeMenuItemCfg struct {
	ID        string
	Text      string
	CommandID string // resolve from command registry
	Submenu   []NativeMenuItemCfg
	Shortcut  Shortcut
	Separator bool
	Disabled  bool
	Checked   bool
}

// NativeMenuCfg defines a single top-level native menu.
type NativeMenuCfg struct {
	Title string
	Items []NativeMenuItemCfg
}

// NativeMenubarCfg configures the native OS menubar.
type NativeMenubarCfg struct {
	OnAction func(id string) // fallback for items without CommandID
	AppName  string          // "About <AppName>", etc.
	// AboutActionID, if non-empty, routes the app-menu "About" click through
	// OnAction with this ID instead of the default system About panel.
	AboutActionID           string
	Menus                   []NativeMenuCfg // File, Edit, View, etc.
	IncludeEditMenu         bool            // auto-wire standard Edit menu
	SuppressSystemEditItems bool            // remove OS-injected AutoFill/WritingTools/Dictation
	// OmitAboutItem drops "About <AppName>" (and its separator) from the app
	// menu, leaving Quit alone. For apps that put About under their own Help
	// menu. Takes precedence over AboutActionID — no About item is created.
	OmitAboutItem bool
	// IncludeWindowMenu auto-wires the standard Window menu (Close, Minimize,
	// Zoom, Bring All to Front) and registers it with the OS window list.
	// Installing a menubar replaces the backend's default one, so without
	// this the app loses Cmd+W / Cmd+M.
	IncludeWindowMenu bool
}

// NativeMenuItemsFromMenuItems converts in-app MenuItemCfg
// to NativeMenuItemCfg. Fields not present in the native type
// (CustomView, Action, styling) are dropped.
func NativeMenuItemsFromMenuItems(
	items []MenuItemCfg,
) []NativeMenuItemCfg {
	out := make([]NativeMenuItemCfg, len(items))
	for i, item := range items {
		out[i] = NativeMenuItemCfg{
			ID:        item.ID,
			Text:      item.Text,
			CommandID: item.CommandID,
			Separator: item.Separator,
			Disabled:  item.disabled,
		}
		if len(item.Submenu) > 0 {
			out[i].Submenu = NativeMenuItemsFromMenuItems(
				item.Submenu)
		}
	}
	return out
}
