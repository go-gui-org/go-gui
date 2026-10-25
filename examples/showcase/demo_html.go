package main

import "github.com/go-gui-org/go-gui/gui"

// htmlDemoSource is the starting snippet of the HTML demo. It shows the
// subset, keeps a `*` that Markdown would read as emphasis, and ends
// with a script that the view drops with its content (#1158).
const htmlDemoSource = `<h2>Spring sale</h2>
<p>Save <b>50%</b> on <i>all</i> shoes until <u>Sunday</u>.
Prices with a * are final.</p>
<ul>
  <li>Free shipping</li>
  <li>Easy <code>returns</code></li>
</ul>
<blockquote>Offer ends <mark>soon</mark>.</blockquote>
<p><a href="https://example.com/sale">See the details</a></p>
<script>this does not run and does not show</script>`

// demoHTML edits an HTML snippet and previews it live with w.HTML.
func demoHTML(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "Edit the HTML below. The preview renders a safe subset: " +
					"no CSS, no scripts, and remote images stay blocked.",
				TextStyle: t.TextStyleCaption,
				Mode:      gui.TextModeWrap,
			}),
			sectionLabel(t, "Source"),
			// A fixed height keeps the preview on screen while the
			// source scrolls inside the input.
			gui.Input(gui.InputCfg{
				ID:         "html-source",
				A11YCfg:    gui.A11YCfg{A11YLabel: "HTML source"},
				Mode:       gui.InputMultiline,
				Scrollable: true,
				Sizing:     gui.FillFixed,
				Height:     200,
				Text:       app.HTMLSource,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					appState(ctx.Window).HTMLSource = s
				},
			}),
			sectionLabel(t, "Preview"),
			w.HTML(gui.MarkdownCfg{
				ID:         "html-preview",
				Focusable:  true,
				Source:     app.HTMLSource,
				Color:      t.ColorPanel,
				SizeBorder: gui.BorderThin,
				Radius:     gui.RadiusMedium,
				Padding:    t.PaddingMedium,
			}),
		},
	})
}
