package datagrid

import (
	"fmt"
	"strconv"

	gg "github.com/go-gui-org/go-gui/gui"
)

// --- Quick filter ---

func dataGridQuickFilterRow(cfg *DataGridCfg) gg.View {
	h := dataGridQuickFilterHeight(cfg)
	queryCallback := cfg.OnQueryChange
	query := cfg.Query
	value := query.QuickFilter
	inputID := cfg.ID + ":quick_filter"
	inputFocusID := inputID
	matchesText := dataGridQuickFilterMatchesText(cfg)
	clearDisabled := value == "" || queryCallback == nil
	debounce := cfg.QuickFilterDebounce

	dimColor := cfg.TextStyleFilter.Color
	dimColor.A = 140
	placeholderStyle := cfg.TextStyleFilter
	placeholderStyle.Color = dimColor

	return gg.Row(gg.ContainerCfg{
		Height:      h,
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorQuickFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     gg.SomeP(0, cfg.PaddingCell.Get(gg.Padding{}).Right, 0, cfg.PaddingCell.Get(gg.Padding{}).Left),
		Spacing:     gg.SomeF(6),
		VAlign:      gg.VAlignMiddle,
		OnClick: func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
			if inputFocusID != "" {
				w.SetFocus(inputFocusID)
			}
			e.IsHandled = true
		},
		Content: []gg.View{
			gg.Input(gg.InputCfg{
				ID:               inputID,
				Text:             value,
				Placeholder:      cfg.QuickFilterPlaceholder,
				Sizing:           gg.FillFill,
				Padding:          gg.NoPadding,
				SizeBorder:       gg.SomeF(0),
				Radius:           gg.SomeF(0),
				Color:            cfg.ColorQuickFilter,
				ColorHover:       cfg.ColorQuickFilter,
				ColorBorder:      cfg.ColorBorder,
				TextStyle:        cfg.TextStyleFilter,
				PlaceholderStyle: placeholderStyle,
				OnTextChanged: func(_ *gg.Layout, text string, w *gg.Window) {
					if queryCallback == nil {
						return
					}
					if debounce <= 0 {
						next := GridQueryState{
							Sorts:       append([]GridSort(nil), query.Sorts...),
							Filters:     append([]GridFilter(nil), query.Filters...),
							QuickFilter: text,
						}
						e := &gg.Event{}
						queryCallback(next, e, w)
						return
					}
					sorts := append([]GridSort(nil), query.Sorts...)
					filters := append([]GridFilter(nil), query.Filters...)
					w.AnimationAdd(&gg.Animate{
						AnimID: inputID + ":debounce",
						Delay:  debounce,
						Callback: func(_ *gg.Animate, w *gg.Window) {
							next := GridQueryState{
								Sorts:       sorts,
								Filters:     filters,
								QuickFilter: text,
							}
							e := &gg.Event{}
							queryCallback(next, e, w)
						},
					})
				},
			}),
			gg.Text(gg.TextCfg{
				Text:      matchesText,
				Mode:      gg.TextModeSingleLine,
				TextStyle: dataGridIndicatorTextStyle(cfg.TextStyleFilter),
			}),
			dataGridIndicatorButton(gg.ActiveLocale.StrClear, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				clearDisabled, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					if queryCallback == nil {
						return
					}
					w.AnimationRemove(inputID + ":debounce")
					next := GridQueryState{
						Sorts:       append([]GridSort(nil), query.Sorts...),
						Filters:     append([]GridFilter(nil), query.Filters...),
						QuickFilter: "",
					}
					queryCallback(next, e, w)
					if inputFocusID != "" {
						w.SetFocus(inputFocusID)
					}
					e.IsHandled = true
				}),
		},
	})
}

func dataGridQuickFilterMatchesText(cfg *DataGridCfg) string {
	if cfg.RowCount != nil {
		return gg.LocaleMatchesFmt(len(cfg.Rows), strconv.Itoa(*cfg.RowCount))
	}
	if dataGridHasSource(cfg) {
		return gg.LocaleMatchesFmt(len(cfg.Rows), "?")
	}
	return fmt.Sprintf("%s %d", gg.ActiveLocale.StrMatches, len(cfg.Rows))
}

