// Markdown renders an embedded markdown document with the built-in
// markdown view.
package main

import (
	_ "embed"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

//go:embed markdown_source.md
var markdownSource string

type App struct{}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	// Enable external APIs for LaTeX math (codecogs.com) and
	// Mermaid diagram (kroki.io) rendering. Disabled by default
	// for privacy — enabling sends rendered content to these
	// third-party services. Use MarkdownCfg.DisableExternalAPIs
	// for per-view control, or MarkdownCfg.MathFetcher /
	// MarkdownCfg.MermaidFetcher to provide custom renderers.
	gui.SetMarkdownExternalAPIsEnabled(true)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Width:  600,
		Height: 600,
		Title:  "Markdown View",
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
		},
	})

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	theme := gui.CurrentTheme()

	style := gui.DefaultMarkdownStyle()
	style.CodeBlockBG = gui.RGB(40, 44, 52)

	return gui.Column(gui.ContainerCfg{
		Width:    float32(ww),
		Height:   float32(wh),
		Sizing:   gui.FixedFixed,
		Padding:  gui.Some(theme.PaddingLarge),
		IDFocus:  1,
		IDScroll: 1,
		Content: []gui.View{
			w.Markdown(gui.MarkdownCfg{
				Source:     markdownSource,
				Style:      style,
				Mode:       gui.Some(gui.TextModeWrap),
				Color:      theme.ColorPanel,
				SizeBorder: gui.SomeF(1),
				Radius:     gui.SomeF(theme.RadiusMedium),
				Padding:    gui.Some(theme.PaddingMedium),
			}),
		},
	})
}
