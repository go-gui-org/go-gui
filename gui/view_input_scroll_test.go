package gui

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// caretStubMeasurer is a fixed-advance measurer that returns a fully
// populated glyph.Layout: lines, per-rune rects and cursor attributes.
// stubTextMeasurer only fills Height, which is enough for the wrap
// passes but not for GetCursorPos, and the horizontal follow is built
// entirely out of cursor geometry.
//
// Wrapping matches glyph.WrapWord, which is the only mode go-gui asks
// for: breaks happen at spaces, and a word wider than the wrap width
// is left to overflow rather than broken. That overflow is the multiline
// half of issue #534.
type caretStubMeasurer struct {
	charWidth  float32
	fontHeight float32
	// measured records every string TextWidth was asked about, so a
	// test can assert which spelling of the text was sized.
	measured []string
}

func (m *caretStubMeasurer) TextWidth(text string, _ TextStyle) float32 {
	m.measured = append(m.measured, text)
	return float32(len([]rune(text))) * m.charWidth
}

func (m *caretStubMeasurer) TextHeight(_ string, _ TextStyle) float32 {
	return m.fontHeight
}

func (m *caretStubMeasurer) FontAscent(s TextStyle) float32 {
	return s.Size * 0.8
}

func (m *caretStubMeasurer) FontHeight(_ TextStyle) float32 {
	return m.fontHeight
}

// caretStubLine is one wrapped line: byte range plus rune count.
type caretStubLine struct {
	start, end, runes int
}

// wrapWords splits text into lines the way WrapWord does.
func (m *caretStubMeasurer) wrapWords(
	text string, wrapWidth float32,
) []caretStubLine {
	if wrapWidth <= 0 {
		return []caretStubLine{{0, len(text), len([]rune(text))}}
	}
	var lines []caretStubLine
	var cur caretStubLine
	pos, first := 0, true
	for word := range strings.SplitSeq(text, " ") {
		runes := len([]rune(word))
		if first {
			cur = caretStubLine{0, len(word), runes}
			pos, first = len(word), false
			continue
		}
		// The joining space counts toward the line it stays on.
		if float32(cur.runes+1+runes)*m.charWidth > wrapWidth {
			lines = append(lines, cur)
			cur = caretStubLine{pos + 1, pos + 1 + len(word), runes}
		} else {
			cur.runes += 1 + runes
			cur.end = pos + 1 + len(word)
		}
		pos += 1 + len(word)
	}
	return append(lines, cur)
}

func (m *caretStubMeasurer) LayoutText(
	text string, _ TextStyle, wrapWidth float32,
) (glyph.Layout, error) {
	l := glyph.Layout{
		Text:            text,
		CharRectByIndex: map[int]int{},
		LogAttrByIndex:  map[int]int{},
	}
	if text == "" {
		l.Height = m.fontHeight
		return l, nil
	}
	// Every byte that starts a rune is a valid cursor position; nothing
	// inside a rune is. This is what makes a byte index taken against
	// the wrong string (raw text vs password bullets) fail loudly.
	for byteIdx := range text {
		l.LogAttrByIndex[byteIdx] = len(l.LogAttrs)
		l.LogAttrs = append(l.LogAttrs,
			glyph.LogAttr{IsCursorPosition: true})
	}
	l.LogAttrByIndex[len(text)] = len(l.LogAttrs)
	l.LogAttrs = append(l.LogAttrs, glyph.LogAttr{IsCursorPosition: true})

	for row, line := range m.wrapWords(text, wrapWidth) {
		y := float32(row) * m.fontHeight
		col := 0
		for byteIdx := range text[line.start:line.end] {
			abs := line.start + byteIdx
			l.CharRectByIndex[abs] = len(l.CharRects)
			l.CharRects = append(l.CharRects, glyph.CharRect{
				Index: abs,
				Rect: glyph.Rect{
					X: float32(col) * m.charWidth, Y: y,
					Width: m.charWidth, Height: m.fontHeight,
				},
			})
			col++
		}
		lineW := float32(line.runes) * m.charWidth
		l.Lines = append(l.Lines, glyph.Line{
			StartIndex: line.start,
			Length:     line.end - line.start,
			Rect: glyph.Rect{
				Y: y, Width: lineW, Height: m.fontHeight,
			},
		})
		l.Width = f32Max(l.Width, lineW)
		l.Height = y + m.fontHeight
	}
	l.VisualWidth, l.VisualHeight = l.Width, l.Height
	return l, nil
}

