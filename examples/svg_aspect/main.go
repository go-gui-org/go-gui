// Svg_aspect renders the same SVG with each preserveAspectRatio
// alignment in a wide rectangular tile so the slack distribution
// is visible. Toggles between "meet" (default, fits) and "slice"
// (fills with overflow clip).
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const sampleSvg = `<svg xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 100 100" preserveAspectRatio="%s %s">
	<rect width="100" height="100" fill="#1e293b"/>
	<circle cx="50" cy="50" r="40" fill="#facc15"/>
	<circle cx="50" cy="50" r="6" fill="#0f172a"/>
</svg>`

type App struct {
	Slice bool
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Width:  900,
		Height: 540,
		Title:  "preserveAspectRatio",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func aligns() []string {
	return []string{
		"xMinYMin", "xMidYMin", "xMaxYMin",
		"xMinYMid", "xMidYMid", "xMaxYMid",
		"xMinYMax", "xMidYMax", "xMaxYMax",
	}
}

func view(w *gui.Window) gui.View {
	app := gui.State[App](w)
	mode := "meet"
	if app.Slice {
		mode = "slice"
	}
	rows := []gui.View{
		gui.Row(gui.ContainerCfg{
			Padding: gui.Some(gui.PaddingTwoFive),
			Sizing:  gui.FillFit,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text: fmt.Sprintf("Mode: %s — click to toggle", mode),
				}),
			},
			OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
				gui.State[App](w).Slice = !gui.State[App](w).Slice
			},
		}),
	}

	all := aligns()
	for r := range 3 {
		cells := []gui.View{}
		for c := range 3 {
			a := all[r*3+c]
			data := fmt.Sprintf(sampleSvg, a, mode)
			cells = append(cells, gui.Column(gui.ContainerCfg{
				Padding: gui.Some(gui.PaddingTwoFive),
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						SvgData: data, Sizing: gui.FixedFixed,
						Width: 220, Height: 100,
					}),
					gui.Text(gui.TextCfg{Text: a}),
				},
			}))
		}
		rows = append(rows, gui.Row(gui.ContainerCfg{
			Sizing: gui.FillFit, Content: cells,
		}))
	}
	return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, Content: rows})
}
