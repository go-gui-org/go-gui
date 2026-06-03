package main

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/highlight"
)

const showcaseDataGridFeaturesSource = `# Data Grid Features

- Virtual row rendering
- Single and multi-column sorting
- Per-column filter row and quick filter input
- Row selection: single, toggle, and range
- Header controls for sort, reorder, resize, and pin
- Controlled pagination and detail rows
- CRUD mutations for data-source backed grids
- Clipboard and export helpers`

func demoTable(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	rows := showcaseTableRowsSorted(app.TableSortBy)
	cfg := gui.TableCfgFromData(rows)
	cfg.ID = "catalog-table"
	cfg.IDScroll = 9110
	cfg.Sizing = gui.FitFit
	cfg.MaxHeight = 260
	cfg.SizeBorder = 1
	cfg.SizeBorderHeader = 2
	cfg.BorderStyle = tableBorderStyleFromValue(app.TableBorderStyle)
	cfg.ColorBorder = gui.Gray
	cfg.TextStyleHead = gui.CurrentTheme().B4
	cfg.MultiSelect = app.TableMultiSelect
	cfg.FreezeHeader = app.TableFreezeHeader
	cfg.Selected = app.TableSelected
	cfg.OnSelect = func(sel map[int]bool, _ int, _ *gui.Event, w *gui.Window) {
		gui.State[ShowcaseApp](w).TableSelected = sel
	}

	if cfg.BorderStyle == gui.TableBorderNone {
		alt := gui.RGBA(128, 128, 128, 20)
		cfg.ColorRowAlt = &alt
	}

	header := make([]gui.TableCellCfg, 0, len(cfg.Data[0].Cells))
	for idx, cell := range cfg.Data[0].Cells {
		col := idx + 1
		label := cell.Value
		switch app.TableSortBy {
		case col:
			label += "  ↓"
		case -col:
			label += " ↑"
		}
		header = append(header, gui.TableCellCfg{
			Value:    label,
			HeadCell: true,
			OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
				app := gui.State[ShowcaseApp](w)
				switch app.TableSortBy {
				case col:
					app.TableSortBy = -col
				case -col:
					app.TableSortBy = 0
				default:
					app.TableSortBy = col
				}
				e.IsHandled = true
			},
		})
	}
	cfg.Data[0] = gui.TR(header)

	selectedText := "none"
	if len(app.TableSelected) > 0 {
		parts := make([]string, 0, len(app.TableSelected))
		for idx := range app.TableSelected {
			if idx > 0 && idx < len(rows) {
				parts = append(parts, rows[idx][0])
			}
		}
		selectedText = strings.Join(parts, ", ")
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Spacing: gui.SomeF(20),
				Padding: gui.NoPadding,
				Content: []gui.View{
					gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
						IDFocus: 9108,
						Title:   "Border style",
						TitleBG: gui.CurrentTheme().ColorBackground,
						Value:   app.TableBorderStyle,
						Options: []gui.RadioOption{
							gui.NewRadioOption("All", "all"),
							gui.NewRadioOption("Horizontal", "horizontal"),
							gui.NewRadioOption("Header only", "header_only"),
							gui.NewRadioOption("None", "none"),
						},
						OnSelect: func(value string, w *gui.Window) {
							gui.State[ShowcaseApp](w).TableBorderStyle = value
						},
					}),
					gui.Column(gui.ContainerCfg{
						Padding:    gui.NoPadding,
						SizeBorder: gui.NoBorder,
						Spacing:    gui.SomeF(6),
						Content: []gui.View{
							gui.Toggle(gui.ToggleCfg{
								IDFocus:  9109,
								Label:    "Multi-select",
								Selected: app.TableMultiSelect,
								OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
									a := gui.State[ShowcaseApp](w)
									a.TableMultiSelect = !a.TableMultiSelect
									a.TableSelected = nil
								},
							}),
							gui.Toggle(gui.ToggleCfg{
								IDFocus:  9111,
								Label:    "Freeze header",
								Selected: app.TableFreezeHeader,
								OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
									gui.State[ShowcaseApp](w).TableFreezeHeader = !app.TableFreezeHeader
								},
							}),
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text: "Selected: " + selectedText,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Click a column header to sort. Scroll to see all rows.",
				TextStyle: gui.CurrentTheme().N5,
			}),
			w.Table(cfg),
		},
	})
}