// arrangeInput builds and arranges one Input filling a fixed window,
// and returns the arranged field layout (the container claiming cfg.ID).
func arrangeInput(
	t *testing.T, w *Window, cfg InputCfg,
) (*Layout, *Layout) {
	t.Helper()
	// A default Input hugs its content, which would make the field as
	// wide as the text and leave nothing to scroll. Pin it to the
	// window, the way a real form's Fill-width field resolves.
	if !cfg.Sizing.IsSet() {
		cfg.Sizing = FixedFixed
		cfg.Width = float32(w.windowWidth)
		cfg.Height = float32(w.windowHeight)
		cfg.noMinWidthFloor = true
	}
	// Fixed, not FillFill: only updateLayout seeds a FillFill root from
	// the window rect, and these tests arrange the tree directly.
	root := generateViewLayout(Column(ContainerCfg{
		Sizing:     FixedFixed,
		Width:      float32(w.windowWidth),
		Height:     float32(w.windowHeight),
		Padding:    NoPadding,
		SizeBorder: NoBorder,
		Content:    []View{Input(cfg)},
	}), w)
	layoutArrange(&root, w)
	field := findLayoutByID(&root, cfg.ID)
	if field == nil {
		t.Fatalf("no layout with ID %q after arrange", cfg.ID)
	}
	return &root, field
}

// findLayoutByID returns the first layout whose effective identity is
// key, or nil.
func findLayoutByID(layout *Layout, key string) *Layout {
	if layout.Shape != nil && layout.Shape.idKey() == key {
		return layout
	}
	for i := range layout.Children {
		if found := findLayoutByID(&layout.Children[i], key); found != nil {
			return found
		}
	}
	return nil
}

func caretTestWindow(width, height int) *Window {
	w := &Window{windowWidth: width, windowHeight: height, focused: true}
	w.SetTextMeasurer(&caretStubMeasurer{charWidth: 10, fontHeight: 20})
	return w
}

// --- Single line ---

// TestInputSingleLineScrollFollowsCursorRight is the core of the
// single-line half of #534: a caret past the right edge pulls the view
// after it instead of disappearing.
func TestInputSingleLineScrollFollowsCursorRight(t *testing.T) {
	w := caretTestWindow(200, 60)
	text := strings.Repeat("a", 60)
	_, field := arrangeInput(t, w, InputCfg{ID: "in", Text: text})
	setInputState(w, "in", inputState{CursorPos: len(text)})

	inputScrollCursorIntoView("in", text, field, w)

	offset := w.scrollX().GetOr("in", 0)
	if offset >= 0 {
		t.Fatalf("scrollX = %v, want negative (view follows the caret)", offset)
	}
	viewportW := field.Shape.Width - field.Shape.paddingWidth()
	want := -(float32(len(text))*10 + inputCaretW - viewportW)
	if !floatNear(offset, want, 0.01) {
		t.Errorf("scrollX = %v, want %v (caret flush at the right edge)",
			offset, want)
	}
}

// TestInputSingleLineScrollFollowsCursorLeft pins the return trip:
// moving the caret home scrolls all the way back to 0.
func TestInputSingleLineScrollFollowsCursorLeft(t *testing.T) {
	w := caretTestWindow(200, 60)
	text := strings.Repeat("a", 60)
	_, field := arrangeInput(t, w, InputCfg{ID: "in", Text: text})
	w.scrollX().Set("in", -400)
	setInputState(w, "in", inputState{CursorPos: 0})

	inputScrollCursorIntoView("in", text, field, w)

	if got := w.scrollX().GetOr("in", 0); got != 0 {
		t.Errorf("scrollX = %v, want 0 (caret at the start)", got)
	}
}

