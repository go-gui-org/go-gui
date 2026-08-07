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
// The classification is internal: it is encoded at the dispatch sites
// and is neither exposed nor configurable.
type evClass bool

const (
	// evNotify leaves IsHandled alone; the callback opts in with
	// ctx.Consume(). Used for OnKeyDown, OnKeyUp, OnHover, OnMouseMove,
	// OnMouseLeave, OnMouseScroll, OnScroll, AmendLayout, OnIMECommit.
	evNotify evClass = false

	// evConsume pre-marks IsHandled before the callback runs; the
	// callback opts out with ctx.Bubble(). Used for OnClick, OnChar,
	// OnMouseUp, OnGesture, OnFileDrop.
	evConsume evClass = true
)