func demoDataGrid(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	rows := showcaseDataGridApplyQuery(showcaseDataGridRows(), app.DataGridQuery)
	gridFeaturesStyle := gui.DefaultMarkdownStyle()
	gridFeaturesStyle.CodeHighlighter = highlight.Default()
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(10),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Simple controlled grid. Sort, filter, and select rows.",
				TextStyle: gui.CurrentTheme().N3,
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("Rows: %d  Selected: %d", len(rows), len(app.DataGridSelection.SelectedRowIDs)),
			}),
			w.DataGrid(gui.DataGridCfg{
				ID:                "catalog-data-grid",
				IDFocus:           9162,
				Sizing:            gui.Some(gui.FitFit),
				Columns:           showcaseDataGridColumns(),
				Rows:              rows,
				Query:             app.DataGridQuery,
				Selection:         app.DataGridSelection,
				Scrollbar:         gui.ScrollbarHidden,
				MaxHeight:         260,
				ShowQuickFilter:   true,
				ShowFilterRow:     true,
				ShowColumnChooser: true,
				OnQueryChange: func(query gui.GridQueryState, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).DataGridQuery = query
				},
				OnSelectionChange: func(selection gui.GridSelection, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).DataGridSelection = selection
				},
			}),
			w.Markdown(gui.MarkdownCfg{
				ID:     "catalog-data-grid-features",
				Source: showcaseDataGridFeaturesSource,
				Style:  gridFeaturesStyle,
			}),
		},
	})
}

func demoDataSource(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	if app.DataSource == nil {
		app.DataSource = gui.NewInMemoryDataSource(showcaseDataSourceRows())
	}
	stats := w.DataGridSourceStats("catalog-data-source")
	countText := "?"
	if stats.RowCount != nil {
		countText = fmt.Sprintf("%d", *stats.RowCount)
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(10),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("loading=%t  req=%d  rows=%d/%s", stats.Loading, stats.RequestCount, stats.ReceivedCount, countText),
			}),
			w.DataGrid(gui.DataGridCfg{
				ID:              "catalog-data-source",
				IDFocus:         9175,
				Sizing:          gui.Some(gui.FitFit),
				Columns:         showcaseDataSourceColumns(),
				DataSource:      app.DataSource,
				PaginationKind:  gui.GridPaginationCursor,
				PageLimit:       50,
				ShowQuickFilter: true,
				ShowCRUDToolbar: true,
				Query:           app.DataSourceQuery,
				Selection:       app.DataSourceSelection,
				MaxHeight:       260,
				OnQueryChange: func(query gui.GridQueryState, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).DataSourceQuery = query
				},
				OnSelectionChange: func(selection gui.GridSelection, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).DataSourceSelection = selection
				},
			}),
			gui.Text(gui.TextCfg{Text: "- DataGridDataSource interface for async backends"}),
			gui.Text(gui.TextCfg{Text: "- InMemoryDataSource for cursor and offset pagination"}),
			gui.Text(gui.TextCfg{Text: "- CRUD mutations and simulated loading states"}),
		},
	})
}

func demoTree(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(16),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Selected: " + treeSelectedText(app.TreeSelected),
				Mode: gui.TextModeWrap,
			}),
			gui.Text(gui.TextCfg{
				Text:      "Expand folders with the mouse or keyboard. The second tree enables virtualization, and the third simulates lazy loading.",
				TextStyle: gui.CurrentTheme().N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Text(gui.TextCfg{Text: "Basic tree", TextStyle: gui.CurrentTheme().B3}),
			gui.Tree(gui.TreeCfg{
				ID:       "showcase-tree-basic",
				IDFocus:  9180,
				Sizing:   gui.FillFit,
				OnSelect: showcaseTreeOnSelect,
				Nodes: []gui.TreeNodeCfg{
					{
						Text: "Mammals",
						Icon: gui.IconGithubAlt,
						Nodes: []gui.TreeNodeCfg{
							{Text: "Lion"},
							{Text: "Cat"},
							{Text: "Human", Icon: gui.IconUser},
						},
					},
					{
						Text: "Birds",
						Icon: gui.IconTwitter,
						Nodes: []gui.TreeNodeCfg{
							{Text: "Condor"},
							{
								Text: "Eagle",
								Nodes: []gui.TreeNodeCfg{
									{Text: "Bald"},
									{Text: "Golden"},
									{Text: "Sea"},
								},
							},
							{Text: "Parrot", Icon: gui.IconCage},
							{Text: "Robin"},
						},
					},
				},
			}),
			gui.Text(gui.TextCfg{Text: "Virtualized tree (scroll)", TextStyle: gui.CurrentTheme().B3}),
			gui.Tree(gui.TreeCfg{
				ID:        "showcase-tree-virtual",
				IDFocus:   9181,
				IDScroll:  9182,
				Sizing:    gui.FillFit,
				MaxHeight: 200,
				OnSelect:  showcaseTreeOnSelect,
				Nodes:     showcaseBigTreeNodes(),
			}),
			gui.Text(gui.TextCfg{Text: "Lazy-loading tree", TextStyle: gui.CurrentTheme().B3}),
			gui.Tree(gui.TreeCfg{
				ID:         "showcase-tree-lazy",
				IDFocus:    9183,
				Sizing:     gui.FillFit,
				OnSelect:   showcaseTreeOnSelect,
				OnLazyLoad: showcaseTreeOnLazyLoad,
				Nodes: []gui.TreeNodeCfg{
					{
						ID:    "remote_a",
						Text:  "Remote folder A",
						Icon:  gui.IconFolder,
						Lazy:  true,
						Nodes: append([]gui.TreeNodeCfg(nil), app.TreeLazyNodes["remote_a"]...),
					},
					{
						ID:    "remote_b",
						Text:  "Remote folder B",
						Icon:  gui.IconFolder,
						Lazy:  true,
						Nodes: append([]gui.TreeNodeCfg(nil), app.TreeLazyNodes["remote_b"]...),
					},
					{ID: "local_item", Text: "Local item"},
				},
			}),
		},
	})
}

