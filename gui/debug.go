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
//
// The duplicate check is strict: no shape may claim an ID another
// shape already claimed, ancestor or not. A composite widget whose
// inner shape needs the owning widget's focus or state references it
// through Shape.focusOwner rather than repeating its ID, which keeps
// "one ID, one widget" true and keeps this check meaningful.

// debugMask gates every check in this file, one bit per
// [DebugCategory]. It is an atomic.Uint32 rather than a plain value
// because [Debug] and [DebugCategories] make it mutable at runtime,
// and the checks read it from the frame goroutine while an
// application may flip it from another. Load compiles to a plain
// load on amd64 and arm64, so the mutability is the reason, not the
// lookup cost.
var debugMask atomic.Uint32

// debugGen increments on every false -> true transition of the gate.
// Per-window warn-once state carries the generation it was built
// under and is discarded when the generation moves, so re-enabling
// the gate after fixing something reports the current state rather
// than staying silent.
var debugGen atomic.Uint64

// debugOut is where findings are written. A variable so tests can
// capture it; never reassigned at runtime.
var debugOut io.Writer = os.Stderr

// DebugCategory names one class of dev-mode finding. Categories gate
// the checks independently; see [DebugCategories].
type DebugCategory uint32

const (
	// DebugDuplicates reports two shapes sharing one ID.
	DebugDuplicates DebugCategory = 1 << iota
	// DebugMissingIDs reports a focusable, scrollable, or
	// OnMouseLeave-bearing shape with no ID, whose behaviour silently
	// does not work.
	DebugMissingIDs
	// DebugUnconsumed reports a callback that acted on an event without
	// consuming it while an ancestor also received it.
	DebugUnconsumed
	// DebugListBoxNoHeight reports a scrollable listbox that resolved to
	// height 0, which disables virtualization so every row builds each
	// frame.
	DebugListBoxNoHeight

	// DebugUnscopedIDs reports a focusable or scrollable shape whose ID
	// resolves to itself — no ID-bearing ancestor above it — so its
	// identity competes in the window-global namespace and cannot be
	// reused elsewhere in the window.
	//
	// Advisory, not a defect: a top-level widget legitimately has no
	// scope, which is why this is the one category [Debug] does not turn
	// on. Ask for it explicitly with
	// [DebugCategories](DebugUnscopedIDs) when auditing a screen for
	// reusability.
	DebugUnscopedIDs

	// DebugAll is every category [Debug] turns on. DebugUnscopedIDs is
	// deliberately absent: it reports a design property, not a bug, and
	// would fire on most widgets in a small app.
	DebugAll = DebugDuplicates | DebugMissingIDs | DebugUnconsumed |
		DebugListBoxNoHeight
)

