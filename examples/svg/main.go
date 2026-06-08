// Svg lets you browse several embedded SVG assets in a simple
// split-view viewer.
package main

import (
	_ "embed"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
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
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &SvgViewerApp{},
		Width:  600,
		Height: 400,
		Title:  "SVG Examples",
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
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

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	app := gui.State[SvgViewerApp](w)

	return gui.Row(gui.ContainerCfg{
		Width:  float32(ww),
		Height: float32(wh),
		Sizing: gui.FixedFixed,
		Content: []gui.View{
			navPanel(app.Selected),
			contentPanel(app.Selected),
		},
	})
}

func navPanel(selected int) gui.View {
	entries := svgEntries()
	items := make([]gui.View, len(entries))

	for i, entry := range entries {
		color := gui.ColorTransparent
		if i == selected {
			color = gui.CurrentTheme().ColorActive
		}
		// Capture the current loop values for the click handler.
		idx := i
		name := entry.Name
		items[i] = gui.Row(gui.ContainerCfg{
			Color:   color,
			Padding: gui.Some(gui.PaddingTwoFive),
			Sizing:  gui.FillFit,
			OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
				gui.State[SvgViewerApp](w).Selected = idx
			},
			OnHover: func(layout *gui.Layout, _ *gui.Event, w *gui.Window) {
				w.SetMouseCursorPointingHand()
				layout.Shape.Color = gui.CurrentTheme().ColorHover
			},
			Content: []gui.View{
				gui.Text(gui.TextCfg{Text: name}),
			},
		})
	}

	return gui.Column(gui.ContainerCfg{
		ID:      "nav",
		Color:   gui.CurrentTheme().ColorPanel,
		Sizing:  gui.FitFill,
		Content: items,
	})
}

func contentPanel(selected int) gui.View {
	entry := svgEntries()[selected]
	return gui.Column(gui.ContainerCfg{
		ID:     "content",
		Color:  gui.CurrentTheme().ColorPanel,
		Sizing: gui.FillFill,
		HAlign: gui.HAlignCenter,
		VAlign: gui.VAlignMiddle,
		Content: []gui.View{
			gui.Svg(gui.SvgCfg{SvgData: entry.Data, Sizing: gui.FillFill}),
		},
	})
}
