package gui

// TitlebarDark sets the window titlebar to dark or light mode.
// Delegates to the native platform backend. Every backend
// implements it as a no-op today. No-op if no native platform is set.
func (w *Window) TitlebarDark(dark bool) {
	if w.nativePlatform != nil {
		w.nativePlatform.TitlebarDark(dark)
	}
}
