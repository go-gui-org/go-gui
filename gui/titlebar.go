package gui

// TitlebarDark sets the window titlebar to dark or light mode.
// Delegates to the native platform backend. Safe to call before the
// backend attaches: the last variant asked for is stored and applied
// when it does, unless the window follows the system appearance and
// the OS reports a setting, which then wins.
//
// It works on macOS, on Windows and on Wayland. On macOS it pins this
// window's appearance: the frame, the title, the window buttons and a
// vibrancy backdrop follow it, while dialogs and the menu bar keep the
// system's. On Windows (10 version 1809 or later) it sets this window's
// caption. A call before Run is applied before the window shows. A
// call once it shows, from WindowCfg.OnInit for one, animates from the
// old variant. With "Show accent color on title bars" turned on,
// Windows paints the active caption in the accent color whichever
// variant is set. On Wayland it switches a frame drawn by libdecor's
// GTK plugin (GNOME). That variant is per process: the last call sets
// it for every window's frame. Elsewhere it is a no-op today: the
// frame keeps the variant the system gives it.
func (w *Window) TitlebarDark(dark bool) {
	w.titlebarDark, w.titlebarDarkSet = dark, true
	if w.nativePlatform != nil {
		w.nativePlatform.TitlebarDark(dark)
	}
}
