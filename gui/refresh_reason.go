package gui

import (
	"fmt"
	"strings"
)

// refreshReason records why a frame rebuilds (#970). Each cause is one bit,
// so a request ORs its bit into Window.refresh and adding a cause never
// allocates. The low 16 bits ask for a full layout rebuild; the high 16 bits
// ask for a render-only pass from the arranged tree. A layout bit wins: the
// full pass clears every bit, the render bits it covered included.
//
// A bit names the kind of cause, not the call site. InvalidateLayout has
// many callers in gui/ and more in the sibling repos; all of them report
// refreshInvalidate.
type refreshReason uint32

// Layout reasons: the next frame runs the view phase and arrange again.
const (
	// refreshInput: EventFn dispatched an input event.
	refreshInput refreshReason = 1 << iota
	// refreshInvalidate: a public InvalidateLayout call, from the app or from
	// a widget.
	refreshInvalidate
	// refreshView: SetView installed a new view generator.
	refreshView
	// refreshAnimation: an animation tick asked for a layout refresh.
	refreshAnimation
	// refreshShown: an occluded window became visible again.
	refreshShown
	// refreshDialog: a dialog opened or closed.
	refreshDialog
	// refreshA11y: a screen-reader action ran a handler.
	refreshA11y
	// refreshTheme: the installed theme changed.
	refreshTheme
	// refreshOverflow: arrange found a different overflow split and asked
	// for one more pass.
	refreshOverflow
	// refreshInitial: the first frame of a new window.
	refreshInitial
	// refreshDeferred: a callback deferred by the frame pass ran, so the
	// frame it changed is stale (window_deferred.go).
	refreshDeferred
	// refreshCommand: a queued command ran (test settle loop).
	refreshCommand
	// refreshTest: a test hook forced the rebuild.
	refreshTest
)

// Render-only reasons: the next frame rebuilds render commands only.
const (
	// refreshRender: a public InvalidateRender call.
	refreshRender refreshReason = 1 << (16 + iota)
	// refreshRenderAnimation: an animation tick asked for a render-only
	// refresh.
	refreshRenderAnimation
	// refreshRenderSvg: an animated SVG advanced a frame.
	refreshRenderSvg
)

const (
	refreshLayoutMask refreshReason = 1<<16 - 1
	refreshRenderMask               = ^refreshLayoutMask
)

// refreshReasonNames maps each bit to its log name, in bit order.
var refreshReasonNames = [...]struct {
	bit  refreshReason
	name string
}{
	{refreshInput, "input"},
	{refreshInvalidate, "invalidate"},
	{refreshView, "set-view"},
	{refreshAnimation, "animation"},
	{refreshShown, "shown"},
	{refreshDialog, "dialog"},
	{refreshA11y, "a11y"},
	{refreshTheme, "theme"},
	{refreshOverflow, "overflow"},
	{refreshInitial, "initial"},
	{refreshDeferred, "deferred"},
	{refreshCommand, "command"},
	{refreshTest, "test"},
	{refreshRender, "render"},
	{refreshRenderAnimation, "render-animation"},
	{refreshRenderSvg, "svg"},
}

// String joins the set bits with "|". Allocates; called only when the
// DebugRebuilds category is on.
func (r refreshReason) String() string {
	if r == 0 {
		return "none"
	}
	var b strings.Builder
	for _, n := range refreshReasonNames {
		if r&n.bit == 0 {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte('|')
		}
		b.WriteString(n.name)
	}
	return b.String()
}

// layoutPending reports that a full layout rebuild is requested.
func (w *Window) layoutPending() bool {
	return refreshReason(w.refresh.Load())&refreshLayoutMask != 0
}

// renderPending reports that a render-only pass is requested and no full
// rebuild is, so the render-only pass is what the next frame runs.
func (w *Window) renderPending() bool {
	r := refreshReason(w.refresh.Load())
	return r&refreshRenderMask != 0 && r&refreshLayoutMask == 0
}

// refreshPending reports that any rebuild is requested.
func (w *Window) refreshPending() bool { return w.refresh.Load() != 0 }

// takeLayoutRefresh clears every bit and returns what was set. Called at the
// start of a full pass, which covers render-only requests too.
func (w *Window) takeLayoutRefresh() refreshReason {
	return refreshReason(w.refresh.Swap(0))
}

// takeRenderRefresh clears the render-only bits and returns them. A layout
// bit set meanwhile by another goroutine stays set for the next frame.
func (w *Window) takeRenderRefresh() refreshReason {
	return refreshReason(w.refresh.And(uint32(^refreshRenderMask))) & refreshRenderMask
}

// noteRefresh records the reasons of the pass that is starting. With
// DebugRebuilds on, it prints one line when the reasons or the pass kind
// differ from the previous pass, so a steady stream (a hover, a running
// animation) prints once rather than every frame:
//
//	gui: rebuild layout: input
//	gui: rebuild render: svg
//
// The view phase allocates on every full rebuild, so an app that rebuilds
// with nothing new to show pays for it on every frame; the log names the
// cause. DebugRebuilds is out of DebugAll because it reports normal
// operation, not a defect. It writes to stderr only: TestFindings renders
// its own frame, which would always log "test", so tests read lastRefresh.
//
// Main-thread only. Does not allocate while the category is off.
func (w *Window) noteRefresh(r refreshReason, full bool) {
	changed := r != w.lastRefresh || full != w.lastRefreshFull
	w.lastRefresh = r
	w.lastRefreshFull = full
	if DebugCategory(debugMask.Load())&DebugRebuilds == 0 {
		return
	}
	// A gate turned on again reports the pass in front of it, even when
	// the reasons did not change while it was off.
	if gen := debugGen.Load(); w.lastRefreshGen != gen {
		w.lastRefreshGen = gen
		changed = true
	}
	if !changed {
		return
	}
	kind := "render"
	if full {
		kind = "layout"
	}
	// Best-effort, like debugWarn: a failed write to stderr is not
	// something a GUI frame can act on.
	_, _ = fmt.Fprintf(debugOut, "gui: rebuild %s: %s\n", kind, r)
}
