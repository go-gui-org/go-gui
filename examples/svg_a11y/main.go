// This example demonstrates SVG accessibility metadata parsing (title, description, aria attributes).
// Svg_a11y demonstrates SVG accessibility metadata parsing — the
// <title>, <desc>, aria-label, aria-roledescription, and
// aria-hidden values surface on SvgParsed.A11y after LoadSvg.
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

const sampleSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"
	aria-label="Search" aria-roledescription="icon" aria-hidden="false">
	<title>Search Magnifier</title>
	<desc>A circle attached to a diagonal line, evoking a magnifying
 	glass. Used to indicate search functionality.</desc>
	<circle cx="10" cy="10" r="6" fill="none" stroke="#3b82f6"
		stroke-width="2"/>
	<line x1="14.5" y1="14.5" x2="20" y2="20"
		stroke="#3b82f6" stroke-width="2" stroke-linecap="round"/>
</svg>`

// App caches the parsed a11y metadata text. sampleSvg is
// constant, so the text is built once on the first frame.
type App struct {
	Meta   string
	MetaOK bool
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  640,
		Height: 360,
		Title:  "SVG A11y Metadata",
		State:  &App{},
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

func view(w *gui.Window) gui.View {
	app := gui.State[App](w)
	// The sample SVG is constant, so its metadata never
	// changes: parse it once and keep the text. The Svg
	// widget below does its own LoadSvg every frame (a
	// cache hit after the first tessellation), which is
	// the single per-frame loading path.
	if !app.MetaOK {
		cached, err := w.LoadSvg(sampleSvg, 200, 200)
		switch {
		case err != nil:
			app.Meta = fmt.Sprintf("LoadSvg error: %v", err)
		case cached.Parsed == nil:
			app.Meta = "(no Parsed metadata)"
		default:
			a := cached.Parsed.A11y
			app.Meta = fmt.Sprintf(
				"Title: %s\nDesc: %s\naria-label: %s\n"+
					"aria-roledescription: %s\naria-hidden: %v",
				a.Title, a.Desc, a.AriaLabel, a.AriaRoleDesc, a.AriaHidden)
		}
		app.MetaOK = true
	}
	return gui.Row(gui.ContainerCfg{
		ID:      gui.ScopeID("svg_a11y_view", "row"),
		Sizing:  gui.FillFill,
		Padding: gui.PaddingTwoFive,

		Content: []gui.View{
			gui.Svg(gui.SvgCfg{
				ID: gui.ScopeID("svg_a11y_view", "graphic"),
				A11YCfg: gui.A11YCfg{
					A11YLabel:       "Search magnifier icon",
					A11YDescription: "A circle attached to a diagonal line, evoking a magnifying glass.",
				},
				SvgData: sampleSvg, Sizing: gui.FixedFixed,
				Width: 200, Height: 200,
			}),
			gui.Column(gui.ContainerCfg{
				ID:      gui.ScopeID("svg_a11y_view", "meta"),
				Sizing:  gui.FillFill,
				Padding: gui.PaddingTwoFive,

				Content: []gui.View{
					gui.Text(gui.TextCfg{
						Text: app.Meta,
					}),
				},
			}),
		},
	})
}
