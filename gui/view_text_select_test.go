package gui

import "testing"

func TestTextSelectAllAndCopy(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello world", IDFocus: 42})
	layout := generateViewLayout(v, w)
	w.SetIDFocus(42)

	// Ctrl+A selects all.
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	layout.Shape.events.OnKeyDown(&layout, e, w)

	is := getInputState(w, 42)
	if is.SelectBeg != 0 || is.SelectEnd != 11 {
		t.Fatalf("select-all: got %d-%d, want 0-11",
			is.SelectBeg, is.SelectEnd)
	}
	if !e.IsHandled {
		t.Fatal("event not marked handled")
	}

	// Ctrl+C copies selected text.
	var clipboard string
	w.SetClipboardFn(func(s string) { clipboard = s })
	e = &Event{KeyCode: KeyC, Modifiers: ModCtrl}
	layout.Shape.events.OnKeyDown(&layout, e, w)

	if clipboard != "hello world" {
		t.Fatalf("copy: got %q, want %q",
			clipboard, "hello world")
	}
}

func TestTextDoubleClickWordSelect(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello world", IDFocus: 42})
	layout := generateViewLayout(v, w)

	// charWidth = 16 * 0.6 = 9.6 in test fallback.
	charWidth := float32(16 * 0.6)
	clickX := layout.Shape.X + charWidth*6 + charWidth*0.5
	clickY := layout.Shape.Y + 1

	// First click: cursor at rune 6.
	e1 := &Event{MouseX: clickX, MouseY: clickY}
	layout.Shape.events.OnClick(&layout, e1, w)

	is := getInputState(w, 42)
	if is.CursorPos != 6 {
		t.Fatalf("single click: cursor %d, want 6",
			is.CursorPos)
	}

	// Second click (within 400ms): selects "world".
	e2 := &Event{MouseX: clickX, MouseY: clickY}
	layout.Shape.events.OnClick(&layout, e2, w)

	is = getInputState(w, 42)
	beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
	if beg != 6 || end != 11 {
		t.Fatalf("double click: got %d-%d, want 6-11",
			beg, end)
	}
}

func TestTextShiftArrowSelection(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "abcdef", IDFocus: 42})
	layout := generateViewLayout(v, w)
	w.SetIDFocus(42)

	// Place cursor at position 2.
	setInputState(w, 42, InputState{CursorPos: 2})

	// Shift+Right x3 → select positions 2-5.
	for range 3 {
		e := &Event{
			KeyCode:   KeyRight,
			Modifiers: ModShift,
		}
		layout.Shape.events.OnKeyDown(&layout, e, w)
	}

	is := getInputState(w, 42)
	beg, end := u32Sort(is.SelectBeg, is.SelectEnd)
	if beg != 2 || end != 5 {
		t.Fatalf("shift-right: got %d-%d, want 2-5",
			beg, end)
	}
}

func TestTextNoHandlersWithoutFocus(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "no focus"})
	layout := generateViewLayout(v, w)

	if layout.Shape.events != nil {
		t.Fatal("events should be nil when IDFocus == 0")
	}
}

func TestTextAmendLayout(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "test text", IDFocus: 42})
	layout := generateViewLayout(v, w)

	// Set selection in input state.
	setInputState(w, 42, InputState{
		CursorPos: 9,
		SelectBeg: 5,
		SelectEnd: 9,
	})

	// AmendLayout should copy to shape.TC.
	layout.Shape.events.AmendLayout(&layout, w)

	if layout.Shape.TC.TextSelBeg != 5 ||
		layout.Shape.TC.TextSelEnd != 9 {
		t.Fatalf("amend: got %d-%d, want 5-9",
			layout.Shape.TC.TextSelBeg,
			layout.Shape.TC.TextSelEnd)
	}
}

func TestTextEscapeClearsSelection(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello", IDFocus: 42})
	layout := generateViewLayout(v, w)
	w.SetIDFocus(42)

	setInputState(w, 42, InputState{
		CursorPos: 5,
		SelectBeg: 0,
		SelectEnd: 5,
	})

	e := &Event{KeyCode: KeyEscape}
	layout.Shape.events.OnKeyDown(&layout, e, w)

	is := getInputState(w, 42)
	if is.SelectBeg != 0 || is.SelectEnd != 0 {
		t.Fatalf("escape: selection %d-%d, want 0-0",
			is.SelectBeg, is.SelectEnd)
	}
}
