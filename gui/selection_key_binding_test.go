package gui

import "testing"

// Key binding tests for the read-only selection widgets: Text, RTF and
// Markdown (#969). On macOS Ctrl+A is "go to line start", not select
// all. The mode is process-wide, so none of these run in parallel.

func TestTextSelectCommandKeyBinding(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	w := newTestWindow()
	layout := generateViewLayout(
		Text(TextCfg{Text: "hello world", Focusable: true, ID: "kbt"}), w)
	w.SetFocus("kbt")
	fire := func(k KeyCode, m Modifier) *Event {
		e := &Event{KeyCode: k, Modifiers: m}
		layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
		return e
	}

	fire(KeyE, ModCtrl)
	if is := getInputState(w, "kbt"); is.CursorPos != 11 {
		t.Fatalf("Ctrl+E: cursor=%d, want 11", is.CursorPos)
	}
	fire(KeyB, ModCtrl)
	if is := getInputState(w, "kbt"); is.CursorPos != 10 {
		t.Fatalf("Ctrl+B: cursor=%d, want 10", is.CursorPos)
	}
	fire(KeyA, ModCtrl)
	is := getInputState(w, "kbt")
	if is.CursorPos != 0 || is.selectBeg != is.selectEnd {
		t.Fatalf("Ctrl+A: cursor=%d sel=%d-%d, want caret 0, no selection",
			is.CursorPos, is.selectBeg, is.selectEnd)
	}
	fire(KeyA, ModSuper)
	if is = getInputState(w, "kbt"); is.selectBeg != 0 || is.selectEnd != 11 {
		t.Fatalf("Cmd+A: got %d-%d, want 0-11", is.selectBeg, is.selectEnd)
	}
	var clipboard string
	w.SetClipboardFn(func(s string) { clipboard = s })
	if e := fire(KeyC, ModCtrl); e.IsHandled || clipboard != "" {
		t.Fatal("Ctrl+C copied on macOS")
	}
	fire(KeyC, ModSuper)
	if clipboard != "hello world" {
		t.Fatalf("Cmd+C: clipboard=%q", clipboard)
	}
}

func TestRtfSelectCommandKeyBinding(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	h := newRtfSelectHarness(t, rtfHelloWorld())
	x, y := rtfRunePoint(h.shape(t), 3)
	h.press(x, y)
	h.release(x, y)

	h.key(t, KeyA, ModCtrl)
	expectRtfSel(t, h, 0, 0)
	expectRtfCursor(t, h, 0)
	h.key(t, KeyE, ModCtrl)
	expectRtfCursor(t, h, 11)
	h.key(t, KeyA, ModSuper)
	expectRtfSel(t, h, 0, 11)
}

func TestMarkdownCommandKeyBinding(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	h := newMdSelectHarnessNested(t)

	if err := h.w.TestKey("panel:md", KeyA, ModCtrl); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if st := h.selState(); st.SelBeg != st.SelEnd {
		t.Fatalf("Ctrl+A selected [%d,%d) on macOS", st.SelBeg, st.SelEnd)
	}
	if err := h.w.TestKey("panel:md", KeyA, ModSuper); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if st := h.selState(); st.SelBeg != 0 || st.SelEnd == 0 {
		t.Fatalf("Cmd+A selection = [%d,%d), want all",
			st.SelBeg, st.SelEnd)
	}
}
