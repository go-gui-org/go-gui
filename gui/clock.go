package gui

import "time"

// clockNow is the window's base clock: the time every animation, timer,
// tooltip delay, caret blink and SVG animation reads (#1262).
//
// Live, it is time.Now(). After the first TestStep it is a manual
// clock: the instant of that first step plus the sum of every dt
// passed since. A soak run can then play ten simulated minutes in
// far less wall time, and the same seed replays the same frames.
//
// clockNow does not follow the time-travel scrub pin; w.Now() adds the
// pin on top of it. Profiling timers keep time.Now(), because they
// measure real cost, not simulated time.
//
// Nil-safe: a nil window reads time.Now(). Safe from any goroutine.
// The cost is one atomic load on the live path, and nothing allocates.
func (w *Window) clockNow() time.Time {
	if w == nil || !w.manualClock.Load() {
		return time.Now()
	}
	return w.manualBase.Add(time.Duration(w.manualOffset.Load()))
}
