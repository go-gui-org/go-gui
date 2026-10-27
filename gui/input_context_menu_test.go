package gui

import "testing"

// inputMenuTestWindow renders one Input whose text lives in *text, so
// OnTextChanged round-trips the way an app would wire it.
func inputMenuTestWindow(t *testing.T, cfg InputCfg, text *string) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		c := cfg
		c.Text = *text
		c.OnTextChanged = func(s string, _ EventCtx) { *text = s }
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Content: []View{Input(c)},
		})
	})
	return w
}

// rightPress sends a right-button press and release at the center of
// the widget, the way the backend would.
func rightPress(t *testing.T, w *Window, id string) *Event {
	t.Helper()
	ly, err := w.testTarget(id)
	if err != nil {
		t.Fatalf("testTarget(%q): %v", id, err)
	}
	x, y, err := testHitPoint(ly, id)
	if err != nil {
		t.Fatalf("testHitPoint(%q): %v", id, err)
	}
	down := Event{
		Type: EventMouseDown, MouseButton: MouseRight,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&down)
	w.settle()
	up := Event{
		Type: EventMouseUp, MouseButton: MouseRight,
		MouseX: x, MouseY: y,
	}
	w.EventFn(&up)
	w.settle()
	return &down
}

// selectRange stores a selection on the field, as a drag would.
func selectRange(w *Window, id string, beg, end int) {
	is := inputStateOrDefault(id, w)
	is.selectBeg = uint32(beg)
	is.selectEnd = uint32(end)
	is.CursorPos = end
	inputStoreState(id, w, is)
}

// With the menu on, the field takes every button (OnAnyClick), so a
// right press reaches inputOnClick. It must not run the left-click path
// there: no caret move, no collapsed selection, no drag lock. Before
// #1151 the handler's OnClick took the left button only, so this held by
// construction; the guard is for the new route.
func TestInputRightClickKeepsSelection(t *testing.T) {
	text := "hello world"
	w := inputMenuTestWindow(t, InputCfg{ID: "name"}, &text)
	w.SetFocus("name")
	w.settle()
	selectRange(w, "name", 0, 5)

	rightPress(t, w, "name")

	is := inputStateOrDefault("name", w)
	if is.selectBeg != 0 || is.selectEnd != 5 {
		t.Errorf("selection = [%d,%d), want [0,5)",
			is.selectBeg, is.selectEnd)
	}
	if w.mouseIsLocked() {
		t.Error("right press took a drag lock")
	}
}

// menuItemID is the effective ID of an item of the open input menu.
func menuItemID(item string) string { return ScopeID(inputMenuFocusID, item) }

// itemDisabled reports whether the open menu shows item as disabled.
func itemDisabled(t *testing.T, w *Window, item string) bool {
	t.Helper()
	ly, ok := w.TestRender(nil).findByID(menuItemID(item))
	if !ok {
		t.Fatalf("menu item %q not in the layout", item)
	}
	return ly.Shape.Disabled
}

// openMenuWith renders a field holding text, selects [beg,end) and
// right-presses it.
func openMenuWith(
	t *testing.T, cfg InputCfg, text *string, clip string, beg, end int,
) *Window {
	t.Helper()
	w := inputMenuTestWindow(t, cfg, text)
	w.SetClipboardGetFn(func() string { return clip })
	w.SetFocus("name")
	w.settle()
	selectRange(w, "name", beg, end)
	if !rightPress(t, w, "name").IsHandled {
		t.Fatal("right press not handled")
	}
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("focus = %q, want the menu", w.FocusID())
	}
	return w
}

func TestInputContextMenuOpensAndKeepsSelection(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	is := inputStateOrDefault("name", w)
	if is.selectBeg != 0 || is.selectEnd != 5 {
		t.Errorf("selection = [%d,%d), want [0,5)", is.selectBeg, is.selectEnd)
	}
	for _, item := range []string{
		inputMenuCut, inputMenuCopy, inputMenuPaste, inputMenuSelectAll,
	} {
		if itemDisabled(t, w, item) {
			t.Errorf("%s disabled, want enabled", item)
		}
	}
}

