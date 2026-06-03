// Svg_use_symbol demonstrates <use href="#id"> and <symbol>
// resolution. A single <symbol> is referenced multiple times via
// <use> with translate offsets and per-instance fill overrides; the
// rendered output is identical to the manually duplicated equivalent
// shown alongside.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const useDemo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 80">
	<defs>
		<symbol id="star">
			<polygon points="20,4 24,16 36,16 26,24 30,36 20,28 10,36 14,24 4,16 16,16"/>
		</symbol>
	</defs>
	<use href="#star" x="0"   y="20" fill="#fbbf24"/>
	<use href="#star" x="60"  y="20" fill="#10b981"/>
	<use href="#star" x="120" y="20" fill="#3b82f6"/>
	<use href="#star" x="180" y="20" fill="#a855f7"/>
</svg>`

const manualDemo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 80">
	<polygon points="20,24 24,36 36,36 26,44 30,56 20,48 10,56 14,44 4,36 16,36" fill="#fbbf24"/>
	<polygon points="80,24 84,36 96,36 86,44 90,56 80,48 70,56 74,44 64,36 76,36" fill="#10b981"/>
	<polygon points="140,24 144,36 156,36 146,44 150,56 140,48 130,56 134,44 124,36 136,36" fill="#3b82f6"/>
	<polygon points="200,24 204,36 216,36 206,44 210,56 200,48 190,56 194,44 184,36 196,36" fill="#a855f7"/>
</svg>`

const useElementDemo = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 100">
	<defs>
		<circle id="dot" cx="20" cy="20" r="14"/>
	</defs>
	<use href="#dot" x="0"   y="0"  fill="#ef4444"/>
	<use href="#dot" x="40"  y="0"  fill="#f59e0b"/>
	<use href="#dot" x="80"  y="0"  fill="#10b981"/>
	<use href="#dot" x="120" y="0"  fill="#3b82f6"/>
	<use href="#dot" x="0"   y="40" fill="#8b5cf6" transform="scale(1.4)"/>
	<use href="#dot" x="60"  y="40" fill="#ec4899" transform="rotate(15 80 60)"/>
</svg>`

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  720,
		Height: 520,
		Title:  "Use + Symbol",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	cell := func(title, data string) gui.View {
		return gui.Column(gui.ContainerCfg{
			Padding: gui.Some(gui.PaddingTwoFive),
			Sizing:  gui.FillFit,
			HAlign:  gui.HAlignCenter,
			Content: []gui.View{
				gui.Svg(gui.SvgCfg{
					SvgData: data, Sizing: gui.FixedFixed,
					Width: 360, Height: 120,
				}),
				gui.Text(gui.TextCfg{Text: title}),
			},
		})
	}
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			cell("<symbol> + <use> (4 instances)", useDemo),
			cell("Manually duplicated polygons", manualDemo),
			cell("<use> on <circle> with transform attrs", useElementDemo),
		},
	})
}
