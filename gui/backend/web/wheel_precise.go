package web

import "math"

// Wheel delta normalization constants. Converts browser delta values to
// gui.Event's scroll unit, lines of text (see Event.ScrollY):
//   - DOM_DELTA_PIXEL: ~17.7px per line, so a 53px trackpad notch stays
//     the three lines every other backend reports
//   - DOM_DELTA_LINE: already lines; passed through
//   - DOM_DELTA_PAGE: a page is ~30 lines
//
// The ratios are unchanged from when this produced "notches" — only the
// unit is scaled, so web scrolling feels exactly as it did.
const (
	wheelPixelDivisor   = 17.7
	wheelLineDivisor    = 1
	wheelPageMultiplier = 30
)

// isWebWheelPrecise classifies a DOM WheelEvent as coming from a precise
// pixel-scrolling device (trackpad, Magic Mouse, precision touchpad) rather
// than a discrete notched mouse wheel. Trackpads already carry OS smoothing
// and momentum, so routing them through gui's wheel smoother would double-smooth
// and make scrolling feel syrupy.
func isWebWheelPrecise(deltaMode int, dx, dy, sinceLastMs float64) bool {
	// Only DOM_DELTA_PIXEL (0) can represent continuous trackpad input.
	// DOM_DELTA_LINE (1) and DOM_DELTA_PAGE (2) are discrete.
	if deltaMode != 0 {
		return false
	}
	if !math.IsNaN(dx) && !math.IsInf(dx, 0) && dx != math.Trunc(dx) {
		return true
	}
	if !math.IsNaN(dy) && !math.IsInf(dy, 0) && dy != math.Trunc(dy) {
		return true
	}
	ax, ay := math.Abs(dx), math.Abs(dy)
	// Horizontal pixel deltas (two-finger swipe) are trackpad input when not
	// matching a standard 100/120px wheel step.
	if ax > 0 && ax != 100 && ax != 120 {
		return true
	}
	// Small non-zero pixel deltas (< 48px) that aren't Firefox's 53px notch
	// or Windows/Chrome's 100/120px wheel step indicate trackpad input.
	if ay > 0 && ay < 48 {
		return true
	}
	// High-frequency continuous pixel events (< 35ms apart) with moderate delta
	// indicate active trackpad or momentum frames.
	if sinceLastMs > 0 && sinceLastMs < 35 && ay < 96 && ay != 53 {
		return true
	}
	return false
}

// normalizeWebWheel converts DOM WheelEvent deltas into gui scroll units and
// classifies whether the event came from a precise pixel-scrolling device.
func normalizeWebWheel(mode int, rawX, rawY, ts float64, lastTS *float64) (dx, dy float64, precise bool) {
	sinceLastMs := -1.0
	if *lastTS >= 0 && ts >= *lastTS {
		sinceLastMs = ts - *lastTS
	}
	*lastTS = ts
	precise = isWebWheelPrecise(mode, rawX, rawY, sinceLastMs)
	dx, dy = rawX, rawY
	switch mode {
	case 0: // DOM_DELTA_PIXEL
		dx /= wheelPixelDivisor
		dy /= wheelPixelDivisor
	case 1: // DOM_DELTA_LINE
		dx /= wheelLineDivisor
		dy /= wheelLineDivisor
	case 2: // DOM_DELTA_PAGE
		dx *= wheelPageMultiplier
		dy *= wheelPageMultiplier
	}
	return dx, dy, precise
}
