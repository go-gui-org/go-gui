package gui

import "testing"

// setKeyBindingForTest switches the process-wide mode for one test and
// restores it after. The mode is global, so a caller must not run in
// parallel with other tests.
func setKeyBindingForTest(t *testing.T, m KeyBindingMode) {
	t.Helper()
	prev := keyBinding.Load()
	keyBinding.Store(uint32(m))
	t.Cleanup(func() { keyBinding.Store(prev) })
}

func TestDefaultKeyBinding(t *testing.T) {
	cases := map[string]KeyBindingMode{
		"darwin":  KeyBindingCommand,
		"ios":     KeyBindingCommand,
		"linux":   KeyBindingControl,
		"windows": KeyBindingControl,
		"android": KeyBindingControl,
		"js":      KeyBindingControl,
	}
	for goos, want := range cases {
		if got := defaultKeyBinding(goos); got != want {
			t.Errorf("defaultKeyBinding(%q) = %d, want %d", goos, got, want)
		}
	}
}

func TestParseKeyBinding(t *testing.T) {
	cases := []struct {
		in   string
		want KeyBindingMode
		ok   bool
	}{
		{"command", KeyBindingCommand, true},
		{" CMD ", KeyBindingCommand, true},
		{"control", KeyBindingControl, true},
		{"ctrl", KeyBindingControl, true},
		{"", KeyBindingControl, false},
		{"emacs", KeyBindingControl, false},
	}
	for _, c := range cases {
		got, ok := parseKeyBinding(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("parseKeyBinding(%q) = %d,%v, want %d,%v",
				c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestSetKeyBindingModeIgnoredWhenPinned(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingControl)
	keyBindingPinned = true
	t.Cleanup(func() { keyBindingPinned = false })
	SetKeyBindingMode(KeyBindingCommand)
	if currentKeyBinding() != KeyBindingControl {
		t.Fatal("SetKeyBindingMode changed a mode pinned by the env var")
	}
}

func TestKeyBindingModifiers(t *testing.T) {
	setKeyBindingForTest(t, KeyBindingCommand)
	if ShortcutModifier() != ModSuper {
		t.Error("command: shortcut modifier is not Cmd")
	}
	if isShortcut(ModCtrl) {
		t.Error("command: Ctrl counts as the shortcut modifier")
	}
	if isWordMod(ModCtrl) || isWordMod(ModSuper) || !isWordMod(ModAlt) {
		t.Error("command: only Option moves by word")
	}
	if !isLineMod(ModSuper) {
		t.Error("command: Cmd+arrow does not move to the line edge")
	}
	if !isCocoaEmacs(ModCtrl|ModLMB) || isCocoaEmacs(ModCtrlShift) {
		t.Error("command: Emacs keys need Ctrl alone (mouse bits ignored)")
	}

	setKeyBindingForTest(t, KeyBindingControl)
	if ShortcutModifier() != ModCtrl {
		t.Error("control: shortcut modifier is not Ctrl")
	}
	if isShortcut(ModSuper) {
		t.Error("control: Super counts as the shortcut modifier")
	}
	if isWordMod(ModSuper) || !isWordMod(ModCtrl) {
		t.Error("control: Ctrl moves by word, Super does not")
	}
	if isLineMod(ModSuper) || isCocoaEmacs(ModCtrl) {
		t.Error("control: macOS-only keys are active")
	}
}
