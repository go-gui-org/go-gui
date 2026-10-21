// The content_transition example shows TextCfg.Transition (#1021), one
// row for each kind: a counter whose digits roll (Numeric), a status
// word that fades to the next (Fade), and an icon that shrinks away
// while the next one grows in (Replace).
package main

import (
	"flag"
	"log"
	"os"
	"strconv"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Content Transition",
		Width:  520,
		Height: 300,
		State:  &app{count: 98},
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

type app struct {
	count  int
	status int
	liked  bool
}

// statuses are the words the Fade row steps through.
var statuses = []string{"Saved", "Saving", "Offline"}

// labelWidth lines up the demos: every row's label takes this width.
const labelWidth = 110

func mainView(w *gui.Window) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		Content: []gui.View{
			kindRow("Numeric", numericDemo(w)...),
			kindRow("Fade", fadeDemo(w)...),
			kindRow("Replace", replaceDemo(w)...),
		},
	})
}

// kindRow is one row of the example: the name of a transition kind,
// then the widgets that show it.
func kindRow(kind string, demo ...gui.View) gui.View {
	theme := gui.CurrentTheme()
	content := []gui.View{
		gui.Column(gui.ContainerCfg{
			Sizing:     gui.FixedFit,
			Width:      labelWidth,
			Padding:    gui.PaddingNone,
			SizeBorder: gui.NoBorder,
			Content: []gui.View{gui.Text(gui.TextCfg{
				Text:      kind,
				TextStyle: theme.TextStyleLabel,
			})},
		}),
	}
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.PaddingNone,
		SizeBorder: gui.NoBorder,
		VAlign:     gui.VAlignMiddle,
		Content:    append(content, demo...),
	})
}

// numericDemo is a counter whose changed digits roll: up when the
// number grows, down when it shrinks.
func numericDemo(w *gui.Window) []gui.View {
	theme := gui.CurrentTheme()
	s := gui.State[app](w)
	return []gui.View{
		gui.Button(gui.ButtonCfg{
			ID:    "dec",
			Label: "−",
			OnClick: func(ctx gui.EventCtx) {
				gui.State[app](ctx.Window).count--
				ctx.Consume()
			},
		}),
		gui.Button(gui.ButtonCfg{
			ID:    "inc",
			Label: "+",
			OnClick: func(ctx gui.EventCtx) {
				gui.State[app](ctx.Window).count++
				ctx.Consume()
			},
		}),
		gui.Text(gui.TextCfg{
			ID:        "count",
			Text:      strconv.Itoa(s.count),
			TextStyle: theme.TextStyleDisplay,
			Transition: gui.ContentTransition{
				Kind: gui.ContentTransitionNumeric,
			},
		}),
	}
}

// fadeDemo is a status word that fades to the next one.
func fadeDemo(w *gui.Window) []gui.View {
	s := gui.State[app](w)
	return []gui.View{
		gui.Button(gui.ButtonCfg{
			ID:    "next",
			Label: "Next",
			OnClick: func(ctx gui.EventCtx) {
				st := gui.State[app](ctx.Window)
				st.status = (st.status + 1) % len(statuses)
				ctx.Consume()
			},
		}),
		gui.Text(gui.TextCfg{
			ID:   "status",
			Text: statuses[s.status],
			Transition: gui.ContentTransition{
				Kind: gui.ContentTransitionFade,
			},
		}),
	}
}

// replaceDemo is a like button whose outline heart shrinks away while
// the filled heart grows in.
func replaceDemo(w *gui.Window) []gui.View {
	theme := gui.CurrentTheme()
	likeIcon := gui.IconHeartO
	if gui.State[app](w).liked {
		likeIcon = gui.IconHeart
	}
	return []gui.View{
		gui.Button(gui.ButtonCfg{
			ID:      "like",
			A11YCfg: gui.A11YCfg{A11YLabel: "Like"},
			Content: []gui.View{gui.Text(gui.TextCfg{
				ID:        "like-icon",
				Text:      likeIcon,
				TextStyle: theme.TextStyleIconMedium,
				Transition: gui.ContentTransition{
					Kind: gui.ContentTransitionReplace,
				},
			})},
			OnClick: func(ctx gui.EventCtx) {
				st := gui.State[app](ctx.Window)
				st.liked = !st.liked
				ctx.Consume()
			},
		}),
	}
}