func TestInputContextMenuItemStates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cfg      InputCfg
		text     string
		clip     string
		beg, end int
		disabled map[string]bool
	}{
		{"no selection", InputCfg{ID: "name"}, "abc", "x", 1, 1,
			map[string]bool{inputMenuCut: true, inputMenuCopy: true}},
		{"empty clipboard", InputCfg{ID: "name"}, "abc", "", 0, 3,
			map[string]bool{inputMenuPaste: true}},
		{"read only", InputCfg{ID: "name", ReadOnly: true}, "abc", "x", 0, 3,
			map[string]bool{inputMenuCut: true, inputMenuPaste: true}},
		{"password", InputCfg{ID: "name", IsPassword: true}, "abc", "x", 0, 3,
			map[string]bool{inputMenuCut: true, inputMenuCopy: true}},
		{"empty text", InputCfg{ID: "name"}, "", "x", 0, 0,
			map[string]bool{
				inputMenuCut: true, inputMenuCopy: true,
				inputMenuSelectAll: true,
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := tc.text
			w := openMenuWith(t, tc.cfg, &text, tc.clip, tc.beg, tc.end)
			for _, item := range []string{
				inputMenuCut, inputMenuCopy, inputMenuPaste, inputMenuSelectAll,
			} {
				if got := itemDisabled(t, w, item); got != tc.disabled[item] {
					t.Errorf("%s disabled = %v, want %v", item, got, tc.disabled[item])
				}
			}
		})
	}
}

func TestInputContextMenuActions(t *testing.T) {
	for _, tc := range []struct {
		item     string
		clip     string
		wantText string
		wantClip string
	}{
		{inputMenuCopy, "", "hello world", "hello"},
		{inputMenuCut, "", " world", "hello"},
		{inputMenuPaste, "HEY", "HEY world", "HEY"},
	} {
		t.Run(tc.item, func(t *testing.T) {
			text := "hello world"
			clip := tc.clip
			w := inputMenuTestWindow(t, InputCfg{ID: "name"}, &text)
			w.SetClipboardGetFn(func() string { return clip })
			w.SetClipboardFn(func(s string) { clip = s })
			w.SetFocus("name")
			w.settle()
			selectRange(w, "name", 0, 5)
			rightPress(t, w, "name")
			if err := w.TestClick(menuItemID(tc.item)); err != nil {
				t.Fatalf("TestClick(%s): %v", tc.item, err)
			}
			if text != tc.wantText {
				t.Errorf("text = %q, want %q", text, tc.wantText)
			}
			if clip != tc.wantClip {
				t.Errorf("clipboard = %q, want %q", clip, tc.wantClip)
			}
			if !w.IsFocus("name") {
				t.Errorf("focus = %q, want name", w.FocusID())
			}
			if _, ok := w.TestRender(nil).findByID(inputMenuFocusID); ok {
				t.Error("menu still open after the action")
			}
		})
	}
}

func TestInputContextMenuSelectAll(t *testing.T) {
	text := "hello"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "", 2, 2)
	if err := w.TestClick(menuItemID(inputMenuSelectAll)); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	is := inputStateOrDefault("name", w)
	if is.selectBeg != 0 || is.selectEnd != 5 {
		t.Errorf("selection = [%d,%d), want [0,5)", is.selectBeg, is.selectEnd)
	}
}

// A paste through the menu obeys the field's validator, as Ctrl+V does.
func TestInputContextMenuPasteHonoursPreTextChange(t *testing.T) {
	text := "ab"
	cfg := InputCfg{
		ID:            "name",
		PreTextChange: func(_, _ string) (string, bool) { return "", false },
	}
	w := openMenuWith(t, cfg, &text, "zz", 2, 2)
	if err := w.TestClick(menuItemID(inputMenuPaste)); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if text != "ab" {
		t.Errorf("text = %q, want the rejected paste to leave ab", text)
	}
}

// Opening the menu moves focus to it, but the field must not commit
// or blur: in a native toolkit it keeps focus under its context menu.
func TestInputContextMenuOpenDoesNotBlur(t *testing.T) {
	text := "abc"
	commits := 0
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Input(InputCfg{
				ID:   "name",
				Text: text,
				OnTextCommit: func(string, InputCommitReason, EventCtx) {
					commits++
				},
			}),
		}})
	})
	w.SetFocus("name")
	w.settle()
	rightPress(t, w, "name")
	w.TestRender(nil)
	if commits != 0 {
		t.Errorf("blur commits on open = %d, want 0", commits)
	}
}

// Escape closes the menu and hands focus back to the field.
func TestInputContextMenuEscapeRestoresFocus(t *testing.T) {
	text := "abc"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "", 0, 1)
	down := Event{Type: EventKeyDown, KeyCode: KeyEscape}
	w.EventFn(&down)
	w.settle()
	if !w.IsFocus("name") {
		t.Errorf("focus = %q, want name", w.FocusID())
	}
	if _, ok := w.TestRender(nil).findByID(inputMenuFocusID); ok {
		t.Error("menu still open after Escape")
	}
}

