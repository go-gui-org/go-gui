// This example gathers the custom control looks into one window, one tab per
// control. Each tab is its own package in a subfolder, with its own README and
// tests:
//
//   - buttons. Flat, Windows 98 and Windows XP buttons.
//   - checkboxes. Material and Windows XP checkboxes.
//   - radios. Material and Windows XP radio groups.
//   - toggles. Material, iOS-like, check-mark and labelled switches.
//   - sliders. Apple, Material and Windows XP sliders.
//   - textinputs. Material and Windows XP text fields.
//   - scrollbars. Classic, Windows 98 and cool-blue scrollbars.
//
// A page does not read the window state. It takes its own state as an
// argument, so all pages can share one window whose state is App.
package main

import (
	"flag"
	"log"
	"os"
	"slices"

	"github.com/go-gui-org/go-gui/examples/custom_controls/buttons"
	"github.com/go-gui-org/go-gui/examples/custom_controls/checkboxes"
	"github.com/go-gui-org/go-gui/examples/custom_controls/radios"
	"github.com/go-gui-org/go-gui/examples/custom_controls/scrollbars"
	"github.com/go-gui-org/go-gui/examples/custom_controls/sliders"
	"github.com/go-gui-org/go-gui/examples/custom_controls/textinputs"
	"github.com/go-gui-org/go-gui/examples/custom_controls/toggles"
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// tabIDs lists the tabs in display order. A tab ID is also the name of its
// subfolder, so -tab takes the same spelling.
var tabIDs = []string{
	"buttons", "checkboxes", "radios", "toggles", "sliders", "textinputs", "scrollbars",
}

// tabLabels maps a tab ID to the text on its tab.
var tabLabels = map[string]string{
	"buttons":    "Buttons",
	"checkboxes": "Checkboxes",
	"radios":     "Radios",
	"toggles":    "Toggles",
	"sliders":    "Sliders",
	"textinputs": "Text inputs",
	"scrollbars": "Scrollbars",
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	tab := flag.String("tab", tabIDs[0], "tab to show first")
	flag.Parse()

	if !slices.Contains(tabIDs, *tab) {
		log.Fatalf("unknown tab %q; want one of %v", *tab, tabIDs)
	}

	gui.SetTheme(gui.ThemeLight)

	app := newApp()
	app.tab = *tab
	w := gui.NewWindow(gui.WindowCfg{
		State:  app,
		Title:  "Custom Controls",
		Width:  960,
		Height: 780,
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

// App holds the selected tab and the state of each page. A page keeps its
// state while another tab is shown, so a switch back finds it unchanged.
type App struct {
	tab        string
	buttons    *buttons.App
	checkboxes *checkboxes.App
	radios     *radios.App
	toggles    *toggles.App
	sliders    *sliders.App
	textinputs *textinputs.App

	// onSelect is built once, so a frame does not allocate a new closure.
	onSelect func(string, gui.EventCtx)
}

func newApp() *App {
	app := &App{
		tab:        tabIDs[0],
		buttons:    buttons.New(),
		checkboxes: checkboxes.New(),
		radios:     radios.New(),
		toggles:    toggles.New(),
		sliders:    sliders.New(),
		textinputs: textinputs.New(),
	}
	app.onSelect = func(id string, ctx gui.EventCtx) {
		app.tab = id
		ctx.Consume()
	}
	return app
}

// page builds the view of one tab.
func (app *App) page(id string) gui.View {
	switch id {
	case "buttons":
		return buttons.View(app.buttons)
	case "checkboxes":
		return checkboxes.View(app.checkboxes)
	case "radios":
		return radios.View(app.radios)
	case "toggles":
		return toggles.View(app.toggles)
	case "sliders":
		return sliders.View(app.sliders)
	case "textinputs":
		return textinputs.View(app.textinputs)
	case "scrollbars":
		return scrollbars.View()
	}
	return nil
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)

	// Only the selected tab gets content. The tab control shows only that
	// one, so building the other pages would be work thrown away.
	items := make([]gui.TabItemCfg, len(tabIDs))
	for i, id := range tabIDs {
		items[i] = gui.TabItemCfg{ID: id, Label: tabLabels[id]}
		if id == app.tab {
			items[i].Content = []gui.View{app.page(id)}
		}
	}

	// Read the theme once: a Theme is about 12 KB, and each call copies it.
	th := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		Sizing: gui.FillFill,
		// The padding keeps the tab control off the window edges.
		Padding:    th.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Spacing:    gui.SomeF(th.SpacingMedium),
		Content: []gui.View{
			intro(&th),
			gui.TabControl(gui.TabControlCfg{
				ID:       "tabs",
				Selected: app.tab,
				OnSelect: app.onSelect,
				Items:    items,
				// The pages paint their own background edge to edge.
				PaddingContent: gui.PaddingNone,
			}),
		},
	})
}

// introText tells what the tabs show. It sits above the tab bar.
const introText = "Custom controls keep the behavior of a standard control and give it a new look. " +
	"Each tab rebuilds one control, such as a button, switch or slider, " +
	"in styles like Material, Windows 98 and Windows XP. " +
	"The app draws the look from the hover, press and focus state; " +
	"go-gui still handles the mouse, the keyboard and the screen reader."

// introGradient is a mild top-to-bottom wash, pale blue fading to a lighter
// blue. Both ends stay light, so the dark text keeps its contrast.
var introGradient = &gui.GradientDef{Direction: gui.GradientToBottom, Stops: []gui.GradientStop{
	{Color: gui.Hex(0xdde8f8), Pos: 0},
	{Color: gui.Hex(0xf2f7fd), Pos: 1},
}}

// intro takes the theme by pointer, so the call does not copy it again.
func intro(th *gui.Theme) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:      gui.FillFit,
		Padding:     th.PaddingMedium,
		Radius:      gui.SomeF(10),
		Gradient:    introGradient,
		SizeBorder:  gui.SomeF(1),
		ColorBorder: gui.Hex(0xd3dcec),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      introText,
				Mode:      gui.TextModeWrap,
				TextStyle: th.TextStyleDef,
			}),
		},
	})
}
