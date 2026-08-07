package gui

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
)

// Dev-mode diagnostics.
//
// A family of defects in this library are silent by construction: a
// focusable widget without an ID renders and clicks but never joins
// the tab order, a scrollable widget without an ID shares the key ""
// with every other ID-less scrollable in the window, an OnMouseLeave
// on an ID-less shape never fires, and a duplicate ID collapses two
// widgets onto one identity. None of these produce an error, a panic,
// or a visual difference.
//
// The debug gate turns them into messages on stderr. It is off by
// default and costs one atomic load per frame when off.

// debugEnabled gates every check in this file. It is an atomic.Bool
// rather than a plain bool because [Debug] makes it mutable at
// runtime, and the checks read it from the frame goroutine while an
// application may flip it from another. atomic.Bool.Load compiles to
// a plain load on amd64 and arm64, so the mutability is the reason,
// not the lookup cost.
var debugEnabled atomic.Bool

// debugGen increments on every false -> true transition of the gate.
// Per-window warn-once state carries the generation it was built
// under and is discarded when the generation moves, so re-enabling
// the gate after fixing something reports the current state rather
// than staying silent.
var debugGen atomic.Uint64

// debugOut is where findings are written. A variable so tests can
// capture it; never reassigned at runtime.
var debugOut io.Writer = os.Stderr

func init() {
	// GOGUI_DEBUG is the general gate. GOGUI_FOCUS_DEBUG is the
	// original focus-only spelling, still honoured so existing
	// workflows keep working.
	if envTruthy("GOGUI_DEBUG") || envTruthy("GOGUI_FOCUS_DEBUG") {
		debugEnabled.Store(true)
		debugGen.Store(1)
	}
}

// envTruthy reports whether an environment variable is set to
// something a developer would read as "on".
func envTruthy(name string) bool {
	v, ok := os.LookupEnv(name)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	return err == nil && b
}

// Debug turns dev-mode diagnostics on or off. When on, each frame is
// checked for widgets whose identity is broken in a way that produces
// no error otherwise:
//
//   - two shapes sharing one ID
//   - a focusable shape with no ID (never keyboard-reachable)
//   - a scrollable shape with no ID (scroll offset shared with every
//     other ID-less scrollable in the window)
//   - a shape with an OnMouseLeave and no ID (the callback never fires)
//
// Findings go to stderr, once per finding per window. Turning the
// gate off and on again clears that memory, so a re-enabled gate
// reports the state of the frame in front of it.
//
// The gate is also set at startup by GOGUI_DEBUG=1.
//
// Not for production: the checks walk the whole layout tree every
// frame and allocate while doing it.
func Debug(on bool) {
	if on && debugEnabled.CompareAndSwap(false, true) {
		debugGen.Add(1)
		return
	}
	debugEnabled.Store(on)
}

// DebugEnabled reports whether dev-mode diagnostics are on.
func DebugEnabled() bool { return debugEnabled.Load() }

// debugCheck identifies one class of finding. Warn-once state is
// keyed by (check, subject), so a window reports each distinct defect
// once rather than at the frame rate.
type debugCheck uint8

const (
	debugCheckDupID debugCheck = iota
	debugCheckFocusNoID
	debugCheckScrollNoID
	debugCheckMouseLeaveNoID
)

// debugWarnKey is the warn-once key. For the ID-less checks the
// subject is the shape's path in the layout tree ("0/3/1"), because
// an empty ID is precisely what is being reported and cannot
// distinguish two findings.
type debugWarnKey struct {
	subject string
	check   debugCheck
}

// debugState is a window's warn-once memory. Zero value is ready.
type debugState struct {
	warned map[debugWarnKey]struct{}
	gen    uint64
}

