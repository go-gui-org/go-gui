package gui

import "testing"

func TestTextSelectAllAndCopy(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello world", Focusable: true, ID: "f42"})
	layout := generateViewLayout(v, w)
	w.SetFocus("f42")

	// Ctrl+A selects all.
	e := &Event{KeyCode: KeyA, Modifiers: ModCtrl}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})

	is := getInputState(w, "f42")
	if is.selectBeg != 0 || is.selectEnd != 11 {
		t.Fatalf("select-all: got %d-%d, want 0-11",
			is.selectBeg, is.selectEnd)
	}
	if !e.IsHandled {
		t.Fatal("event not marked handled")
	}

	// Ctrl+C copies selected text.
	var clipboard string
	w.SetClipboardFn(func(s string) { clipboard = s })
	e = &Event{KeyCode: KeyC, Modifiers: ModCtrl}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})

	if clipboard != "hello world" {
		t.Fatalf("copy: got %q, want %q",
			clipboard, "hello world")
	}
}

func TestTextDoubleClickWordSelect(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello world", Focusable: true, ID: "f42"})
	layout := generateViewLayout(v, w)

	// charWidth = 16 * 0.6 = 9.6 in test fallback.
	charWidth := float32(16 * 0.6)
	clickX := layout.Shape.X + charWidth*6 + charWidth*0.5
	clickY := layout.Shape.Y + 1

	// First click: cursor at rune 6.
	e1 := &Event{MouseX: clickX, MouseY: clickY}
	layout.Shape.events.OnClick(EventCtx{&layout, e1, w})

	is := getInputState(w, "f42")
	if is.CursorPos != 6 {
		t.Fatalf("single click: cursor %d, want 6",
			is.CursorPos)
	}

	// Second click (within 400ms): selects "world".
	e2 := &Event{MouseX: clickX, MouseY: clickY}
	layout.Shape.events.OnClick(EventCtx{&layout, e2, w})

	is = getInputState(w, "f42")
	beg, end := u32Sort(is.selectBeg, is.selectEnd)
	if beg != 6 || end != 11 {
		t.Fatalf("double click: got %d-%d, want 6-11",
			beg, end)
	}
}

func TestTextShiftArrowSelection(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "abcdef", Focusable: true, ID: "f42"})
	layout := generateViewLayout(v, w)
	w.SetFocus("f42")

	// Place cursor at position 2.
	setInputState(w, "f42", inputState{CursorPos: 2})

	// Shift+Right x3 → select positions 2-5.
	for range 3 {
		e := &Event{
			KeyCode:   KeyRight,
			Modifiers: ModShift,
		}
		layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
	}

	is := getInputState(w, "f42")
	beg, end := u32Sort(is.selectBeg, is.selectEnd)
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
	v := Text(TextCfg{Text: "test text", Focusable: true, ID: "f42"})
	layout := generateViewLayout(v, w)

	// Set selection in input state.
	setInputState(w, "f42", inputState{
		CursorPos: 9,
		selectBeg: 5,
		selectEnd: 9,
	})

	// AmendLayout should copy to shape.TC.
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})

	if layout.Shape.TC.textSelBeg != 5 ||
		layout.Shape.TC.textSelEnd != 9 {
		t.Fatalf("amend: got %d-%d, want 5-9",
			layout.Shape.TC.textSelBeg,
			layout.Shape.TC.textSelEnd)
	}
}

func TestTextEscapeClearsSelection(t *testing.T) {
	w := newTestWindow()
	v := Text(TextCfg{Text: "hello", Focusable: true, ID: "f42"})
	layout := generateViewLayout(v, w)
	w.SetFocus("f42")

	setInputState(w, "f42", inputState{
		CursorPos: 5,
		selectBeg: 0,
		selectEnd: 5,
	})

	e := &Event{KeyCode: KeyEscape}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})

	is := getInputState(w, "f42")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Fatalf("escape: selection %d-%d, want 0-0",
			is.selectBeg, is.selectEnd)
	}
}

// --- Issue #281: capture-loss cancel zeroes the partial drag selection ---

// textSelCharW is the fallback char width the text click/drag paths use
// with no glyph backend: style.Size * 0.6 = 16 * 0.6 = 9.6.
const textSelCharW = float32(16 * 0.6)

// newTextSelWindow renders a focusable text widget and returns the
// window plus its layout.
func newTextSelWindow(t *testing.T, cfg TextCfg) (*Window, *Layout) {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(win *Window) View { return Text(cfg) })
	ly, ok := w.layout.FindByID(cfg.ID)
	if !ok {
		t.Fatalf("no text with effective ID %s", cfg.ID)
	}
	return w, ly
}

