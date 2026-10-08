package gui

import "testing"

// Key binding tests for Input (#969). The mode is process-wide, so none
// of these run in parallel.

// TestInputCommandCtrlADoesNotSelectAll is the #969 regression: on
// macOS Ctrl+A moves to the start of the line, like every Cocoa text
// field, and only Cmd+A selects all.
func TestInputCommandCtrlADoesNotSelectAll(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTest("abc", "kb1", 2)
	ctx.fireKeyDown(KeyA, ModCtrl)
	is := ctx.state()
	if is.selectBeg != is.selectEnd {
		t.Fatalf("Ctrl+A selected %d-%d, want no selection",
			is.selectBeg, is.selectEnd)
	}
	if is.CursorPos != 0 {
		t.Fatalf("Ctrl+A cursor=%d, want 0", is.CursorPos)
	}
}

func TestInputCommandCmdASelectsAll(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTest("abc", "kb2", 1)
	ctx.fireKeyDown(KeyA, ModSuper)
	is := ctx.state()
	if is.selectBeg != 0 || is.selectEnd != 3 {
		t.Fatalf("Cmd+A: got %d-%d, want 0-3", is.selectBeg, is.selectEnd)
	}
}

// TestInputControlSuperADoesNotSelectAll: on Linux and Windows the
// Super key is the OS key, not a shortcut modifier.
func TestInputControlSuperADoesNotSelectAll(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	ctx := newInputTest("abc", "kb3", 1)
	ctx.fireKeyDown(KeyA, ModSuper)
	is := ctx.state()
	if is.selectBeg != is.selectEnd {
		t.Fatalf("Super+A selected %d-%d, want none",
			is.selectBeg, is.selectEnd)
	}
}

func TestInputCommandCtrlCDoesNotCopy(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	clipboard := "old"
	ctx := newInputTest("hello", "kb4", 0)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	setInputState(ctx.w, "kb4", inputState{CursorPos: 5, selectEnd: 5})
	ctx.fireKeyDown(KeyC, ModCtrl)
	if clipboard != "old" {
		t.Fatalf("Ctrl+C copied %q on macOS", clipboard)
	}
	ctx.fireKeyDown(KeyC, ModSuper)
	if clipboard != "hello" {
		t.Fatalf("Cmd+C: clipboard=%q, want hello", clipboard)
	}
}

func TestInputCommandEmacsCaretKeys(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	cases := []struct {
		key  KeyCode
		from int
		want int
	}{
		{KeyE, 1, 5},
		{KeyA, 4, 0},
		{KeyF, 1, 2},
		{KeyB, 3, 2},
	}
	for _, c := range cases {
		ctx := newInputTest("hello", "kb5", c.from)
		ctx.fireKeyDown(c.key, ModCtrl)
		if got := ctx.state().CursorPos; got != c.want {
			t.Errorf("Ctrl+%d from %d: cursor=%d, want %d",
				c.key, c.from, got, c.want)
		}
	}
}

// TestInputCommandCtrlAEStayInParagraph: Ctrl+A/E go to the paragraph
// edge and stop there, unlike Home/End, which cycle out to the
// document edge on a second press.
func TestInputCommandCtrlAEStayInParagraph(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTestMultiline("ab\ncd\nef", "kb6", 4)
	ctx.fireKeyDown(KeyA, ModCtrl)
	ctx.fireKeyDown(KeyA, ModCtrl)
	if got := ctx.state().CursorPos; got != 3 {
		t.Fatalf("Ctrl+A twice: cursor=%d, want 3", got)
	}
	ctx.fireKeyDown(KeyE, ModCtrl)
	ctx.fireKeyDown(KeyE, ModCtrl)
	if got := ctx.state().CursorPos; got != 5 {
		t.Fatalf("Ctrl+E twice: cursor=%d, want 5", got)
	}
}

func TestInputCommandCtrlNP(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTestMultiline("abc\ndef", "kb7", 1)
	ctx.fireKeyDown(KeyN, ModCtrl)
	if got := ctx.state().CursorPos; got != 5 {
		t.Fatalf("Ctrl+N: cursor=%d, want 5", got)
	}
	ctx.fireKeyDown(KeyP, ModCtrl)
	if got := ctx.state().CursorPos; got != 1 {
		t.Fatalf("Ctrl+P: cursor=%d, want 1", got)
	}
}

func TestInputCommandCtrlDH(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTest("abcd", "kb8", 2)
	ctx.fireKeyDown(KeyD, ModCtrl)
	if ctx.lastText != "abd" {
		t.Fatalf("Ctrl+D: got %q, want abd", ctx.lastText)
	}
	ctx.fireKeyDown(KeyH, ModCtrl)
	if ctx.lastText != "ad" {
		t.Fatalf("Ctrl+H: got %q, want ad", ctx.lastText)
	}
}