// debugAudit runs the dev-mode checks over one frame's composed
// layout tree. No-op unless [Debug] is on.
//
// Called from updateLayoutLocked after composeLayout, so it sees the
// same tree the renderer does, including floating and overlay layers.
func (w *Window) debugAudit(root *Layout) {
	if !debugEnabled.Load() {
		return
	}
	// Discard warn-once memory built under an older generation of the
	// gate, so Debug(false) then Debug(true) re-reports.
	if gen := debugGen.Load(); w.debug.gen != gen {
		w.debug.gen = gen
		w.debug.warned = nil
	}
	// ids maps an ID to the path of the shape that claimed it first,
	// so a duplicate can name both sites. Frame-scoped, unlike
	// w.debug.warned.
	ids := make(map[string]string)
	var path []int
	w.debugWalk(root, &path, ids)
}

// debugWalk is the depth-first audit. path is the index chain from
// the root to layout, maintained in place to keep the walk to one
// allocation.
func (w *Window) debugWalk(layout *Layout, path *[]int, ids map[string]string) {
	if s := layout.Shape; s != nil {
		w.debugCheckShape(s, *path, ids)
	}
	for i := range layout.Children {
		*path = append(*path, i)
		w.debugWalk(&layout.Children[i], path, ids)
		*path = (*path)[:len(*path)-1]
	}
}

// debugCheckShape runs the per-shape checks. path is the shape's
// index chain from the root; ids records which path first claimed
// each ID this frame.
func (w *Window) debugCheckShape(s *Shape, path []int, ids map[string]string) {
	if s.ID != "" {
		if first, dup := ids[s.ID]; dup {
			w.debugWarn(debugCheckDupID, s.ID,
				"duplicate ID %q at %s, first claimed at %s; ID is the "+
					"identity key for focus, scroll, and per-widget state, so "+
					"the two collapse to one tab stop and one state slot",
				s.ID, debugPath(path), first)
		} else {
			ids[s.ID] = debugPath(path)
		}
		return
	}
	// An ID-less shape is ordinary unless it claims a feature that is
	// keyed by ID. Render the path only when there is something to
	// report.
	focusBad := s.Focusable && !s.FocusSkip && !s.Disabled
	// OnMouseLeave is tracked through a map keyed by ID
	// (layoutMouseLeave, layout_pipeline.go), and that guard has no
	// Focusable precondition — so this fires on shapes the focus check
	// deliberately passes over, including FocusDisabled controls.
	leaveBad := !s.Disabled && s.events != nil && s.events.OnMouseLeave != nil
	if !focusBad && !s.Scrollable && !leaveBad {
		return
	}
	p := debugPath(path)
	if focusBad {
		w.debugWarn(debugCheckFocusNoID, p,
			"focusable shape at %s has no ID; focus traversal is keyed by "+
				"ID, so it renders and clicks but never joins the tab order", p)
	}
	if s.Scrollable {
		w.debugWarn(debugCheckScrollNoID, p,
			"scrollable shape at %s has no ID; scroll offsets are keyed by "+
				"ID, so it shares one offset with every other ID-less "+
				"scrollable in this window", p)
	}
	if leaveBad {
		w.debugWarn(debugCheckMouseLeaveNoID, p,
			"shape at %s has an OnMouseLeave but no ID; leave tracking is "+
				"keyed by ID, so the callback never fires", p)
	}
}

// debugWarn prints a finding to stderr the first time this window
// sees it. subject is the warn-once discriminator within check.
func (w *Window) debugWarn(check debugCheck, subject, format string, args ...any) {
	key := debugWarnKey{check: check, subject: subject}
	if _, seen := w.debug.warned[key]; seen {
		return
	}
	if w.debug.warned == nil {
		w.debug.warned = make(map[debugWarnKey]struct{})
	}
	w.debug.warned[key] = struct{}{}
	// Diagnostics are best-effort; a failed write to stderr is not
	// something a GUI frame can act on.
	_, _ = fmt.Fprintf(debugOut, "gui: "+format+"\n", args...)
}

// debugPath renders a tree path as "0/3/1". The root is "root".
func debugPath(path []int) string {
	if len(path) == 0 {
		return "root"
	}
	var b strings.Builder
	for i, n := range path {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(strconv.Itoa(n))
	}
	return b.String()
}
