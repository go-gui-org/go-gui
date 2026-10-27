package gui

import (
	"strings"
	"testing"
)

// Issue #1186: the press handlers sit on the field, so the whole field
// takes a press, not just the inner row. In a Scrollable multiline field
// that row is only as tall as the text and slides with the horizontal
// offset, so the presses below are the ones that used to fall through.

// clickTestWindow renders one Input under a FillFill root, with a
// measurer that produces real glyph layouts so a press places a caret.
func clickTestWindow(t *testing.T, cfg InputCfg) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 600})
	w.SetTextMeasurer(&caretStubMeasurer{charWidth: 10, fontHeight: 20})
	w.SetClipboardGetFn(func() string { return "x" })
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(cfg),
		}})
	})
	return w
}

// clickAt sends a press and release of btn at window point x, y.
func clickAt(w *Window, btn MouseButton, x, y float32) {
	pressAt(w, btn, x, y)
	releaseAt(w, btn, x, y)
}

// clickTestField returns the arranged field that claims id.
func clickTestField(t *testing.T, w *Window, id string) *Layout {
	t.Helper()
	field := findLayoutByID(&w.layout, id)
	if field == nil {
		t.Fatalf("no layout with ID %q", id)
	}
	return field
}

func scrollableMultilineCfg(text string) InputCfg {
	return InputCfg{
		ID: "ta", Mode: InputMultiline, Scrollable: true, Text: text,
		Width: 200, Height: 140, Sizing: FixedFixed, NoMinWidthFloor: true,
	}
}

// The reproduction from the issue: a right press below the only line
// of an empty field opens the built-in menu.
func TestInputScrollableMultilineRightPressBelowTextOpensMenu(t *testing.T) {
	w := clickTestWindow(t, scrollableMultilineCfg(""))
	field := clickTestField(t, w, "ta")
	clickAt(w, MouseRight,
		field.Shape.X+50, field.Shape.Y+field.Shape.Height-15)
	if got := w.FocusID(); got != inputMenuFocusID {
		t.Fatalf("focus = %q, want the input context menu", got)
	}
}

func TestInputScrollableMultilineLeftPressBelowTextPlacesCaret(t *testing.T) {
	text := "hello world"
	w := clickTestWindow(t, scrollableMultilineCfg(text))
	field := clickTestField(t, w, "ta")
	clickAt(w, MouseLeft,
		field.Shape.X+field.Shape.Width-30, field.Shape.Y+field.Shape.Height-15)
	if got := w.FocusID(); got != "ta" {
		t.Fatalf("focus = %q, want %q", got, "ta")
	}
	if got := inputStateOrDefault("ta", w).CursorPos; got != len(text) {
		t.Errorf("CursorPos = %d, want %d (end of the last line)",
			got, len(text))
	}
}

// Scrolled fully sideways, the inner row sits left of the field
// entirely. Both buttons must still reach the field.
func TestInputScrolledSidewaysTakesPress(t *testing.T) {
	text := strings.Repeat("a", 60) // no break opportunity
	w := clickTestWindow(t, scrollableMultilineCfg(text))
	field := clickTestField(t, w, "ta")
	maxX := scrollMaxOffsetX(field)
	if maxX >= 0 {
		t.Fatalf("scrollMaxOffsetX = %v, want negative", maxX)
	}
	w.scrollX().Set("ta", maxX)
	w.TestRender(nil)
	field = clickTestField(t, w, "ta")
	inner := field.Children[0].Shape
	if inner.X+inner.Width > field.Shape.X {
		t.Fatalf("inner row [%v,%v] overlaps field at %v; the test no "+
			"longer covers the sideways case",
			inner.X, inner.X+inner.Width, field.Shape.X)
	}
	x, y := field.Shape.X+field.Shape.Width-30, field.Shape.Y+15

	clickAt(w, MouseLeft, x, y)
	if got := inputStateOrDefault("ta", w).CursorPos; got < len(text)-3 {
		t.Errorf("CursorPos = %d, want near the end (%d): the press "+
			"is on the visible tail of the line", got, len(text))
	}

	clickAt(w, MouseRight, x, y)
	if got := w.FocusID(); got != inputMenuFocusID {
		t.Errorf("focus = %q, want the input context menu", got)
	}
}

// A press in the padding places the caret too, as in a native field:
// the I-beam cursor already covers it.
func TestInputPaddingPressPlacesCaret(t *testing.T) {
	text := "hello"
	w := clickTestWindow(t, InputCfg{ID: "in", Text: text, Width: 200,
		Sizing: FixedFit, NoMinWidthFloor: true})
	w.SetFocus("in")
	w.settle()
	is := inputStateOrDefault("in", w)
	is.CursorPos = len(text)
	inputStoreState("in", w, is)

	field := clickTestField(t, w, "in")
	inner := field.Children[0].Shape
	padLeft := inner.X - field.Shape.X
	if padLeft < 2 {
		t.Fatalf("left inset = %v, want room for a press in it", padLeft)
	}
	clickAt(w, MouseLeft, field.Shape.X+padLeft/2,
		field.Shape.Y+field.Shape.Height/2)
	if got := inputStateOrDefault("in", w).CursorPos; got != 0 {
		t.Errorf("CursorPos = %d, want 0 (press left of the text)", got)
	}
}

