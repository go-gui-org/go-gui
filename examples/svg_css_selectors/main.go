// Svg_css_selectors demonstrates v0.14.0 CSS selector additions:
// adjacent (`+`), general-sibling (`~`), and attribute selectors,
// plus :not(). Each tile renders the same path collection styled by
// a different selector strategy. The cascade is run at parse time;
// no runtime hover state is involved.
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
			"Adjacent (rect + circle)",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		circle  { fill: #94a3b8 }
		rect + circle { fill: #ef4444 }
	</style>
	<rect x="10" y="10" width="30" height="30" fill="#1e293b"/>
	<circle cx="60" cy="25" r="14"/>
	<circle cx="25" cy="70" r="14"/>
</svg>`,
		},
		{
			"General sibling (rect ~ circle)",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		circle { fill: #94a3b8 }
		rect ~ circle { fill: #10b981 }
	</style>
	<circle cx="20" cy="20" r="10"/>
	<rect x="35" y="10" width="20" height="20" fill="#1e293b"/>
	<circle cx="70" cy="25" r="10"/>
	<circle cx="50" cy="70" r="10"/>
</svg>`,
		},
		{
			"Attr equal ([data-state=active])",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		rect { fill: #475569 }
		rect[data-state=active] { fill: #f59e0b }
	</style>
	<rect x="10" y="10" width="20" height="20"/>
	<rect x="40" y="10" width="20" height="20" data-state="active"/>
	<rect x="70" y="10" width="20" height="20"/>
	<rect x="40" y="60" width="20" height="20" data-state="hover"/>
</svg>`,
		},
		{
			"Attr prefix ([data-kind^=hot])",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		circle { fill: #475569 }
		circle[data-kind^=hot] { fill: #ef4444 }
	</style>
	<circle cx="20" cy="50" r="14" data-kind="cold-blue"/>
	<circle cx="50" cy="50" r="14" data-kind="hot-red"/>
	<circle cx="80" cy="50" r="14" data-kind="hot-orange"/>
</svg>`,
		},
		{
			":not(.skip)",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		circle:not(.skip) { fill: #6366f1 }
		.skip             { fill: #475569 }
	</style>
	<circle cx="20" cy="50" r="14"/>
	<circle cx="50" cy="50" r="14" class="skip"/>
	<circle cx="80" cy="50" r="14"/>
</svg>`,
		},
		{
			"Compound: rect + [tag=ok]",
			`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		* { fill: #475569 }
		rect + [tag=ok] { fill: #22d3ee }
	</style>
	<rect x="5" y="40" width="20" height="20"/>
	<circle cx="40" cy="50" r="10" tag="ok"/>
	<rect x="55" y="40" width="20" height="20"/>
	<circle cx="90" cy="50" r="10" tag="other"/>
</svg>`,
		},
	}
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		Width:  720,
		Height: 540,
		Title:  "CSS Selectors (v0.14.0)",
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
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill, Content: rows,
	})
}