// TestInputCommandKillYank: Ctrl+K cuts to the end of the paragraph
// into the kill buffer, not the clipboard; Ctrl+Y pastes it back.
func TestInputCommandKillYank(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	clipboard := "clip"
	ctx := newInputTestMultiline("hello\nworld", "kb9", 2)
	ctx.w.SetClipboardFn(func(s string) { clipboard = s })
	ctx.fireKeyDown(KeyK, ModCtrl)
	if ctx.lastText != "he\nworld" {
		t.Fatalf("Ctrl+K: got %q, want %q", ctx.lastText, "he\nworld")
	}
	if clipboard != "clip" {
		t.Fatalf("Ctrl+K wrote the clipboard: %q", clipboard)
	}
	// At the paragraph end, Ctrl+K joins the next line.
	ctx.fireKeyDown(KeyK, ModCtrl)
	if ctx.lastText != "heworld" {
		t.Fatalf("Ctrl+K at line end: got %q, want heworld", ctx.lastText)
	}
	ctx.fireKeyDown(KeyY, ModCtrl)
	if ctx.lastText != "he\nworld" {
		t.Fatalf("Ctrl+Y: got %q, want %q", ctx.lastText, "he\nworld")
	}
}

// TestInputCommandKillUndoNoSelection: Ctrl+K sets its range as a
// selection only to drive the delete. Undo must bring the text back
// with the caret where it was, not with the killed range highlighted.
func TestInputCommandKillUndoNoSelection(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTest("hello world", "kb9u", 5)
	ctx.fireKeyDown(KeyK, ModCtrl)
	if ctx.lastText != "hello" {
		t.Fatalf("Ctrl+K: got %q, want hello", ctx.lastText)
	}
	ctx.fireKeyDown(KeyZ, ModSuper)
	if ctx.lastText != "hello world" {
		t.Fatalf("Cmd+Z: got %q, want %q", ctx.lastText, "hello world")
	}
	is := ctx.state()
	if is.selectBeg != is.selectEnd {
		t.Fatalf("Cmd+Z after Ctrl+K left selection %d..%d",
			is.selectBeg, is.selectEnd)
	}
	if is.CursorPos != 5 {
		t.Fatalf("Cmd+Z after Ctrl+K: cursor=%d, want 5", is.CursorPos)
	}
}

// TestInputCommandKillAtTextEndKeepsBuffer:Ctrl+K with nothing after
// the caret deletes nothing and must not empty the kill buffer, so a
// stray press does not lose the text Ctrl+Y would paste.
func TestInputCommandKillAtTextEndKeepsBuffer(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTest("hello", "kb9e", 5)
	ctx.w.killBuffer = "kept"
	ctx.fireKeyDown(KeyK, ModCtrl)
	if ctx.lastText != "hello" {
		t.Fatalf("Ctrl+K at end: got %q, want hello", ctx.lastText)
	}
	if ctx.w.killBuffer != "kept" {
		t.Fatalf("Ctrl+K at end: kill buffer %q, want kept", ctx.w.killBuffer)
	}
}

func TestInputCommandKillPasswordKeepsNoCopy(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	w := newTestWindow()
	w.SetFocus("kb10")
	setInputState(w, "kb10", inputState{CursorPos: 0})
	layout := generateViewLayout(Input(InputCfg{
		Text: "secret", ID: "kb10", IsPassword: true,
		OnTextChanged: func(string, EventCtx) {},
	}), w)
	e := &Event{Type: EventKeyDown, KeyCode: KeyK, Modifiers: ModCtrl}
	layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
	if w.killBuffer != "" {
		t.Fatalf("password kill stored %q", w.killBuffer)
	}
}

func TestInputCommandArrowModifiers(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	// Option+Left moves by word.
	ctx := newInputTest("hello world", "kb11", 11)
	ctx.fireKeyDown(KeyLeft, ModAlt)
	if got := ctx.state().CursorPos; got != 6 {
		t.Fatalf("Option+Left: cursor=%d, want 6", got)
	}
	// Cmd+Left goes to the line start, Cmd+Right to the line end.
	ctx.fireKeyDown(KeyLeft, ModSuper)
	if got := ctx.state().CursorPos; got != 0 {
		t.Fatalf("Cmd+Left: cursor=%d, want 0", got)
	}
	ctx.fireKeyDown(KeyRight, ModSuper)
	if got := ctx.state().CursorPos; got != 11 {
		t.Fatalf("Cmd+Right: cursor=%d, want 11", got)
	}
	// Ctrl+Left is not a word move on macOS.
	ctx.fireKeyDown(KeyLeft, ModCtrl)
	if got := ctx.state().CursorPos; got != 10 {
		t.Fatalf("Ctrl+Left: cursor=%d, want 10", got)
	}
}

