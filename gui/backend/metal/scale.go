//go:build darwin && !ios

package metal

import "github.com/go-gui-org/go-gui/gui/backend/internal/devscale"

// scaleOverride is GOGUI_DEVICE_SCALE, or 0 for the screen's scale (#971).
// The C side sizes the drawable from it.
func scaleOverride() float32 {
	s, _ := devscale.Override()
	return s
}

// drawableScale is the DPI scale for a drawable fbW pixels wide on a view
// logW points wide, or fallback when logW is not usable.
func drawableScale(fbW, logW int32, fallback float32) float32 {
	s, ok := devscale.Override()
	return drawableScaleWith(s, ok, fbW, logW, fallback)
}

// drawableScaleWith returns the override as it is when ok. The C side
// truncates the drawable to whole pixels, so fbW/logW at a fractional
// override would drift with the window width, and every resize would
// reshape the text.
func drawableScaleWith(s float32, ok bool, fbW, logW int32, fallback float32) float32 {
	if ok {
		return s
	}
	if logW <= 0 {
		return fallback
	}
	return float32(fbW) / float32(logW)
}
