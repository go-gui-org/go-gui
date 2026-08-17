// The virtual_list example demonstrates a virtualized list whose rows
// differ in height: a message feed of 50,000 cards, none of which
// knows its own height until the layout engine measures it.
//
// It exercises the three things a uniform virtualized list cannot do:
// rows of differing heights, jumping to a row by index rather than by
// ID, and pinning to the bottom while a background goroutine appends.
package main

import (
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const listID = "feed"

// message is one row. Body length varies, so the row height varies
// with it — and only after wrapping, which is why the list measures
// rather than computes.
type message struct {
	Who  string
	Body string
	Seq  int
}

type App struct {
	Messages []message
	JumpText string
	Pinned   bool
}

// lorem supplies bodies of visibly different lengths.
var lorem = []string{
	"ack",
	"Rebased onto main and re-ran the gate; all green.",
	"The measurement lands one frame late by construction: the view " +
		"phase runs before sizing, so a row's height is known only " +
		"after arrange. That is what the height model absorbs — it " +
		"seeds from an estimate and converges on what was measured.",
	"Shipping tomorrow.",
	"A long one: virtualization means the rows outside the viewport " +
		"are never built, so their space has to come from somewhere. " +
		"Two transparent spacers hold it, sized from the prefix-sum " +
		"tree. Scroll far enough and the spacer above you is standing " +
		"in for tens of thousands of rows that have never existed.",
}

func newMessage(i int) message {
	return message{
		Seq:  i,
		Who:  [...]string{"ana", "bo", "cy", "dee"}[i%4],
		Body: lorem[i%len(lorem)],
	}
}

func main() {
	const size = 50_000
	msgs := make([]message, size)
	for i := range msgs {
		msgs[i] = newMessage(i)
	}

	gui.SetTheme(gui.ThemeDark.WithBorders(true))

	w := gui.NewWindow(gui.WindowCfg{
		State:  &App{Messages: msgs},
		Title:  "VirtualList — variable row heights",
		Width:  520,
		Height: 620,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			go appender(w)
		},
	})

	backend.Run(w)
}

// appender adds a message every second, so the pin-to-bottom toggle
// has something to follow. Window state is main-goroutine only, so
// the mutation goes through QueueCommand.
func appender(w *gui.Window) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for range time.Tick(time.Second) {
		w.QueueCommand(func(w *gui.Window) {
			app := gui.State[App](w)
			msg := newMessage(len(app.Messages) + rng.Intn(3))
			// Seq is the row's ItemKey, so it has to be unique: the
			// random part above only varies who and what, and two rows
			// sharing a key would share one measured height.
			msg.Seq = len(app.Messages)
			app.Messages = append(app.Messages, msg)
			if app.Pinned {
				// The reason ScrollToEnd exists: the content height
				// under virtualization is assembled from spacers over
				// estimated rows, so a percentage-based "scroll to
				// 100%" drifts a little further off with every append.
				w.ScrollToEnd(listID)
			}
			w.UpdateWindow()
		})
	}
}

func mainView(w *gui.Window) gui.View {
	app := gui.State[App](w)
	theme := gui.CurrentTheme()

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.Some(theme.SpacingMedium),
		Padding: gui.PadAll(10),
		Content: []gui.View{
			controls(app),
			gui.VirtualList(gui.VirtualListCfg{
				ID:        listID,
				ItemCount: len(app.Messages),
				Sizing:    gui.FillFill,
				// Identity, so a height measured for a message follows
				// that message when others are inserted above it.
				ItemKey: func(i int) string {
					return strconv.Itoa(app.Messages[i].Seq)
				},
				ItemView: func(i int, _ float32) gui.View {
					return card(app.Messages[i], i, w)
				},
			}),
			gui.Text(gui.TextCfg{
				Text: fmt.Sprintf("%d messages · focused row %d",
					len(app.Messages),
					w.VirtualListFocusedIndex(listID)),
				TextStyle: theme.TextStyleSecondary,
			}),
		},
	})
}

// controls is the jump-to-index box and the pin toggle.
func controls(app *App) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.Some(gui.CurrentTheme().SpacingSmall),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID:          "jump_to",
				Text:        app.JumpText,
				Placeholder: "row number",
				Width:       120,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[App](ctx.Window).JumpText = s
				},
			}),
			gui.TextButton("jump", "Jump", func(ctx gui.EventCtx) {
				a := gui.State[App](ctx.Window)
				n, err := strconv.Atoi(a.JumpText)
				if err != nil {
					return
				}
				// Index-addressed: the target row does not exist yet,
				// so there is no ID to resolve and no view to find.
				ctx.Window.ScrollToIndexAt(listID, n, 0.5)
			}),
			gui.TextButton("top", "Top", func(ctx gui.EventCtx) {
				ctx.Window.ScrollToIndex(listID, 0)
			}),
			gui.TextButton("end", "End", func(ctx gui.EventCtx) {
				ctx.Window.ScrollToEnd(listID)
			}),
			gui.Toggle(gui.ToggleCfg{
				ID:       "pin",
				Label:    "Pin to bottom",
				Selected: app.Pinned,
				OnClick: func(ctx gui.EventCtx) {
					a := gui.State[App](ctx.Window)
					a.Pinned = !a.Pinned
					if a.Pinned {
						ctx.Window.ScrollToEnd(listID)
					}
				},
			}),
		},
	})
}

// card builds one row. Its height falls out of the wrapped body text,
// which is exactly the case a scalar row height cannot describe.
//
// It ignores ItemView's width argument, and that is the point worth
// copying: the width a row is *given* must not become a width the row
// *demands*. Setting MinWidth from it — even minus the row's own
// padding — asks the list for a content width at least as wide as the
// one it just reported, and the row's border pushes that a few pixels
// further every frame. The list then widens without bound, and since a
// width change invalidates the wrap, every frame re-wraps and
// re-measures. Rows fill the width they are handed through Sizing;
// use the argument for decisions (a compact layout under some
// threshold), never for a minimum.
func card(m message, i int, w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	bg := theme.ColorPanel
	if i == w.VirtualListFocusedIndex(listID) {
		bg = theme.ColorSelect
	}
	return gui.Column(gui.ContainerCfg{
		// Composed, never hand-joined: ergonomics-audit mode `ids`
		// fails a literal ":" here.
		ID:      gui.ScopeIDN(listID, "row", i),
		Sizing:  gui.FillFit,
		Color:   bg,
		Padding: gui.PadAll(8),
		// Row spacing lives inside the row: the list itself is fixed
		// at zero spacing, because a gap between rows is height the
		// model does not account for.
		Spacing: gui.Some(theme.SpacingTight),
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("#%d — %s", m.Seq, m.Who),
				TextStyle: theme.TextStyleLabel,
			}),
			gui.Text(gui.TextCfg{
				Text: m.Body,
				Mode: gui.TextModeWrap,
				// Fill horizontally, fit vertically: the wrap happens
				// against whatever width the row is allocated, and the
				// height that falls out of it is what the list measures.
				Sizing: gui.FillFit,
			}),
		},
	})
}