func TestInputCommandCmdUpDownDocumentEdges(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTestMultiline("ab\ncd\nef", "kb12", 4)
	ctx.fireKeyDown(KeyDown, ModSuper)
	if got := ctx.state().CursorPos; got != 8 {
		t.Fatalf("Cmd+Down: cursor=%d, want 8", got)
	}
	ctx.fireKeyDown(KeyUp, ModSuper|ModShift)
	is := ctx.state()
	if is.CursorPos != 0 || is.selectBeg != 8 || is.selectEnd != 0 {
		t.Fatalf("Cmd+Shift+Up: cursor=%d sel=%d-%d, want 0 sel 8-0",
			is.CursorPos, is.selectBeg, is.selectEnd)
	}
}

func TestInputControlSuperLeftNotWordMove(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	ctx := newInputTest("hello world", "kb13", 11)
	ctx.fireKeyDown(KeyLeft, ModSuper)
	if got := ctx.state().CursorPos; got != 10 {
		t.Fatalf("Super+Left: cursor=%d, want 10", got)
	}
}

func TestInputReadOnlyBlocksCommandEmacsEdits(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	for _, k := range []KeyCode{KeyD, KeyH, KeyK, KeyY} {
		e := &Event{KeyCode: k, Modifiers: ModCtrl}
		if !inputKeyMutatesText(e, inputSingleLine) {
			t.Errorf("read-only lets Ctrl+%d through on macOS", k)
		}
	}
	e := &Event{KeyCode: KeyV, Modifiers: ModCtrl}
	if inputKeyMutatesText(e, inputSingleLine) {
		t.Error("read-only swallows Ctrl+V on macOS, which is not paste")
	}
}

// TestCommandCmdArrowStaysAtLineEdge: a second Cmd+Left/Right stays at
// the line edge, as in Cocoa. Home/End would cycle on to the text edge.
func TestCommandCmdArrowStaysAtLineEdge(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	ctx := newInputTestMultiline("ab\ncd\nef", "kb14", 4)
	ctx.fireKeyDown(KeyLeft, ModSuper)
	ctx.fireKeyDown(KeyLeft, ModSuper)
	if got := ctx.state().CursorPos; got != 3 {
		t.Fatalf("Cmd+Left twice: cursor=%d, want 3", got)
	}
	ctx.fireKeyDown(KeyRight, ModSuper)
	ctx.fireKeyDown(KeyRight, ModSuper)
	if got := ctx.state().CursorPos; got != 5 {
		t.Fatalf("Cmd+Right twice: cursor=%d, want 5", got)
	}
}

func TestTextSelectCmdArrowStaysAtLineEdge(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	w := newTestWindow()
	layout := generateViewLayout(Text(TextCfg{
		Text: "ab\ncd\nef", Mode: TextModeMultiline, Focusable: true, ID: "kbt2",
	}), w)
	w.SetFocus("kbt2")
	setInputState(w, "kbt2", inputState{CursorPos: 4})
	for range 2 {
		e := &Event{KeyCode: KeyLeft, Modifiers: ModSuper}
		layout.Shape.events.OnKeyDown(EventCtx{&layout, e, w})
	}
	if got := getInputState(w, "kbt2").CursorPos; got != 3 {
		t.Fatalf("Cmd+Left twice: cursor=%d, want 3", got)
	}
}

// TestInputKillMasked: a masked delete keeps the literals, so the kill
// buffer is cleared instead of holding a slice that was not removed.
func TestInputKillMasked(t *testing.T) {
	w := newTestWindow()
	id := "kill-mask"
	compiled, err := compileInputMask("99-99", nil)
	if err != nil {
		t.Fatal(err)
	}
	w.killBuffer = "old"
	setInputState(w, id, inputState{CursorPos: 1})
	hcfg := inputHandlerCfg{CompiledMask: &compiled}
	got, changed := inputKeyKill(hcfg, nil, "12-34", id, 1, w)
	if !changed || got == "12-34" {
		t.Fatalf("masked kill: got %q changed=%v, want a delete", got, changed)
	}
	if w.killBuffer != "" {
		t.Fatalf("masked kill stored %q, want empty", w.killBuffer)
	}
}

// TestInputKillNothingRestoresState: when the delete removes nothing,
// the range Ctrl+K chose must not stay behind as a selection.
func TestInputKillNothingRestoresState(t *testing.T) {
	w := newTestWindow()
	id := "kill-none"
	compiled, err := compileInputMask("99-99", nil)
	if err != nil {
		t.Fatal(err)
	}
	setInputState(w, id, inputState{CursorPos: 5})
	hcfg := inputHandlerCfg{CompiledMask: &compiled}
	// Caret before the literal "-" of an empty-slot tail: nothing to
	// delete but a literal.
	got, changed := inputKeyKill(hcfg, nil, "12-", id, 2, w)
	if changed {
		t.Skipf("mask deleted the literal (%q); no restore path to test", got)
	}
	is := getInputState(w, id)
	if is.selectBeg != is.selectEnd || is.CursorPos != 5 {
		t.Fatalf("state after no-op kill: cursor=%d sel=%d-%d, want 5, none",
			is.CursorPos, is.selectBeg, is.selectEnd)
	}
}
