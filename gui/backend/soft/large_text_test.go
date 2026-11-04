//go:build !wasm

package soft

import (
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-glyph"
	"github.com/go-gui-org/go-gui/gui"
)

// Issue #1190: a Text or Input whose text is longer than go-glyph's
// per-layout budget (MaxTextLength, 10,240 bytes) drew nothing. The text
// is now shaped in chunks, so every case the issue tabulated paints.

var (
	// overBudget is the issue's repro text: 257 lines of 39 x's, cut one
	// byte past the budget.
	overBudget = strings.Repeat(strings.Repeat("x", 39)+"\n", 257)[:glyph.MaxTextLength+1]
	// oneLongLine is a minified-JSON-like response with no line break,
	// the shape of the paste the issue came from.
	oneLongLine = strings.Repeat(`{"id":17,"ok":true,"name":"response"},`, 2500)
)

func TestRenderTextOverGlyphBudgetDraws(t *testing.T) {
	white := gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)}
	cases := []struct {
		name string
		view func(*gui.Window) gui.View
	}{
		{"text multiline", func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
				gui.Text(gui.TextCfg{Text: overBudget, Mode: gui.TextModeMultiline, TextStyle: white}),
			}})
		}},
		{"text wrap one long line", func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
				gui.Text(gui.TextCfg{Text: oneLongLine, Mode: gui.TextModeWrap, TextStyle: white}),
			}})
		}},
		{"input single line", func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
				gui.Input(gui.InputCfg{ID: "f", Text: strings.Repeat("x", glyph.MaxTextLength+1), TextStyle: white}),
			}})
		}},
		{"input multiline scrollable", func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
				gui.Input(gui.InputCfg{
					ID: "f", Text: overBudget, Mode: gui.InputMultiline, Scrollable: true,
					Sizing: gui.FillFixed, Height: 200, TextStyle: white,
				}),
			}})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			win := newWin(t, 400, 300, tc.view)
			img, err := RenderToImage(win, 1)
			if err != nil {
				t.Fatalf("RenderToImage: %v", err)
			}
			lit := countLitPixels(img)
			if lit == 0 {
				t.Fatal("no text pixels rendered")
			}
			t.Logf("lit pixels: %d", lit)
		})
	}
}

// shapeCounter wraps the window's measurer and counts the layouts it is
// asked to shape.
type shapeCounter struct {
	gui.TextMeasurer
	layouts int
}

func (c *shapeCounter) LayoutText(
	text string, style gui.TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	c.layouts++
	return c.TextMeasurer.LayoutText(text, style, wrapWidth)
}

// TestRenderLargeTextRebuildIsCheap is the spike's exit criterion: once
// the block is shaped, a rebuild of a 100 KB wrapped text re-shapes
// nothing, so a frame costs what its visible lines cost to rasterize.
// The claim is asserted as a count of shaper calls; the timing is only
// logged, since it depends on the machine and on -race.
func TestRenderLargeTextRebuildIsCheap(t *testing.T) {
	text := strings.Repeat("the quick brown fox jumps over the lazy dog ", 2300) // ~100 KB, one paragraph
	win := newWin(t, 600, 400, func(*gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Scrollable: true, ID: "scroll", Content: []gui.View{
			gui.Text(gui.TextCfg{ID: "big", Text: text, Mode: gui.TextModeWrap,
				TextStyle: gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)}}),
		}})
	})
	first := time.Now()
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first render: %v", err)
	}
	warm := time.Since(first)
	tm, ok := win.TextMeasurer().(*textMeasurer)
	if !ok {
		t.Fatal("RenderToImage installed no soft text measurer")
	}
	// Layout metrics come from the window's measurer and drawing from
	// tm, so the counter sees every layout the frame asks to shape.
	counter := &shapeCounter{TextMeasurer: tm}
	win.SetTextMeasurer(counter)
	const frames = 10
	start := time.Now()
	for range frames {
		win.InvalidateLayout()
		if _, err := renderFrame(win, tm, 1); err != nil {
			t.Fatalf("render: %v", err)
		}
	}
	t.Logf("first frame %v, then %v per rebuilt frame",
		warm, time.Since(start)/frames)
	if counter.layouts != 0 {
		t.Errorf("%d rebuilt frames shaped %d layouts, want none",
			frames, counter.layouts)
	}
}

