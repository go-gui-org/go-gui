// This example shows a Table with frozen leading columns (#1363): the
// row header and the first columns stay in place while the others scroll
// sideways.
//
// The buttons change TableCfg.FreezeColumns. The toggles switch
// FreezeRowHeader, which freezes the row header on its own, and
// FreezeHeader, which keeps the column header above the rows.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	tableCols = 12
	tableRows = 100

	// maxFreeze is as far as the buttons go. The window fits that many
	// frozen columns and still shows others.
	maxFreeze = 3
)

// App holds the window state.
type App struct {
	Selected        map[int]bool
	Data            []gui.TableRowCfg
	FreezeColumns   int
	FreezeRowHeader bool
	FreezeHeader    bool
}

func newApp() *App {
	return &App{
		Selected:      map[int]bool{},
		Data:          tableData(),
		FreezeColumns: 1,
		FreezeHeader:  true,
	}
}

func main() {
	w := gui.SimpleWindow("Table Freeze", 720, 480, newApp(),
		func(w *gui.Window) { w.SetView(mainView) })
	backend.Run(w)
}

// tableData returns the header row and the data rows.
func tableData() []gui.TableRowCfg {
	rows := make([]gui.TableRowCfg, 0, tableRows+1)
	for r := range tableRows + 1 {
		cells := make([]gui.TableCellCfg, 0, tableCols)
		for c := range tableCols {
			if r == 0 {
				cells = append(cells, gui.TH(fmt.Sprintf("Column %d", c+1)))
				continue
			}
			cells = append(cells, gui.TD(fmt.Sprintf("row %d, %d", r, c+1)))
		}
		rows = append(rows, gui.TR(cells))
	}
	return rows
}

// moveFreeze changes the number of frozen columns by d, in 0..maxFreeze.
func moveFreeze(app *App, d int) {
	app.FreezeColumns = min(max(app.FreezeColumns+d, 0), maxFreeze)
}

// mainView builds the UI from the current state. It runs again for each
// frame.
func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(gui.PadMedium),
		Content: []gui.View{
			gui.Wrap(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					freezeButton("freeze-less", "Fewer", -1),
					freezeButton("freeze-more", "More", 1),
					gui.Label(fmt.Sprintf("FreezeColumns = %d", app.FreezeColumns),
						gui.TextStyle{}),
					gui.Toggle(gui.ToggleCfg{
						ID: "freeze-row-header", Label: "FreezeRowHeader",
						Selected: app.FreezeRowHeader,
						OnClick: func(ctx gui.EventCtx) {
							a := gui.State[App](ctx.Window)
							a.FreezeRowHeader = !a.FreezeRowHeader
							ctx.Consume()
						},
					}),
					gui.Toggle(gui.ToggleCfg{
						ID: "freeze-header", Label: "FreezeHeader",
						Selected: app.FreezeHeader,
						OnClick: func(ctx gui.EventCtx) {
							a := gui.State[App](ctx.Window)
							a.FreezeHeader = !a.FreezeHeader
							ctx.Consume()
						},
					}),
				},
			}),
			gui.Table(gui.TableCfg{
				ID:              "wide-table",
				Data:            app.Data,
				Sizing:          gui.FillFill,
				Height:          380,
				BorderStyle:     gui.TableBorderHorizontal,
				SizeBorder:      1,
				RowHeader:       true,
				FreezeColumns:   app.FreezeColumns,
				FreezeRowHeader: app.FreezeRowHeader,
				FreezeHeader:    app.FreezeHeader,
				MultiSelect:     true,
				Selected:        app.Selected,
				OnSelect: func(sel map[int]bool, _ int, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Selected = sel
				},
			}),
		},
	})
}

func freezeButton(id, label string, d int) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      id,
		Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
		OnClick: func(ctx gui.EventCtx) {
			moveFreeze(gui.State[App](ctx.Window), d)
			ctx.Consume()
		},
	})
}
