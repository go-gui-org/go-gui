//go:build js && wasm

package web

import (
	"strings"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// setKeyBinding tells gui which key binding mode the browser's OS uses.
// The web build runs with GOOS=js, so gui cannot pick the mode from
// runtime.GOOS: a Mac browser needs Cmd shortcuts and the Cocoa Emacs
// keys, every other browser needs Ctrl (#969).
func setKeyBinding() {
	gui.SetKeyBindingMode(browserKeyBinding(browserPlatform()))
}

// browserPlatform returns the OS name the browser reports. It prefers
// navigator.userAgentData.platform ("macOS") and falls back to the
// older navigator.platform ("MacIntel", "iPhone").
func browserPlatform() string {
	nav := js.Global().Get("navigator")
	if nav.IsUndefined() || nav.IsNull() {
		return ""
	}
	if uad := nav.Get("userAgentData"); !uad.IsUndefined() && !uad.IsNull() {
		if p := uad.Get("platform"); p.Type() == js.TypeString && p.String() != "" {
			return p.String()
		}
	}
	if p := nav.Get("platform"); p.Type() == js.TypeString {
		return p.String()
	}
	return ""
}

// browserKeyBinding maps a browser platform string to a mode. Apple
// platforms use Cmd. iPadOS Safari reports "MacIntel", which is also
// right: an iPad keyboard uses Cmd.
func browserKeyBinding(platform string) gui.KeyBindingMode {
	p := strings.ToLower(platform)
	for _, apple := range []string{"mac", "iphone", "ipad", "ipod"} {
		if strings.HasPrefix(p, apple) {
			return gui.KeyBindingCommand
		}
	}
	return gui.KeyBindingControl
}
