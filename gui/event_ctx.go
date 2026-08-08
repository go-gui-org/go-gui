package gui

// EventCtx bundles the three values every input-event callback needs.
// Passed by value. It is three pointers (24 bytes), so Go's register
// ABI passes it in registers with no heap allocation even through an
// indirect call. Methods use value receivers and mutate through the
// Event pointer, so they match the alloc story and avoid addressability
// footguns on non-addressable EventCtx values.
//
// Value semantics also make nested (reentrant) dispatch safe: each
// callback frame owns its own copy, so an OnClick that synchronously
// triggers OnScroll cannot have its context corrupted by the inner
// frame.
type EventCtx struct {
	Layout *Layout
	Event  *Event // nil for AmendLayout and OnScroll (no originating event)
	Window *Window
}

// Consume marks the event handled so ancestors do not receive it.
// Needed only in notify-class callbacks (OnKeyDown, OnKeyUp, OnHover,
// OnMouseMove, OnMouseLeave, OnMouseScroll); consume-class callbacks
// (OnClick, OnChar, OnMouseUp, OnGesture, OnFileDrop) are already
// marked handled before they run.
//
// Safe to call when Event is nil.
func (c EventCtx) Consume() {
	if c.Event != nil {
		c.Event.IsHandled = true
		// Record that consumption was asked for, not merely inherited
		// from the consume-class pre-mark. One unconditional store,
		// cheaper than gating it on the debug atomic.
		c.Event.explicitConsume = true
	}
}

// Bubble marks the event unhandled so ancestors receive it. This is
// the explicit opt-out for consume-class callbacks.
//
// Bubble opts out of *this* callback's auto-consume only. It does not
// un-handle an event that an earlier handler already consumed, because
// the coordinate save/restore in callRelative re-applies the incoming
// handled flag.
//
// Safe to call when Event is nil.
func (c EventCtx) Bubble() {
	if c.Event != nil {
		c.Event.IsHandled = false
	}
}

// Handled reports whether the event has been consumed. Reports false
// when Event is nil.
func (c EventCtx) Handled() bool {
	return c.Event != nil && c.Event.IsHandled
}

// evClass distinguishes the two event-dispatch classes. Consume-class
// events are pre-marked handled before the callback runs; notify-class
// events are not, and the callback must call ctx.Consume() to stop
// propagation.
//
// Consume-class values additionally name *which* event is being
// dispatched. The class alone is enough to dispatch, but the
// event-collapse debug check (debug_event.go) has to ask "would an
// ancestor also have received this?", and that question is
// per-callback: it means OnClick on a containing shape for evClick and
// the focused target's OnChar for evChar. Carrying the name in the
// existing parameter keeps the two facts from drifting apart.
//
// The classification is internal: it is encoded at the dispatch sites
// and is neither exposed nor configurable.
type evClass uint8

const (
	// evNotify leaves IsHandled alone; the callback opts in with
	// ctx.Consume(). Used for OnKeyDown, OnKeyUp, OnHover, OnMouseMove,
	// OnMouseLeave, OnMouseScroll, OnScroll, AmendLayout, OnIMECommit.
	evNotify evClass = iota

	// evConsume pre-marks IsHandled before the callback runs; the
	// callback opts out with ctx.Bubble(). Consume-class without naming
	// an event: dispatches identically to the named values below but
	// carries no ancestor rule, so the collapse check skips it.
	evConsume

	// The named consume-class events. Each pre-marks IsHandled exactly
	// as evConsume does.
	evClick
	evChar
	evMouseUp
	evFileDrop
	evGesture
)

// consumes reports whether this class pre-marks IsHandled before the
// callback runs.
func (c evClass) consumes() bool { return c != evNotify }

// named reports whether this class identifies a specific event, which
// is the precondition for the collapse check. Ordered so the test is
// one comparison: the named values are the ones above evConsume.
//
// Dispatch guards the debugCollapse call with this rather than letting
// the check reject the class itself, so notify-class dispatch — the
// overwhelming majority of events — does not pay for a function call
// it will always return from immediately.
func (c evClass) named() bool { return c > evConsume }
