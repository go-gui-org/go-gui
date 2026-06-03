package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// --- renderLayout tree walking ---

func TestRenderLayoutColorFilterBracket(t *testing.T) {
	w := makeWindow()
	cf := &ColorFilter{}
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		Width:     50, Height: 50,
		FX: &ShapeEffects{ColorFilter: cf},
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 100, 100)

	renderLayout(layout, ColorTransparent, clip, w)

	if w.inFilter {
		t.Error("inFilter should be restored to false")
	}
	foundBegin := false
	foundEnd := false
	for _, r := range w.renderers {
		if r.Kind == RenderFilterBegin {
			foundBegin = true
		}
		if r.Kind == RenderFilterEnd {
			foundEnd = true
		}
	}
	if !foundBegin {
		t.Error("missing RenderFilterBegin")
	}
	if !foundEnd {
		t.Error("missing RenderFilterEnd")
	}
}

func TestRenderLayoutColorFilterNilFX(t *testing.T) {
	w := makeWindow()
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		Width:     50, Height: 50,
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 100, 100)

	renderLayout(layout, ColorTransparent, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderFilterBegin || r.Kind == RenderFilterEnd {
			t.Error("should not emit filter brackets with nil FX")
		}
	}
}

func TestRenderLayoutOverDrawVertical(t *testing.T) {
	w := makeWindow()
	parentClip := makeClip(0, 0, 200, 300)
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         10, Y: 20,
		Width: 50, Height: 50,
		OverDraw:             true,
		ScrollbarOrientation: ScrollbarVertical,
		ShapeClip:            DrawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	layout := &Layout{Shape: shape}

	renderLayout(layout, ColorTransparent, parentClip, w)

	// Find the first RenderClip emitted for OverDraw.
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			// Vertical scrollbar: Y and Height come from parent clip.
			if r.Y != parentClip.Y {
				t.Errorf("clip Y = %f, want %f", r.Y, parentClip.Y)
			}
			if r.H != parentClip.Height {
				t.Errorf("clip H = %f, want %f", r.H, parentClip.Height)
			}
			return
		}
	}
	t.Error("no RenderClip emitted for OverDraw")
}

func TestRenderLayoutOverDrawHorizontal(t *testing.T) {
	w := makeWindow()
	parentClip := makeClip(0, 0, 200, 300)
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         10, Y: 20,
		Width: 50, Height: 50,
		OverDraw:             true,
		ScrollbarOrientation: ScrollbarHorizontal,
		ShapeClip:            DrawClip{X: 10, Y: 20, Width: 50, Height: 50},
	}
	layout := &Layout{Shape: shape}

	renderLayout(layout, ColorTransparent, parentClip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			if r.X != parentClip.X {
				t.Errorf("clip X = %f, want %f", r.X, parentClip.X)
			}
			if r.W != parentClip.Width {
				t.Errorf("clip W = %f, want %f", r.W, parentClip.Width)
			}
			return
		}
	}
	t.Error("no RenderClip emitted for OverDraw")
}

func TestRenderLayoutOverDrawDefault(t *testing.T) {
	w := makeWindow()
	parentClip := makeClip(0, 0, 200, 300)
	shapeClip := DrawClip{X: 10, Y: 20, Width: 50, Height: 50}
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         10, Y: 20,
		Width: 50, Height: 50,
		OverDraw:             true,
		ScrollbarOrientation: ScrollbarNone,
		ShapeClip:            shapeClip,
	}
	layout := &Layout{Shape: shape}

	renderLayout(layout, ColorTransparent, parentClip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			if r.X != shapeClip.X || r.Y != shapeClip.Y {
				t.Errorf("clip = (%f,%f), want (%f,%f)",
					r.X, r.Y, shapeClip.X, shapeClip.Y)
			}
			return
		}
	}
	t.Error("no RenderClip emitted for OverDraw")
}

func TestRenderLayoutClipRTLPadding(t *testing.T) {
	w := makeWindow()
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         0, Y: 0,
		Width: 100, Height: 50,
		Clip:       true,
		TextDir:    TextDirRTL,
		Padding:    Padding{Left: 5, Right: 10, Top: 3, Bottom: 3},
		SizeBorder: 2,
		ShapeClip:  DrawClip{X: 0, Y: 0, Width: 100, Height: 50},
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			// RTL: padX = Padding.Right + SizeBorder = 10 + 2 = 12
			wantX := float32(12)
			if r.X != wantX {
				t.Errorf("clip X = %f, want %f (RTL)", r.X, wantX)
			}
			return
		}
	}
	t.Error("no RenderClip emitted for Clip+RTL")
}

