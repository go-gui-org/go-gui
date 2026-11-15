//go:build js && wasm

package web

// visibility.go — the page hidden or shown again (issue #1170).

import (
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// registerVisibility reports the page's visibility to gui: another tab
// or a minimized browser hides it. requestAnimationFrame stops while
// hidden, so gui has to learn it here. The first report covers a page
// opened in a background tab; for a visible one it changes nothing.
func (b *Backend) registerVisibility(w *gui.Window, doc js.Value) {
	f := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		syncPageHidden(w, doc)
		return nil
	})
	b.callbacks = append(b.callbacks, f)
	doc.Call("addEventListener", "visibilitychange", f)
	syncPageHidden(w, doc)
}

// syncPageHidden tells gui whether the browser shows the page at all
// (document.hidden). gui ignores a repeat of the current state, and a
// document without the property reads as shown.
func syncPageHidden(w *gui.Window, doc js.Value) {
	gui.DispatchWindowOccluded(w, doc.Get("hidden").Truthy())
}

// CanHideWindow tells gui that Hide cannot take the window off the
// screen, so Window.IsVisible stays true (issue #1170).
func (n *nativePlatform) CanHideWindow() bool { return false }