// With the menu off, the right press is not the field's: it travels on
// to an app's own ContextMenu.
func TestInputContextMenuDisabledLeavesPress(t *testing.T) {
	text := "abc"
	w := inputMenuTestWindow(t, InputCfg{ID: "name", ContextMenuDisabled: true}, &text)
	if rightPress(t, w, "name").IsHandled {
		t.Error("right press handled with the menu off")
	}
	if _, ok := w.TestRender(nil).findByID(inputMenuFocusID); ok {
		t.Error("menu opened with ContextMenuDisabled")
	}
}

// A field in the bottom-right corner opens its menu inside the window:
// the menu hangs from an anchor and flips back from the edges.
func TestInputContextMenuStaysInWindow(t *testing.T) {
	text := "abc"
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			HAlign: HAlignEnd,
			VAlign: VAlignBottom,
			Content: []View{Input(InputCfg{
				ID: "name", Text: text, Width: 60, Sizing: FixedFit,
			})},
		})
	})
	rightPress(t, w, "name")
	ly, ok := w.TestRender(nil).findByID(inputMenuFocusID)
	if !ok {
		t.Fatal("menu not open")
	}
	s := ly.Shape
	ww, wh := float32(w.windowWidth), float32(w.windowHeight)
	if s.X < 0 || s.Y < 0 || s.X+s.Width > ww || s.Y+s.Height > wh {
		t.Errorf("menu at (%v,%v %vx%v), outside the %vx%v window",
			s.X, s.Y, s.Width, s.Height, ww, wh)
	}
}

// An Input in a modal dialog opens its menu too, and its actions reach
// the field: the menu is a layer above the dialog.
func TestInputContextMenuInDialog(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	w.Dialog(DialogCfg{
		Title: "t", DialogType: DialogPrompt, FocusID: "reply", Reply: "abc",
	})
	w.TestRender(nil)
	ids := w.ResolveID("reply")
	if len(ids) != 1 {
		t.Fatalf("ResolveID(reply) = %v, want one", ids)
	}
	id := ids[0]
	w.SetFocus(id)
	w.settle()
	selectRange(w, id, 0, 3)
	rightPress(t, w, id)
	if err := w.TestClick(menuItemID(inputMenuCut)); err != nil {
		t.Fatalf("TestClick(cut): %v", err)
	}
	if got := w.dialogCfg.Reply; got != "" {
		t.Errorf("reply = %q, want the cut to empty it", got)
	}
	if !w.IsFocus(id) {
		t.Errorf("focus = %q, want %q", w.FocusID(), id)
	}
}

// The layer under an open menu must not see the pointer. layoutHover
// stops only at a shape that handles OnHover, and a disabled item, a
// separator or the panel's padding handles none, so without a handler
// on the panel the hover fell through to the widgets below it.
func TestInputContextMenuBlocksHoverBelow(t *testing.T) {
	text := "hello world"
	fired := 0
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			OnHover: func(EventCtx) { fired++ },
			Content: []View{Input(InputCfg{
				ID:            "name",
				Text:          text,
				OnTextChanged: func(s string, _ EventCtx) { text = s },
			})},
		})
	})
	w.SetClipboardGetFn(func() string { return "" })
	w.SetFocus("name")
	w.settle()
	rightPress(t, w, "name")
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("focus = %q, want the menu", w.FocusID())
	}
	// No selection and an empty clipboard: Paste is disabled.
	fired = 0
	hoverOver(t, w, menuItemID(inputMenuPaste))
	if fired != 0 {
		t.Errorf("OnHover under the menu fired %d times, want 0", fired)
	}
}

// Enter on an item runs it the way a click does, and the field keeps
// focus after it. menuOnKeyDown cleared focus after the action even when
// the action had given focus back to the field, so the field lost focus
// and fired its blur commit.
func TestInputContextMenuEnterKeepsFieldFocus(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "", 0, 5)
	// The first selectable item, Cut, is highlighted on open.
	down := Event{Type: EventKeyDown, KeyCode: KeyEnter}
	w.EventFn(&down)
	w.settle()
	if text != " world" {
		t.Errorf("text = %q, want %q", text, " world")
	}
	if !w.IsFocus("name") {
		t.Errorf("focus = %q, want name", w.FocusID())
	}
}

// A second right press on the field while its menu is open opens the
// menu again on the same selection. The press closes the menu and clears
// focus first (handleMouseDownEvent), and that clears every input
// selection, so the reopened menu read an empty one.
func TestInputContextMenuSecondRightPressKeepsSelection(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "", 0, 5)
	rightPress(t, w, "name")
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("focus = %q, want the menu", w.FocusID())
	}
	if itemDisabled(t, w, inputMenuCut) || itemDisabled(t, w, inputMenuCopy) {
		t.Error("Cut or Copy disabled after the second right press")
	}
	is := inputStateOrDefault("name", w)
	if is.selectBeg != 0 || is.selectEnd != 5 {
		t.Errorf("selection = [%d,%d), want [0,5)", is.selectBeg, is.selectEnd)
	}
}