// --- Column chooser ---

func dataGridColumnChooserRow(cfg *DataGridCfg, isOpen bool, focusID string) gg.View {
	onHiddenColumnsChange := cfg.OnHiddenColumnsChange
	hasVisibilityCallback := onHiddenColumnsChange != nil
	chooserLabel := gg.ActiveLocale.StrColumns + " ▶" // ▶
	if isOpen {
		chooserLabel = gg.ActiveLocale.StrColumns + " ▼" // ▼
	}
	rowH := cfg.RowHeight
	if rowH <= 0 {
		rowH = dataGridHeaderHeight(cfg)
	}
	gridID := cfg.ID
	columns := cfg.Columns

	content := make([]gg.View, 0, 2)
	content = append(content, gg.Row(gg.ContainerCfg{
		Height:  rowH,
		Sizing:  gg.FillFixed,
		Padding: cfg.PaddingFilter,
		Spacing: gg.SomeF(6),
		VAlign:  gg.VAlignMiddle,
		Content: []gg.View{
			dataGridIndicatorButton(chooserLabel, cfg.TextStyleFilter, cfg.ColorHeaderHover,
				false, 0, func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
					dataGridToggleColumnChooserOpen(gridID, w)
					if focusID != "" {
						w.SetFocus(focusID)
					}
					e.IsHandled = true
				}),
		},
	}))
	if isOpen {
		options := make([]gg.View, 0, len(columns))
		for _, col := range columns {
			if col.ID == "" {
				continue
			}
			hidden := cfg.HiddenColumnIDs[col.ID]
			colID := col.ID
			options = append(options, gg.Toggle(gg.ToggleCfg{
				ID:       gridID + ":col-chooser:" + col.ID,
				Label:    col.Title,
				Selected: !hidden,
				Disabled: !hasVisibilityCallback,
				OnClick: dataGridMakeColumnChooserOnClick(onHiddenColumnsChange,
					cfg.HiddenColumnIDs, columns, colID, focusID),
			}))
		}
		content = append(content, gg.Row(gg.ContainerCfg{
			Height:      rowH,
			Sizing:      gg.FillFixed,
			Padding:     cfg.PaddingFilter,
			Spacing:     gg.SomeF(8),
			Color:       gg.ColorTransparent,
			ColorBorder: cfg.ColorBorder,
			SizeBorder:  gg.SomeF(0),
			Content:     options,
		}))
	}
	return gg.Column(gg.ContainerCfg{
		Height:      dataGridColumnChooserHeight(cfg, isOpen),
		Sizing:      gg.FillFixed,
		Color:       cfg.ColorFilter,
		ColorBorder: cfg.ColorBorder,
		SizeBorder:  gg.SomeF(0),
		Padding:     gg.NoPadding,
		Spacing:     gg.SomeF(0),
		Content:     content,
	})
}

func dataGridMakeColumnChooserOnClick(onHiddenColumnsChange func(map[string]bool, *gg.Event, *gg.Window), hiddenColumnIDs map[string]bool, columns []GridColumnCfg, colID string, focusID string) func(*gg.Layout, *gg.Event, *gg.Window) {
	return func(_ *gg.Layout, e *gg.Event, w *gg.Window) {
		if onHiddenColumnsChange == nil {
			return
		}
		nextHidden := dataGridNextHiddenColumns(hiddenColumnIDs, colID, columns)
		onHiddenColumnsChange(nextHidden, e, w)
		if focusID != "" {
			w.SetFocus(focusID)
		}
		e.IsHandled = true
	}
}

func dataGridToggleColumnChooserOpen(gridID string, w *gg.Window) {
	dgCO := gg.StateMap[string, bool](w, nsDgChooserOpen, capModerate)
	// Default false: absent entry means column chooser is closed.
	isOpen := dgCO.GetOr(gridID, false)
	dgCO.Set(gridID, !isOpen)
}
