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
//
// One rule, every event: a callback that acts on an event says so. No
// event is marked handled on a callback's behalf, so a callback that
// does not call Consume lets the event travel on. This holds for every
// callback — OnClick and OnChar no differently from OnKeyDown and
// OnHover.
//
// Safe to call when Event is nil.
func (c EventCtx) Consume() {
	if c.Event != nil {
		c.Event.IsHandled = true
	}
}

// Handled reports whether the event has been consumed. Reports false
// when Event is nil.
func (c EventCtx) handled() bool {
	return c.Event != nil && c.Event.IsHandled
}

// EffID resolves a leaf ID a handler closed over to the effective ID
// the framework's stores are keyed by (see gui/id_resolve.go).
//
// A widget factory that builds its tree eagerly — Input, for one — has
// no Window when it captures cfg.ID, so it cannot call
// [Window.EffID] at generation time. Its handlers resolve here instead:
// the leaf names this shape or one of its ancestors, and by the time an
// event is dispatched every shape carries its resolved identity.
//
// An absolute leaf (one containing IDSep) and a leaf naming nothing in
// the ancestor chain both resolve exactly as the resolve pass would
// resolve them in this position.
// exportaudit:keep — public seam for widget factories (see CLAUDE.md)
func (c EventCtx) EffID(leaf string) string {
	if leaf == "" || c.Layout == nil {
		return leaf
	}
	// The leaf usually names this shape or the composite's outer
	// container, so the walk stops within a step or two.
	scope := ""
	for p := c.Layout; p != nil; p = p.Parent {
		s := p.Shape
		if s == nil || s.ID == "" {
			continue
		}
		if s.ID == leaf {
			return s.idKey()
		}
		if scope == "" {
			// Nearest ID-bearing ancestor: the scope a leaf in this
			// position would join to, kept for the fallback below.
			scope = s.idKey()
		}
	}
	return c.Window.joinLeaf(scope, leaf)
}

// evClass names which event is being dispatched.
//
// It no longer carries a dispatch rule. Until v0.55.0 there were two
// classes — consume-class events were marked handled before the
// callback ran, notify-class events were not — and the difference was
// invisible in a callback's signature, so which one you had written was
// something you had to know rather than something you could see. That
// split is gone: nothing is pre-marked and every callback consumes
// explicitly.
//
// What remains is the name, which the unconsumed-event debug check
// (debug_event.go) needs: "would an ancestor also have received this?"
// is a per-event question, meaning OnClick on a containing shape for
// evClick and the focused target's OnChar for evChar.
//
// The classification is internal: it is encoded at the dispatch sites
// and is neither exposed nor configurable.
type evClass uint8

const (
	// evNotify is the unnamed default: dispatched like every other
	// event, but carrying no ancestor rule, so the debug check skips
	// it.
	evNotify evClass = iota

	// The named events, all of which had been consume-class.
	evClick
	evChar
	evMouseUp
	evFileDrop
	evGesture
	evMouseDown
)

// named reports whether this class identifies a specific event, which
// is the precondition for the debug check.
//
// Dispatch guards the debugUnconsumed call with this rather than
// letting the check reject the class itself, so unnamed dispatch — the
// overwhelming majority of events — does not pay for a function call
// it will always return from immediately.
func (c evClass) named() bool { return c != evNotify }
