// Visibility shows how an app learns that nobody sees its window: it is
// minimized, the app hid it, or the app went to the background. A media
// player stops decoding and pauses its audio there.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// App keeps what the event handler saw.
type App struct {
	Log      []string
	Changes  int
	hiddenAt time.Time
}

// logLines is how many changes the log keeps.
const logLines = 8

// hideFor is how long the Hide button keeps the window away.
const hideFor = 3 * time.Second

// visScope owns every ID in this window.
const visScope = "visibility"

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "visibility",
		Width:  560,
		Height: 420,
		OnInit: func(w *gui.Window) {
			w.OnEvent = onEvent
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

// doc:snippet-begin visibility-event

// onEvent runs inside the call that changed the state, also while the
// window has no focus. The event carries no state: the two getters do.
func onEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventVisibilityChanged {
		return
	}
	shown := w.IsVisible() && !w.IsOccluded()
	// A player would call player.SetPaused(!shown) here.
	record(gui.State[App](w), shown, w.IsVisible(), w.IsOccluded(), time.Now())
}

// doc:snippet-end visibility-event

// record adds one line to the log. Going out of sight starts the clock;
// coming back says how long it took.
func record(app *App, shown, visible, occluded bool, now time.Time) {
	app.Changes++
	line := fmt.Sprintf("%s  not shown (IsVisible %v, IsOccluded %v)",
		now.Format("15:04:05"), visible, occluded)
	if shown {
		line = fmt.Sprintf("%s  shown again after %s", now.Format("15:04:05"),
			now.Sub(app.hiddenAt).Round(100*time.Millisecond))
	} else {
		app.hiddenAt = now
	}
	app.Log = append(app.Log, line)
	if len(app.Log) > logLines {
		app.Log = app.Log[len(app.Log)-logLines:]
	}
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)
	return gui.Column(gui.ContainerCfg{
		ID:         gui.ScopeID(visScope, "root"),
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PadAll(theme.SpacingLarge),
		Spacing:    gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Minimize the window, or switch away",
				TextStyle: theme.TextStyleDisplay,
			}),
			gui.Text(gui.TextCfg{
				Text: "Each change adds a line. The bar keeps its place " +
					"in the animation while nobody sees it.",
				TextStyle: theme.TextStyleSecondary,
				Mode:      gui.TextModeWrap,
			}),
			gui.ProgressBar(gui.ProgressBarCfg{
				ID:         gui.ScopeID(visScope, "busy"),
				Indefinite: true,
				Sizing:     gui.FillFit,
			}),
			gui.Button(gui.ButtonCfg{
				ID: gui.ScopeID(visScope, "hide"),
				Content: []gui.View{gui.Text(gui.TextCfg{
					Text: fmt.Sprintf("Hide for %v", hideFor),
				})},
				OnClick: func(ctx gui.EventCtx) {
					hideThenShow(ctx.Window)
					ctx.Consume()
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Changes: %d", app.Changes),
				TextStyle: theme.TextStyleTitleSmall,
			}),
			logView(app),
		},
	})
}

// hideThenShow takes the window off the screen and brings it back. Show
// runs through QueueCommand: the timer fires on another goroutine.
func hideThenShow(w *gui.Window) {
	w.Hide()
	time.AfterFunc(hideFor, func() {
		w.QueueCommand(func(win *gui.Window) { win.Show() })
	})
}

func logView(app *App) gui.View {
	theme := gui.CurrentTheme()
	content := make([]gui.View, 0, len(app.Log))
	for _, line := range app.Log {
		content = append(content, gui.Text(gui.TextCfg{
			Text:      line,
			TextStyle: theme.TextStyleSecondary,
		}))
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
		Content:    content,
	})
}
