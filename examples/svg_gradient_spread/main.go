// Svg_gradient_spread demonstrates spreadMethod="pad|reflect|repeat"
// on linear and radial gradients. The same gradient is rendered with
// each spread mode in a 2x3 grid so the falloff differences are
// directly comparable. Stops are placed at 0..0.4 to leave headroom
// for reflect/repeat to show the wrap.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const linearTpl = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<linearGradient id="g" x1="0%%" y1="0%%" x2="40%%" y2="0%%"
			spreadMethod="%s">
			<stop offset="0" stop-color="#0ea5e9"/>
			<stop offset="1" stop-color="#a855f7"/>
		</linearGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`

const radialTpl = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<defs>
		<radialGradient id="g" cx="50%%" cy="50%%" r="20%%"
			spreadMethod="%s">
			<stop offset="0" stop-color="#fde68a"/>
			<stop offset="1" stop-color="#7c2d12"/>
		</radialGradient>
	</defs>
	<rect width="100" height="100" fill="url(#g)"/>
</svg>`

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		Width:  720,
		Height: 540,
		Title:  "Gradient spreadMethod",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	modes := []string{"pad", "reflect", "repeat"}
	cell := func(title, data string) gui.View {
		return gui.Column(gui.ContainerCfg{
			Padding: gui.Some(gui.PaddingTwoFive),
			Sizing:  gui.FillFit,
			HAlign:  gui.HAlignCenter,
			Content: []gui.View{
				gui.Svg(gui.SvgCfg{
					SvgData: data, Sizing: gui.FixedFixed,
					Width: 180, Height: 180,
				}),
				gui.Text(gui.TextCfg{Text: title}),
			},
		})
	}
	linearRow := []gui.View{}
	radialRow := []gui.View{}
	for _, m := range modes {
		linearRow = append(linearRow,
			cell("linear "+m, fmt.Sprintf(linearTpl, m)))
		radialRow = append(radialRow,
			cell("radial "+m, fmt.Sprintf(radialTpl, m)))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Row(gui.ContainerCfg{Sizing: gui.FillFit, Content: linearRow}),
			gui.Row(gui.ContainerCfg{Sizing: gui.FillFit, Content: radialRow}),
		},
	})
}
