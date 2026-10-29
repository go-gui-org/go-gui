package gui

// goldenDropdownCases records open dropdowns and popup menus. Kept out
// of golden_cases_test.go, which is past the file-size limit.
func goldenDropdownCases() []goldenCase {
	return []goldenCase{
		{
			// A Fill field wider than its options: the open list is
			// as wide as the field and its rows fill it (#1155).
			name: "select_open_wide",
			build: func(w *Window) View {
				StateMap[string, bool](w, nsSelect, capModerate).
					Set("sel", true)
				return goldenWideColumn(Select(SelectCfg{
					ID:       "sel",
					Items:    []string{"alpha", "beta"},
					Selected: []string{"beta"},
					Sizing:   FillFit,
				}))
			},
		},
		{
			// Same as select_open_wide, for Combobox (#1155).
			name: "combobox_open_wide",
			build: func(w *Window) View {
				comboboxOpen("cb", "", w)
				return goldenWideColumn(Combobox(ComboboxCfg{
					ID:     "cb",
					Items:  []string{"alpha", "beta"},
					Value:  "alpha",
					Sizing: FillFit,
				}))
			},
		},
		{
			// The context menu open at a click point with room below and
			// right. The popup hangs from a zero-size anchor at the point
			// (#1193), which draws nothing: with no flip the recording is
			// the same as the popup offset from the area's corner. The
			// state is seeded rather than driven by a right press.
			name:    "context_menu_open",
			focusID: ScopeID("cm", "popup"),
			build: func(w *Window) View {
				StateMap[string, contextMenuState](w, nsContextMenu, capFew).
					Set("cm", contextMenuState{Open: true, X: 40, Y: 30})
				return ContextMenu(w, ContextMenuCfg{
					ID:     "cm",
					Sizing: FillFill,
					Items: []MenuItemCfg{
						MenuItemText("open", "Open"),
						MenuItemText("save", "Save"),
					},
				})
			},
		},
	}
}

// goldenWideColumn holds v in a column wider than v's content, so a
// Fill v is wider than its own options.
func goldenWideColumn(v View) View {
	return Column(ContainerCfg{
		Sizing:     FixedFit,
		Width:      300,
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    []View{v},
	})
}
