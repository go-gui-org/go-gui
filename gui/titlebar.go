package gui

// TitlebarDark sets the window titlebar to dark or light mode.
// Delegates to the native platform backend. No-op if no native
// platform is set.
//
// Only the GL backend on Wayland implements it, for a frame drawn by
// libdecor's GTK plugin (GNOME). That variant is per process: the
// last call sets it for every window's frame. Elsewhere it is a no-op
// today: the frame keeps the variant the system gives it.
func (w *Window) TitlebarDark(dark bool) {
	if w.nativePlatform != nil {
		w.nativePlatform.TitlebarDark(dark)
	}
}
