// This example demonstrates Stack, a container that lays its children in
// layers. Three cards overlap; click one to bring it to the front.
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

// Stack
//
// A Row puts its children side by side and a Column puts them one under
// the other. A Stack puts them one over the other. The first child is the
// bottom layer and the last child is the top layer.
//
// Each layer starts at the Stack's content origin. Here each card sits in
// a FillFill wrapper whose padding moves the card down and right, so the
// cards overlap. A card keeps its place; only its layer changes. A click
// goes to the top layer under the pointer: click where two cards overlap
// and the upper one comes to the front.

// card is one layer: a name and its fill. The fills are translucent, so
// where two cards overlap the lower one shows through.
type card struct {
	id, name string
	color    gui.Color
}

var cards = [...]card{
	{"red", "Red", gui.RGBA(220, 60, 60, 220)},
	{"green", "Green", gui.RGBA(40, 160, 90, 220)},
	{"blue", "Blue", gui.RGBA(50, 100, 220, 220)},
}

// app holds the layer order: order[0] is the bottom card, the last entry
// the top card. Each entry is an index into cards.
type app struct {
	order [len(cards)]int
}

// newApp starts with the cards in their listed order: Blue on top.
func newApp() *app {
	return &app{order: [len(cards)]int{0, 1, 2}}
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Title:  "Stack",
		Width:  480,
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
	s := gui.State[app](w)
	theme := gui.CurrentTheme()
	top := cards[s.order[len(s.order)-1]].name
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		HAlign:  gui.HAlignCenter,
		Spacing: gui.SpacingMedium,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Click a card to bring it to the front. Where cards overlap, the top one gets the click.",
				TextStyle: theme.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			stage(s, theme),
			gui.Text(gui.TextCfg{Text: "On top: " + top, TextStyle: theme.TextStyleTitleSmall}),
		},
	})
}

// raise moves card c to the top of the order. The cards above it each
// drop one layer.
func raise(s *app, c int) {
	k := 0
	for k < len(s.order) && s.order[k] != c {
		k++
	}
	if k == len(s.order) {
		return // not a card in the order; a raise would duplicate one
	}
	for ; k < len(s.order)-1; k++ {
		s.order[k] = s.order[k+1]
	}
	s.order[len(s.order)-1] = c
}

// doc:snippet-begin stack-layers

// stage lays the cards in a Stack, bottom first. Each card's wrapper
// pads it down and right by a fixed step per card, so a card keeps its
// place when it changes layer: only which card covers which changes.
func stage(s *app, theme gui.Theme) gui.View {
	// The card colors are the same in every theme, so the text on them
	// is too.
	name := theme.TextStyleTitle
	name.Color = gui.White
	note := theme.TextStyleBodySmall
	note.Color = gui.White
	layers := make([]gui.View, 0, len(s.order))
	for layer, c := range s.order {
		step := float32(c)
		layers = append(layers, gui.Column(gui.ContainerCfg{
			// The wrapper covers the whole Stack. It has no click
			// handler, so a press outside its card goes to the layers
			// under it.
			Sizing:     gui.FillFill,
			Padding:    gui.NewPadding(step*50, 0, 0, step*80),
			SizeBorder: gui.NoBorder,
			Content: []gui.View{
				gui.Column(gui.ContainerCfg{
					ID:          cards[c].id,
					Width:       170,
					Height:      110,
					Sizing:      gui.FixedFixed,
					Color:       cards[c].color,
					ColorBorder: gui.White,
					Radius:      gui.RadiusMedium,
					Padding:     gui.PadAll(10),
					Spacing:     gui.SpacingSmall,
					OnClick: func(ctx gui.EventCtx) {
						raise(gui.State[app](ctx.Window), c)
						ctx.Consume()
					},
					Content: []gui.View{
						gui.Text(gui.TextCfg{Text: cards[c].name, TextStyle: name}),
						gui.Text(gui.TextCfg{Text: fmt.Sprintf("layer %d", layer+1), TextStyle: note}),
					},
				}),
			},
		}))
	}
	return gui.Stack(gui.ContainerCfg{
		ID:      "stage",
		Width:   330,
		Height:  210,
		Sizing:  gui.FixedFixed,
		Padding: gui.NoPadding,
		Content: layers,
	})
}

// doc:snippet-end stack-layers
