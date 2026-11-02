package gui

import "testing"

// TestGoldenCanvasFitChild records a Canvas holding a Fit card, placed
// directly and inside a Fixed box (#1216). The card's background and
// its button's frame draw only once the measuring passes size the card.
func TestGoldenCanvasFitChild(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{
		State:  new(int),
		Width:  goldenWidth,
		Height: goldenHeight,
	})
	w.SetView(wrapGoldenRoot(goldenCanvasFitChild))
	w.TestGolden(t, GoldenCfg{Dir: "testdata", Name: "canvas_fit_child"})
}

func goldenCanvasFitChild(_ *Window) View {
	card := func(id string) View {
		return Column(ContainerCfg{
			Sizing: FitFit,
			Color:  RGB(50, 70, 160),
			Content: []View{
				Text(TextCfg{Text: "Title"}),
				Button(ButtonCfg{ID: id, Content: []View{Text(TextCfg{Text: "OK"})}}),
			},
		})
	}
	return Canvas(ContainerCfg{
		Sizing: FillFill,
		Content: []View{
			Column(ContainerCfg{
				X: 10, Y: 10, Sizing: FitFit, Padding: NoPadding, SizeBorder: NoBorder,
				Content: []View{card("direct")},
			}),
			Column(ContainerCfg{
				X: 150, Y: 10, Width: 140, Height: 180, Sizing: FixedFixed,
				Content: []View{card("boxed")},
			}),
		},
	})
}
