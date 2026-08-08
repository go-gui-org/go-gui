// Cursor demo shows every mouse cursor shape the backends support.
// Hover a cell to change the cursor; hover empty space to reset it.
// On Linux the two diagonal resize shapes come from the desktop's
// cursor theme (issue #47); elsewhere they are platform cursors.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct {
	Hovered string // label of the hovered cell, "" = default
}

type cursorCell struct {
	label string
	set   func(*gui.Window)
}

// The 11 MouseCursor values include CursorDefault, which renders the
// same arrow as CursorArrow, so only the 10 distinct shapes appear.
var cursorCells = []cursorCell{
	{"Arrow", (*gui.Window).SetMouseCursorArrow},
	{"I-Beam", (*gui.Window).SetMouseCursorIBeam},
	{"Crosshair", (*gui.Window).SetMouseCursorCrosshair},
	{"Hand", (*gui.Window).SetMouseCursorPointingHand},
	{"Resize E/W", (*gui.Window).SetMouseCursorEW},
	{"Resize N/S", (*gui.Window).SetMouseCursorNS},
	{"Resize NW-SE", (*gui.Window).SetMouseCursorResizeNWSE},
	{"Resize NE-SW", (*gui.Window).SetMouseCursorResizeNESW},
	{"Resize All", (*gui.Window).SetMouseCursorAll},
	{"Not Allowed", (*gui.Window).SetMouseCursorNotAllowed},
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "Cursor Demo",
		Width:  900,
		Height: 420,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	cells := make([]gui.View, 0, len(cursorCells))
	for _, cc := range cursorCells {
		cell := cc // capture per iteration
		cells = append(cells, gui.Row(gui.ContainerCfg{
			ID:         "cursor." + cell.label,
			Width:      160,
			Height:     56,
			Sizing:     gui.FixedFixed,
			SizeBorder: gui.SomeF(1),
			Radius:     gui.SomeF(8),
			HAlign:     gui.HAlignCenter,
			VAlign:     gui.VAlignMiddle,
			Padding:    gui.NoPadding,
			OnHover: func(ctx gui.EventCtx) {
				cell.set(ctx.Window)
				app := gui.State[App](ctx.Window)
				if app.Hovered != cell.label {
					app.Hovered = cell.label
					ctx.Window.UpdateWindow()
				}
			},
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: cell.label, TextStyle: theme.B3}),
			},
		}))
	}

	row := func(views []gui.View) gui.View {
		return gui.Row(gui.ContainerCfg{
			Sizing:     gui.FitFit,
			SizeBorder: gui.NoBorder,
			Padding:    gui.NoPadding,
			Spacing:    gui.SomeF(12),
			Content:    views,
		})
	}

	status := "default"
	if app.Hovered != "" {
		status = app.Hovered
	}

	// The root column's OnHover fires whenever no cell contains the
	// mouse (hover dispatch stops at the first handled shape), giving
	// clean leave semantics for the cursor reset.
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.SomeP(16, 16, 16, 16),
		Spacing: gui.SomeF(12),
		OnHover: func(ctx gui.EventCtx) {
			ctx.Window.SetMouseCursorArrow()
			app := gui.State[App](ctx.Window)
			if app.Hovered != "" {
				app.Hovered = ""
				ctx.Window.UpdateWindow()
			}
		},
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Cursor Demo - hover a cell to change the cursor shape",
				TextStyle: theme.B1,
			}),
			gui.Text(gui.TextCfg{
				Text:      "On Linux the diagonal resize cursors come from the desktop cursor theme.",
				TextStyle: theme.M3,
			}),
			row(cells[:5]),
			row(cells[5:]),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				SizeBorder: gui.NoBorder,
				Padding:    gui.NoPadding,
				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text:      "Current: " + status,
						TextStyle: theme.M2,
					}),
				},
			}),
		},
	})
}
