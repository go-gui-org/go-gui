// This example demonstrates the HTML view: a live preview of an HTML snippet.
// The html_preview example edits a short HTML snippet on the left and
// renders it with w.HTML on the right, the way an admin panel
// previews a promo banner that a mobile app shows (#1158).
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const sampleHTML = `<h2>Spring sale</h2>
<p>Save <b>50%</b> on <i>all</i> shoes until <u>Sunday</u>.
Prices with a * are final.</p>
<ul>
  <li>Free shipping</li>
  <li>Easy returns</li>
</ul>
<p><a href="https://example.com/sale">See the details</a></p>
<script>this does not run and does not show</script>`

type App struct {
	Source string
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Source: sampleHTML},
		Width:  900,
		Height: 500,
		Title:  "HTML Preview",
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
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
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: theme.PaddingLarge,
		Spacing: gui.SpacingLarge,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:         "source",
				A11YCfg:    gui.A11YCfg{A11YLabel: "HTML source"},
				Mode:       gui.InputMultiline,
				Scrollable: true,
				Sizing:     gui.FillFill,
				Text:       app.Source,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Source = s
				},
			}),
			gui.Column(gui.ContainerCfg{
				// Structural scroll wrapper: no inset or border of its own,
				// so the preview's top lines up with the input's.
				ID:         "preview-scroll",
				Sizing:     gui.FillFill,
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Scrollable: true,
				Content:    []gui.View{preview(w, app.Source)},
			}),
		},
	})
}

// doc:snippet-begin html-preview

// preview renders the snippet with the HTML view. It takes the same
// MarkdownCfg as w.Markdown. Remote images stay blocked unless
// AllowRemoteImages is set, so a preview does not fetch from hosts
// the snippet names.
func preview(w *gui.Window, source string) gui.View {
	theme := gui.CurrentTheme()
	return w.HTML(gui.MarkdownCfg{
		ID:         "preview",
		Source:     source,
		Color:      theme.ColorPanel,
		SizeBorder: gui.BorderThin,
		Radius:     gui.RadiusMedium,
		Padding:    theme.PaddingMedium,
	})
}

// doc:snippet-end html-preview
