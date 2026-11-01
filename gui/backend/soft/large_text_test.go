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
