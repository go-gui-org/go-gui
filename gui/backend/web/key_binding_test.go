//go:build js && wasm

package web

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestBrowserKeyBinding(t *testing.T) {
	cases := map[string]gui.KeyBindingMode{
		"macOS":        gui.KeyBindingCommand,
		"MacIntel":     gui.KeyBindingCommand,
		"iPhone":       gui.KeyBindingCommand,
		"iPad":         gui.KeyBindingCommand,
		"Windows":      gui.KeyBindingControl,
		"Win32":        gui.KeyBindingControl,
		"Linux x86_64": gui.KeyBindingControl,
		"Android":      gui.KeyBindingControl,
		"":             gui.KeyBindingControl,
	}
	for in, want := range cases {
		if got := browserKeyBinding(in); got != want {
			t.Errorf("browserKeyBinding(%q) = %d, want %d", in, got, want)
		}
	}
}