// TestPasteIssueResponseLandsAndDraws is #1187 end to end on the real
// shaper: the issue's 92,359-byte API response pasted into a multiline
// Input lands whole, where it used to be cut at 65,536 runes, and draws,
// where anything past 10,240 bytes used to draw nothing.
func TestPasteIssueResponseLandsAndDraws(t *testing.T) {
	text := ""
	win := newWin(t, 500, 300, func(*gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
			gui.Input(gui.InputCfg{
				ID: "resp", Text: text, Mode: gui.InputMultiline, Scrollable: true,
				Sizing: gui.FillFixed, Height: 240,
				TextStyle:     gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)},
				OnTextChanged: func(s string, _ gui.EventCtx) { text = s },
			}),
		}})
	})
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first render: %v", err)
	}
	response := oneLongLine[:92_359]
	win.SetClipboardGetFn(func() string { return response })
	if err := win.TestKey("resp", gui.KeyV, gui.ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if text != response {
		t.Fatalf("pasted text has %d bytes, want the whole %d", len(text), len(response))
	}
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("render after paste: %v", err)
	}
	if lit := countLitPixels(img); lit == 0 {
		t.Fatal("the pasted response draws nothing")
	} else {
		t.Logf("lit pixels: %d", lit)
	}
}

// TestInputCutPasteAcrossThreeChunks pins the complete widget path at a
// layout seam. Each 1,536-byte paragraph fits the production 2,048-byte
// chunk budget, while no two fit together, so the initial value is exactly
// three shaped chunks. Selection and editing still speak whole-text byte
// offsets: select-all, cut and paste must neither lose nor duplicate a byte.
func TestInputCutPasteAcrossThreeChunks(t *testing.T) {
	const paragraphBytes = 1536
	seed := strings.Repeat("a", paragraphBytes) + "\n" +
		strings.Repeat("b", paragraphBytes) + "\n" +
		strings.Repeat("c", paragraphBytes)
	text := seed
	var changes []string
	var clipboard string
	win := newWin(t, 900, 1100, func(*gui.Window) gui.View {
		return gui.Input(gui.InputCfg{
			ID: "f", Text: text, Mode: gui.InputMultiline, Scrollable: true,
			Sizing: gui.FillFixed, Height: 1040,
			TextStyle: gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)},
			OnTextChanged: func(s string, _ gui.EventCtx) {
				text = s
				changes = append(changes, s)
			},
		})
	})
	win.SetClipboardFn(func(s string) { clipboard = s })
	win.SetClipboardGetFn(func() string { return clipboard })
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first render: %v", err)
	}

	// Rendering one command for each paragraph proves this test reached the
	// intended three production-sized chunks rather than only a large string.
	chunks := 0
	for _, cmd := range win.Renderers() {
		if cmd.Kind == gui.RenderLayout || cmd.Kind == gui.RenderLayoutTransformed {
			chunks++
		}
	}
	if chunks != 3 {
		t.Fatalf("initial value rendered %d layout chunks, want 3", chunks)
	}

	if err := win.TestKey("f", gui.KeyA, gui.ModCtrl); err != nil {
		t.Fatalf("select all: %v", err)
	}
	if err := win.TestKey("f", gui.KeyX, gui.ModCtrl); err != nil {
		t.Fatalf("cut: %v", err)
	}
	if clipboard != seed {
		t.Fatalf("cut copied %d bytes, want all %d", len(clipboard), len(seed))
	}
	if text != "" {
		t.Fatalf("cut left %d bytes, want empty", len(text))
	}
	if err := win.TestKey("f", gui.KeyV, gui.ModCtrl); err != nil {
		t.Fatalf("paste: %v", err)
	}
	if text != seed {
		t.Fatalf("paste restored %d bytes, want the original %d", len(text), len(seed))
	}
	if len(changes) != 2 || changes[0] != "" || changes[1] != seed {
		t.Fatalf("OnTextChanged values = %d entries, want cut then exact paste", len(changes))
	}

	// Pasting collapses the selection and leaves the caret after the insert.
	// A following character must append, not replace any restored chunk.
	if err := win.TestType("f", "!"); err != nil {
		t.Fatalf("type after paste: %v", err)
	}
	if text != seed+"!" {
		t.Fatalf("type after paste produced %d bytes, want %d-byte append",
			len(text), len(seed)+1)
	}
	img, err := RenderToImage(win, 1)
	if err != nil {
		t.Fatalf("render restored value: %v", err)
	}
	if lit := countLitPixels(img); lit == 0 {
		t.Fatal("restored three-chunk value draws nothing")
	}

	// Input remains controlled: replacing InputCfg.Text through application
	// state is the public setter and must not fire the user-edit callback.
	text = "externally set value"
	win.InvalidateLayout()
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("render externally set value: %v", err)
	}
	if len(changes) != 3 {
		t.Fatalf("external Text update fired OnTextChanged: %d calls, want 3", len(changes))
	}
	renderedExternal := false
	for _, cmd := range win.Renderers() {
		if strings.Contains(cmd.Text, text) {
			renderedExternal = true
			break
		}
	}
	if !renderedExternal {
		t.Fatal("externally set InputCfg.Text was not rendered")
	}
}

