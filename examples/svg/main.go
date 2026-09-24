// This example demonstrates a split-view browser for several embedded SVG assets.
// Svg lets you browse several embedded SVG assets in a simple
// split-view viewer.
package main

import (
	_ "embed"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

//go:embed assets/drop_shadow_filter.svg
var dropShadowFilter string

//go:embed assets/gradient_logo.svg
var gradientLogo string

//go:embed assets/loading_spinner.svg
var loadingSpinner string

//go:embed assets/text_with_fonts.svg
var textWithFonts string

//go:embed assets/transparent_icon.svg
var transparentIcon string

//go:embed assets/sample_transparent.svg
var sampleTransparent string

//go:embed assets/sample_with_bg.svg
var sampleWithBg string

//go:embed assets/sample_landscape.svg
var sampleLandscape string

//go:embed assets/tiger.svg
var tiger string

//go:embed assets/red_green.svg
var redGreen string

type SvgViewerApp struct {
	Selected int
}

type svgEntry struct {
	Name string
	Data string
}

func svgEntries() []svgEntry {
	return []svgEntry{
		{"Drop Shadow Filter", dropShadowFilter},
		{"Gradient Logo", gradientLogo},
		{"Loading Spinner", loadingSpinner},
		{"Text with Fonts", textWithFonts},
		{"Transparent Icon", transparentIcon},
		{"Flower Transparent", sampleTransparent},
		{"Flower with BG", sampleWithBg},
		{"Landscape no BG", sampleLandscape},
		{"Tiger", tiger},
		{"Red Green", redGreen},
	}
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &SvgViewerApp{},
		Width:  600,
		Height: 400,
		Title:  "SVG Examples",
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
		OnEvent: func(e *gui.Event, w *gui.Window) {
			if e.Type == gui.EventKeyDown &&
				e.KeyCode == gui.KeyP &&
				e.Modifiers == gui.ModCtrl {
				job := gui.NewPrintJob()
				job.Title = "SVG Examples"
				w.RunPrintJob(job)
				e.IsHandled = true
			}
		},
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[SvgViewerApp](w)
	entries := svgEntries()

	return gui.Row(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			navPanel(w, entries, app.Selected),
			contentPanel(entries, app.Selected),
		},
	})
}

func navPanel(w *gui.Window, entries []svgEntry, selected int) gui.View {
	items := make([]gui.View, len(entries))

	for i, entry := range entries {
		leaf := gui.ScopeIDN("svg_nav", "row", i)
		// Hover look is re-picked at generation from the
		// last arranged frame (cf. buttonOnHover in
		// gui/view_button.go) instead of mutating
		// Shape.Color in OnHover with no leave-reset.
		color := gui.ColorTransparent
		if i == selected {
			color = gui.CurrentTheme().ColorActive
		} else if w.IsHovered(leaf) {
			color = gui.CurrentTheme().ColorHover
		}
		// Capture the current loop values for the click handler.
		idx := i
		name := entry.Name
		items[i] = gui.Row(gui.ContainerCfg{
			ID:      leaf,
			Color:   color,
			Padding: gui.PaddingTwoFive,

			Sizing: gui.FillFit,
			// No ctx.Consume: no ancestor in this tree
			// handles OnClick, so nothing to stop.
			OnClick: func(ctx gui.EventCtx) {
				gui.State[SvgViewerApp](ctx.Window).Selected = idx
				ctx.Window.InvalidateRender()
			},
			OnHover: func(ctx gui.EventCtx) {
				ctx.Window.SetMouseCursorPointingHand()
			},
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: name}),
			},
		})
	}

	return gui.Column(gui.ContainerCfg{
		ID:         "nav",
		Color:      gui.CurrentTheme().ColorPanel,
		SizeBorder: gui.NoBorder,
		Sizing:     gui.FitFill,
		Content:    items,
	})
}

func contentPanel(entries []svgEntry, selected int) gui.View {
	if selected < 0 || selected >= len(entries) {
		selected = 0
	}
	entry := entries[selected]
	return gui.Column(gui.ContainerCfg{
		ID:         "content",
		Color:      gui.CurrentTheme().ColorPanel,
		SizeBorder: gui.NoBorder,
		Sizing:     gui.FillFill,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Svg(gui.SvgCfg{SvgData: entry.Data, Sizing: gui.FillFill}),
		},
	})
}
