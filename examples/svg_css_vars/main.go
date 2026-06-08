// Svg_css_vars demonstrates v0.14.0 CSS custom-property additions:
// `var(--name, fallback)` resolution and `calc()` arithmetic. The
// theme switcher rebuilds the SVG source with a different `--primary`
// token; the same icon picks up the theme via `var(--primary)`. The
// stroke-width slider rebuilds the SVG with a different calc()
// argument.
package main

import (
	"fmt"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

type appState struct {
	ThemeIdx   int
	StrokeBase int // calc(<base>px + 1px) → final stroke width
}

type theme struct {
	Name    string
	Primary string
	Accent  string
}

var themes = []theme{
	{"Indigo", "#6366f1", "#22d3ee"},
	{"Sunset", "#f97316", "#fde047"},
	{"Forest", "#16a34a", "#a3e635"},
	{"Crimson", "#dc2626", "#fca5a5"},
}

func iconSvg(t theme, strokeBase int) string {
	// `var(--primary)` resolves from :root; `var(--missing, ...)` shows
	// fallback when the named var is undefined; `calc(Npx + 1px)`
	// computes stroke-width.
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
	<style>
		:root { --primary: %s; --accent: %s; --base: %dpx }
		.body   { fill: var(--primary) }
		.ring   { fill: none; stroke: var(--accent);
		          stroke-width: calc(var(--base) + 1px) }
		.label  { fill: var(--missing, #f1f5f9) }
	</style>
	<circle class="ring" cx="50" cy="50" r="42"/>
	<circle class="body" cx="50" cy="50" r="30"/>
	<rect class="label" x="40" y="44" width="20" height="12"/>
</svg>`,
		t.Primary, t.Accent, strokeBase)
}

func main() {
	gui.SetTheme(gui.ThemeDark.WithBorders(true))
	w := gui.NewWindow(gui.WindowCfg{
		Width:  640,
		Height: 480,
		Title:  "CSS var() + calc() (v0.14.0)",
		State:  &appState{ThemeIdx: 0, StrokeBase: 2},
		OnInit: func(w *gui.Window) { w.UpdateView(view) },
	})
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	app := gui.State[appState](w)
	t := themes[app.ThemeIdx]

	themeButtons := []gui.View{}
	for i, th := range themes {
		idx := i
		themeButtons = append(themeButtons, gui.Button(gui.ButtonCfg{
			Content: []gui.View{gui.Text(gui.TextCfg{Text: th.Name})},
			Sizing:  gui.FitFit,
			Padding: gui.Some(gui.PaddingTwoFive),
			OnClick: func(*gui.Layout, *gui.Event, *gui.Window) {
				app.ThemeIdx = idx
				w.UpdateView(view)
			},
		}))
	}

	strokeButtons := []gui.View{}
	for _, n := range []int{0, 2, 4, 6, 8} {
		base := n
		strokeButtons = append(strokeButtons, gui.Button(gui.ButtonCfg{
			Content: []gui.View{gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("base=%dpx", base),
			})},
			Sizing:  gui.FitFit,
			Padding: gui.Some(gui.PaddingTwoFive),
			OnClick: func(*gui.Layout, *gui.Event, *gui.Window) {
				app.StrokeBase = base
				w.UpdateView(view)
			},
		}))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.Some(gui.PaddingTwoFive),
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Theme (--primary, --accent)"}),
			gui.Row(gui.ContainerCfg{
				Sizing: gui.FillFit, Content: themeButtons,
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf(
					"Stroke base = %dpx → calc(--base + 1px) = %dpx",
					app.StrokeBase, app.StrokeBase+1),
			}),
			gui.Row(gui.ContainerCfg{
				Sizing: gui.FillFit, Content: strokeButtons,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing: gui.FillFill, HAlign: gui.HAlignCenter,
				Content: []gui.View{
					gui.Svg(gui.SvgCfg{
						SvgData: iconSvg(t, app.StrokeBase),
						Sizing:  gui.FixedFixed,
						Width:   240, Height: 240,
					}),
					gui.Text(gui.TextCfg{
						Text: t.Name + " — fallback fills inner rect via var(--missing, #f1f5f9)",
					}),
				},
			}),
		},
	})
}
