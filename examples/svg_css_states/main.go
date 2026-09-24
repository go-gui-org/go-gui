// This example demonstrates :hover and :focus pseudo-class matching on SVG elements.
// Svg_css_states demonstrates :hover and :focus pseudo-class
// matching driven by HoveredElementID / FocusedElementID on SvgCfg.
//
// Automatic mouse-driven hover detection on the Svg widget itself is
// not yet wired (deferred from v0.15.0). The sample below toggles
// the IDs through buttons so the cascade re-runs and the widget
// re-renders with the new styles applied. Apps that want
// pointer-tracking hover today can hit-test paths in the cached
// SvgParsed and feed the discovered element id back through SvgCfg
// on the next View pass.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const stateSvg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 120 120">
	<style>
		#ring { fill: #1e293b; stroke: #475569; stroke-width: 4; }
		#ring:hover { fill: #f59e0b; stroke: #fef3c7; }
		#ring:focus { fill: #2563eb; stroke: #bae6fd; }
		#dot { fill: #94a3b8; }
		#dot:hover { fill: #ef4444; }
	</style>
	<circle id="ring" cx="60" cy="60" r="40"/>
	<circle id="dot"  cx="60" cy="60" r="12"/>
</svg>`

type state struct {
	hoverRing bool
	focusRing bool
	hoverDot  bool
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{
		Width:  720,
		Height: 360,
		Title:  ":hover / :focus",
		State:  &state{},
		OnInit: func(w *gui.Window) { w.SetView(view) },
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

func view(w *gui.Window) gui.View {
	s := gui.State[state](w)
	// Explicit precedence: ring wins when both hover flags are
	// set (only Reset clears both, so this is belt and braces).
	hoverID := ""
	if s.hoverRing {
		hoverID = "ring"
	} else if s.hoverDot {
		hoverID = "dot"
	}
	focusID := ""
	if s.focusRing {
		focusID = "ring"
	}
	canvas := gui.Svg(gui.SvgCfg{
		ID:      gui.ScopeID("svg_css_states_view", "canvas"),
		A11YCfg: gui.A11YCfg{A11YLabel: "Hover and focus state demo"},
		SvgData: stateSvg,
		Sizing:  gui.FixedFixed,
		Width:   240, Height: 240,
		HoveredElementID: hoverID,
		FocusedElementID: focusID,
	})
	btn := func(n int, label string, toggle func(*state)) gui.View {
		return gui.Button(gui.ButtonCfg{
			ID:      gui.ScopeIDN("svg_css_states_view", "btn", n),
			Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
			Sizing:  gui.FillFit,
			// No ctx.Consume: no ancestor in this tree
			// handles OnClick, so nothing to stop.
			OnClick: func(ctx gui.EventCtx) {
				toggle(gui.State[state](ctx.Window))
				ctx.Window.InvalidateRender()
			},
		})
	}
	clear := func(s *state) { *s = state{} }
	controls := gui.Column(gui.ContainerCfg{
		Padding: gui.PaddingTwoFive,

		Sizing: gui.FillFill,
		Content: []gui.View{
			btn(0, "Hover #ring", func(s *state) {
				clear(s)
				s.hoverRing = true
			}),
			btn(1, "Hover #dot", func(s *state) {
				clear(s)
				s.hoverDot = true
			}),
			btn(2, "Focus #ring", func(s *state) {
				clear(s)
				s.focusRing = true
			}),
			btn(3, "Reset", func(s *state) { clear(s) }),
		},
	})
	return gui.Row(gui.ContainerCfg{
		Sizing: gui.FillFill,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:  gui.FillFill,
				HAlign:  gui.HAlignCenter,
				Content: []gui.View{canvas},
			}),
			controls,
		},
	})
}