// textRunePoint returns the window-absolute center of localRune — valid
// for the click path (shape-relative after callRelative) and the
// MouseLock drag path (window-absolute; the drag subtracts shape.X).
func textRunePoint(shp *Shape, localRune int) (float32, float32) {
	return shp.X + float32(localRune)*textSelCharW + textSelCharW/2,
		shp.Y + 1
}

// TestTextDragCancelZeroesSelection pins issue #281: capture loss
// mid-drag (MouseCancel — no release will ever arrive) must zero the
// partial selection, not leave a stuck highlight.
func TestTextDragCancelZeroesSelection(t *testing.T) {
	w, ly := newTextSelWindow(t, TextCfg{
		Text: "hello world", Focusable: true, ID: "f281",
	})

	x0, y0 := textRunePoint(ly.Shape, 2)
	x1, y1 := textRunePoint(ly.Shape, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	if !w.mouseIsLocked() {
		t.Fatal("press on the text did not lock the mouse")
	}
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()
	is := getInputState(w, "f281")
	if is.selectBeg != 2 || is.selectEnd != 8 {
		t.Fatalf("drag selected [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}

	w.MouseCancel()
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	is = getInputState(w, "f281")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("selection after cancel = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
}

// TestTextDragCancelScopedToOwnState asserts the Text Cancel hook
// zeroes only the dragged widget's nsInput entry: another widget's
// state written after the drag started survives the cancel.
func TestTextDragCancelScopedToOwnState(t *testing.T) {
	w, ly := newTextSelWindow(t, TextCfg{
		Text: "hello world", Focusable: true, ID: "f281",
	})

	x0, y0 := textRunePoint(ly.Shape, 2)
	x1, y1 := textRunePoint(ly.Shape, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()

	setInputState(w, "other", inputState{CursorPos: 3, selectBeg: 1, selectEnd: 3})
	w.MouseCancel()
	w.settle()
	if is := getInputState(w, "f281"); is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("dragged text selection = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
	if is := getInputState(w, "other"); is.selectBeg != 1 || is.selectEnd != 3 {
		t.Errorf("unrelated widget selection = [%d,%d), want [1,3)",
			is.selectBeg, is.selectEnd)
	}
}

// TestTextReleaseOutsideCommitsSelection pins the asymmetry: a normal
// MouseUp — even at a point far outside the widget — COMMITS the drag
// selection. The lock delivers the release anywhere; only capture loss
// goes through the Cancel hook.
func TestTextReleaseOutsideCommitsSelection(t *testing.T) {
	w, ly := newTextSelWindow(t, TextCfg{
		Text: "hello world", Focusable: true, ID: "f281",
	})

	x0, y0 := textRunePoint(ly.Shape, 2)
	x1, y1 := textRunePoint(ly.Shape, 8)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x0, MouseY: y0})
	w.settle()
	w.EventFn(&Event{Type: EventMouseMove, MouseButton: MouseInvalid,
		MouseX: x1, MouseY: y1})
	w.settle()
	if is := getInputState(w, "f281"); is.selectBeg != 2 || is.selectEnd != 8 {
		t.Fatalf("drag selected [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}

	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: 700, MouseY: 700})
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after release")
	}
	is := getInputState(w, "f281")
	if is.selectBeg != 2 || is.selectEnd != 8 {
		t.Errorf("selection after outside release = [%d,%d), want [2,8)",
			is.selectBeg, is.selectEnd)
	}
}

// TestTextDragCancelCollapsedSelectionHarmless asserts the guard: a
// cancel when no selection is in progress (the press only collapsed to
// a caret) leaves the state clean — no corruption, no stuck lock.
func TestTextDragCancelCollapsedSelectionHarmless(t *testing.T) {
	w, ly := newTextSelWindow(t, TextCfg{
		Text: "hello world", Focusable: true, ID: "f281",
	})

	x, y := textRunePoint(ly.Shape, 2)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y})
	w.settle()
	if !w.mouseIsLocked() {
		t.Fatal("press on the text did not lock the mouse")
	}
	if is := getInputState(w, "f281"); is.selectBeg != 2 || is.selectEnd != 2 {
		t.Fatalf("press left selection [%d,%d), want caret at 2",
			is.selectBeg, is.selectEnd)
	}

	w.MouseCancel()
	w.settle()
	if w.mouseIsLocked() {
		t.Error("mouse still locked after MouseCancel")
	}
	is := getInputState(w, "f281")
	if is.selectBeg != 0 || is.selectEnd != 0 {
		t.Errorf("selection after cancel = [%d,%d), want [0,0)",
			is.selectBeg, is.selectEnd)
	}
	if is.CursorPos != 2 {
		t.Errorf("cursor = %d, want 2", is.CursorPos)
	}
}