// TestInputSingleLineScrollZeroWhenTextFits asserts a field wider than
// its text never scrolls, so short inputs are untouched by the change.
func TestInputSingleLineScrollZeroWhenTextFits(t *testing.T) {
	w := caretTestWindow(400, 60)
	text := "abc"
	_, field := arrangeInput(t, w, InputCfg{ID: "in", Text: text})
	setInputState(w, "in", inputState{CursorPos: len(text)})

	inputScrollCursorIntoView("in", text, field, w)

	if got := w.scrollX().GetOr("in", 0); got != 0 {
		t.Errorf("scrollX = %v, want 0 (text fits)", got)
	}
}

// TestInputScrollXShiftsTextShapeOnly pins the mechanism: the offset
// moves the text leaf and nothing else, so the click target still
// covers the whole field.
func TestInputScrollXShiftsTextShapeOnly(t *testing.T) {
	w := caretTestWindow(200, 60)
	text := strings.Repeat("a", 60)

	_, before := arrangeInput(t, w, InputCfg{ID: "in", Text: text})
	rowX := before.Children[0].Shape.X
	txtX := inputTextShape(t, before).X

	w.scrollX().Set("in", -30)
	_, after := arrangeInput(t, w, InputCfg{ID: "in", Text: text})

	if got := after.Children[0].Shape.X; !floatNear(got, rowX, 0.01) {
		t.Errorf("inner row X = %v, want %v (must not move)", got, rowX)
	}
	if got := inputTextShape(t, after).X; !floatNear(got, txtX-30, 0.01) {
		t.Errorf("text X = %v, want %v", got, txtX-30)
	}
}

// TestInputScrollXIgnoresPlaceholder pins that placeholder text is
// never shifted: it is not the user's text, and the caret sits at its
// start.
func TestInputScrollXIgnoresPlaceholder(t *testing.T) {
	w := caretTestWindow(200, 60)
	_, before := arrangeInput(t, w, InputCfg{
		ID: "in", Placeholder: strings.Repeat("p", 60),
	})
	txtX := inputTextShape(t, before).X

	w.scrollX().Set("in", -30)
	_, after := arrangeInput(t, w, InputCfg{
		ID: "in", Placeholder: strings.Repeat("p", 60),
	})

	if got := inputTextShape(t, after).X; !floatNear(got, txtX, 0.01) {
		t.Errorf("placeholder X = %v, want %v (unshifted)", got, txtX)
	}
}

// TestInputSingleLineInjectsNoScrollbars asserts the single-line field
// stays a plain container: no scrollbar children, and not Scrollable —
// which two sizing rules would otherwise treat as elastic.
func TestInputSingleLineInjectsNoScrollbars(t *testing.T) {
	w := caretTestWindow(200, 60)
	_, field := arrangeInput(t, w, InputCfg{
		ID: "in", Text: strings.Repeat("a", 60),
	})
	if got := len(field.Children); got != 1 {
		t.Errorf("field has %d children, want 1 (no scrollbars)", got)
	}
	if field.Shape.Scrollable {
		t.Error("single-line field must not be Scrollable")
	}
}