func TestRenderLayoutRotationBracket(t *testing.T) {
	w := makeWindow()
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         10, Y: 20,
		Width: 60, Height: 40,
		QuarterTurns: 1,
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)

	foundBegin := false
	foundEnd := false
	for _, r := range w.renderers {
		if r.Kind == RenderRotateBegin {
			foundBegin = true
			wantAngle := float32(90)
			if r.RotAngle != wantAngle {
				t.Errorf("angle = %f, want %f", r.RotAngle, wantAngle)
			}
			wantCX := float32(10) + 60/2
			wantCY := float32(20) + 40/2
			if r.RotCX != wantCX || r.RotCY != wantCY {
				t.Errorf("center = (%f,%f), want (%f,%f)",
					r.RotCX, r.RotCY, wantCX, wantCY)
			}
		}
		if r.Kind == RenderRotateEnd {
			foundEnd = true
		}
	}
	if !foundBegin {
		t.Error("missing RenderRotateBegin")
	}
	if !foundEnd {
		t.Error("missing RenderRotateEnd")
	}
}

func TestRenderLayoutRotationZeroNoOp(t *testing.T) {
	w := makeWindow()
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		X:         10, Y: 20,
		Width: 60, Height: 40,
		QuarterTurns: 0,
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderRotateBegin || r.Kind == RenderRotateEnd {
			t.Error("should not emit rotation cmds for QuarterTurns=0")
		}
	}
}

func TestRenderLayoutChildBgColorInheritance(t *testing.T) {
	w := makeWindow()
	parentColor := RGB(200, 100, 50)
	child := &Shape{
		ShapeType: ShapeRectangle,
		Color:     ColorTransparent,
		Width:     20, Height: 20,
	}
	parent := &Shape{
		ShapeType: ShapeRectangle,
		Color:     parentColor,
		Width:     100, Height: 100,
		Opacity: 1.0,
	}
	layout := &Layout{
		Shape:    parent,
		Children: []Layout{{Shape: child}},
	}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)

	// Parent rect should be emitted. Child is transparent so
	// should not emit. No panic is the key assertion.
	foundParent := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.W == 100 && r.H == 100 {
			foundParent = true
		}
	}
	if !foundParent {
		t.Error("parent rect not found")
	}
}

func TestRenderLayoutStencilDepthClampsAt255(t *testing.T) {
	w := makeWindow()
	w.stencilDepth = 254
	shape := &Shape{
		ShapeType: ShapeRectangle,
		Color:     RGB(100, 100, 100),
		Width:     50, Height: 50,
		ClipContents: true,
	}
	layout := &Layout{Shape: shape}
	clip := makeClip(0, 0, 200, 200)

	renderLayout(layout, ColorTransparent, clip, w)

	// Should increment to 255 and decrement back to 254.
	if w.stencilDepth != 254 {
		t.Errorf("stencilDepth = %d, want 254", w.stencilDepth)
	}

	// Verify the stencil bracket was emitted with depth 255.
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderStencilBegin && r.StencilDepth == 255 {
			found = true
		}
	}
	if !found {
		t.Error("expected RenderStencilBegin with depth 255")
	}
}

// --- renderText nil-safe fallbacks ---

func TestRenderTextEmptyFocusedEmitsCursor(t *testing.T) {
	w := makeWindowWithScratch()
	w.viewState.idFocus = 100
	w.viewState.inputCursorOn = true
	style := DefaultTextStyle
	shape := &Shape{
		ShapeType: ShapeText,
		IDFocus:   100,
		Width:     100, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:      "",
			TextStyle: &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderText(shape, clip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill {
			found = true
		}
	}
	if !found {
		t.Error("cursor rect not emitted for empty focused text")
	}
}

func TestRenderTextEmptyUnfocusedNoOutput(t *testing.T) {
	w := makeWindowWithScratch()
	style := DefaultTextStyle
	shape := &Shape{
		ShapeType: ShapeText,
		IDFocus:   100,
		Width:     100, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:      "",
			TextStyle: &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderText(shape, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for empty unfocused text",
			len(w.renderers))
	}
}

func TestRenderTextOutsideClipSkips(t *testing.T) {
	w := makeWindowWithScratch()
	style := TextStyle{Color: RGB(255, 255, 255), Size: 16}
	shape := &Shape{
		ShapeType: ShapeText,
		X:         500, Y: 500,
		Width: 100, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:      "hello",
			TextStyle: &style,
		},
	}
	clip := makeClip(0, 0, 100, 100)

	renderText(shape, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for out-of-clip text",
			len(w.renderers))
	}
}

