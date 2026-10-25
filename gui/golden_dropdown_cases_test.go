package gui

// goldenDropdownCases records open dropdowns. Kept out of
// golden_cases_test.go, which is past the file-size limit.
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