// Over a modal dialog the menu still takes the pointer. layoutHover let
// only shapes with the dialog as an ancestor see it, and the menu layer
// is injected above the dialog with no such ancestor: its items did not
// highlight, and its panel could not hold the hover off the dialog.
func TestInputContextMenuHoverInDialog(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	w.Dialog(DialogCfg{
		Title: "t", DialogType: DialogPrompt, FocusID: "reply", Reply: "abc",
	})
	w.TestRender(nil)
	id := w.ResolveID("reply")[0]
	w.SetFocus(id)
	w.settle()
	selectRange(w, id, 0, 3)
	rightPress(t, w, id)
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("focus = %q, want the menu", w.FocusID())
	}
	hoverOver(t, w, menuItemID(inputMenuCopy))
	got := StateReadOr(w, nsMenu, inputMenuFocusID, "")
	if got != inputMenuCopy {
		t.Errorf("highlighted item = %q, want %q", got, inputMenuCopy)
	}
}

// A press on a part of the menu that runs nothing (a disabled item, the
// separator, the padding) leaves the menu open, as a native menu does:
// a press inside the focused menu dismisses nothing (pressInFocusedMenu).
// The field still counts as focused and gets no blur commit.
func TestInputContextMenuPressOnDisabledItemKeepsMenu(t *testing.T) {
	text := "hello world"
	commits := 0
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Input(InputCfg{
				ID:            "name",
				Text:          text,
				OnTextChanged: func(s string, _ EventCtx) { text = s },
				OnTextCommit: func(string, InputCommitReason, EventCtx) {
					commits++
				},
			})},
		})
	})
	w.SetClipboardGetFn(func() string { return "" })
	w.SetFocus("name")
	w.settle()
	rightPress(t, w, "name")
	if !itemDisabled(t, w, inputMenuPaste) {
		t.Fatal("Paste enabled with an empty clipboard")
	}
	x, y := hitPoint(t, w, menuItemID(inputMenuPaste))
	pressAt(w, MouseLeft, x, y)
	releaseAt(w, MouseLeft, x, y)
	if !w.IsFocus(inputMenuFocusID) {
		t.Errorf("focus = %q, want the menu", w.FocusID())
	}
	if !inputMenuHolds(w, "name") {
		t.Error("menu no longer holds the field")
	}
	if commits != 0 {
		t.Errorf("OnTextCommit fired %d times, want 0", commits)
	}
}

// An item runs against the field as it is now, not as it was when the
// menu opened. Here OnTextChanged writes to a target the app swaps while
// the menu is open, and the cut must reach the new one.
func TestInputContextMenuUsesCurrentHandlers(t *testing.T) {
	var first, second string
	target := &first
	text := "hello world"
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		dst := target
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Input(InputCfg{
				ID:            "name",
				Text:          text,
				OnTextChanged: func(s string, _ EventCtx) { *dst = s },
			})},
		})
	})
	w.SetFocus("name")
	w.settle()
	selectRange(w, "name", 0, 5)
	rightPress(t, w, "name")
	target = &second
	w.TestRender(nil)
	if err := w.TestClick(menuItemID(inputMenuCut)); err != nil {
		t.Fatalf("TestClick(cut): %v", err)
	}
	if first != "" || second != " world" {
		t.Errorf("first = %q, second = %q, want \"\" and %q",
			first, second, " world")
	}
}

// A menu item runs on the release, not the press. The press only
// highlights it, so the menu is still there while the button is held, and
// whatever is under the menu gets no press when it closes.
func TestInputContextMenuItemRunsOnRelease(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	x, y := hitPoint(t, w, menuItemID(inputMenuCut))
	pressAt(w, MouseLeft, x, y)
	if text != "hello world" {
		t.Fatalf("after press: text = %q, want it unchanged", text)
	}
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("after press: focus = %q, want the menu", w.FocusID())
	}
	releaseAt(w, MouseLeft, x, y)
	if text != " world" {
		t.Errorf("after release: text = %q, want %q", text, " world")
	}
	if !w.IsFocus("name") {
		t.Errorf("after release: focus = %q, want name", w.FocusID())
	}
}

