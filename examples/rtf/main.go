// This example demonstrates rich text runs, links, abbreviations, and wrapping in the RTF widget.
// RTF demonstrates rich text runs, links, abbreviations, and
// wrapping inside the RTF widget.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

type App struct{}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "RTF Viewer",
		Width:  500,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
		},
		OnEvent: func(e *gui.Event, w *gui.Window) {
			if e.Type == gui.EventKeyDown &&
				e.KeyCode == gui.KeyP &&
				e.Modifiers == gui.ModCtrl {
				job := gui.NewPrintJob()
				job.Title = "RTF Viewer"
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
	t := gui.CurrentTheme()

	// Compose the document from styled runs so each feature is easy to spot.
	rt := gui.RichText{Runs: []gui.RichTextRun{
		gui.RichRun("Rich Text Demo", t.TextStyleDisplay),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("This is normal text. ", t.TextStyleBody),
		gui.RichRun("This is bold text. ", t.TextStyleTitleSmall),
		gui.RichRun("This is italic text. ", t.TextStyleBody.Italic()),
		gui.RichRun("This is bold-italic text.", t.TextStyleBody.Italic().Bold()),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Links are supported: ", t.TextStyleBody),
		gui.RichLink("Go Website", "https://go.dev", t.TextStyleBody),
		gui.RichRun(" and ", t.TextStyleBody),
		gui.RichLink("Go GUI Repo", "https://github.com/go-gui-org/go-gui", t.TextStyleBody),
		gui.RichRun(".", t.TextStyleBody),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Abbreviations show tooltips on hover: ", t.TextStyleBody),
		gui.RichAbbr("HTML", "HyperText Markup Language", t.TextStyleBody),
		gui.RichRun(" and ", t.TextStyleBody),
		gui.RichAbbr("CSS", "Cascading Style Sheets", t.TextStyleBody),
		gui.RichRun(".", t.TextStyleBody),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Long paragraphs wrap automatically when "+
			"TextModeWrap is enabled. This paragraph contains "+
			"enough text to demonstrate wrapping behavior in "+
			"the RTF widget. Resize the window to see how the "+
			"text reflows to fit the available width.", t.TextStyleBody),
	}}

	return gui.Column(gui.ContainerCfg{
		ID:         "rtf-scroll",
		Focusable:  true,
		Sizing:     gui.FillFill,
		Scrollable: true,
		Padding:    gui.PadAll(10),
		Content: []gui.View{
			gui.RTF(gui.RTFCfg{
				RichText:      rt,
				Mode:          gui.TextModeWrap,
				BaseTextStyle: &t.TextStyleBody,
			}),
		},
	})
}
