// Svg_hittest demonstrates TessellatedPath.ContainsPoint by
// reporting which authored path the cursor sits inside on click.
// SvgCfg.OnClick gives shape-relative MouseX/MouseY in display
// space; convert to viewBox coords using cached.Scale + ViewBoxX/Y.
package main

import (
	"fmt"
	"slices"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const sampleSvg = `<svg xmlns="http://www.w3.org/2000/svg"
	viewBox="0 0 200 200">
	<circle cx="60" cy="60" r="40" fill="#0ea5e9"/>
	<rect x="100" y="20" width="80" height="60" fill="#f59e0b"/>
	<polygon points="40,180 100,110 160,180" fill="#10b981"/>
	<rect x="80" y="130" width="40" height="40" fill="#ef4444"/>
</svg>`

const svgSize = 400

type App struct {
	Last string
}

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Last: "Click a shape."},
		Width:  720,
		Height: 480,
		Title:  "Svg Hit Test",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func hitLabel(cached *gui.CachedSvg, localX, localY float32) string {
	if cached == nil || cached.Parsed == nil {
		return "(no cache)"
	}
	scale := cached.Scale
	if scale == 0 {
		scale = 1
	}
	vx := cached.ViewBoxX + localX/scale
	vy := cached.ViewBoxY + localY/scale
	for i := range slices.Backward(cached.Parsed.Paths) {
		p := &cached.Parsed.Paths[i]
		if p.ContainsPoint(vx, vy) {
			return fmt.Sprintf(
				"PathID=%d (idx %d) at viewBox (%.1f,%.1f)",
				p.PathID, i, vx, vy)
		}
	}
	return fmt.Sprintf("(empty) at viewBox (%.1f,%.1f)", vx, vy)
}

func view(w *gui.Window) gui.View {
	app := gui.State[App](w)
	cached, _ := w.LoadSvg(sampleSvg, svgSize, svgSize)

	return gui.Row(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Padding: gui.Some(gui.PaddingTwoFive),
				Sizing:  gui.FitFit,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						SvgData: sampleSvg, Sizing: gui.FixedFixed,
						Width: svgSize, Height: svgSize,
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							gui.State[App](w).Last = hitLabel(
								cached, e.MouseX, e.MouseY)
						},
					}),
				},
			}),
			gui.Column(gui.ContainerCfg{
				Padding: gui.Some(gui.PaddingTwoFive),
				Sizing:  gui.FillFill,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Click a shape."}),
					gui.Text(gui.TextCfg{Text: app.Last}),
				},
			}),
		},
	})
}
