// Svg_a11y demonstrates SVG accessibility metadata parsing — the
// <title>, <desc>, aria-label, aria-roledescription, and
// aria-hidden values surface on SvgParsed.A11y after LoadSvg.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
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

func main() {
	gui.SetTheme(gui.ThemeDarkBordered)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  640,
		Height: 360,
		Title:  "SVG A11y Metadata",
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	cached, err := w.LoadSvg(sampleSvg, 200, 200)
	var meta string
	switch {
	case err != nil:
		meta = fmt.Sprintf("LoadSvg error: %v", err)
	case cached.Parsed == nil:
		meta = "(no Parsed metadata)"
	default:
		a := cached.Parsed.A11y
		meta = fmt.Sprintf(
			"Title: %s\nDesc: %s\naria-label: %s\n"+
				"aria-roledescription: %s\naria-hidden: %v",
			a.Title, a.Desc, a.AriaLabel, a.AriaRoleDesc, a.AriaHidden)
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.Some(gui.PaddingTwoFive),
		Content: []gui.View{
			gui.Svg(gui.SvgCfg{
				SvgData: sampleSvg, Sizing: gui.FixedFixed,
				Width: 200, Height: 200,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				Padding: gui.Some(gui.PaddingTwoFive),
				Content: []gui.View{
					gui.Text(gui.TextCfg{Text: meta}),
				},
			}),
		},
	})
}
