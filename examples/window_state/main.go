// Window state drives the OS window from the app: minimize, maximize,
// restore, fullscreen, resize and move. Each call only asks; the readout
// comes from the getters, which report what the OS confirmed, so it also
// follows the title-bar buttons and the keyboard shortcuts of the
// desktop.
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

// App has no state of its own: the window holds it all.
type App struct{}

// moveStep is how far the Move button shifts the window, in the OS
// desktop units Position reports.
const moveStep = 40

// stateScope owns every ID in this window.
const stateScope = "window_state"

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{},
		Title:  "window state",
		Width:  520,
		Height: 360,
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
	theme := gui.CurrentTheme()
	return gui.Column(gui.ContainerCfg{
		ID:         gui.ScopeID(stateScope, "root"),
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PadAll(theme.SpacingLarge),
		Spacing:    gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Ask the OS, then read what it did",
				TextStyle: theme.TextStyleDisplay,
			}),
			buttons(w),
			readout(w),
		},
	})
}

// buttons is one row per kind of request. The maximize and fullscreen
// labels come from the getters, so they stay right when the user changes
// the state through the OS.
func buttons(w *gui.Window) gui.View {
	maximizeLabel, maximize := "Maximize", (*gui.Window).Maximize
	if w.IsMaximized() {
		maximizeLabel, maximize = "Restore", (*gui.Window).Restore
	}
	fullscreenLabel := "Fullscreen"
	if w.IsFullscreen() {
		fullscreenLabel = "Leave fullscreen"
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
		Spacing:    gui.SpacingSmall,
		Content: []gui.View{
			buttonRow(
				button("minimize", "Minimize", (*gui.Window).Minimize),
				button("maximize", maximizeLabel, maximize),
				button("fullscreen", fullscreenLabel, func(win *gui.Window) {
					win.SetFullscreen(!win.IsFullscreen())
				}),
			),
			buttonRow(
				button("small", "640×420", func(win *gui.Window) {
					win.SetWindowSize(640, 420)
				}),
				button("large", "900×600", func(win *gui.Window) {
					win.SetWindowSize(900, 600)
				}),
				button("move", "Move", moveWindow),
			),
		},
	})
}

// moveWindow shifts the window from where it is. Position has no answer
// on Wayland, where a client cannot place its window.
func moveWindow(w *gui.Window) {
	if x, y, ok := w.Position(); ok {
		w.SetPosition(x+moveStep, y+moveStep)
	}
}

func buttonRow(content ...gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		SizeBorder: gui.NoBorder,
		Padding:    gui.PaddingNone,
		Spacing:    gui.SpacingSmall,
		Content:    content,
	})
}

func button(id, label string, do func(*gui.Window)) gui.View {
	return gui.Button(gui.ButtonCfg{
		ID:      gui.ScopeID(stateScope, id),
		Content: []gui.View{gui.Text(gui.TextCfg{Text: label})},
		OnClick: func(ctx gui.EventCtx) {
			do(ctx.Window)
			ctx.Consume()
		},
	})
}

// readout shows what the getters say this frame.
func readout(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	width, height := w.WindowSize()
	position := "not reported here"
	if x, y, ok := w.Position(); ok {
		position = fmt.Sprintf("%d, %d", x, y)
	}
	lines := []string{
		fmt.Sprintf("Maximized: %v", w.IsMaximized()),
		fmt.Sprintf("Fullscreen: %v", w.IsFullscreen()),
		fmt.Sprintf("Size: %d × %d", width, height),
		"Position: " + position,
	}
	content := make([]gui.View, 0, len(lines))
	for _, line := range lines {
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
