package gui

import (
	"strings"
	"testing"
)

// An Input changes its text on every key, and a whole-text measure then
// shapes it all to learn a width the sizing pass replaces from the chunks.
// Past one chunk it estimates. A plain Text keeps its exact width.

// widthCountingMeasurer records how much text TextWidth was asked to shape.
type widthCountingMeasurer struct {
	blockStubMeasurer
	longest *int
}

func (m widthCountingMeasurer) TextWidth(text string, style TextStyle) float32 {
	*m.longest = max(*m.longest, len(text))
	return m.blockStubMeasurer.TextWidth(text, style)
}

// linesOfBytes is about n bytes of 39-letter lines.
func linesOfBytes(n int) string {
	return strings.Repeat(strings.Repeat("x", 39)+"\n", n/40+1)[:n]
}

func TestInputDoesNotMeasureWholeTextPastOneChunk(t *testing.T) {
	cases := []struct {
		name string
		mode inputMode
		text string
	}{
		{"multiline", InputMultiline, linesOfBytes(5000)},
		{"single line", inputSingleLine, strings.Repeat("x", 5000)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newBlockTestWindow(t)
			longest := 0
			w.textMeasurer = widthCountingMeasurer{longest: &longest}
			text := tc.text
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Input(InputCfg{ID: "f", Text: text, Mode: tc.mode,
						Sizing:        FillFit,
						OnTextChanged: func(s string, _ EventCtx) { text = s }}),
				}})
			})
			longest = 0
			_ = w.TestType("f", "a")
			if len(text) != len(tc.text)+1 {
				t.Fatalf("typing left %d bytes", len(text))
			}
			if longest > textChunkBytes {
				t.Errorf("TextWidth shaped %d bytes for one key", longest)
			}
		})
	}
}

// A plain Text keeps its exact measure: a fit-sized wrap breaks its lines
// at that width, and no key retypes it.
func TestPlainTextMeasuresWholeText(t *testing.T) {
	w := newBlockTestWindow(t)
	longest := 0
	w.textMeasurer = widthCountingMeasurer{longest: &longest}
	text := linesOfBytes(5000)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Text(TextCfg{Text: text, Mode: TextModeMultiline}),
		}})
	})
	if longest != len(text) {
		t.Errorf("TextWidth measured at most %d of %d bytes", longest, len(text))
	}
}

// The width past one chunk still ends up exact: single-line sizing takes it
// from the chunks, which is what the horizontal scroll range reads.
func TestSingleLineInputWidthExactPastOneChunk(t *testing.T) {
	w := newBlockTestWindow(t)
	text := strings.Repeat("x", 5000)
	ly := w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f", Text: text, Sizing: FillFit}),
		}})
	})
	got := float32(0)
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if l.Shape.shapeType == shapeText && l.Shape.TC != nil {
			got = max(got, l.Shape.MinWidth)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	f, _ := ly.FindByID("f")
	walk(f)
	if want := float32(len(text)) * stubCW; got != want {
		t.Errorf("text width %v, want %v", got, want)
	}
}

// A password paints bullets, so its measured string outgrows one chunk
// before its text does. The sizing pass does not touch such a text, so it
// must keep its exact measure.
func TestPasswordKeepsExactWidth(t *testing.T) {
	w := newBlockTestWindow(t)
	text := strings.Repeat("s", 1000)
	masked := maskPassword(text)
	if len(text) > textChunkBytes || len(masked) <= textChunkBytes {
		t.Fatalf("want a text of one chunk and a mask past it: %d, %d",
			len(text), len(masked))
	}
	ly := w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f", Text: text, IsPassword: true, Sizing: FillFit}),
		}})
	})
	got := float32(0)
	var walk func(l *Layout)
	walk = func(l *Layout) {
		if l.Shape.shapeType == shapeText && l.Shape.TC != nil {
			got = max(got, l.Shape.MinWidth)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	f, _ := ly.FindByID("f")
	walk(f)
	if want := float32(len(masked)) * stubCW; got != want {
		t.Errorf("password width %v, want %v", got, want)
	}
}

// A box that fits a multi-line text is as wide as its longest line, at any
// length: an estimate over the whole text would be one enormous line.
func TestFitBoxAroundLongMultilineText(t *testing.T) {
	text := linesOfBytes(12000)
	cases := []struct {
		name string
		view View
	}{
		{"Text past the shaper limit",
			Text(TextCfg{Text: text, Mode: TextModeMultiline})},
		{"Input past one chunk", Input(InputCfg{ID: "f", Text: linesOfBytes(5000),
			Mode: InputMultiline, Sizing: FitFit})},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newBlockTestWindow(t)
			ly := w.TestRender(func(*Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Column(ContainerCfg{ID: "box", Sizing: FitFit,
						Content: []View{tc.view}}),
				}})
			})
			box, ok := ly.FindByID("box")
			if !ok {
				t.Fatal("box not found")
			}
			// 39 cells plus the padding and borders around them.
			if box.Shape.Width > 39*stubCW+100 {
				t.Errorf("box %v wide around lines of %v", box.Shape.Width, 39*stubCW)
			}
		})
	}
}

func TestLongestLine(t *testing.T) {
	cases := []struct{ text, want string }{
		{"", ""},
		{"abc", "abc"},
		{"a\nbcd\nef", "bcd"},
		{"abc\nde\n", "abc"},
		{"\n\nab", "ab"},
	}
	for _, tc := range cases {
		if got := longestLine(tc.text); got != tc.want {
			t.Errorf("longestLine(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
