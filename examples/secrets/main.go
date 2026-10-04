// This example demonstrates the secret store: gui.SaveSecret keeps an access token in the OS credential store, gui.LoadSecret reads it back and gui.DeleteSecret removes it.
package main

import (
	_ "embed"
	"errors"
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

// info gives the window its app ID. The secret store uses it as the
// service name.
var info = appinfo.MustParse(manifest)

// tokenKey names the one secret this app keeps.
const tokenKey = "api-token"

// App is the window state. It never holds the saved token itself: the
// view shows only whether one is saved and its length. It is read and
// written only on the main thread.
type App struct {
	Draft  string // what the user typed, not yet saved
	Status string // last result, shown in the view
	Busy   bool   // a store call is running; the buttons are disabled
}

func windowCfg() gui.WindowCfg {
	return gui.WindowCfg{
		AppInfo: info,
		State:   &App{},
		Width:   400,
		Height:  220,
		OnInit: func(w *gui.Window) {
			gui.State[App](w).Status = "Reading the secret store..."
			run(w, func() string { return status(w) })
			w.SetView(mainView)
		},
	}
}

// run calls the store off the main thread. A call can block until the
// user answers an unlock or access prompt, and on the main thread that
// would freeze the window. The result goes back through QueueCommand,
// so App changes only on the main thread.
func run(w *gui.Window, call func() string) {
	gui.State[App](w).Busy = true
	spawn(func() {
		msg := call()
		w.QueueCommand(func(w *gui.Window) {
			app := gui.State[App](w)
			app.Status = msg
			app.Busy = false
		})
	})
}

// spawn starts a store call. The tests replace it with a direct call,
// so the result is queued before the test's next action settles.
var spawn = func(f func()) { go f() }

// status reads the saved token and describes what it found. The token
// is cleared as soon as its length is known.
func status(w *gui.Window) string {
	token, err := gui.LoadSecret(w, tokenKey)
	switch {
	case err == nil:
		defer clear(token)
		return fmt.Sprintf("A token is saved (%d bytes).", len(token))
	case errors.Is(err, gui.ErrSecretNotFound):
		return "No token is saved."
	case errors.Is(err, gui.ErrSecretsUnsupported):
		return "This platform has no secure store: " + err.Error()
	default:
		return "Error: " + err.Error()
	}
}

// save stores the draft. It copies the draft on the main thread, so the
// goroutine reads no App field.
func save(w *gui.Window) {
	app := gui.State[App](w)
	value := []byte(app.Draft)
	app.Draft = ""
	run(w, func() string {
		defer clear(value)
		if err := gui.SaveSecret(w, tokenKey, value); err != nil {
			return "Error: " + err.Error()
		}
		return status(w)
	})
}

func remove(w *gui.Window) {
	run(w, func() string {
		if err := gui.DeleteSecret(w, tokenKey); err != nil {
			return "Error: " + err.Error()
		}
		return status(w)
	})
}

// screenshotCfg is windowCfg for a -screenshot run. That run has no
// backend and so no secret store; it shows the first-run status and
// does not read the store.
func screenshotCfg() gui.WindowCfg {
	cfg := windowCfg()
	cfg.OnInit = func(w *gui.Window) {
		gui.State[App](w).Status = "No token is saved."
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

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		Padding:    theme.PaddingLarge,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: info.Name, TextStyle: theme.TextStyleDisplay}),
			gui.Input(gui.InputCfg{
				ID:          "token",
				Text:        app.Draft,
				Placeholder: "Access token",
				IsPassword:  true,
				Sizing:      gui.FillFit,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).Draft = s
					ctx.Consume()
				},
			}),
			gui.Row(gui.ContainerCfg{
				Padding:    gui.PaddingNone,
				SizeBorder: gui.NoBorder,
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:       "save",
						Disabled: app.Busy || app.Draft == "",
						Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Save"})},
						OnClick: func(ctx gui.EventCtx) {
							save(ctx.Window)
							ctx.Consume()
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:       "delete",
						Disabled: app.Busy,
						Content:  []gui.View{gui.Text(gui.TextCfg{Text: "Delete"})},
						OnClick: func(ctx gui.EventCtx) {
							remove(ctx.Window)
							ctx.Consume()
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{Text: app.Status, TextStyle: theme.TextStyleSecondary}),
		},
	})
}
