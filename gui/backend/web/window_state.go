//go:build js && wasm

package web

// No window to place on the web, only fullscreen (fullscreen.go).
func (n *nativePlatform) MinimizeWindow()                  {}
func (n *nativePlatform) MaximizeWindow()                  {}
func (n *nativePlatform) RestoreWindow()                   {}
func (n *nativePlatform) ResizeWindow(_, _ int)            {}
func (n *nativePlatform) MoveWindow(_, _ int)              {}
func (n *nativePlatform) WindowPosition() (int, int, bool) { return 0, 0, false }

// SetWindowFullscreen enters or leaves fullscreen for the whole page.
func (n *nativePlatform) SetWindowFullscreen(on bool) {
	if n.fullscreen != nil {
		n.fullscreen.set(on)
	}
}
