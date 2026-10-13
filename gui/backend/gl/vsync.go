package gl

// applyVSync sets the swap interval for a new window: 0 when it set
// WindowCfg.VSyncOff (issue #907), 1 otherwise. It reports a refusal of
// either through report. A refused 1 matters since #1018: the animation
// loop ticks once for each presented frame, so a present that does not
// wait for the display ticks as fast as frames draw (capped at 500 each
// second) and spends CPU on frames the display drops.
//
// setInterval sets the swap interval on the context that is current and
// reports whether the driver accepted it. report is the window's
// DebugWindowVSync. Both are passed in so EGL and WGL share this logic
// and a test can drive it without a GPU. api names the call in the
// gui.Debug report.
func applyVSync(vsyncOff bool, setInterval func(int32) bool,
	report func(string), api string) {
	if vsyncOff {
		if !setInterval(0) {
			report("VSyncOff requested but " + api + "(0) was refused by the driver")
		}
		return
	}
	if !setInterval(1) {
		report(api + "(1) was refused by the driver; " +
			"frames are not paced to the display")
	}
}
