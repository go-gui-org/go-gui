package gui

import "testing"

// With GOGUI_EMULATE_CLIPBOARD set, the clipboard and PRIMARY live in the
// window. The functions a backend installs are never called, whatever
// order the backend installs them in (#971).
func TestEmulatedClipboardIgnoresBackendFns(t *testing.T) {
	t.Setenv(emulateClipboardEnv, "1")
	t.Setenv(clipboardTextEnv, "seed")
	w := NewWindow(WindowCfg{})

	called := false
	w.SetClipboardFn(func(string) { called = true })
	w.SetClipboardGetFn(func() string { called = true; return "system" })
	w.SetPrimaryFn(func(string) { called = true })
	w.SetPrimaryGetFn(func() string { called = true; return "system" })

	if got := w.GetClipboard(); got != "seed" {
		t.Fatalf("initial clipboard = %q, want %q", got, "seed")
	}
	w.SetClipboard("copied")
	if got := w.GetClipboard(); got != "copied" {
		t.Fatalf("clipboard = %q, want %q", got, "copied")
	}
	// PRIMARY is a separate buffer: setting it leaves the clipboard alone.
	if got := w.GetPrimary(); got != "" {
		t.Fatalf("initial primary = %q, want empty", got)
	}
	w.SetPrimary("selected")
	if got := w.GetPrimary(); got != "selected" {
		t.Fatalf("primary = %q, want %q", got, "selected")
	}
	if got := w.GetClipboard(); got != "copied" {
		t.Fatalf("clipboard after SetPrimary = %q, want %q", got, "copied")
	}
	if called {
		t.Fatal("a backend clipboard function was called under emulation")
	}
}

// Each window gets its own buffers, the same as each window of a real app
// would see one system clipboard only through its own backend fns. The
// seed text applies to every window.
func TestEmulatedClipboardPerWindow(t *testing.T) {
	t.Setenv(emulateClipboardEnv, "true")
	t.Setenv(clipboardTextEnv, "seed")
	a := NewWindow(WindowCfg{})
	b := NewWindow(WindowCfg{})
	a.SetClipboard("a")
	if got := b.GetClipboard(); got != "seed" {
		t.Fatalf("window b clipboard = %q, want %q", got, "seed")
	}
}

// Without the variable, the backend functions are used as before.
func TestClipboardUsesBackendFnsByDefault(t *testing.T) {
	t.Setenv(emulateClipboardEnv, "0")
	t.Setenv(clipboardTextEnv, "seed")
	w := NewWindow(WindowCfg{})
	var stored string
	w.SetClipboardFn(func(s string) { stored = s })
	w.SetClipboardGetFn(func() string { return stored })
	if got := w.GetClipboard(); got != "" {
		t.Fatalf("initial clipboard = %q, want empty (seed needs emulation)", got)
	}
	w.SetClipboard("x")
	if stored != "x" || w.GetClipboard() != "x" {
		t.Fatalf("backend fns not used: stored=%q", stored)
	}
}
