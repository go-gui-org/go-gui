// This example demonstrates hit testing: reporting which authored path the cursor sits inside on click.
// Svg_hittest demonstrates TessellatedPath.ContainsPoint by
// reporting which authored path the cursor sits inside on click.
// SvgCfg.OnClick gives shape-relative MouseX/MouseY in display
// space; convert to viewBox coords using cached.Scale + ViewBoxX/Y.
package main

import (
	"fmt"
	"slices"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
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
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Last: "Click a shape."},
		Width:  720,
		Height: 480,
		Title:  "Svg Hit Test",
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

func hitLabel(cached *gui.CachedSvg, localX, localY float32) string {
	if cached == nil || cached.Parsed == nil {
		return "(no cache)"
	}
	// Assumes uniform scale and a zero draw offset: the tile
	// renders the SVG at its natural aspect (400x400 over a
	// 200x200 viewBox), so one scale plus the viewBox origin
	// maps display space back to viewBox space. CachedSvg
	// exposes no rendered-bounds offset (letterbox-aware
	// geometry is not part of the Svg API), so a letterboxed
	// tile would need that API first. Stroke-only paths never
	// hit by framework design (gui/svg_hittest.go:18 keeps
	// ContainsPoint on fill tessellation), so use a shape's
	// fill path for hit-testing.
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
	_, loadErr := w.LoadSvg(sampleSvg, svgSize, svgSize)
	loadMsg := ""
	if loadErr != nil {
		loadMsg = fmt.Sprintf("LoadSvg error: %v", loadErr)
	}

	return gui.Row(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Padding: gui.PaddingTwoFive,

				Sizing: gui.FitFit,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						SvgData: sampleSvg, Sizing: gui.FixedFixed,
						Width: svgSize, Height: svgSize,
						// No ctx.Consume: no ancestor in this
						// tree handles OnClick, so nothing
						// to stop.
						OnClick: func(ctx gui.EventCtx) {
							// Re-load on the event window:
							// the cached value above is from
							// the generation frame and may be
							// stale by the time the click runs.
							fresh, err := ctx.Window.LoadSvg(sampleSvg, svgSize, svgSize)
							if err != nil {
								gui.State[App](ctx.Window).Last = fmt.Sprintf("LoadSvg error: %v", err)
								ctx.Window.InvalidateRender()
								return
							}
							gui.State[App](ctx.Window).Last = hitLabel(
								fresh, ctx.Event.MouseX, ctx.Event.MouseY)
							ctx.Window.InvalidateRender()
						},
					}),
				},
			}),
			gui.Column(gui.ContainerCfg{
				Padding: gui.PaddingTwoFive,

				Sizing: gui.FillFill,
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: "Click a shape."}),
					gui.Text(gui.TextCfg{Text: loadMsg}),
					gui.Text(gui.TextCfg{Text: app.Last}),
				},
			}),
		},
	})
}
