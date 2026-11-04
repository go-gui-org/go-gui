package gui

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// A gradient spans the layout glyph draws, so a gradient text stays one
// layout as far as the shaper allows, and past that draws in its plain
// colour. Shimmer is a gradient too, and the plain text past the limit
// must still draw.

func testTextGradient() *glyph.GradientConfig {
	return &glyph.GradientConfig{
		Direction: glyph.GradientVertical,
		Stops: []glyph.GradientStop{
			{Color: glyph.Color{R: 255, A: 255}, Position: 0},
			{Color: glyph.Color{B: 255, A: 255}, Position: 1},
		},
	}
}

func gradientTextStyle(g *glyph.GradientConfig) TextStyle {
	return TextStyle{Size: 16, Color: RGB(20, 20, 20), Gradient: g}
}

// paragraphsOfBytes is exactly n bytes of paragraphs, some empty.
func paragraphsOfBytes(n int) string {
	return strings.Repeat("abcd efgh ijkl\n\n", n/16+1)[:n]
}

func gradientTextView(text string, style TextStyle) func(*Window) View {
	return func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap, TextStyle: style}),
		}})
	}
}

// textLayoutDraws returns the frame's glyph-layout draws.
func textLayoutDraws(w *Window) []RenderCmd {
	var out []RenderCmd
	for _, c := range w.Renderers() {
		if c.Kind == RenderLayout || c.Kind == RenderLayoutTransformed {
			out = append(out, c)
		}
	}
	return out
}

func TestTextBlockGradientStaysOneLayout(t *testing.T) {
	cases := []struct {
		name, text string
	}{
		{"trailing newline", "Title\n"},
		{"blank line", "Line one\n\nLine two"},
		{"blank lines", "A\n\nB\n\nC"},
		{"past one chunk", paragraphsOfBytes(5000)},
		{"at the shaper limit", paragraphsOfBytes(glyph.MaxTextLength)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newBlockTestWindow(t)
			g := testTextGradient()
			ly := w.TestRender(gradientTextView(tc.text, gradientTextStyle(g)))
			sh, ok := ly.FindByID("t")
			if !ok {
				t.Fatal("text not found")
			}
			if n := len(sh.Shape.TC.block.chunks); n != 1 {
				t.Errorf("%d chunks, want one layout", n)
			}
			draws := textLayoutDraws(w)
			if len(draws) != 1 || draws[0].TextGradient == nil {
				t.Fatalf("%d draws, want one carrying the gradient", len(draws))
			}
		})
	}
}

func TestTextBlockGradientPastTheLimitDrawsPlain(t *testing.T) {
	w := newBlockTestWindow(t)
	color := RGB(20, 20, 20)
	text := paragraphsOfBytes(glyph.MaxTextLength + 1)
	w.TestRender(gradientTextView(text, gradientTextStyle(testTextGradient())))
	draws := textLayoutDraws(w)
	if len(draws) == 0 {
		t.Fatal("the text drew nothing")
	}
	for _, d := range draws {
		if d.TextGradient != nil || d.TextStylePtr.Gradient != nil {
			t.Fatal("a draw past the limit still carries the gradient")
		}
		if d.TextStylePtr.Color != color {
			t.Fatalf("drawn in %v, want the plain colour", d.TextStylePtr.Color)
		}
	}
}

// Whole layout or chunks, a text takes the same room: crossing the limit
// must not move the lines.
func TestTextBlockOneLayoutMatchesChunks(t *testing.T) {
	w := newBlockTestWindow(t)
	text := paragraphsOfBytes(5000)
	plain := TextStyle{Size: 16, Color: RGB(20, 20, 20)}
	whole := newTextBlock(text, 300, gradientTextStyle(testTextGradient()),
		TextModeWrap, w)
	chunked := newTextBlock(text, 300, plain, TextModeWrap, w)
	if len(whole.chunks) != 1 || len(chunked.chunks) < 2 {
		t.Fatalf("chunks = %d and %d", len(whole.chunks), len(chunked.chunks))
	}
	if whole.boxHeight() != chunked.boxHeight() {
		t.Errorf("height %v whole, %v in chunks",
			whole.boxHeight(), chunked.boxHeight())
	}
	for off := 0; off <= len(text); off += 97 {
		a, ok1 := whole.cursorPos(off)
		b, ok2 := chunked.cursorPos(off)
		if !ok1 || !ok2 || a.X != b.X || a.Y != b.Y {
			t.Fatalf("caret at %d: %+v whole, %+v in chunks", off, a, b)
		}
	}
}

