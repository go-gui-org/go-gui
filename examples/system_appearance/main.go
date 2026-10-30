// This example demonstrates following the desktop light/dark setting: gui.FollowSystemAppearance swaps the theme when the setting changes, and Window.TitlebarDark keeps the window frame in step.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// The theme choices, as radio values.
const (
	modeSystem = "system" // follow the desktop
	modeLight  = "light"
	modeDark   = "dark"
)

// App is the window state.
type App struct {
	Mode string // modeSystem, modeLight or modeDark
	// Desktop is the last desktop setting the window heard of.
	// HasDesktop stays false until the OS reports one, which it never
	// does with no backend or on a desktop go-gui cannot read.
	Desktop    gui.Appearance
	HasDesktop bool
}

func windowCfg() gui.WindowCfg {
	return gui.WindowCfg{
		Title:  "System Appearance",
		State:  &App{Mode: modeSystem},
		Width:  460,
		Height: 220,
		OnInit: func(w *gui.Window) {
			// The hook runs on every desktop change, following or not,
			// and with the current setting whenever a following
			// window applies it.
			w.OnSystemAppearance(func(a gui.Appearance) {
				app := gui.State[App](w)
				app.Desktop, app.HasDesktop = a, true
			})
			w.FollowSystemAppearance(gui.ThemeLight, gui.ThemeDark)
			w.SetView(mainView)
		},
	}
}

// setMode applies a theme choice.
func setMode(w *gui.Window, mode string) {
	gui.State[App](w).Mode = mode
	switch mode {
	case modeSystem:
		// Applies the desktop setting at once, frame included, and
		// again on every change.
		w.FollowSystemAppearance(gui.ThemeLight, gui.ThemeDark)
	case modeLight:
		pin(w, gui.ThemeLight)
	case modeDark:
		pin(w, gui.ThemeDark)
	}
}

// pin sets a fixed theme. SetTheme ends following the desktop and leaves
// the window frame alone, so the app asks for the frame that matches the
// theme itself.
func pin(w *gui.Window, t gui.Theme) {
	w.SetTheme(t)
	w.TitlebarDark(t.TitlebarDark)
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Spacing: gui.SpacingLarge,
		Content: []gui.View{
			gui.Label("Desktop: "+desktopText(app), gui.TextStyle{}),
			gui.Row(gui.ContainerCfg{
				VAlign: gui.VAlignMiddle,
				Content: []gui.View{
					gui.Label("Theme:", gui.TextStyle{}),
					gui.RadioButtonGroupRow(gui.RadioButtonGroupCfg{
						ID:         "theme",
						A11YCfg:    gui.A11YCfg{A11YLabel: "Theme"},
						SizeBorder: gui.NoBorder,
						Options: []gui.RadioOption{
							gui.NewRadioOption("Follow desktop", modeSystem),
							gui.NewRadioOption("Light", modeLight),
							gui.NewRadioOption("Dark", modeDark),
						},
						Value: app.Mode,
						OnSelect: func(mode string, ctx gui.EventCtx) {
							setMode(ctx.Window, mode)
							ctx.Consume()
						},
					}),
				},
			}),
		},
	})
}

// desktopText describes the desktop setting for the label.
func desktopText(app *App) string {
	switch {
	case !app.HasDesktop:
		return "no light/dark setting reported"
	case app.Desktop == gui.AppearanceDark:
		return "dark"
	default:
		return "light"
	}
}

// screenshotCfg is windowCfg for a -screenshot run. That run has no
// backend and so no desktop setting; it shows a dark desktop followed.
func screenshotCfg() gui.WindowCfg {
	cfg := windowCfg()
	cfg.OnInit = func(w *gui.Window) {
		app := gui.State[App](w)
		app.Desktop, app.HasDesktop = gui.AppearanceDark, true
		w.SetTheme(gui.ThemeDark)
		w.SetView(mainView)
	}
	return cfg
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	if *screenshot != "" {
		w := gui.NewWindow(screenshotCfg())
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(gui.NewWindow(windowCfg()))
}