// TestSingleLineInputKeepsOneLineHeight: a single-line Input keeps one
// line's height whatever it holds. Text past one chunk takes the chunked
// layout, which stacks lines; a paste with line breaks into a
// single-line field made the field grow by every line it held.
func TestSingleLineInputKeepsOneLineHeight(t *testing.T) {
	heightWith := func(text string) float32 {
		win := newWin(t, 600, 200, func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{Sizing: gui.FillFill, SizeBorder: gui.NoBorder, Content: []gui.View{
				gui.Input(gui.InputCfg{ID: "f", Text: text, Sizing: gui.FillFit}),
			}})
		})
		if _, err := RenderToImage(win, 1); err != nil {
			t.Fatal(err)
		}
		ly, ok := win.TestRender(nil).FindByID("f")
		if !ok {
			t.Fatal("no field")
		}
		return ly.Shape.Height
	}
	lines := strings.Repeat("a line of pasted text\n", 200) // 4,400 bytes
	want := heightWith("short")
	for name, text := range map[string]string{
		"one chunk of lines":    lines[:2000],
		"several chunks":        lines,
		"past the shaper's cap": strings.Repeat(lines, 3),
		"one long line":         strings.Repeat("x", 5000),
	} {
		if got := heightWith(text); got != want {
			t.Errorf("%s: field height %.1f, want the one-line %.1f", name, got, want)
		}
	}
}

// A long mixed-direction value exercises real bidi cursor geometry past the
// chunk budget. End must scroll the logical tail into the field; an early
// shape-box cull used to drop both the visible tail and its caret once the
// Fill-width text leaf had moved left.
func TestSingleLineLargeMixedDirectionCaretStaysVisible(t *testing.T) {
	text := strings.Repeat("latin \u05d0\u05d1\u05d2 text ", 7000) + "VISIBLE-TAIL"
	win := newWin(t, 400, 100, func(*gui.Window) gui.View {
		return gui.Column(gui.ContainerCfg{
			Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
			Content: []gui.View{gui.Input(gui.InputCfg{
				ID: "f", Text: text, Sizing: gui.FillFit,
				TextStyle: gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)},
			})},
		})
	})
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first render: %v", err)
	}
	if err := win.TestKey("f", gui.KeyEnd, gui.ModNone); err != nil {
		t.Fatalf("End: %v", err)
	}
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("render at end: %v", err)
	}

	tailDrawn := false
	caretVisible := false
	for _, cmd := range win.Renderers() {
		if (cmd.Kind == gui.RenderLayout ||
			cmd.Kind == gui.RenderLayoutTransformed) &&
			strings.Contains(cmd.Text, "VISIBLE-TAIL") {
			tailDrawn = true
		}
		if cmd.Kind == gui.RenderRect && cmd.W > 1 && cmd.W < 2 &&
			cmd.H > 5 && cmd.X >= 0 && cmd.X+cmd.W <= 400 {
			caretVisible = true
		}
	}
	if !tailDrawn {
		t.Error("the mixed-direction logical tail is not drawn in the field")
	}
	if !caretVisible {
		t.Error("the caret at the mixed-direction logical end is not visible")
	}
}