// TestInputSingleLineStaleOffsetReclamped pins the re-clamp in
// inputApplyScrollX: nothing else clamps a single-line offset, because
// the field is not Scrollable.
func TestInputSingleLineStaleOffsetReclamped(t *testing.T) {
	w := caretTestWindow(400, 60)
	w.scrollX().Set("in", -900)
	_, field := arrangeInput(t, w, InputCfg{ID: "in", Text: "abc"})

	txt := inputTextShape(t, field)
	if txt.X < field.Shape.X {
		t.Errorf("text X = %v, left of the field at %v: stale offset "+
			"was applied unclamped", txt.X, field.Shape.X)
	}
	// The clamp is stored, not just painted. Leaving -900 in the map
	// would keep every later reader of the offset (the next follow, a
	// drag seed, ScrollHorizontalPct) disagreeing with the screen.
	if got := w.scrollX().GetOr("in", 0); got != 0 {
		t.Errorf("stored scrollX = %v, want 0: the re-clamp must be "+
			"written back, not only applied to the shape", got)
	}
}

// TestInputPasswordScrollUsesMaskedByteIndex pins the password fix.
// The glyph layout is built from bullets, so the caret's byte index has
// to be taken against the mask: a two-byte rune and its three-byte
// bullet do not line up, and the raw index lands mid-rune, where
// GetCursorPos refuses and the follow silently does nothing.
func TestInputPasswordScrollUsesMaskedByteIndex(t *testing.T) {
	w := caretTestWindow(200, 60)
	text := strings.Repeat("é", 40) // 2 bytes per rune, 3 per bullet
	_, field := arrangeInput(t, w, InputCfg{
		ID: "in", Text: text, IsPassword: true,
	})
	setInputState(w, "in", inputState{CursorPos: 40})

	inputScrollCursorIntoView("in", text, field, w)

	if got := w.scrollX().GetOr("in", 0); got >= 0 {
		t.Errorf("scrollX = %v, want negative: the caret is past the "+
			"right edge of a masked 40-bullet field", got)
	}
}

// --- Multiline ---

func multilineCfg(id, text string) InputCfg {
	return InputCfg{
		ID: id, Text: text, Mode: InputMultiline, Scrollable: true,
	}
}

// TestInputMultilineInkOverflowWidensContent is the multiline half of
// #534: a run with no break opportunity overflows the wrap width, and
// the field must be able to scroll to reach it.
func TestInputMultilineInkOverflowWidensContent(t *testing.T) {
	w := caretTestWindow(200, 200)
	text := strings.Repeat("a", 60) // no space: nowhere to break
	_, field := arrangeInput(t, w, multilineCfg("in", text))

	txt := inputTextShape(t, field)
	if txt.inkOverflowW <= txt.Width {
		t.Fatalf("inkOverflowW = %v, want > text width %v",
			txt.inkOverflowW, txt.Width)
	}
	if got := contentWidth(field); got < txt.inkOverflowW {
		t.Errorf("contentWidth(field) = %v, want >= ink %v",
			got, txt.inkOverflowW)
	}
	if got := scrollMaxOffsetX(field); got >= 0 {
		t.Errorf("scrollMaxOffsetX = %v, want negative (scrollable)", got)
	}
}

// TestInputMultilineWrapWidthUnchangedByInkOverflow pins the reason the
// overflow is recorded rather than added to Shape.Width: the shape's
// width IS the wrap width, so growing it would re-wrap the text.
func TestInputMultilineWrapWidthUnchangedByInkOverflow(t *testing.T) {
	w := caretTestWindow(200, 200)
	text := strings.Repeat("a", 60)
	_, field := arrangeInput(t, w, multilineCfg("in", text))

	txt := inputTextShape(t, field)
	widthArg := txt.TC.textLayoutWidth
	if !floatNear(widthArg, txt.Width, 0.01) {
		t.Errorf("cached wrap width %v != shape width %v",
			widthArg, txt.Width)
	}
	style := textStyleOrDefault(txt)
	if _, ok := plainTextLayoutResolved(text, txt, style, w); !ok {
		t.Fatal("second layout resolve failed")
	}
	if !floatNear(txt.TC.textLayoutWidth, widthArg, 0.01) {
		t.Errorf("wrap width moved to %v after a re-resolve, want %v: "+
			"the text would re-wrap every frame",
			txt.TC.textLayoutWidth, widthArg)
	}
}

