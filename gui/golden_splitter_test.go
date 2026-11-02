package gui

import "testing"

// TestGoldenSplitter records a Splitter, whose panes are sized and
// placed by its layoutSizer (#1214). Filled panes pin each pane's box
// and its clip against the splitter's border inset. The RTL case pins
// that the panes keep their physical order under an RTL locale: the
// splitter's Canvas aligns left, so positioning adds no mirrored shift.
func TestGoldenSplitter(t *testing.T) {
	for _, c := range []struct {
		name string
		dir  TextDirection
	}{
		{"splitter", TextDirLTR},
		{"splitter_rtl", TextDirRTL},
	} {
		t.Run(c.name, func(t *testing.T) {
			saved := CurrentLocale()
			t.Cleanup(func() { SetLocale(saved) })
			l := saved
			l.TextDir = c.dir
			SetLocale(l)

			w := NewTestWindow(t, WindowCfg{
				State:  new(int),
				Width:  goldenWidth,
				Height: goldenHeight,
			})
			w.SetView(wrapGoldenRoot(goldenSplitter))
			w.TestGolden(t, GoldenCfg{Dir: "testdata", Name: c.name})
		})
	}
}

func goldenSplitter(_ *Window) View {
	pane := func(label string, c Color) []View {
		return []View{Column(ContainerCfg{
			Sizing:  FillFill,
			Color:   c,
			Content: []View{Text(TextCfg{Text: label})},
		})}
	}
	return Splitter(SplitterCfg{
		ID:     "sp",
		Ratio:  SomeF(0.3),
		Sizing: FillFill,
		First:  SplitterPaneCfg{Content: pane("first", RGB(200, 60, 60))},
		Second: SplitterPaneCfg{Content: pane("second", RGB(60, 60, 200))},
	})
}