// A release runs an item only when the press landed on that same item.
// A press on Copy dragged to Cut runs nothing, and so does a release
// with no press behind it, such as the release of the right press that
// opened the menu.
func TestInputContextMenuReleaseNeedsPressOnItem(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	cx, cy := hitPoint(t, w, menuItemID(inputMenuCut))
	releaseAt(w, MouseLeft, cx, cy)
	if text != "hello world" {
		t.Fatalf("release alone: text = %q, want it unchanged", text)
	}
	px, py := hitPoint(t, w, menuItemID(inputMenuCopy))
	pressAt(w, MouseLeft, px, py)
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx, MouseY: cy})
	w.settle()
	releaseAt(w, MouseLeft, cx, cy)
	if text != "hello world" {
		t.Errorf("press Copy, release Cut: text = %q, want it unchanged", text)
	}
}

// A touch tap releases through synthMouse, not handleMouseUpEvent, and
// must run the item too: the release target is kept where every release
// path passes (releaseMouseButton).
func TestInputContextMenuTouchTapRunsItem(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	x, y := hitPoint(t, w, menuItemID(inputMenuCut))
	touchAt(w, EventTouchesBegan, 0, x, y)
	touchAt(w, EventTouchesEnded, 60_000_000, x, y)
	if text != " world" {
		t.Errorf("after tap: text = %q, want %q", text, " world")
	}
}

// A right click on an item runs nothing: only a left press arms the
// release (recordPressTarget), and the menu stays open for a left click.
func TestInputContextMenuRightClickOnItemRunsNothing(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	x, y := hitPoint(t, w, menuItemID(inputMenuCut))
	pressAt(w, MouseRight, x, y)
	releaseAt(w, MouseRight, x, y)
	if text != "hello world" {
		t.Errorf("text = %q, want it unchanged", text)
	}
	if !w.IsFocus(inputMenuFocusID) {
		t.Errorf("focus = %q, want the menu", w.FocusID())
	}
}

// A right click while the left button is held on an item does not drop
// the left press: its release still runs the item (#1098 keeps the left
// target across another button's release).
func TestInputContextMenuRightClickMidPressKeepsItem(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	x, y := hitPoint(t, w, menuItemID(inputMenuCut))
	pressAt(w, MouseLeft, x, y)
	pressAt(w, MouseRight, x, y)
	releaseAt(w, MouseRight, x, y)
	if text != "hello world" {
		t.Fatalf("after right click: text = %q, want it unchanged", text)
	}
	releaseAt(w, MouseLeft, x, y)
	if text != " world" {
		t.Errorf("after left release: text = %q, want %q", text, " world")
	}
}

// While the menu is open the field keeps its focus border, also under
// the pointer. The amend pass counts the menu (inputMenuHolds); the hover
// pass read IsFocus alone, so a move over the field dropped the border.
func TestInputContextMenuHoverKeepsFocusBorder(t *testing.T) {
	text := "hello world"
	w := openMenuWith(t, InputCfg{ID: "name"}, &text, "x", 0, 5)
	ly, ok := w.layout.FindByID("name")
	if !ok {
		t.Fatal("no field")
	}
	want := ly.Shape.ColorBorder
	// The top-left corner is outside the menu, which opened at the
	// press point (the field's center).
	w.EventFn(&Event{
		Type: EventMouseMove, MouseX: ly.Shape.X + 2, MouseY: ly.Shape.Y + 2,
	})
	w.settle()
	ly, _ = w.layout.FindByID("name")
	if got := ly.Shape.ColorBorder; got != want {
		t.Errorf("border under hover = %v, want the focus border %v", got, want)
	}
}

// A NumericInput's frame follows its inner field's focus. The open
// Input menu holds that focus, so the frame keeps its focus border too,
// in the amend pass and under the pointer.
func TestInputContextMenuNumericFrameKeepsFocusBorder(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{NumericInput(NumericInputCfg{
				ID: "ni", StepCfg: NumericStepCfg{ShowButtons: true},
			})},
		})
	})
	field := ScopeID("ni", "field")
	frame, ok := w.layout.FindByID("ni")
	if !ok {
		t.Fatal("no numeric frame")
	}
	unfocused := frame.Shape.ColorBorder
	w.SetFocus(field)
	w.settle()
	frame, _ = w.TestRender(nil).findByID("ni")
	want := frame.Shape.ColorBorder
	if want == unfocused {
		t.Fatal("frame shows no focus border with the field focused")
	}
	rightPress(t, w, field)
	if !w.IsFocus(inputMenuFocusID) {
		t.Fatalf("focus = %q, want the menu", w.FocusID())
	}
	frame, _ = w.layout.FindByID("ni")
	if got := frame.Shape.ColorBorder; got != want {
		t.Errorf("frame border with menu open = %v, want %v", got, want)
	}
}
