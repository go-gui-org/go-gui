package gui

import (
	"runtime"
	"testing"
)

func TestShortcutString(t *testing.T) {
	s := Shortcut{Key: KeyS, Modifiers: ModCtrl}
	got := s.String()
	if runtime.GOOS == "darwin" {
		want := "⌃S"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	} else {
		want := "Ctrl+S"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestShortcutStringMultiModifier(t *testing.T) {
	s := Shortcut{Key: KeyZ, Modifiers: ModCtrlShift}
	got := s.String()
	if runtime.GOOS == "darwin" {
		want := "⌃⇧Z"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	} else {
		want := "Ctrl+Shift+Z"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestShortcutStringNoKey(t *testing.T) {
	s := Shortcut{}
	if got := s.String(); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestShortcutIsSet(t *testing.T) {
	if (Shortcut{}).IsSet() {
		t.Error("zero shortcut should not be set")
	}
	if !(Shortcut{Key: KeyA}).IsSet() {
		t.Error("shortcut with key should be set")
	}
}

func TestShortcutMatches(t *testing.T) {
	s := Shortcut{Key: KeyS, Modifiers: ModCtrl}
	hit := &Event{KeyCode: KeyS, Modifiers: ModCtrl}
	miss := &Event{KeyCode: KeyS, Modifiers: ModNone}
	if !s.matches(hit) {
		t.Error("should match")
	}
	if s.matches(miss) {
		t.Error("should not match without modifier")
	}
}

func TestRegisterCommand(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{ID: "test", Label: "Test"})
	cmd, ok := w.CommandByID("test")
	if !ok || cmd.Label != "Test" {
		t.Error("command not found")
	}
}

func TestRegisterCommandDuplicateReturnsError(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	if err := w.RegisterCommand(Command{ID: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := w.RegisterCommand(Command{ID: "x"}); err == nil {
		t.Error("expected error on duplicate ID")
	}
}

func TestRegisterCommandDuplicateShortcutReturnsError(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	s := Shortcut{Key: KeyS, Modifiers: ModCtrl}
	if err := w.RegisterCommand(Command{ID: "a", Shortcut: s}); err != nil {
		t.Fatal(err)
	}
	if err := w.RegisterCommand(Command{ID: "b", Shortcut: s}); err == nil {
		t.Error("expected error on duplicate shortcut")
	}
}

func TestUnregisterCommand(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{ID: "rm"})
	w.UnregisterCommand("rm")
	if _, ok := w.CommandByID("rm"); ok {
		t.Error("command should be removed")
	}
}

func TestCommandCanExecute(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:         "c",
		CanExecute: func(_ *Window) bool { return false },
	})
	if w.CommandCanExecute("c") {
		t.Error("should be disabled")
	}
	if w.CommandCanExecute("nonexistent") {
		t.Error("nonexistent should return false")
	}
}

func TestCommandCanExecuteNil(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{ID: "d"})
	if !w.CommandCanExecute("d") {
		t.Error("nil CanExecute = always enabled")
	}
}

func TestCommandDispatchGlobal(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	called := false
	w.RegisterCommand(Command{
		ID:       "g",
		Shortcut: Shortcut{Key: KeyN, Modifiers: ModCtrl},
		Global:   true,
		Execute:  func(_ *Event, _ *Window) { called = true },
	})
	e := &Event{KeyCode: KeyN, Modifiers: ModCtrl}
	w.commandDispatch(e, true)
	if !called {
		t.Error("global command not dispatched")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

func TestCommandDispatchNonGlobal(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	called := false
	w.RegisterCommand(Command{
		ID:       "ng",
		Shortcut: Shortcut{Key: KeyZ, Modifiers: ModCtrl},
		Execute:  func(_ *Event, _ *Window) { called = true },
	})
	e := &Event{KeyCode: KeyZ, Modifiers: ModCtrl}
	// Should not fire when checking global only.
	w.commandDispatch(e, true)
	if called {
		t.Error("non-global should not fire in global pass")
	}
	// Should fire in non-global pass.
	w.commandDispatch(e, false)
	if !called {
		t.Error("non-global command not dispatched")
	}
}

func TestCommandDispatchCanExecuteFalse(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	called := false
	w.RegisterCommand(Command{
		ID:         "ce",
		Shortcut:   Shortcut{Key: KeyS, Modifiers: ModCtrl},
		Execute:    func(_ *Event, _ *Window) { called = true },
		CanExecute: func(_ *Window) bool { return false },
	})
	e := &Event{KeyCode: KeyS, Modifiers: ModCtrl}
	w.commandDispatch(e, false)
	if called {
		t.Error("should not execute when CanExecute=false")
	}
}

func TestCommandDispatchNoShortcut(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	called := false
	w.RegisterCommand(Command{
		ID:      "ns",
		Execute: func(_ *Event, _ *Window) { called = true },
	})
	e := &Event{KeyCode: KeyA}
	w.commandDispatch(e, false)
	if called {
		t.Error("command with no shortcut should not dispatch")
	}
}

func TestCommandPaletteItems(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommands(
		Command{ID: "a", Label: "Alpha", Group: "G1",
			Shortcut: Shortcut{Key: KeyA, Modifiers: ModCtrl}},
		Command{ID: "b", Label: "Beta"},
		Command{ID: "c"}, // no label, excluded
	)
	items := w.CommandPaletteItems()
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].ID != "a" || items[0].Group != "G1" {
		t.Error("first item mismatch")
	}
	if items[0].Detail == "" {
		t.Error("expected shortcut detail text")
	}
}

func TestKeyNameLetters(t *testing.T) {
	if got := keyName(KeyA); got != "A" {
		t.Errorf("got %q, want A", got)
	}
	if got := keyName(KeyZ); got != "Z" {
		t.Errorf("got %q, want Z", got)
	}
}

func TestKeyNameNumbers(t *testing.T) {
	if got := keyName(Key0); got != "0" {
		t.Errorf("got %q, want 0", got)
	}
	if got := keyName(Key9); got != "9" {
		t.Errorf("got %q, want 9", got)
	}
}

func TestKeyNameFunctionKeys(t *testing.T) {
	if got := keyName(KeyF1); got != "F1" {
		t.Errorf("got %q, want F1", got)
	}
	if got := keyName(KeyF12); got != "F12" {
		t.Errorf("got %q, want F12", got)
	}
	if got := keyName(KeyF25); got != "F25" {
		t.Errorf("got %q, want F25", got)
	}
}

func TestKeyNameSpecial(t *testing.T) {
	tests := []struct {
		key  KeyCode
		want string
	}{
		{KeySpace, "Space"},
		{KeyEnter, "Enter"},
		{KeyEscape, "Esc"},
		{KeyDelete, "Del"},
		{KeyTab, "Tab"},
		{KeyHome, "Home"},
		{KeyEnd, "End"},
	}
	for _, tt := range tests {
		if got := keyName(tt.key); got != tt.want {
			t.Errorf("keyName(%d) = %q, want %q",
				tt.key, got, tt.want)
		}
	}
}

func TestCommandButtonUnknownReturnsErrorView(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	v := CommandButton("nonexistent", ButtonCfg{})
	if v == nil {
		t.Fatal("should return error placeholder view")
	}
	// Verify the error text appears at layout time.
	l := v.GenerateLayout(w)
	if l.Shape == nil {
		t.Fatal("error view should have a shape")
	}
	if l.Shape.TC == nil || l.Shape.TC.Text != "unknown command: nonexistent" {
		t.Errorf("unexpected error text: %q", l.Shape.TC.Text)
	}
}

// A CommandButton must carry an ID: focus traversal is keyed by it, so
// Focusable: true is a silent no-op without one.
func TestCommandButtonDefaultsIDToCommandID(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "edit.increment",
		Label:   "Increment",
		Execute: func(_ *Event, _ *Window) {},
	})

	v := CommandButton("edit.increment", ButtonCfg{})
	l := v.GenerateLayout(w)
	want := commandButtonIDPrefix + "edit.increment"
	if got := l.Shape.ID; got != want {
		t.Errorf("ID = %q, want %q", got, want)
	}
	if !l.Shape.Focusable {
		t.Error("Focusable should survive to the shape")
	}
	// Focusable && ID != "" is what isFocusedTarget requires.
	w.SetFocus(want)
	if !isFocusedTarget(&l, w) {
		t.Error("button is not keyboard-reachable after SetFocus")
	}
}

// The auto-filled ID must not collide with the menu item that the same
// command drives: menu item shapes carry the raw command ID, so an
// unprefixed button ID would put two shapes under one focus ID.
func TestCommandButtonIDDoesNotCollideWithMenuItem(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "file.new",
		Label:   "New",
		Execute: func(_ *Event, _ *Window) {},
	})

	btn := CommandButton("file.new", ButtonCfg{})
	btnID := btn.GenerateLayout(w).Shape.ID

	// A menubar item for the same command renders a shape keyed by the
	// raw command ID (view_menu_item.go sets ID: itemCfg.ID).
	if btnID == "file.new" {
		t.Errorf("button ID %q collides with the menu item ID for the "+
			"same command", btnID)
	}
}