// TestInputMultilineWrappedTextDoesNotOverflow asserts ordinary text
// with break opportunities claims no overflow, so the horizontal bar
// stays hidden for the common case.
func TestInputMultilineWrappedTextDoesNotOverflow(t *testing.T) {
	w := caretTestWindow(200, 200)
	text := strings.Repeat("ab cd ", 20)
	_, field := arrangeInput(t, w, multilineCfg("in", text))

	txt := inputTextShape(t, field)
	if txt.inkOverflowW != 0 {
		t.Errorf("inkOverflowW = %v, want 0 (text wraps)", txt.inkOverflowW)
	}
	if got := scrollMaxOffsetX(field); got != 0 {
		t.Errorf("scrollMaxOffsetX = %v, want 0 (nothing to scroll)", got)
	}
}

// TestInputMultilineVerticalFollowUnchanged is a regression pin: the
// vertical follow that already worked must keep working now that the
// same function also handles X.
func TestInputMultilineVerticalFollowUnchanged(t *testing.T) {
	w := caretTestWindow(200, 80)
	text := strings.Repeat("ab cd ", 40)
	_, field := arrangeInput(t, w, multilineCfg("in", text))
	setInputState(w, "in", inputState{CursorPos: len([]rune(text))})

	inputScrollCursorIntoView("in", text, field, w)

	if got := w.scrollY().GetOr("in", 0); got >= 0 {
		t.Errorf("scrollY = %v, want negative (caret on a lower line)", got)
	}
}

// --- Drag ---

// TestInputDragAutoScrollHorizontal pins the X half of the drag
// auto-scroll: a pointer carried past the right edge keeps the field
// scrolling under it.
func TestInputDragAutoScrollHorizontal(t *testing.T) {
	w := newTestWindow()
	text := "abcd"
	gl := glyph.Layout{
		Text:            text,
		CharRectByIndex: map[int]int{0: 0, 1: 1, 2: 2, 3: 3},
		Lines: []glyph.Line{{
			StartIndex: 0, Length: 4,
			Rect: glyph.Rect{Width: 40, Height: 10},
		}},
		CharRects: []glyph.CharRect{
			{Index: 0, Rect: glyph.Rect{X: 0, Width: 10, Height: 10}},
			{Index: 1, Rect: glyph.Rect{X: 10, Width: 10, Height: 10}},
			{Index: 2, Rect: glyph.Rect{X: 20, Width: 10, Height: 10}},
			{Index: 3, Rect: glyph.Rect{X: 30, Width: 10, Height: 10}},
		},
	}
	d := &inputDragState{
		displayText: text,
		gl:          gl,
		focusID:     "f",
		scrollID:    "inp",
		lastMouseX:  150, // past viewRight
		lastMouseY:  50,  // inside the vertical band
		viewTop:     0, viewBot: 100,
		viewLeft: 0, viewRight: 100,
		maxScrollNegX: -200,
	}
	d.scrollCallback(nil, w)

	if got := w.scrollX().GetOr("inp", 0); got >= 0 {
		t.Errorf("scrollX = %v, want negative (dragged past the right "+
			"edge)", got)
	}
	if got := w.scrollY().GetOr("inp", 0); got != 0 {
		t.Errorf("scrollY = %v, want 0 (pointer inside the band)", got)
	}
}

// TestInputDragAutoScrollStopsWhenBothAxesInside pins the restructured
// stop condition: the animation ends only when neither axis is still
// dragging past an edge.
func TestInputDragAutoScrollStopsWhenBothAxesInside(t *testing.T) {
	w := newTestWindow()
	d := &inputDragState{
		focusID: "f", scrollID: "inp",
		lastMouseX: 50, lastMouseY: 50,
		viewTop: 0, viewBot: 100,
		viewLeft: 0, viewRight: 100,
	}
	w.AnimationAdd(&Animate{AnimID: animIDDragScroll, Delay: 1})
	d.scrollCallback(nil, w)
	if w.HasAnimation(animIDDragScroll) {
		t.Error("animation must stop once both axes are inside")
	}

	// Still outside on X alone: the animation must survive.
	d.lastMouseX = 150
	w.AnimationAdd(&Animate{AnimID: animIDDragScroll, Delay: 1})
	d.scrollCallback(nil, w)
	if !w.HasAnimation(animIDDragScroll) {
		t.Error("animation must continue while X is still outside")
	}
}