func TestRenderTextZeroAlphaSkips(t *testing.T) {
	w := makeWindowWithScratch()
	style := TextStyle{Color: RGBA(255, 255, 255, 0), Size: 16}
	shape := &Shape{
		ShapeType: ShapeText,
		Width:     100, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:      "invisible",
			TextStyle: &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderText(shape, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for zero alpha text",
			len(w.renderers))
	}
}

func TestRenderTextPlaceholderCursor(t *testing.T) {
	w := makeWindowWithScratch()
	w.viewState.idFocus = 200
	w.viewState.inputCursorOn = true
	style := TextStyle{Color: RGB(128, 128, 128), Size: 16}
	shape := &Shape{
		ShapeType: ShapeText,
		IDFocus:   200,
		Width:     100, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			Text:              "Enter name",
			TextIsPlaceholder: true,
			TextStyle:         &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderText(shape, clip, w)

	// Should emit text + cursor. Cursor uses empty string for
	// positioning when placeholder.
	foundText := false
	foundCursor := false
	for _, r := range w.renderers {
		if r.Kind == RenderText {
			foundText = true
		}
		if r.Kind == RenderRect && r.Fill && r.W < 5 {
			foundCursor = true
		}
	}
	if !foundText {
		t.Error("placeholder text not emitted")
	}
	if !foundCursor {
		t.Error("cursor not emitted for focused placeholder")
	}
}

// --- renderInputCursor fallback ---

func TestRenderInputCursorNotFocusedSkips(t *testing.T) {
	w := makeWindow()
	w.viewState.idFocus = 999
	style := DefaultTextStyle
	shape := &Shape{
		IDFocus: 100,
		TC:      &ShapeTextConfig{TextStyle: &style},
	}

	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)

	if len(w.renderers) != 0 {
		t.Error("cursor should not render when not focused")
	}
}

func TestRenderInputCursorBlinkOffSkips(t *testing.T) {
	w := makeWindow()
	w.viewState.idFocus = 100
	w.viewState.inputCursorOn = false
	style := DefaultTextStyle
	shape := &Shape{
		IDFocus: 100,
		TC:      &ShapeTextConfig{TextStyle: &style},
	}

	renderInputCursor(shape, "hello", 0, 0, glyph.Layout{}, false, w)

	if len(w.renderers) != 0 {
		t.Error("cursor should not render when blink off")
	}
}

func TestRenderInputCursorFallbackPosition(t *testing.T) {
	w := makeWindow()
	w.viewState.idFocus = 100
	w.viewState.inputCursorOn = true
	style := TextStyle{Color: RGB(0, 0, 0), Size: 14}
	shape := &Shape{
		IDFocus: 100,
		TC:      &ShapeTextConfig{TextStyle: &style},
	}
	setInputState(w, 100, InputState{CursorPos: 3})

	renderInputCursor(shape, "hello", 10, 20, glyph.Layout{}, false, w)

	if len(w.renderers) == 0 {
		t.Fatal("expected cursor rect via fallback")
	}
	r := w.renderers[0]
	// Fallback: X = baseX + runeCount("hel") * size * 0.6
	wantX := float32(10) + 3*14*0.6
	if !f32AreClose(r.X, wantX) {
		t.Errorf("cursor X = %f, want %f", r.X, wantX)
	}
	if r.Y != 20 {
		t.Errorf("cursor Y = %f, want 20", r.Y)
	}
}

// --- renderInputSelection fallback ---

func TestRenderInputSelectionEqualBegEndSkips(t *testing.T) {
	w := makeWindow()
	style := DefaultTextStyle
	shape := &Shape{
		TC: &ShapeTextConfig{
			Text:       "hello",
			TextSelBeg: 2,
			TextSelEnd: 2,
			TextStyle:  &style,
		},
	}

	renderInputSelection(shape, "hello", 0, 0,
		glyph.Layout{}, false, w)

	if len(w.renderers) != 0 {
		t.Error("no selection highlight for equal beg/end")
	}
}

