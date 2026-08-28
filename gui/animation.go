package gui

import "time"

// posSnapshot captures element position/size for transitions.
type posSnapshot struct {
	x, y, width, height float32
}

// transitionBase holds shared fields for LayoutTransition and
// HeroTransition.
type transitionBase struct {
	start    time.Time
	easing   EasingFn
	OnDone   func(*Window)
	duration time.Duration
	progress float32
	stopped  bool
}

func (tb *transitionBase) IsStopped() bool        { return tb.stopped }
func (tb *transitionBase) SetStart(now time.Time) { tb.start = now }

// updateTransition advances a duration-based transition, returning
// false when already stopped.
func updateTransition(tb *transitionBase, ac *AnimationCommands) bool {
	if tb.stopped {
		return false
	}
	progress, done := durationProgress(tb.start, tb.duration)
	if done {
		tb.progress = 1.0
		tb.stopped = true
		ac.appendOnDone(tb.OnDone)
		return true
	}
	easing := tb.easing
	if easing == nil {
		easing = EaseOutCubic
	}
	tb.progress = easing(progress)
	return true
}

// durationProgress returns progress [0,1] and whether the animation
// is complete.
func durationProgress(start time.Time, duration time.Duration) (float32, bool) {
	elapsed := time.Since(start)
	if duration <= 0 || elapsed >= duration {
		return 1.0, true
	}
	return float32(elapsed) / float32(duration), false
}

// AnimationRefreshKind indicates what type of refresh an animation
// requires each tick.
type AnimationRefreshKind uint8

// AnimationRefreshKind constants.
const (
	animationRefreshNone AnimationRefreshKind = iota
	// AnimationRefreshRenderOnly rebuilds the render commands from the
	// layout already in hand: no view function runs, and nothing is
	// re-arranged. It is the right kind for an animation whose every
	// frame changes only what a widget paints — a canvas, a spinner —
	// rather than what the widget tree contains.
	//
	// A DrawCanvas driven this way must set
	// DrawCanvasCfg.AlwaysRedraw, because its Version never reaches
	// the cache without a view pass. Anything that does change the
	// tree still needs a layout refresh from its own event handler.
	//
	// exportaudit:keep — the refresh kind an app sets on Animate
	AnimationRefreshRenderOnly
	AnimationRefreshLayout // full layout rebuild
)

// maxAnimationRefreshKind returns the higher-priority refresh kind.
func maxAnimationRefreshKind(current, incoming AnimationRefreshKind) AnimationRefreshKind {
	if incoming > current {
		return incoming
	}
	return current
}

// Animation is the interface for all animation types. Update is
// called each tick with the elapsed seconds since the previous tick
// and an AnimationCommands batch into which the animation may enqueue
// deferred callbacks (OnDone / OnValue) — those run after Update
// returns so callback bodies cannot reenter the animation loop mutex.
// Return false once the animation has stopped so the loop retires it.
// exportaudit:keep — reachable from an exported signature
type Animation interface {
	ID() string
	RefreshKind() AnimationRefreshKind
	IsStopped() bool
	SetStart(t time.Time)
	Update(w *Window, dt float32, ac *AnimationCommands) bool
}

// BlinkCursorAnimation toggles cursor visibility on a timer.
// exportaudit:keep — reachable from an exported signature
type BlinkCursorAnimation struct {
	start   time.Time
	stopped bool
}

const blinkCursorAnimationID = "___blinky_cursor_animation___"
const blinkCursorAnimationDelay = 600 * time.Millisecond

// NewBlinkCursorAnimation creates a cursor blink animation.
func newBlinkCursorAnimation() *BlinkCursorAnimation {
	return &BlinkCursorAnimation{}
}

// ID implements Animation.
func (a *BlinkCursorAnimation) ID() string { return blinkCursorAnimationID }

// RefreshKind implements Animation. None: each toggle patches the
// caret renderer in place via commandToggleCaretBlink, so a blink
// tick needs no refresh at all (issue #404).
func (a *BlinkCursorAnimation) RefreshKind() AnimationRefreshKind { return animationRefreshNone }

// IsStopped implements Animation.
func (a *BlinkCursorAnimation) IsStopped() bool { return a.stopped }

// SetStart implements Animation.
func (a *BlinkCursorAnimation) SetStart(t time.Time) { a.start = t }

// Update implements Animation.
func (a *BlinkCursorAnimation) Update(w *Window, _ float32, ac *AnimationCommands) bool {
	return updateBlinkCursor(a, w, ac)
}

// Animate waits the specified delay then executes the callback.
type Animate struct {
	start    time.Time
	Callback func(*Animate, *Window)
	AnimID   string
	Delay    time.Duration
	Repeat   bool
	// Refresh controls what is refreshed each tick. Zero
	// defaults to AnimationRefreshLayout (full layout rebuild).
	Refresh AnimationRefreshKind
	stopped bool
}

// ID implements Animation.
func (a *Animate) ID() string { return a.AnimID }

// RefreshKind implements Animation.
func (a *Animate) RefreshKind() AnimationRefreshKind {
	if a.Refresh != 0 {
		return a.Refresh
	}
	return AnimationRefreshLayout
}

// IsStopped implements Animation.
func (a *Animate) IsStopped() bool { return a.stopped }

// SetStart implements Animation.
func (a *Animate) SetStart(t time.Time) { a.start = t }

// Update implements Animation.
func (a *Animate) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	return updateAnimate(a, ac)
}