// --- Render ---

// TestRenderTextInkOverflowNotCulled pins the cull fix that goes with
// the scroll: a run that could not wrap paints past its shape's right
// edge, so once the field is scrolled the box can sit entirely left of
// the clip while the visible part of the text does not. Culling on the
// box alone drew nothing at all.
func TestRenderTextInkOverflowNotCulled(t *testing.T) {
	w := makeWindowWithScratch()
	style := TextStyle{Color: RGB(255, 255, 255), Size: 16}
	shape := &Shape{
		shapeType: shapeText,
		X:         -300, Y: 0,
		Width: 100, Height: 20,
		inkOverflowW: 600,
		Opacity:      1.0,
		TC: &shapeTextConfig{
			Text: "wide", TextStyle: &style,
			TextMode: TextModeWrapKeepSpaces,
		},
	}
	renderText(shape, makeClip(0, 0, 200, 100), w)

	if len(w.renderers) == 0 {
		t.Error("no render commands: the ink reaches into the clip " +
			"at x 0..300, only the box does not")
	}
}

// TestInputMultilineNotScrollableFollowsCaret is the issue's own
// multiline repro: the field opts into no scrolling at all, so the
// offset is applied to the text shape the same way a single-line
// field's is.
func TestInputMultilineNotScrollableFollowsCaret(t *testing.T) {
	w := caretTestWindow(200, 60)
	text := strings.Repeat("a", 60) // unbreakable: cannot wrap
	_, field := arrangeInput(t, w, InputCfg{
		ID: "in", Mode: InputMultiline, Text: text,
	})
	setInputState(w, "in", inputState{CursorPos: len(text)})

	inputScrollCursorIntoView("in", text, field, w)

	offset := w.scrollX().GetOr("in", 0)
	if offset >= 0 {
		t.Fatalf("scrollX = %v, want negative", offset)
	}
	if field.Shape.Scrollable {
		t.Fatal("field must not be Scrollable in this case")
	}

	// The offset only reaches the screen through AmendLayout, so
	// re-arrange and check the text shape actually moved.
	_, after := arrangeInput(t, w, InputCfg{
		ID: "in", Mode: InputMultiline, Text: text,
	})
	txt := inputTextShape(t, after)
	if txt.X >= after.Shape.X {
		t.Errorf("text X = %v, want left of the field at %v",
			txt.X, after.Shape.X)
	}
}

// TestInputScrollFollowsCaretOnStaleLayout pins the one-frame lag the
// follow has to survive. The handlers call inputScrollCursorIntoView
// with the text the keystroke just produced, but ctx.Layout is still
// the arrangement of the frame before it, so the text shape and the
// field's content width are one character short. Clamping the offset
// against that stale extent alone parks the caret exactly one character
// past the right edge and leaves it there for every keystroke after.
func TestInputScrollFollowsCaretOnStaleLayout(t *testing.T) {
	w := caretTestWindow(200, 60)
	arranged := strings.Repeat("a", 60)
	_, field := arrangeInput(t, w, InputCfg{ID: "in", Text: arranged})

	// One character more than the layout above was built from.
	typed := arranged + "a"
	setInputState(w, "in", inputState{CursorPos: len(typed)})

	inputScrollCursorIntoView("in", typed, field, w)

	viewportW := field.Shape.Width - field.Shape.paddingWidth()
	want := -(float32(len(typed))*10 + inputCaretW - viewportW)
	if got := w.scrollX().GetOr("in", 0); !floatNear(got, want, 0.01) {
		t.Errorf("scrollX = %v, want %v (caret of the new character "+
			"visible, not clamped to the previous frame's width)",
			got, want)
	}
}

