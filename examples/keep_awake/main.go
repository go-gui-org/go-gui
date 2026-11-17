// Keep_awake shows Window.KeepAwake: while a player plays, the display
// stays on. Each player holds its own request, so stopping one does not
// let the display sleep while the other still plays.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// players is how many players the window has.
const players = 2

// App holds each player's release function, nil while it is stopped.
type App struct {
	release [players]func()
}

// awakeScope owns every ID in this window.
const awakeScope = "keep-awake"

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "keep_awake",
		Width:  520,
		Height: 360,
		OnInit: func(w *gui.Window) { w.SetView(mainView) },
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

// doc:snippet-begin keep-awake

// setPlaying starts or stops player i. A playing player holds its own
// request to keep the display on; stopping it ends only that request.
func setPlaying(w *gui.Window, i int, playing bool) {
	app := gui.State[App](w)
	switch {
	case playing && app.release[i] == nil:
		app.release[i] = w.KeepAwake(fmt.Sprintf("Player %d is playing", i+1))
	case !playing && app.release[i] != nil:
		app.release[i]()
		app.release[i] = nil
	}
}

// doc:snippet-end keep-awake

// held counts the players that hold a request.
func (app *App) held() int {
	n := 0
	for _, release := range app.release {
		if release != nil {
			n++
		}
	}
	return n
}

// playerID is the ID of player i's switch.
func playerID(i int) string { return gui.ScopeIDN(awakeScope, "player", i) }

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	app := gui.State[App](w)
	content := []gui.View{
		gui.Text(gui.TextCfg{
			Text:      "Keep the display awake",
			TextStyle: theme.TextStyleDisplay,
		}),
		gui.Text(gui.TextCfg{
			Text: "While a player plays, the display stays on. A minimized " +
				"or hidden window lets it sleep again.",
			TextStyle: theme.TextStyleSecondary,
			Mode:      gui.TextModeWrap,
		}),
	}
	for i := range players {
		content = append(content, gui.Switch(gui.SwitchCfg{
			ID:       playerID(i),
			Label:    fmt.Sprintf("Player %d plays", i+1),
			Selected: app.release[i] != nil,
			OnClick: func(ctx gui.EventCtx) {
				playing := gui.State[App](ctx.Window).release[i] == nil
				setPlaying(ctx.Window, i, playing)
				ctx.Consume()
			},
		}))
	}
	content = append(content, gui.Text(gui.TextCfg{
		Text:      fmt.Sprintf("Requests held: %d", app.held()),
		TextStyle: theme.TextStyleTitleSmall,
	}))
	return gui.Column(gui.ContainerCfg{
		ID:         gui.ScopeID(awakeScope, "root"),
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PadAll(theme.SpacingLarge),
		Spacing:    gui.SpacingMedium,
		Content:    content,
	})
}
