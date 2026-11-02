// This example shows a chat-style scrollable list inside a Splitter
// pane, with a switch to compare it against the same list in a plain
// Row (issue #1214).
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

const messageCount = 60

type App struct {
	plainRow bool
	ratio    float32
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{ratio: 0.3},
		Title:  "Splitter Scroll",
		Width:  800,
		Height: 600,
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
	app := gui.State[App](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			toolbar(w, app),
			body(app),
		},
	})
}

func toolbar(w *gui.Window, app *App) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Switch(gui.SwitchCfg{
				ID:       "use_splitter",
				Label:    "Use gui.Splitter",
				Selected: !app.plainRow,
				OnClick: func(ctx gui.EventCtx) {
					a := gui.State[App](ctx.Window)
					a.plainRow = !a.plainRow
					ctx.Consume()
				},
			}),
			gui.Button(gui.ButtonCfg{
				ID:      "jump_newest",
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Jump to newest"})},
				OnClick: func(ctx gui.EventCtx) {
					if ids := ctx.Window.ResolveID("messages"); len(ids) > 0 {
						ctx.Window.ScrollVerticalToPct(ids[0], 1)
					}
					ctx.Consume()
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      scrollStatus(w),
				TextStyle: w.Theme().TextStyleSecondary,
			}),
		},
	})
}

// scrollStatus reports how far down the message list is scrolled.
// ScrollVerticalPct divides by the full scroll range, so a list that
// cannot reach its end shows it as a percentage stuck below 100.
func scrollStatus(w *gui.Window) string {
	ids := w.ResolveID("messages")
	if len(ids) == 0 {
		// The first frame has no arranged tree to resolve against yet.
		return "Scrolled 0%"
	}
	return fmt.Sprintf("Scrolled %.0f%% (offset %.0f px)",
		100*w.ScrollVerticalPct(ids[0]), w.ScrollVerticalOffset(ids[0]))
}

func body(app *App) gui.View {
	sidebar := gui.Text(gui.TextCfg{Text: "Channels"})
	if app.plainRow {
		// The workaround from the issue: a fixed-width first pane
		// beside a Fill second pane, no Splitter.
		return gui.Row(gui.ContainerCfg{
			Sizing:     gui.FillFill,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					Width:   240,
					Sizing:  gui.FixedFill,
					Content: []gui.View{sidebar},
				}),
				messageList(),
			},
		})
	}
	return gui.Splitter(gui.SplitterCfg{
		ID:          "split",
		Orientation: gui.SplitterHorizontal,
		Sizing:      gui.FillFill,
		Ratio:       gui.SomeF(app.ratio),
		OnChange: func(ratio float32, _ gui.SplitterCollapsed, ctx gui.EventCtx) {
			gui.State[App](ctx.Window).ratio = ratio
		},
		First:  gui.SplitterPaneCfg{Content: []gui.View{sidebar}},
		Second: gui.SplitterPaneCfg{Content: []gui.View{messageList()}},
	})
}

// messageList builds the chat list the issue describes: every message
// is a Fill-width Row holding a Fit Column of a header line and the
// text. Plain Text rows would not show the bug.
func messageList() gui.View {
	rows := make([]gui.View, 0, messageCount)
	for i := range messageCount {
		text := fmt.Sprintf("Message %d of %d", i+1, messageCount)
		id := ""
		if i == messageCount-1 {
			text = "Newest message. If you can scroll to here, the scroll range is right."
			id = "newest"
		}
		rows = append(rows, gui.Row(gui.ContainerCfg{
			ID:     id,
			Sizing: gui.FillFit,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					Sizing: gui.FitFit,
					Content: []gui.View{
						gui.Text(gui.TextCfg{
							Text:      fmt.Sprintf("user%d · 12:%02d", i%3+1, i%60),
							TextStyle: gui.CurrentTheme().TextStyleSecondary,
						}),
						gui.Text(gui.TextCfg{Text: text}),
					},
				}),
			},
		}))
	}
	return gui.Column(gui.ContainerCfg{
		ID:         "messages",
		Sizing:     gui.FillFill,
		Scrollable: true,
		ScrollMode: gui.ScrollVerticalOnly,
		Content:    rows,
	})
}
