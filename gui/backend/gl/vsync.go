package gl

// applyVSyncOff turns vsync off for a window that set WindowCfg.VSyncOff
// (issue #907). Context creation always asks for swap interval 1, so a
// window without the flag needs nothing here.
//
// setInterval sets the swap interval on the context that is current and
// reports whether the driver accepted it. report is the window's
// DebugWindowVSync. Both are passed in so EGL and WGL share this logic
// and a test can drive it without a GPU. api names the call in the
// gui.Debug report when the driver refuses it.
func applyVSyncOff(vsyncOff bool, setInterval func(int32) bool,
	report func(string), api string) {
	if !vsyncOff {
		return
	}
	if !setInterval(0) {
		report(api + "(0) was refused by the driver")
	}
}