// Scrollbars are later children of the field, so they keep their own
// presses: a gutter press scrolls and leaves the caret alone.
func TestInputScrollbarKeepsItsPress(t *testing.T) {
	text := strings.Repeat("ab cd ", 60)
	w := clickTestWindow(t, scrollableMultilineCfg(text))
	w.SetFocus("ta")
	w.settle()
	field := clickTestField(t, w, "ta")
	var bar *Shape
	for i := range field.Children {
		if s := field.Children[i].Shape; s.scrollbarOrientation == scrollbarVertical {
			bar = s
		}
	}
	if bar == nil || bar.Height <= 0 {
		t.Fatal("no vertical scrollbar on a field whose text overflows")
	}
	// Near the bottom of the gutter: below the thumb, which starts at
	// the top while the field is unscrolled.
	clickAt(w, MouseLeft, bar.X+bar.Width/2, bar.Y+bar.Height-2)

	if got := inputStateOrDefault("ta", w).CursorPos; got != 0 {
		t.Errorf("CursorPos = %d, want 0 (a scrollbar press is not "+
			"a caret press)", got)
	}
	if got := w.scrollY().GetOr("ta", 0); got >= 0 {
		t.Errorf("scrollY = %v, want negative (the gutter press "+
			"scrolls)", got)
	}
}

// The fix moves handlers, not geometry: the inner row still hugs the
// text, so the scroll range is what the text alone makes it.
func TestInputScrollableMultilineGeometryUnchanged(t *testing.T) {
	for _, tc := range []struct {
		name   string
		text   string
		scroll bool
	}{
		{"short", "one line", false},
		{"long", strings.Repeat("ab cd ", 60), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := clickTestWindow(t, scrollableMultilineCfg(tc.text))
			field := clickTestField(t, w, "ta")
			inner := field.Children[0].Shape
			txt := inputTextShapeFromLayout(field)
			if inner.Height != txt.Height {
				t.Errorf("inner row height = %v, want the text's %v",
					inner.Height, txt.Height)
			}
			if got := scrollMaxOffsetY(field) < 0; got != tc.scroll {
				t.Errorf("scrolls = %v, want %v", got, tc.scroll)
			}
		})
	}
}

// a11yPressInput sends a screen reader's activate action to the field
// claiming id, through the real node array.
func a11yPressInput(t *testing.T, w *Window, id string) {
	t.Helper()
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, "", &live)
	field := clickTestField(t, w, id)
	idx := -1
	for i := range w.a11y.nodes {
		if a11yFindLayout(&w.layout, i) == field {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("no a11y node for %q", id)
	}
	a11yActionCallback(w, A11yActionPress, idx)
	w.settle()
}

// A screen reader's activate action has no pointer position. With the
// handler on the field it reaches inputOnClick, which must only focus
// the field: no caret move to the corner and no drag lock left behind
// with no button down. A field that already has focus keeps its
// selection too.
func TestInputA11yPressFocusesWithoutMovingCaret(t *testing.T) {
	text := "hello world"
	for _, tc := range []struct {
		name    string
		focused bool
	}{
		{"unfocused", false},
		{"focused", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := clickTestWindow(t, scrollableMultilineCfg(text))
			if tc.focused {
				w.SetFocus("ta")
				w.settle()
			}
			is := inputStateOrDefault("ta", w)
			is.CursorPos, is.selectBeg, is.selectEnd = 5, 2, 5
			inputStoreState("ta", w, is)

			a11yPressInput(t, w, "ta")

			if got := w.FocusID(); got != "ta" {
				t.Errorf("focus = %q, want %q", got, "ta")
			}
			got := inputStateOrDefault("ta", w)
			if got.CursorPos != 5 {
				t.Errorf("CursorPos = %d, want 5 (unmoved)", got.CursorPos)
			}
			// Taking focus clears a stale selection, as any focus
			// change does; a field already focused keeps it.
			if tc.focused && (got.selectBeg != 2 || got.selectEnd != 5) {
				t.Errorf("selection [%d,%d), want [2,5)",
					got.selectBeg, got.selectEnd)
			}
			if w.mouseIsLocked() {
				t.Error("a11y press left a mouse lock")
			}
		})
	}
}

// A right press on the field's scrollbar travels on to the app instead
// of opening the edit menu over the scrollbar.
func TestInputRightPressOnScrollbarTravelsOn(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 800, Height: 600})
	w.SetTextMeasurer(&caretStubMeasurer{charWidth: 10, fontHeight: 20})
	w.SetClipboardGetFn(func() string { return "x" })
	appRight := 0
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			OnAnyClick: func(ctx EventCtx) {
				if ctx.Event.MouseButton == MouseRight {
					appRight++
					ctx.Consume()
				}
			},
			Content: []View{
				Input(scrollableMultilineCfg(strings.Repeat("ab cd ", 60))),
			},
		})
	})
	field := clickTestField(t, w, "ta")
	var bar *Shape
	for i := range field.Children {
		if s := field.Children[i].Shape; s.scrollbarOrientation == scrollbarVertical {
			bar = s
		}
	}
	if bar == nil || bar.Height <= 0 {
		t.Fatal("no vertical scrollbar on a field whose text overflows")
	}
	clickAt(w, MouseRight, bar.X+bar.Width/2, bar.Y+bar.Height/2)

	if got := w.FocusID(); got == inputMenuFocusID {
		t.Error("right press on the scrollbar opened the edit menu")
	}
	if appRight != 1 {
		t.Errorf("app right-press handler ran %d times, want 1", appRight)
	}
}
