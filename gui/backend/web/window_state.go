//go:build js && wasm

package web

// No window to place on the web. Fullscreen is a follow-up to #1171.
func (n *nativePlatform) MinimizeWindow()                  {}
func (n *nativePlatform) MaximizeWindow()                  {}
func (n *nativePlatform) RestoreWindow()                   {}
func (n *nativePlatform) SetWindowFullscreen(_ bool)       {}
func (n *nativePlatform) ResizeWindow(_, _ int)            {}
func (n *nativePlatform) MoveWindow(_, _ int)              {}
func (n *nativePlatform) WindowPosition() (int, int, bool) { return 0, 0, false }