// An explicit ID must win over the command-ID default.
func TestCommandButtonExplicitIDWins(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "edit.undo",
		Label:   "Undo",
		Execute: func(_ *Event, _ *Window) {},
	})

	v := CommandButton("edit.undo", ButtonCfg{ID: "custom"})
	l := v.GenerateLayout(w)
	if got := l.Shape.ID; got != "custom" {
		t.Errorf("ID = %q, want %q", got, "custom")
	}
}

func TestUnregisterCommandNoOp(_ *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	// Should not panic when unregistering a non-existent ID.
	w.UnregisterCommand("does-not-exist")
}

func TestCommandButtonAutoLabel(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:       "test.cmd",
		Label:    "Test",
		Shortcut: Shortcut{Key: KeyD, Modifiers: ModCtrl},
		Execute:  func(_ *Event, _ *Window) {},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	l := v.GenerateLayout(w)

	// auto-label should produce a Text child with the command label.
	if len(l.Children) == 0 || l.Children[0].Shape == nil ||
		l.Children[0].Shape.TC == nil ||
		l.Children[0].Shape.TC.Text == "" {
		t.Fatal("auto-label should set text content in child")
	}
}

func TestCommandButtonAutoDisable(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "test.cmd",
		Label:   "Test",
		Execute: func(_ *Event, _ *Window) {},
		CanExecute: func(_ *Window) bool {
			return false
		},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	l := v.GenerateLayout(w)

	if !l.Shape.Disabled {
		t.Error("button should be disabled when CanExecute returns false")
	}
}