// Arrow motion through bidi runs is visual rather than monotonic in logical
// byte offsets. Caret following must use each shaped visual X and keep it in
// the viewport while the direction changes and after word moves cross a
// chunk seam.
func TestSingleLineMixedDirectionArrowCaretStaysVisible(t *testing.T) {
	text := strings.Repeat("abc \u05d0\u05d1\u05d2 def \u05d3\u05d4\u05d5 ", 500)
	win := newWin(t, 400, 100, func(*gui.Window) gui.View {
		return gui.Input(gui.InputCfg{
			ID: "f", Text: text, Sizing: gui.FillFit,
			TextStyle: gui.TextStyle{Size: 16, Color: gui.RGB(255, 255, 255)},
		})
	})
	if _, err := RenderToImage(win, 1); err != nil {
		t.Fatalf("first render: %v", err)
	}

	assertCaretVisible := func(step int) {
		t.Helper()
		for _, cmd := range win.Renderers() {
			if cmd.Kind == gui.RenderRect && cmd.W > 1 && cmd.W < 2 &&
				cmd.H > 5 && cmd.X >= 0 && cmd.X+cmd.W <= 400 {
				return
			}
		}
		t.Fatalf("step %d: no caret inside the input viewport", step)
	}

	for step := 1; step <= 160; step++ {
		if err := win.TestKey("f", gui.KeyRight, gui.ModNone); err != nil {
			t.Fatalf("Right at step %d: %v", step, err)
		}
		assertCaretVisible(step)
	}
	for step := 161; step <= 420; step++ {
		if err := win.TestKey("f", gui.KeyRight, gui.ModCtrl); err != nil {
			t.Fatalf("Ctrl+Right at step %d: %v", step, err)
		}
		assertCaretVisible(step)
	}
}

// A gradient text is one layout up to the shaper's budget and plain past
// it, in the real shaper as in the stub: both draw, and one layout takes
// the room the chunks of the same text take.
func TestGradientTextAroundGlyphBudget(t *testing.T) {
	gradient := &glyph.GradientConfig{
		Direction: glyph.GradientVertical,
		Stops: []glyph.GradientStop{
			{Color: glyph.Color{R: 255, A: 255}, Position: 0},
			{Color: glyph.Color{B: 255, A: 255}, Position: 1},
		},
	}
	white := gui.RGB(255, 255, 255)
	render := func(text string, g *glyph.GradientConfig) (int, float32) {
		t.Helper()
		win := newWin(t, 600, 400, func(*gui.Window) gui.View {
			return gui.Column(gui.ContainerCfg{
				Sizing: gui.FillFill, SizeBorder: gui.NoBorder,
				Content: []gui.View{gui.Text(gui.TextCfg{
					ID: "t", Text: text, Mode: gui.TextModeWrap,
					TextStyle: gui.TextStyle{Size: 16, Color: white, Gradient: g},
				})},
			})
		})
		img, err := RenderToImage(win, 1)
		if err != nil {
			t.Fatal(err)
		}
		sh, ok := win.TestRender(nil).FindByID("t")
		if !ok {
			t.Fatal("text not found")
		}
		return countLitPixels(img), sh.Shape.Height
	}
	para := "The quick brown fox jumps over the lazy dog.\n\n"
	long := strings.Repeat(para, glyph.MaxTextLength/len(para)+2)
	for _, n := range []int{5000, glyph.MaxTextLength, glyph.MaxTextLength + 1} {
		lit, h := render(long[:n], gradient)
		if lit == 0 {
			t.Errorf("%d bytes with a gradient drew nothing", n)
		}
		if _, plainH := render(long[:n], nil); h != plainH {
			t.Errorf("%d bytes: height %v with a gradient, %v without", n, h, plainH)
		}
	}
}
