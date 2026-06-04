package gui

type queuedCommandKind uint8

const (
	queuedCommandWindowFn queuedCommandKind = iota
	queuedCommandValueFn
	queuedCommandAnimateFn
)

type queuedCommand struct {
	windowFn  func(*Window)
	valueFn   func(float32, *Window)
	animateFn func(*Animate, *Window)

	animate *Animate

	value float32
	kind  queuedCommandKind
}

// AnimationCommands batches deferred callbacks produced by an
// Animation.Update. The animation loop drains these after the update
// pass so callbacks run on the main command queue rather than under
// the animation-loop mutex — preventing reentry deadlocks if a
// callback mutates window state.
//
// Exported so third-party Animation implementations can enqueue
// OnDone / OnValue callbacks without reaching for the private
// queuedCommand type.
type AnimationCommands struct {
	inner *[]queuedCommand
}

// newAnimationCommands wraps a raw deferred slice. Package-internal;
// only the animation loop should construct these.
func newAnimationCommands(deferred *[]queuedCommand) AnimationCommands {
	return AnimationCommands{inner: deferred}
}

// AppendOnDone queues fn to run after the current frame's Update
// pass. Nil fn is a no-op so callers can pass a pointer to an
// optional callback field unconditionally.
func (ac *AnimationCommands) AppendOnDone(fn func(*Window)) {
	if fn == nil || ac == nil || ac.inner == nil {
		return
	}
	*ac.inner = append(*ac.inner, queuedCommand{
		kind: queuedCommandWindowFn, windowFn: fn,
	})
}

// AppendOnValue queues fn with the supplied value. Typical use: a
// per-frame interpolation callback from a tween / spring animation.
func (ac *AnimationCommands) AppendOnValue(fn func(float32, *Window), v float32) {
	if fn == nil || ac == nil || ac.inner == nil {
		return
	}
	*ac.inner = append(*ac.inner, queuedCommand{
		kind: queuedCommandValueFn, valueFn: fn, value: v,
	})
}

// appendAnimate is Animate's self-dispatch helper — enqueues the
// animation's own Callback with its receiver. Package-internal:
// external animations implement their tick logic directly inside
// Update and do not need this kind.
func (ac *AnimationCommands) appendAnimate(cb func(*Animate, *Window), a *Animate) {
	if cb == nil || ac == nil || ac.inner == nil {
		return
	}
	*ac.inner = append(*ac.inner, queuedCommand{
		kind: queuedCommandAnimateFn, animateFn: cb, animate: a,
	})
}

func commandMarkLayoutRefresh(w *Window) {
	w.markLayoutRefresh()
}

func commandMarkRenderOnlyRefresh(w *Window) {
	w.markRenderOnlyRefresh()
}