// TestInputMultilineInkOverflowReservesCaret pins the caret width in the
// recorded ink extent. A multiline field is a real scroll container, so
// layoutAdjustScrollOffsets re-clamps whatever the follow wrote against
// contentWidth. Without the caret in that extent the pipeline trims the
// caret's own width back off and the caret at the end of an unbreakable
// run can never be fully reached.
func TestInputMultilineInkOverflowReservesCaret(t *testing.T) {
	w := caretTestWindow(200, 80)
	text := strings.Repeat("a", 60)
	_, field := arrangeInput(t, w, multilineCfg("in", text))

	txt := inputTextShape(t, field)
	wantInk := float32(len(text))*10 + inputCaretW
	if !floatNear(txt.inkOverflowW, wantInk, 0.01) {
		t.Fatalf("inkOverflowW = %v, want %v (run plus the caret)",
			txt.inkOverflowW, wantInk)
	}
	viewportW := field.Shape.Width - field.Shape.paddingWidth()
	if got := scrollMaxOffsetX(field); !floatNear(got, viewportW-wantInk, 0.5) {
		t.Errorf("scrollMaxOffsetX = %v, want %v (caret reachable)",
			got, viewportW-wantInk)
	}
}

// TestInputPasswordWidthMeasuresPaintedMask pins that a password field
// is sized on the string renderText paints, not on a different mask.
// maskPassword keeps newlines and passwordMask does not, so measuring
// the wrong one collapses a multiline password to a single line's worth
// of bullets and parks the horizontal scroll at the wrong maximum.
func TestInputPasswordWidthMeasuresPaintedMask(t *testing.T) {
	m := &caretStubMeasurer{charWidth: 10, fontHeight: 20}
	w := &Window{windowWidth: 200, windowHeight: 200, focused: true}
	w.SetTextMeasurer(m)
	text := "ab\ncd"
	arrangeInput(t, w, InputCfg{
		ID: "in", Text: text, IsPassword: true, Mode: InputMultiline,
	})

	painted, wrong := maskPassword(text), passwordMask(text)
	if !slices.Contains(m.measured, painted) {
		t.Errorf("measured %q, none of them the painted mask %q",
			m.measured, painted)
	}
	if slices.Contains(m.measured, wrong) {
		t.Errorf("measured %q, the newline-flattening mask, instead of "+
			"the painted %q", wrong, painted)
	}
}

// TestInputApplyScrollXIgnoresZeroWidthField pins the degenerate-
// geometry guard. A field sized to nothing this frame (a collapsed
// pane, a hidden tab) has a negative viewport once padding is taken
// off; clamping against that is arithmetic on nonsense, so the stored
// offset must be left alone until the field has a real width.
func TestInputApplyScrollXIgnoresZeroWidthField(t *testing.T) {
	w := caretTestWindow(400, 60)
	_, field := arrangeInput(t, w, InputCfg{
		ID: "in", Text: strings.Repeat("a", 60),
	})
	txt := inputTextShape(t, field)

	// Collapse the field after arrange: a real one gets here by sitting
	// in a pane the user dragged shut, which no headless Cfg reproduces.
	field.Shape.Width = 0
	w.scrollX().Set("in", -50)
	before := txt.X

	inputApplyScrollX(inputHandlerCfg{scrollID: "in"}, field, w)

	if got := w.scrollX().GetOr("in", 0); got != -50 {
		t.Errorf("stored scrollX = %v, want -50 untouched: a zero-width "+
			"field must not re-clamp the offset", got)
	}
	if txt.X != before {
		t.Errorf("text X = %v, want %v: nothing should shift against a "+
			"negative viewport", txt.X, before)
	}
}

