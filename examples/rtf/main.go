// Rtf demonstrates rich text runs, links, abbreviations, and
// wrapping inside the RTF widget.
package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type App struct{}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "RTF Viewer",
		Width:  500,
		Height: 400,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
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

	backend.Run(w)
}

func mainView(w *gui.Window) gui.View {
	ww, wh := w.WindowSize()
	t := gui.CurrentTheme()

	// Compose the document from styled runs so each feature is easy to spot.
	rt := gui.RichText{Runs: []gui.RichTextRun{
		gui.RichRun("Rich Text Demo", t.B1),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("This is normal text. ", t.N3),
		gui.RichRun("This is bold text. ", t.B3),
		gui.RichRun("This is italic text. ", t.I3),
		gui.RichRun("This is bold-italic text.", t.BI3),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Links are supported: ", t.N3),
		gui.RichLink("Go Website", "https://go.dev", t.N3),
		gui.RichRun(" and ", t.N3),
		gui.RichLink("Go GUI Repo", "https://github.com/go-gui-org/go-gui", t.N3),
		gui.RichRun(".", t.N3),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Abbreviations show tooltips on hover: ", t.N3),
		gui.RichAbbr("HTML", "HyperText Markup Language", t.N3),
		gui.RichRun(" and ", t.N3),
		gui.RichAbbr("CSS", "Cascading Style Sheets", t.N3),
		gui.RichRun(".", t.N3),
		gui.RichBr(),
		gui.RichBr(),
		gui.RichRun("Long paragraphs wrap automatically when "+
			"TextModeWrap is enabled. This paragraph contains "+
			"enough text to demonstrate wrapping behavior in "+
			"the RTF widget. Resize the window to see how the "+
			"text reflows to fit the available width.", t.N3),
	}}

	return gui.Column(gui.ContainerCfg{
		ID:         "rtf-scroll",
		Width:      float32(ww),
		Height:     float32(wh),
		Sizing:     gui.FixedFixed,
		Scrollable: true,
		Padding:    gui.SomeP(10, 10, 10, 10),
		Content: []gui.View{
			gui.RTF(gui.RtfCfg{
				RichText:      rt,
				Mode:          gui.TextModeWrap,
				BaseTextStyle: &t.N3,
			}),
		},
	})
}
