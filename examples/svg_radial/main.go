// Svg_radial demonstrates radialGradient parsing and rendering. A
// 2x3 grid renders the same shape filled with: linear gradient,
// centered radial, off-center radial, large-R radial,
// multi-stop radial, and a shifted-focal radial.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type sample struct {
	Title string
	Data  string
}

func samples() []sample {
	return []sample{
		{
			"Linear baseline",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<linearGradient id="g">
			<stop offset="0" stop-color="#0ea5e9"/>
			<stop offset="1" stop-color="#a855f7"/>
		</linearGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
		{
			"Radial centered",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g">
			<stop offset="0" stop-color="#fde68a"/>
			<stop offset="1" stop-color="#7c2d12"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
		{
			"Radial off-center",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g" cx="25%" cy="25%" r="60%">
			<stop offset="0" stop-color="#ffffff"/>
			<stop offset="1" stop-color="#1e3a8a"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
		{
			"Radial large R",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g" r="80%">
			<stop offset="0" stop-color="#10b981"/>
			<stop offset="1" stop-color="#0f172a"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
		{
			"Multi-stop radial",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g">
			<stop offset="0" stop-color="#fef3c7"/>
			<stop offset="0.4" stop-color="#fb923c"/>
			<stop offset="0.7" stop-color="#b91c1c"/>
			<stop offset="1" stop-color="#1e1b4b"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
		{
			"Focal shifted",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g" cx="50%" cy="50%" r="50%"
			fx="20%" fy="20%">
			<stop offset="0" stop-color="#fafafa"/>
			<stop offset="1" stop-color="#475569"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`,
		},
	}
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  720,
		Height: 540,
		Title:  "Radial Gradients",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	rows := []gui.View{}
	all := samples()
	for r := range 2 {
		cells := []gui.View{}
		for c := range 3 {
			s := all[r*3+c]
			cells = append(cells, gui.Column(gui.ContainerCfg{
				Padding: gui.Some(gui.PaddingTwoFive),
				Sizing:  gui.FillFit,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						SvgData: s.Data, Sizing: gui.FixedFixed,
						Width: 180, Height: 180,
					}),
					gui.Text(gui.TextCfg{Text: s.Title}),
				},
			}))
		}
		rows = append(rows, gui.Row(gui.ContainerCfg{
			Sizing: gui.FillFit, Content: cells,
		}))
	}
	return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, Content: rows})
}