// Typing across the limit moves the text between one layout and chunks,
// and deleting moves it back.
func TestInputGradientAcrossTheLimit(t *testing.T) {
	w := newBlockTestWindow(t)
	text := paragraphsOfBytes(glyph.MaxTextLength - 1)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{ID: "f", Text: text, Mode: InputMultiline,
				Scrollable: true, Sizing: FixedFixed, Width: 300, Height: 200,
				TextStyle:     gradientTextStyle(testTextGradient()),
				OnTextChanged: func(s string, _ EventCtx) { text = s }}),
		}})
	})
	w.SetFocus("f")
	StateMap[string, inputState](w, nsInput, capMany).Set(
		"f", inputState{CursorPos: len(text)})
	_ = w.TestKey("f", KeyEnd, ModNone)
	check := func(step string, wantLen int) {
		t.Helper()
		if len(text) != wantLen {
			t.Fatalf("%s: %d bytes, want %d", step, len(text), wantLen)
		}
		draws := textLayoutDraws(w)
		if len(draws) == 0 {
			t.Fatalf("%s: nothing drawn", step)
		}
		for _, d := range draws {
			if got := d.TextGradient != nil; got != (wantLen <= glyph.MaxTextLength) {
				t.Fatalf("%s: gradient drawn = %v at %d bytes", step, got, wantLen)
			}
		}
		if wantLen <= glyph.MaxTextLength && len(draws) != 1 {
			t.Fatalf("%s: %d draws, want one layout", step, len(draws))
		}
	}
	check("start", glyph.MaxTextLength-1)
	_ = w.TestType("f", "a")
	check("at the limit", glyph.MaxTextLength)
	_ = w.TestType("f", "b")
	check("past the limit", glyph.MaxTextLength+1)
	_ = w.TestKey("f", KeyBackspace, ModNone)
	check("back at the limit", glyph.MaxTextLength)
}

// A fade judges each text by itself.
func TestContentTransitionFadeGradientLimit(t *testing.T) {
	long := strings.Repeat("old text line\n", 1000)
	cases := []struct {
		name, from string
		oldHas     bool
	}{
		{"long old text", long, false},
		{"old text past one chunk", long[:5000], true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{State: new(int), Width: 300, Height: 200})
			w.textMeasurer = blockStubMeasurer{}
			text := tc.from
			g := testTextGradient()
			w.viewGenerator = func(*Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Text(TextCfg{ID: "n", Text: text, Mode: TextModeWrap,
						TextStyle:  gradientTextStyle(g),
						Transition: ContentTransition{Kind: ContentTransitionFade}}),
				}})
			}
			w.markLayoutRefresh(refreshTest)
			w.FrameFn()
			text = "new text"
			w.markLayoutRefresh(refreshTest)
			w.FrameFn()
			w.stopAnimationLoop()
			var oldSeen, newSeen bool
			for _, c := range renderContentTransitionAt(t, w, 0.5) {
				if c.Kind != RenderLayout {
					continue
				}
				switch {
				case strings.HasPrefix(c.Text, "old"):
					oldSeen = true
					if (c.TextGradient != nil) != tc.oldHas {
						t.Errorf("old text gradient = %v, want %v",
							c.TextGradient != nil, tc.oldHas)
					}
				case strings.HasPrefix(c.Text, "new"):
					newSeen = true
					if c.TextGradient == nil {
						t.Error("new text lost its gradient")
					}
				}
			}
			if !oldSeen || !newSeen {
				t.Fatalf("old drawn %v, new drawn %v", oldSeen, newSeen)
			}
		})
	}
}