func TestRenderInputSelectionFallbackEmitsRect(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Color: RGB(0, 0, 0), Size: 14}
	shape := &Shape{
		TC: &ShapeTextConfig{
			Text:       "hello",
			TextSelBeg: 1,
			TextSelEnd: 3,
			TextStyle:  &style,
		},
	}

	renderInputSelection(shape, "hello", 10, 20,
		glyph.Layout{}, false, w)

	if len(w.renderers) != 1 {
		t.Fatalf("got %d renderers, want 1 selection rect",
			len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderRect || !r.Fill {
		t.Error("expected filled rect for selection highlight")
	}
	// Fallback: width proportional to rune positions.
	x0 := float32(1) * 14 * 0.6
	x1 := float32(3) * 14 * 0.6
	wantW := x1 - x0
	if !f32AreClose(r.W, wantW) {
		t.Errorf("selection W = %f, want %f", r.W, wantW)
	}
}

// --- renderRtf: RtfFlatText selection highlight ---

// makeCharLayout builds a glyph.Layout with CharRects for each byte of text
// (ASCII assumed: one byte per char). Each char is 10px wide, 20px tall.
// GetSelectionRects uses CharRects/Lines, not Items, so this is sufficient.
func makeCharLayout(text string) glyph.Layout {
	n := len(text)
	charRects := make([]glyph.CharRect, n)
	charRectByIndex := make(map[int]int, n)
	for i := range n {
		charRects[i] = glyph.CharRect{
			Rect:  glyph.Rect{X: float32(i) * 10, Y: 0, Width: 10, Height: 20},
			Index: i,
		}
		charRectByIndex[i] = i
	}
	return glyph.Layout{
		Text:            text,
		CharRects:       charRects,
		CharRectByIndex: charRectByIndex,
		Lines: []glyph.Line{{
			StartIndex: 0,
			Length:     n,
			Rect:       glyph.Rect{X: 0, Y: 0, Width: float32(n) * 10, Height: 20},
		}},
		Width:  float32(n) * 10,
		Height: 20,
	}
}

func TestRenderRtf_SelectionHighlight_Emitted(t *testing.T) {
	w := makeWindow()
	const text = "hello world"
	gl := makeCharLayout(text)
	style := DefaultTextStyle
	shape := &Shape{
		ShapeType: ShapeRTF,
		Width:     float32(len(text)) * 10, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			RtfLayout:   &gl,
			RtfFlatText: text,
			TextSelBeg:  2,
			TextSelEnd:  7,
			TextStyle:   &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)
	renderRtf(shape, clip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill {
			found = true
		}
	}
	if !found {
		t.Error("expected selection highlight rect when RtfFlatText is set and selection is non-empty")
	}
}

func TestRenderRtf_EmptyRtfFlatText_NoSelectionHighlight(t *testing.T) {
	w := makeWindow()
	const text = "hello world"
	gl := makeCharLayout(text)
	style := DefaultTextStyle
	shape := &Shape{
		ShapeType: ShapeRTF,
		Width:     float32(len(text)) * 10, Height: 20,
		Opacity: 1.0,
		TC: &ShapeTextConfig{
			RtfLayout:   &gl,
			RtfFlatText: "", // empty → renderInputSelection not called
			TextSelBeg:  2,
			TextSelEnd:  7,
			TextStyle:   &style,
		},
	}
	clip := makeClip(0, 0, 200, 200)
	renderRtf(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderRect && r.Fill {
			t.Error("should not emit selection highlight when RtfFlatText is empty")
			return
		}
	}
}

// --- Utility functions ---

func TestTextStyleOrDefaultNilTC(t *testing.T) {
	shape := &Shape{}
	got := textStyleOrDefault(shape)
	if got != DefaultTextStyle {
		t.Error("expected DefaultTextStyle for nil TC")
	}
}

func TestTextStyleOrDefaultNilTextStyle(t *testing.T) {
	shape := &Shape{TC: &ShapeTextConfig{}}
	got := textStyleOrDefault(shape)
	if got != DefaultTextStyle {
		t.Error("expected DefaultTextStyle for nil TextStyle")
	}
}

func TestFontHeightFallbackZeroSize(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Size: 0}
	got := fontHeight(style, w)
	if got != 16 {
		t.Errorf("fontHeight = %f, want 16", got)
	}
}

func TestFontHeightFallbackWithSize(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Size: 14}
	got := fontHeight(style, w)
	want := float32(14) * 1.2
	if !f32AreClose(got, want) {
		t.Errorf("fontHeight = %f, want %f", got, want)
	}
}

func TestTextWidthFallbackPasswordMask(t *testing.T) {
	w := makeWindow()
	style := TextStyle{Size: 14}
	tc := &ShapeTextConfig{TextIsPassword: true}
	got := textWidthFallback("abc", 3, tc, style, w)
	// Password: 3 mask chars * size * 0.6
	want := float32(3) * 14 * 0.6
	if !f32AreClose(got, want) {
		t.Errorf("textWidthFallback = %f, want %f", got, want)
	}
}
