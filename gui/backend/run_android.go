//go:build android

package backend

import "github.com/go-gui-org/go-gui/gui"

func Run(w *gui.Window) {
	panic("android: use android.SetWindow/Start/Render pattern")
}

// RunApp is not supported on Android.
func RunApp(app *gui.App, windows ...*gui.Window) {
	panic("android: use android.SetWindow/Start/Render pattern")
}
