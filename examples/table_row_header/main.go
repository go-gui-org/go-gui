// This example shows the Table row header column (#1156) driven the way
// a WinForms DataGridView port drives it: the app owns the current row.
//
// The table is not focusable. The "Marker up" and "Marker down" buttons
// stand in for an app's own keyboard navigation and move
// TableCfg.CurrentRow. The "Marker" radio group picks the glyph
// (TableCfg.RowHeaderMarker). A large TextStyle sets a tall row, and a small
// RowHeaderTextStyle keeps the ">" marker small and centred in it.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

// tableData is the header row plus five data rows. Row 0 is the header,
// so data rows are 1 to 5.
var tableData = [][]string{
	{"TrxID", "Status"},
	{"1710178", "ok"},
	{"1710177", "ok"},
	{"1710176", "fail"},
	{"1710175", "ok"},
	{"1710174", "ok"},
}

// markerOptions are the glyphs the radio group offers for
// TableCfg.RowHeaderMarker.
var markerOptions = []gui.RadioOption{
	gui.NewRadioOption("Chevron  >", ">"),
	gui.NewRadioOption("Triangle  ▶", "▶"),
	gui.NewRadioOption("Smiley  😀", "😀"),
}

// App holds the window state. Current is the cfg.Data index that shows
// the row header marker; Marker is the glyph it shows.
type App struct {
	Selected map[int]bool
	Marker   string
	Current  int
}

func main() {
	w := gui.SimpleWindow("Table Row Header", 520, 420,
		&App{Current: 1, Marker: ">", Selected: map[int]bool{}},
		func(w *gui.Window) { w.SetView(mainView) })
	backend.Run(w)
}

// moveCurrent moves the marker by d rows. It stops one step past each
// end, at -1 and at len(tableData), so the "no marker" case can be seen
// without the counter running away. Row 0 is the header row: it shows
// the corner, never the marker.
func moveCurrent(app *App, d int) {
	app.Current = min(max(app.Current+d, -1), len(tableData))
}

// mainView builds the UI from the current state. It runs again for each
// frame.
func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()
	// A large body style sets the row height; the row header glyphs take
	// a smaller one so the marker does not grow with it.
	body := theme.TextStyleDef
	body.Size *= 1.7
	marker := theme.TextStyleDef
	marker.Size *= 0.75

	cfg := gui.TableCfgFromData(tableData)
	cfg.ID = "trx-table"
	cfg.RowHeader = true
	cfg.MultiSelect = true
	cfg.BorderStyle = gui.TableBorderAll
	cfg.SizeBorder = 1
	cfg.TextStyle = body
	cfg.RowHeaderTextStyle = marker
	cfg.RowHeaderMarker = app.Marker
	cfg.CurrentRow = gui.Some(app.Current)
	cfg.Selected = app.Selected
	cfg.OnSelect = func(sel map[int]bool, _ int, ctx gui.EventCtx) {
		gui.State[App](ctx.Window).Selected = sel
	}

	move := func(d int) func(gui.EventCtx) {
		return func(ctx gui.EventCtx) {
			moveCurrent(gui.State[App](ctx.Window), d)
			ctx.Consume()
		}
	}
	button := func(id, label string, d int) gui.View {
		return gui.Button(gui.ButtonCfg{
			ID:      id,
			OnClick: move(d),
			Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PadAll(gui.PadMedium),
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{
				SizeBorder: gui.NoBorder,
				VAlign:     gui.VAlignMiddle,
				Content: []gui.View{
					button("marker-up", "Marker up", -1),
					button("marker-down", "Marker down", 1),
					gui.Label(fmt.Sprintf("CurrentRow = %d", app.Current),
						gui.TextStyle{}),
				},
			}),
			gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
				ID:      "marker-choice",
				Title:   "Marker",
				Value:   app.Marker,
				Options: markerOptions,
				OnSelect: func(v string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Marker = v
					ctx.Consume()
				},
			}),
			gui.Table(cfg),
		},
	})
}