// TestInputInkOverflowStopsAtViewport pins that the overflow is not
// leaked past the scroll container. propagateInkOverflow refreshes a
// viewport's content width, which is what makes scrolling possible, but
// must not carry the ink to that viewport's own parent — doing so would
// widen the whole page around an input holding one long word.
func TestInputInkOverflowStopsAtViewport(t *testing.T) {
	w := caretTestWindow(200, 200)
	root, field := arrangeInput(t, w, multilineCfg("in",
		strings.Repeat("a", 60)))

	if txt := inputTextShape(t, field); txt.inkOverflowW == 0 {
		t.Fatal("text shape recorded no ink overflow: the run did not " +
			"overflow, so this test proves nothing")
	}
	if got := field.Shape.inkOverflowW; got != 0 {
		t.Errorf("field inkOverflowW = %v, want 0: a Scrollable node is "+
			"the viewport and absorbs the overflow", got)
	}
	if got := root.Shape.inkOverflowW; got != 0 {
		t.Errorf("root inkOverflowW = %v, want 0: the overflow escaped "+
			"the scroll container and widened the page", got)
	}
}

// TestDragScrollDeltaBoundaries pins the auto-scroll ramp, including
// the two edges: a pointer exactly on a band edge is inside, not out,
// so a drag that stops at the edge stops scrolling.
func TestDragScrollDeltaBoundaries(t *testing.T) {
	const lo, hi = 100, 300
	cases := []struct {
		name string
		pos  float32
		want float32
	}{
		{"inside", 200, 0},
		{"on the low edge", lo, 0},
		{"on the high edge", hi, 0},
		{"past the low edge scrolls forward", 90, 3},
		{"past the high edge scrolls back", 310, -3},
		{"further out scrolls faster", 50, 15},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := dragScrollDelta(c.pos, lo, hi); !floatNear(
				got, c.want, 0.001) {
				t.Errorf("dragScrollDelta(%v) = %v, want %v",
					c.pos, got, c.want)
			}
		})
	}
}

// TestComputeContentWidthIgnoresNaNInk pins the harden guard: a NaN
// ink extent from a faulty measurer must not poison the content width.
func TestComputeContentWidthIgnoresNaNInk(t *testing.T) {
	lay := &Layout{Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom}}
	lay.Children = []Layout{{Shape: &Shape{shapeType: shapeRectangle, Width: 50, inkOverflowW: 50}}}
	if got := computeContentWidth(lay); got != 50 {
		t.Fatalf("contentWidth = %v, want 50", got)
	}
	lay.Children[0].Shape.inkOverflowW =
		float32(math.NaN())
	if got := computeContentWidth(lay); got != 50 {
		t.Errorf("contentWidth with NaN ink = %v, want 50", got)
	}
}

// TestInputApplyScrollXResetsNaNOffset pins that a poisoned scroll
// entry is cleared rather than painted: NaN would shift the text to
// nowhere and disagree with every later reader of the offset.
func TestInputApplyScrollXResetsNaNOffset(t *testing.T) {
	w := caretTestWindow(200, 60)
	_, field := arrangeInput(t, w, InputCfg{
		ID: "in", Text: strings.Repeat("a", 60),
	})
	w.scrollX().Set("in", float32(math.NaN()))
	inputApplyScrollX(inputHandlerCfg{scrollID: "in"}, field, w)
	if got := w.scrollX().GetOr("in", 0); got != 0 {
		t.Errorf("stored scrollX = %v, want 0 after NaN reset", got)
	}
}

// TestApplyDragScrollRejectsNaNDelta pins that a NaN auto-scroll step
// moves nothing and writes nothing back to the scroll map.
func TestApplyDragScrollRejectsNaNDelta(t *testing.T) {
	w := newTestWindow()
	if applyDragScroll(w.scrollX(), "inp",
		float32(math.NaN()), -200) {
		t.Error("applyDragScroll(NaN) = true, want false")
	}
	if got := w.scrollX().GetOr("inp", 0); got != 0 {
		t.Errorf("scrollX = %v, want 0 untouched", got)
	}
}
