// The flip_panel example shows gui.FlipPanel as a deck of flash cards:
// each card turns over to show its answer (#1020). Click a card, or tab
// to it and press Enter or Space.
package main

import (
	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	cardW = 200
	cardH = 120
)

// cards are the flash cards: a question on the front, its answer on the
// back. The last card turns about its horizontal axis.
var cards = []struct {
	id, question, answer string
	axis                 gui.FlipAxis
}{
	{"capital", "Capital of France?", "Paris", gui.FlipAxisY},
	{"planet", "Largest planet?", "Jupiter", gui.FlipAxisY},
	{"boil", "Water boils at?", "100 °C at sea level", gui.FlipAxisX},
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		Title:  "Flip Panel",
		Width:  720,
		Height: 280,
		State:  &app{flipped: make([]bool, len(cards))},
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
	// flipped[i] is true while card i shows its answer. The app owns
	// it; each FlipPanel turns when its value changes.
	flipped []bool
}

func mainView(w *gui.Window) gui.View {
	theme := gui.CurrentTheme()
	row := make([]gui.View, len(cards))
	for i := range cards {
		row[i] = card(w, theme, i)
	}
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Spacing: gui.SpacingLarge,
		Padding: gui.PaddingLarge,
		HAlign:  gui.HAlignCenter,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Flash cards",
				TextStyle: theme.TextStyleTitle,
			}),
			gui.Row(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				Spacing:    gui.SpacingLarge,
				SizeBorder: gui.NoBorder,
				Content:    row,
			}),
		},
	})
}

// card is one flash card. The app keeps which face shows and inverts it
// in OnFlip; the panel runs the turn on a spring, so a second click
// during a turn sends the card back with no jump.
func card(w *gui.Window, theme gui.Theme, i int) gui.View {
	c := cards[i]
	return gui.FlipPanel(gui.FlipPanelCfg{
		ID:      c.id,
		Sizing:  gui.FixedFixed,
		Width:   cardW,
		Height:  cardH,
		Axis:    c.axis,
		Flipped: gui.State[app](w).flipped[i],
		Front:   face(theme, c.question, theme.ColorPanel),
		Back:    face(theme, c.answer, theme.ColorActive),
		A11YCfg: gui.A11YCfg{A11YLabel: c.question},
		OnFlip: func(ctx gui.EventCtx) {
			s := gui.State[app](ctx.Window)
			s.flipped[i] = !s.flipped[i]
		},
	})
}

// face fills the panel, so both faces cover the card exactly.
func face(theme gui.Theme, text string, color gui.Color) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFill,
		Padding: gui.PaddingMedium,
		Radius:  gui.RadiusMedium,
		Color:   color,
		HAlign:  gui.HAlignCenter,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      text,
				TextStyle: theme.TextStyleTitleSmall,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}
