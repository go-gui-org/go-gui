// The data grid data source example shows a large grid backed by
// a paged in-memory data source.
package main

import (
	"fmt"
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	datagrid "github.com/go-gui-org/go-gui/gui/datagrid"
)

type App struct {
	Selection       datagrid.GridSelection
	Source          datagrid.DataGridDataSource
	Query           datagrid.GridQueryState
	LastAction      string
	AllRows         []datagrid.GridRow
	Columns         []datagrid.GridColumnCfg
	UseOffset       bool
	SimulateLatency bool
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{SimulateLatency: true},
		Title:  "Data Grid Data Source Demo",
		Width:  1240,
		Height: 760,
		OnInit: func(w *gui.Window) {
			app := gui.State[App](w)
			// Build the large row set once, then switch paging modes on demand.
			app.AllRows = makeRows(50000)
			app.Columns = makeColumns()
			rebuildSource(app)
			w.UpdateView(mainView)
		},
	})
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[App](w)
	stats := datagrid.GetSourceStats(w, "source-grid")
	theme := gui.CurrentTheme()

	mode := "cursor"
	if app.UseOffset {
		mode = "offset"
	}
	loading := "no"
	if stats.Loading {
		loading = "yes"
	}
	countText := "?"
	if stats.RowCount != nil {
		countText = strconv.Itoa(*stats.RowCount)
	}

	paginationKind := datagrid.GridPaginationCursor
	if app.UseOffset {
		paginationKind = datagrid.GridPaginationOffset
	}

	return gui.Column(gui.ContainerCfg{
		Width:   float32(ww),
		Height:  float32(wh),
		Sizing:  gui.FixedFixed,
		Padding: gui.Some(theme.PaddingSmall),
		Spacing: gui.Some(theme.SpacingSmall),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Data Source Demo (50k rows)",
				TextStyle: theme.B2,
			}),
			gui.Row(gui.ContainerCfg{
				VAlign:  gui.VAlignMiddle,
				Sizing:  gui.FillFit,
				Spacing: gui.Some(theme.SpacingSmall),
				Content: []gui.View{
					gui.Switch(gui.SwitchCfg{
						ID:       "dgds_use_offset",
						Label:    "Use offset pagination",
						Selected: app.UseOffset,
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							state := gui.State[App](w)
							state.UseOffset = !state.UseOffset
							rebuildSource(state)
							e.IsHandled = true
						},
					}),
					gui.Switch(gui.SwitchCfg{
						ID:       "dgds_slow_mode",
						Label:    "Simulate latency",
						Selected: app.SimulateLatency,
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							state := gui.State[App](w)
							state.SimulateLatency = !state.SimulateLatency
							rebuildSource(state)
							e.IsHandled = true
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf(
					"mode=%s loading=%s req=%d cancel=%d stale=%d rows=%d/%s %s",
					mode, loading, stats.RequestCount, stats.CancelledCount,
					stats.StaleDropCount, stats.ReceivedCount, countText, app.LastAction,
				),
				TextStyle: theme.N4,
			}),
			datagrid.New(w, datagrid.DataGridCfg{
				ID:              "source-grid",
				MaxHeight:       620,
				ShowCRUDToolbar: true,
				ShowQuickFilter: true,
				Columns:         app.Columns,
				DataSource:      app.Source,
				PaginationKind:  paginationKind,
				PageLimit:       220,
				Query:           app.Query,
				Selection:       app.Selection,
				OnQueryChange: func(query datagrid.GridQueryState, _ *gui.Event, w *gui.Window) {
					gui.State[App](w).Query = query
				},
				OnSelectionChange: func(selection datagrid.GridSelection, _ *gui.Event, w *gui.Window) {
					gui.State[App](w).Selection = selection
				},
				OnCellEdit: func(edit datagrid.GridCellEdit, _ *gui.Event, w *gui.Window) {
					gui.State[App](w).LastAction = fmt.Sprintf("Edited %s.%s", edit.RowID, edit.ColID)
				},
				OnCRUDError: func(msg string, _ *gui.Event, w *gui.Window) {
					gui.State[App](w).LastAction = "CRUD error: " + msg
				},
			}),
		},
	})
}

func rebuildSource(app *App) {
	latency := 0
	if app.SimulateLatency {
		latency = 140
	}
	// Recreate the source so pagination mode and latency toggles take effect.
	app.Source = &datagrid.InMemoryDataSource{
		Rows:           app.AllRows,
		DefaultLimit:   220,
		LatencyMs:      latency,
		SupportsCursor: !app.UseOffset,
	}
}

func makeColumns() []datagrid.GridColumnCfg {
	return []datagrid.GridColumnCfg{
		{
			ID:           "name",
			Title:        "Name",
			Width:        gui.SomeF(180),
			Editable:     true,
			DefaultValue: "New User",
		},
		{
			ID:            "team",
			Title:         "Team",
			Width:         gui.SomeF(140),
			Editable:      true,
			Editor:        datagrid.GridCellEditorSelect,
			EditorOptions: []string{"Core", "Data", "Platform", "R&D", "Web", "Security"},
			DefaultValue:  "Core",
		},
		{
			ID:           "email",
			Title:        "Email",
			Width:        gui.SomeF(250),
			Editable:     true,
			DefaultValue: "new@grid.dev",
		},
		{
			ID:            "status",
			Title:         "Status",
			Width:         gui.SomeF(120),
			Editable:      true,
			Editor:        datagrid.GridCellEditorSelect,
			EditorOptions: []string{"Open", "Paused", "Closed"},
			DefaultValue:  "Open",
		},
		{
			ID:           "active",
			Title:        "Active",
			Width:        gui.SomeF(90),
			Editable:     true,
			Editor:       datagrid.GridCellEditorCheckbox,
			DefaultValue: "true",
		},
		{
			ID:           "score",
			Title:        "Score",
			Width:        gui.SomeF(110),
			Align:        gui.HAlignEnd,
			Editable:     true,
			DefaultValue: "70",
		},
		{
			ID:           "start",
			Title:        "Start",
			Width:        gui.SomeF(130),
			Editable:     true,
			Editor:       datagrid.GridCellEditorDate,
			DefaultValue: "1/1/2026",
		},
	}
}

func makeRows(count int) []datagrid.GridRow {
	names := []string{"Ada", "Grace", "Alan", "Katherine", "Barbara", "Linus", "Margaret", "Edsger"}
	teams := []string{"Core", "Data", "Platform", "R&D", "Web", "Security"}
	statuses := []string{"Open", "Paused", "Closed"}
	startDates := []string{"1/12/2026", "2/5/2026", "3/18/2026", "4/22/2026", "5/9/2026"}

	rows := make([]datagrid.GridRow, 0, count)
	for i := range count {
		id := i + 1
		active := "false"
		if i%2 == 0 {
			active = "true"
		}
		rows = append(rows, datagrid.GridRow{
			ID: strconv.Itoa(id),
			Cells: map[string]string{
				"name":   names[i%len(names)] + " " + strconv.Itoa(id),
				"team":   teams[(i/300)%len(teams)],
				"email":  "user" + strconv.Itoa(id) + "@grid.dev",
				"status": statuses[i%len(statuses)],
				"active": active,
				"score":  strconv.Itoa(60 + ((i * 7) % 41)),
				"start":  startDates[i%len(startDates)],
			},
		})
	}
	return rows
}
