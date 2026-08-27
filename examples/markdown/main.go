// This example demonstrates an embedded markdown document in the markdown view.
// Markdown renders an embedded markdown document with the built-in
// markdown view.
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

//go:embed markdown_source.md
var markdownSource string

type App struct{}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
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

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()

	style := gui.DefaultMarkdownStyle()
	style.CodeBlockBG = gui.RGB(40, 44, 52)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: theme.PaddingLarge,

		Focusable:  true,
		ID:         "markdown-scroll",
		Scrollable: true,
		Content: []gui.View{
			w.Markdown(gui.MarkdownCfg{
				Source:     markdownSource,
				Style:      style,
				Mode:       gui.Some(gui.TextModeWrap),
				Color:      theme.ColorPanel,
				SizeBorder: gui.SomeF(1),
				Radius:     gui.SomeF(theme.RadiusMedium),
				Padding:    theme.PaddingMedium,
			}),
		},
	})
}