func TestCommandButtonAutoDisableWhenCanExecuteTrue(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "test.cmd",
		Label:   "Test",
		Execute: func(_ *Event, _ *Window) {},
		CanExecute: func(_ *Window) bool {
			return true
		},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	l := v.GenerateLayout(w)

	if l.Shape.Disabled {
		t.Error("button should not be disabled when CanExecute returns true")
	}
}

func TestCommandButtonOnClickWiring(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	executed := false
	w.RegisterCommand(Command{
		ID:    "test.cmd",
		Label: "Test",
		Execute: func(_ *Event, _ *Window) {
			executed = true
		},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	l := v.GenerateLayout(w)

	if l.Shape.events.OnClick == nil {
		t.Fatal("OnClick should be wired")
	}

	e := &Event{}
	l.Shape.events.OnClick(&l, e, w)
	if !executed {
		t.Error("OnClick should execute the command")
	}
}

func TestCommandButtonOnClickChecksCanExecute(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	executed := false
	w.RegisterCommand(Command{
		ID:    "test.cmd",
		Label: "Test",
		Execute: func(_ *Event, _ *Window) {
			executed = true
		},
		CanExecute: func(_ *Window) bool {
			return false
		},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	l := v.GenerateLayout(w)

	e := &Event{}
	l.Shape.events.OnClick(&l, e, w)
	if executed {
		t.Error("OnClick should not execute when CanExecute returns false")
	}
}

func TestCommandButtonUserContentOverridesAutoLabel(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "test.cmd",
		Label:   "Auto",
		Execute: func(_ *Event, _ *Window) {},
	})

	userContent := []View{Text(TextCfg{Text: "Custom"})}
	v := CommandButton("test.cmd", ButtonCfg{Content: userContent})
	l := v.GenerateLayout(w)

	// User-provided Content should be preserved, not overwritten.
	if len(l.Children) == 0 || l.Children[0].Shape == nil ||
		l.Children[0].Shape.TC == nil ||
		l.Children[0].Shape.TC.Text != "Custom" {
		t.Errorf("user content should be preserved, got %q",
			l.Children[0].Shape.TC.Text)
	}
}

func TestCommandButtonUserOnClickOverridesAutoWiring(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	cmdExecuted := false
	w.RegisterCommand(Command{
		ID:    "test.cmd",
		Label: "Test",
		Execute: func(_ *Event, _ *Window) {
			cmdExecuted = true
		},
	})

	userExecuted := false
	v := CommandButton("test.cmd", ButtonCfg{
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			userExecuted = true
		},
	})
	l := v.GenerateLayout(w)

	e := &Event{}
	l.Shape.events.OnClick(&l, e, w)
	if cmdExecuted {
		t.Error("command should not execute when user provides OnClick")
	}
	if !userExecuted {
		t.Error("user OnClick should be called")
	}
}

func TestCommandButtonViewFuncType(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:      "test.cmd",
		Label:   "Test",
		Execute: func(_ *Event, _ *Window) {},
	})

	v := CommandButton("test.cmd", ButtonCfg{})
	if _, ok := v.(ViewFunc); !ok {
		t.Error("CommandButton should return ViewFunc")
	}

	// Content() should return nil (ViewFunc never has eager content).
	if c := v.Content(); c != nil {
		t.Error("ViewFunc Content() should return nil")
	}
}

func TestCommandDispatchNoMatch(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int)})
	w.RegisterCommand(Command{
		ID:       "x",
		Shortcut: Shortcut{Key: KeyA, Modifiers: ModCtrl},
		Execute:  func(_ *Event, _ *Window) {},
	})
	e := &Event{KeyCode: KeyB, Modifiers: ModCtrl}
	w.commandDispatch(e, false)
	if e.IsHandled {
		t.Error("event should not be handled when no command matches")
	}
}

func TestShortcutMatchesZeroValue(t *testing.T) {
	s := Shortcut{} // Key == KeyInvalid
	e := &Event{KeyCode: KeyInvalid}
	if s.matches(e) {
		t.Error("zero-value shortcut should not match")
	}
}