func showcaseDataSourceColumns() []gui.GridColumnCfg {
	return []gui.GridColumnCfg{
		{ID: "name", Title: "Name", Width: gui.SomeF(180), Sortable: true, Filterable: true, Reorderable: true, Editable: true, DefaultValue: "New"},
		{ID: "team", Title: "Team", Width: gui.SomeF(140), Sortable: true, Filterable: true, Reorderable: true, Editable: true, Editor: gui.GridCellEditorSelect, EditorOptions: []string{"Core", "Data", "Platform", "R&D", "Web", "Security"}, DefaultValue: "Core"},
		{ID: "status", Title: "Status", Width: gui.SomeF(120), Sortable: true, Filterable: true, Reorderable: true, Editable: true, Editor: gui.GridCellEditorSelect, EditorOptions: []string{"Open", "Paused", "Closed"}, DefaultValue: "Open"},
	}
}

func showcaseDataGridColumns() []gui.GridColumnCfg {
	return []gui.GridColumnCfg{
		{ID: "name", Title: "Name", Width: gui.SomeF(180), Sortable: true, Filterable: true, Reorderable: true},
		{ID: "team", Title: "Team", Width: gui.SomeF(140), Sortable: true, Filterable: true, Reorderable: true},
		{ID: "status", Title: "Status", Width: gui.SomeF(120), Sortable: true, Filterable: true, Reorderable: true},
	}
}

func showcaseDataGridRows() []gui.GridRow {
	return []gui.GridRow{
		{ID: "1", Cells: map[string]string{"name": "Alex", "team": "Core", "status": "Active"}},
		{ID: "2", Cells: map[string]string{"name": "Mina", "team": "Data", "status": "Active"}},
		{ID: "3", Cells: map[string]string{"name": "Noah", "team": "Platform", "status": "Paused"}},
		{ID: "4", Cells: map[string]string{"name": "Priya", "team": "Core", "status": "Active"}},
		{ID: "5", Cells: map[string]string{"name": "Sam", "team": "Security", "status": "Offline"}},
	}
}

func showcaseDataGridApplyQuery(rows []gui.GridRow, query gui.GridQueryState) []gui.GridRow {
	filtered := make([]gui.GridRow, 0, len(rows))
	for _, row := range rows {
		if showcaseDataGridRowMatchesQuery(row, query) {
			filtered = append(filtered, row)
		}
	}
	for _, gridSort := range slices.Backward(query.Sorts) {
		slices.SortStableFunc(filtered, func(a, b gui.GridRow) int {
			va := a.Cells[gridSort.ColID]
			vb := b.Cells[gridSort.ColID]
			c := cmp.Compare(va, vb)
			if gridSort.Dir == gui.GridSortDesc {
				c = -c
			}
			return c
		})
	}
	return filtered
}

func showcaseDataGridRowMatchesQuery(row gui.GridRow, query gui.GridQueryState) bool {
	if query.QuickFilter != "" {
		needle := strings.ToLower(query.QuickFilter)
		match := false
		for _, value := range row.Cells {
			if strings.Contains(strings.ToLower(value), needle) {
				match = true
				break
			}
		}
		if !match {
			return false
		}
	}
	for _, filter := range query.Filters {
		if !strings.Contains(strings.ToLower(row.Cells[filter.ColID]), strings.ToLower(filter.Value)) {
			return false
		}
	}
	return true
}