// A shimmer stays per chunk below the limit, since it can end while the
// text stays, and is dropped past it.
func TestTextShimmerGradientLimit(t *testing.T) {
	cases := []struct {
		name string
		size int
		want bool
	}{
		{"past one chunk", 5000, true},
		{"past the limit", glyph.MaxTextLength + 1, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newBlockTestWindow(t)
			text := paragraphsOfBytes(tc.size)
			w.TestRender(func(*Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Text(TextCfg{ID: "t", Text: text, Mode: TextModeWrap,
						Anim: TextAnimCfg{Kind: TextAnimShimmer, Repeat: true}}),
				}})
			})
			draws := textLayoutDraws(w)
			if len(draws) == 0 {
				t.Fatal("nothing drawn")
			}
			for _, d := range draws {
				if got := d.TextGradient != nil; got != tc.want {
					t.Fatalf("shimmer gradient = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestTextGradientDroppedFinding(t *testing.T) {
	w := newBlockTestWindow(t)
	text := paragraphsOfBytes(glyph.MaxTextLength)
	style := gradientTextStyle(testTextGradient())
	w.viewGenerator = func(*Window) View { return gradientTextView(text, style)(w) }
	if found := w.TestFindings(DebugGlyphLayoutFallback); len(found) != 0 {
		t.Fatalf("a text at the limit reported %q", found)
	}
	text += "x"
	found := strings.Join(w.TestFindings(DebugGlyphLayoutFallback), "\n")
	if !strings.Contains(found, "gradient") || !strings.Contains(found, `"t"`) {
		t.Fatalf("want a finding naming the text and its gradient, got %q", found)
	}
}

// countingStubMeasurer counts the layouts shaped.
type countingStubMeasurer struct {
	blockStubMeasurer
	n *int
}

func (m countingStubMeasurer) LayoutText(
	text string, style TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	*m.n++
	return m.blockStubMeasurer.LayoutText(text, style, wrapWidth)
}

// What a view rebuilds each frame must not re-shape the text: only what
// changes glyph positions is a shaping change.
func TestTextBlockRebuiltStyleDoesNotReshape(t *testing.T) {
	features := func() *glyph.FontFeatures {
		return &glyph.FontFeatures{
			OpenTypeFeatures: []glyph.FontFeature{{Tag: "tnum", Value: 1}},
		}
	}
	base := TextStyle{Size: 16, Color: RGB(20, 20, 20)}
	frame := 0
	cases := []struct {
		name  string
		style func() TextStyle
	}{
		{"gradient", func() TextStyle {
			s := base
			s.Gradient = testTextGradient()
			return s
		}},
		{"transform", func() TextStyle {
			s := base
			s.AffineTransform = &glyph.AffineTransform{XX: 1, YY: 1, X0: 1}
			return s
		}},
		{"rotation", func() TextStyle {
			s := base
			s.RotationRadians = 0.1 + float32(frame)/100
			return s
		}},
		{"features", func() TextStyle {
			s := base
			s.Features = features()
			return s
		}},
	}
	text := numberedLines(300)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newBlockTestWindow(t)
			n := 0
			w.textMeasurer = countingStubMeasurer{n: &n}
			w.TestRender(func(*Window) View {
				frame++
				return gradientTextView(text, tc.style())(w)
			})
			n = 0
			for range 5 {
				w.TestRender(nil)
			}
			if n != 0 {
				t.Errorf("%d layouts shaped over 5 rebuilds, want 0", n)
			}
		})
	}
}

func TestTextLayoutStyleEqualShaping(t *testing.T) {
	base := TextStyle{Size: 16}
	feat := func(v int) *glyph.FontFeatures {
		return &glyph.FontFeatures{
			OpenTypeFeatures: []glyph.FontFeature{{Tag: "tnum", Value: v}},
		}
	}
	axis := func(v float32) *glyph.FontFeatures {
		return &glyph.FontFeatures{
			VariationAxes: []glyph.FontAxis{{Tag: "wght", Value: v}},
		}
	}
	with := func(f func(*TextStyle)) TextStyle {
		s := base
		f(&s)
		return s
	}
	cases := []struct {
		name string
		a, b TextStyle
		want bool
	}{
		{"a gradient appears", base,
			with(func(s *TextStyle) { s.Gradient = testTextGradient() }), false},
		{"another gradient",
			with(func(s *TextStyle) { s.Gradient = testTextGradient() }),
			with(func(s *TextStyle) { s.Gradient = testTextGradient() }), true},
		{"equal features",
			with(func(s *TextStyle) { s.Features = feat(1) }),
			with(func(s *TextStyle) { s.Features = feat(1) }), true},
		{"other features",
			with(func(s *TextStyle) { s.Features = feat(1) }),
			with(func(s *TextStyle) { s.Features = feat(2) }), false},
		{"other axes",
			with(func(s *TextStyle) { s.Features = axis(400) }),
			with(func(s *TextStyle) { s.Features = axis(700) }), false},
		{"features removed",
			with(func(s *TextStyle) { s.Features = feat(1) }), base, false},
		{"other size", base, with(func(s *TextStyle) { s.Size = 18 }), false},
	}
	for _, tc := range cases {
		if got := textLayoutStyleEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}
