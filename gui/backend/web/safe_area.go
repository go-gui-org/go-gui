//go:build js && wasm

package web

import (
	"strconv"
	"strings"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// The safe area (issue #1171) comes from the CSS environment variables
// env(safe-area-inset-*), which JavaScript cannot read directly. A
// hidden probe element takes them as its padding, and getComputedStyle
// reads the padding back in CSS pixels, which are gui's logical pixels.
//
// A browser reports non-zero values only when the page asks to draw
// under the notch, with viewport-fit=cover in its viewport meta.
// Without it the browser keeps the page clear of the notch itself, the
// values stay 0, and so does Window.SafeAreaPadding. The page decides.

// safeAreaSettleMs is how long after a resize or fullscreen change the
// safe area is read a second time, for a browser that updates it late.
const safeAreaSettleMs = 300

// safeAreaProbeCSS places the probe out of layout and out of sight.
// The 0px fallbacks cover browsers without the variables.
const safeAreaProbeCSS = "position:fixed;top:0;left:0;width:0;height:0;" +
	"visibility:hidden;pointer-events:none;" +
	"padding:env(safe-area-inset-top,0px) env(safe-area-inset-right,0px) " +
	"env(safe-area-inset-bottom,0px) env(safe-area-inset-left,0px)"

// safeAreaWatcher reads the probe when the page asks, and once more a
// moment after the last request of a burst. Chrome on Android updates
// env(safe-area-inset-*) after the resize event when the page enters
// fullscreen (measured: T0 at the event, T52 later), and a rotation
// fires several resizes. JS runs on one thread, so no lock.
type safeAreaWatcher struct {
	w     *gui.Window
	probe js.Value
	timer js.Value // pending setTimeout id; undefined when none
	later js.Func
}

// newSafeAreaWatcher adds the probe to the page and reads it once.
// The delayed-read callback joins b.callbacks, which live as long as
// the page, like every other listener.
func newSafeAreaWatcher(b *Backend, w *gui.Window, doc js.Value) *safeAreaWatcher {
	s := &safeAreaWatcher{w: w, probe: newSafeAreaProbe(doc)}
	s.later = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		s.timer = js.Undefined()
		reportSafeArea(s.w, s.probe)
		return nil
	})
	b.callbacks = append(b.callbacks, s.later)
	s.read()
	return s
}

// read reports the safe area now and restarts the settle timer, so the
// second read follows the last event of a burst. gui ignores a repeat.
func (s *safeAreaWatcher) read() {
	reportSafeArea(s.w, s.probe)
	if s.timer.Truthy() {
		js.Global().Call("clearTimeout", s.timer)
	}
	s.timer = js.Global().Call("setTimeout", s.later, safeAreaSettleMs)
}

// newSafeAreaProbe adds the probe element to the page. It is hidden
// from screen readers as well as from view.
func newSafeAreaProbe(doc js.Value) js.Value {
	p := doc.Call("createElement", "div")
	p.Call("setAttribute", "aria-hidden", "true")
	p.Get("style").Set("cssText", safeAreaProbeCSS)
	doc.Get("body").Call("appendChild", p)
	return p
}

// reportSafeArea reads the probe and hands the four edges to gui,
// which ignores a repeat.
func reportSafeArea(w *gui.Window, probe js.Value) {
	cs := js.Global().Call("getComputedStyle", probe)
	gui.DispatchSafeArea(w,
		cssPixels(cs.Get("paddingTop").String()),
		cssPixels(cs.Get("paddingRight").String()),
		cssPixels(cs.Get("paddingBottom").String()),
		cssPixels(cs.Get("paddingLeft").String()))
}

// cssPixels parses a computed length such as "47px". Anything else
// reads as 0; gui clamps what is left.
func cssPixels(s string) float32 {
	v, err := strconv.ParseFloat(strings.TrimSuffix(s, "px"), 32)
	if err != nil {
		return 0
	}
	return float32(v)
}