func init() {
	// GOGUI_DEBUG is the general gate. GOGUI_FOCUS_DEBUG is the
	// original focus-only spelling, still honoured so existing
	// workflows keep working. Either enables every category.
	if envTruthy("GOGUI_DEBUG") || envTruthy("GOGUI_FOCUS_DEBUG") {
		debugMask.Store(uint32(DebugAll))
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

// Debug turns dev-mode diagnostics on or off. When on, every category
// of finding is checked:
//
//   - two shapes sharing one ID
//   - a focusable shape with no ID (never keyboard-reachable)
//   - a scrollable shape with no ID (scroll offset shared with every
//     other ID-less scrollable in the window)
//   - a shape with an OnMouseLeave and no ID (the callback never fires)
//   - a scrollable listbox that resolved to height 0 (virtualization
//     off, every row builds each frame)
//
// It also reports, from dispatch rather than from the frame audit, any
// consume-class callback that relies on automatic handling while an
// ancestor would also have received the event — the sites that would
// silently start firing twice under the one-rule event model. See
// debug_event.go.
//
// [DebugCategories] enables these classes independently; Debug is the
// same API with both extremes (all on, all off).
//
// Findings go to stderr, once per finding per window. Turning a
// category off and on again clears that memory, so a re-enabled gate
// reports the state of the frame in front of it.
//
// The gate is also set at startup by GOGUI_DEBUG=1.
//
// Not for production: the checks walk the whole layout tree every
// frame and allocate while doing it.
func Debug(on bool) {
	if on {
		DebugCategories(DebugAll)
	} else {
		DebugCategories(0)
	}
}

// DebugCategories sets which classes of dev-mode finding are reported.
// Each category gates its checks independently, so an app that
// deliberately lets events bubble can keep the identity audits without
// the unconsumed-event noise.
//
// A zero mask is everything off; [DebugAll] is everything on. Turning a
// category on after it was off clears that category's warn-once memory,
// so a re-enabled category reports the frame in front of it.
func DebugCategories(mask DebugCategory) {
	for {
		cur := debugMask.Load()
		next := uint32(mask)
		if next == cur {
			return
		}
		if debugMask.CompareAndSwap(cur, next) {
			// An off -> on transition moves the generation, discarding
			// warn-once memory built under a narrower gate.
			if cur == 0 && next != 0 {
				debugGen.Add(1)
			}
			return
		}
	}
}

// DebugEnabled reports whether any dev-mode category is on.
func DebugEnabled() bool { return debugMask.Load() != 0 }

// debugCheck identifies one class of finding. Warn-once state is
// keyed by (check, subject), so a window reports each distinct defect
// once rather than at the frame rate.
type debugCheck uint8

const (
	debugCheckDupID debugCheck = iota
	debugCheckFocusNoID
	debugCheckScrollNoID
	debugCheckMouseLeaveNoID
	// debugCheckUnconsumed is the only check that runs from dispatch
	// rather than from the per-frame layout audit; see debug_event.go.
	debugCheckUnconsumed
	// debugCheckListBoxNoHeight fires from the listbox view phase
	// rather than from the layout audit; see listBoxVisibleRange.
	debugCheckListBoxNoHeight
	// debugCheckUnscopedID reports an identity that has no ID-bearing
	// ancestor, so it is still a window-global name.
	debugCheckUnscopedID
)

// checkCategory maps an internal check to the public category that
// gates it. debugWarn consults this so every finding site stays behind
// the mask even when reached outside the gated entry points.
func checkCategory(check debugCheck) DebugCategory {
	switch check {
	case debugCheckDupID:
		return DebugDuplicates
	case debugCheckFocusNoID, debugCheckScrollNoID, debugCheckMouseLeaveNoID:
		return DebugMissingIDs
	case debugCheckUnconsumed:
		return DebugUnconsumed
	case debugCheckListBoxNoHeight:
		return DebugListBoxNoHeight
	case debugCheckUnscopedID:
		return DebugUnscopedIDs
	}
	return 0
}

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
	// collect, when non-nil, receives findings instead of debugOut. Set
	// only by TestUnconsumedEvents, which needs the findings as data
	// rather than as text on stderr.
	collect *[]string
	gen     uint64
}

// debugAudit runs the dev-mode checks over one frame's composed
// layout tree. No-op unless an audit category is on.
//
// The tree walk is skipped unless a category that audits the frame —
// duplicates or missing IDs — is on; the unconsumed check runs from
// dispatch and the listbox check from the view phase, so they gate
// themselves.
//
// Called from updateLayoutLocked after composeLayout, so it sees the
// same tree the renderer does, including floating and overlay layers.
func (w *Window) debugAudit(root *Layout) {
	const walkCategories = DebugDuplicates | DebugMissingIDs |
		DebugUnscopedIDs
	if DebugCategory(debugMask.Load())&walkCategories == 0 {
		return
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
		// Uniqueness is on the *effective* ID: the same leaf under two
		// different ID-bearing ancestors is two identities and is not a
		// duplicate. The message names the effective path, since that is
		// the string the stores and the public APIs use.
		key := s.idKey()
		if first, dup := ids[key]; dup {
			w.debugWarn(debugCheckDupID, key,
				"duplicate ID %q at %s, first claimed at %s; ID is the "+
					"identity key for focus, scroll, and per-widget state, so "+
					"the two collapse to one tab stop and one state slot",
				key, debugPath(path), first)
		} else {
			ids[key] = debugPath(path)
		}
		// An identity that resolves to itself has no ID-bearing ancestor
		// to scope it, so the leaf is still a window-global name and the
		// widget cannot be dropped into a second panel as it stands.
		// Only state-keyed shapes are worth reporting: an ID on a plain
		// container is documentation, not a key.
		if key == s.ID && (s.Focusable || s.Scrollable) {
			w.debugWarn(debugCheckUnscopedID, key,
				"ID %q at %s has no ID-bearing ancestor, so it is a "+
					"window-global name; give an ancestor an ID to scope "+
					"it and the leaf becomes reusable elsewhere",
				key, debugPath(path))
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

// TestDuplicateIDs renders the window and returns every identity
// finding in the frame as data: duplicate IDs, and the ID-less shapes
// whose focus, scroll, or OnMouseLeave behaviour silently does not
// work. An empty result means the frame's identities are sound.
//
// This is the assertable form of what GOGUI_DEBUG=1 prints to stderr.
// Unlike [Window.TestUnconsumedEvents] it dispatches nothing and fires
// no callbacks, so it is safe to call on a window an assertion still
// depends on.
//
// An empty result covers the window as rendered, not the app: a widget
// behind a tab or a dialog is only in the tree once that state is on
// screen, so drive the app into each interesting state and check again.
//
// The debug gate is turned on for the duration and restored after.
func (w *Window) TestDuplicateIDs() []string {
	root := w.TestRender(nil)
	if root == nil {
		return nil
	}
	var found []string
	prevOn := DebugEnabled()
	// A fresh warn-once map: a sweep should report the window in front
	// of it, not skip what an earlier sweep or a stray frame reported.
	prevWarned := w.debug.warned
	w.debug.warned = nil
	w.debug.collect = &found
	Debug(true)
	defer func() {
		Debug(prevOn)
		w.debug.collect = nil
		w.debug.warned = prevWarned
	}()
	w.debugAudit(root)
	return found
}

// debugWarn prints a finding to stderr the first time this window
// sees it. subject is the warn-once discriminator within check.
//
// The mask gates here, at the sink, so a finding is never reported (and
// never remembered) while its category is off — turning the category on
// later reports fresh. The generation discard also happens here, not in
// the audit walk, because dispatch-side checks (unconsumed) warn with no
// walk in between.
func (w *Window) debugWarn(check debugCheck, subject, format string, args ...any) {
	if DebugCategory(debugMask.Load())&checkCategory(check) == 0 {
		return
	}
	// Discard warn-once memory built under an older generation of the
	// gate, so Debug(false) then Debug(true) — or re-enabling a single
	// category — re-reports.
	if gen := debugGen.Load(); w.debug.gen != gen {
		w.debug.gen = gen
		w.debug.warned = nil
	}
	key := debugWarnKey{check: check, subject: subject}
	if _, seen := w.debug.warned[key]; seen {
		return
	}
	if w.debug.warned == nil {
		w.debug.warned = make(map[debugWarnKey]struct{})
	}
	w.debug.warned[key] = struct{}{}
	if w.debug.collect != nil {
		*w.debug.collect = append(*w.debug.collect,
			fmt.Sprintf(format, args...))
		return
	}
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
