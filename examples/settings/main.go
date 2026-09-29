// This example demonstrates the app settings store: a typed struct that gui.LoadSettings reads at start-up and gui.SaveSettings writes on each change.
package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/appinfo"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

//go:embed appinfo.toml
var manifest []byte

// info gives the window its app ID. The settings store needs it: the
// saved data is keyed by the ID, not by the window.
var info = appinfo.MustParse(manifest)

// doc:snippet-begin settings-store
// The quoted block below is pinned by
// examples/showcase/sound_snippet_test.go against docs/dx-cheat-sheet.md.

// Settings is what the app saves. Adding a field later is the whole
// migration: a file saved by an older version leaves the new field at
// the default set before LoadSettings.
type Settings struct {
	Launches int  `json:"launches"`
	Dark     bool `json:"dark"`
}

// App is the window state. It holds the settings the view shows.
type App struct {
	Settings Settings
	Err      string // last load or save error, shown in the view
}

func windowCfg() gui.WindowCfg {
	return gui.WindowCfg{
		AppInfo: info,                                 // the store keys the saved data by info.ID
		State:   &App{Settings: Settings{Dark: true}}, // defaults
		Width:   360,
		Height:  180,
		OnInit: func(w *gui.Window) {
			app := gui.State[App](w)
			// A first run finds nothing and keeps the defaults. A
			// corrupt file returns an error and also keeps them.
			if err := gui.LoadSettings(w, &app.Settings); err != nil {
				app.Err = err.Error()
			}
			app.Settings.Launches++
			save(w)
			applyTheme(w)
			w.SetView(mainView)
		},
	}
}

// save writes the whole struct. Call it after each change: settings are
// small, and the file is replaced atomically.
func save(w *gui.Window) {
	app := gui.State[App](w)
	if err := gui.SaveSettings(w, app.Settings); err != nil {
		app.Err = err.Error()
	}
}

// doc:snippet-end settings-store

func applyTheme(w *gui.Window) {
	if gui.State[App](w).Settings.Dark {
		w.SetTheme(gui.ThemeDark)
	} else {
		w.SetTheme(gui.ThemeLight)
	}
}

// screenshotCfg is windowCfg for a -screenshot run. That run has no
// backend, so the store would be the real settings file: the image
// would depend on what this machine saved, and each run would change
// the user's settings. It shows the defaults and neither loads nor
// saves. It counts one launch so the image matches a real first run.
func screenshotCfg() gui.WindowCfg {
	cfg := windowCfg()
	cfg.OnInit = func(w *gui.Window) {
		gui.State[App](w).Settings.Launches++
		applyTheme(w)
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

// mainView shows the saved values and changes one of them.
func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)
	content := []gui.View{
		gui.Text(gui.TextCfg{Text: info.Name, TextStyle: theme.TextStyleDisplay}),
		gui.Text(gui.TextCfg{
			Text:      fmt.Sprintf("Launches: %d", app.Settings.Launches),
			TextStyle: theme.TextStyleBody,
		}),
		gui.Switch(gui.SwitchCfg{
			ID:       "dark",
			Label:    "Dark theme",
			Selected: app.Settings.Dark,
			OnClick: func(ctx gui.EventCtx) {
				a := gui.State[App](ctx.Window)
				a.Settings.Dark = !a.Settings.Dark
				save(ctx.Window)
				applyTheme(ctx.Window)
				ctx.Consume()
			},
		}),
		gui.Text(gui.TextCfg{
			Text:      "Quit and start again: both values are kept.",
			TextStyle: theme.TextStyleSecondary,
		}),
	}
	if app.Err != "" {
		content = append(content, gui.Text(gui.TextCfg{Text: app.Err, TextStyle: theme.TextStyleSecondary}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    theme.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Content:    content,
	})
}