func showcaseTableRows() [][]string {
	return [][]string{
		{"Name", "Role", "Team", "City"},
		{"Alex", "Designer", "Foundations", "Austin"},
		{"Riley", "Engineer", "Core UI", "Seattle"},
		{"Jordan", "PM", "Platform", "New York"},
		{"Sam", "QA", "Core UI", "Denver"},
		{"Priya", "Engineer", "Platform", "Chicago"},
		{"Noah", "Designer", "Growth", "Boston"},
		{"Mina", "Engineer", "Foundations", "San Diego"},
		{"Omar", "PM", "Growth", "Atlanta"},
		{"Lena", "Engineer", "Security", "Portland"},
		{"Kai", "Designer", "Platform", "Miami"},
		{"Ivy", "QA", "Growth", "Dallas"},
		{"Theo", "PM", "Core UI", "Phoenix"},
		{"Zara", "Engineer", "Foundations", "Nashville"},
		{"Ravi", "Designer", "Security", "Houston"},
		{"Eli", "QA", "Platform", "Detroit"},
	}
}

func showcaseTableRowsSorted(sortBy int) [][]string {
	rows := append([][]string(nil), showcaseTableRows()...)
	if sortBy == 0 || len(rows) <= 2 {
		return rows
	}
	isAscending := sortBy > 0
	idx := int(math.Abs(float64(sortBy))) - 1
	body := append([][]string(nil), rows[1:]...)
	for i := 0; i < len(body)-1; i++ {
		for j := i + 1; j < len(body); j++ {
			a := strings.ToLower(body[i][idx])
			b := strings.ToLower(body[j][idx])
			swap := (isAscending && a > b) || (!isAscending && a < b)
			if swap {
				body[i], body[j] = body[j], body[i]
			}
		}
	}
	return append([][]string{rows[0]}, body...)
}

func tableBorderStyleFromValue(value string) gui.TableBorderStyle {
	switch value {
	case "horizontal":
		return gui.TableBorderHorizontal
	case "header_only":
		return gui.TableBorderHeaderOnly
	case "none":
		return gui.TableBorderNone
	default:
		return gui.TableBorderAll
	}
}

func showcaseDataSourceRows() []gui.GridRow {
	names := []string{"Ada", "Grace", "Alan", "Katherine", "Barbara", "Linus", "Margaret", "Edsger"}
	teams := []string{"Core", "Data", "Platform", "R&D", "Web", "Security"}
	statuses := []string{"Open", "Paused", "Closed"}
	rows := make([]gui.GridRow, 0, 200)
	for i := range 200 {
		id := i + 1
		rows = append(rows, gui.GridRow{
			ID: fmt.Sprintf("%d", id),
			Cells: map[string]string{
				"name":   fmt.Sprintf("%s %d", names[i%len(names)], id),
				"team":   teams[(i/30)%len(teams)],
				"status": statuses[i%len(statuses)],
			},
		})
	}
	return rows
}

func showcaseBigTreeNodes() []gui.TreeNodeCfg {
	nodes := make([]gui.TreeNodeCfg, 0, 20)
	for i := range 20 {
		children := make([]gui.TreeNodeCfg, 0, 10)
		for j := range 10 {
			children = append(children, gui.TreeNodeCfg{
				ID:   fmt.Sprintf("group_%02d_item_%02d", i, j),
				Text: fmt.Sprintf("Item %d-%d", i, j),
			})
		}
		nodes = append(nodes, gui.TreeNodeCfg{
			ID:    fmt.Sprintf("group_%02d", i),
			Text:  fmt.Sprintf("Group %d", i),
			Icon:  gui.IconFolder,
			Nodes: children,
		})
	}
	return nodes
}

func showcaseTreeOnSelect(id string, _ *gui.Event, w *gui.Window) {
	gui.State[ShowcaseApp](w).TreeSelected = id
}

func showcaseTreeOnLazyLoad(_ string, nodeID string, w *gui.Window) {
	go func() {
		time.Sleep(800 * time.Millisecond)

		children := []gui.TreeNodeCfg{{ID: nodeID + "_empty", Text: "(empty)"}}
		switch nodeID {
		case "remote_a":
			children = []gui.TreeNodeCfg{
				{ID: "remote_a_alpha", Text: "alpha.txt"},
				{ID: "remote_a_beta", Text: "beta.txt"},
				{ID: "remote_a_gamma", Text: "gamma.txt"},
			}
		case "remote_b":
			children = []gui.TreeNodeCfg{
				{ID: "remote_b_one", Text: "one.rs"},
				{ID: "remote_b_two", Text: "two.rs"},
			}
		}

		w.QueueCommand(func(w *gui.Window) {
			gui.State[ShowcaseApp](w).TreeLazyNodes[nodeID] = children
			w.UpdateWindow()
		})
	}()
}

func treeSelectedText(selected string) string {
	if selected == "" {
		return "none"
	}
	return selected
}
