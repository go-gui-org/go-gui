// This example demonstrates preserveAspectRatio meet and slice alignment on SVG renders.
// Svg_aspect renders the same SVG with each preserveAspectRatio
// alignment in a wide rectangular tile so the slack distribution
// is visible. Toggles between "meet" (default, fits) and "slice"
// (fills with overflow clip).
package main

import (
	"fmt"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const sampleSvg = `<svg xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 100 100" preserveAspectRatio="%s %s">
	<rect width="100" height="100" fill="#1e293b"/>
	<circle cx="50" cy="50" r="40" fill="#facc15"/>
	<circle cx="50" cy="50" r="6" fill="#0f172a"/>
</svg>`

type App struct {
	Slice bool
	// cellSvgs holds the 9 prebuilt tile sources for the
	// current mode so the Sprintf runs on toggle, not per
	// frame. cellMode names the mode they were built for.
	cellSvgs []string
	cellMode string
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Width:  900,
		Height: 540,
		Title:  "preserveAspectRatio",
		OnInit: func(w *gui.Window) { w.SetView(view) },
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
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
			Padding: gui.PaddingTwoFive,

			Sizing: gui.FillFit,
			Content: []gui.View{
				gui.Text(gui.TextCfg{
					Text: fmt.Sprintf("Mode: %s — click to toggle", mode),
				}),
			},
			OnClick: func(ctx gui.EventCtx) {
				st := gui.State[App](ctx.Window)
				st.Slice = !st.Slice
				// No ctx.Consume: no ancestor in this tree
				// handles OnClick, so nothing to stop.
				ctx.Window.InvalidateRender()
			},
		}),
	}

	all := aligns()
	if app.cellMode != mode || len(app.cellSvgs) != len(all) {
		built := make([]string, len(all))
		for i, a := range all {
			built[i] = fmt.Sprintf(sampleSvg, a, mode)
		}
		app.cellSvgs = built
		app.cellMode = mode
	}
	for r := range 3 {
		cells := []gui.View{}
		for c := range 3 {
			n := r*3 + c
			if n >= len(all) || n >= len(app.cellSvgs) {
				continue
			}
			a := all[n]
			data := app.cellSvgs[n]
			cells = append(cells, gui.Column(gui.ContainerCfg{
				ID:      gui.ScopeIDN("svg_aspect", "tile", n),
				Padding: gui.PaddingTwoFive,

				Sizing: gui.FillFit,
				HAlign: gui.HAlignCenter,
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
