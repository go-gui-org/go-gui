package gui

import (
	"fmt"
	"io"
	"os"
	"slices"
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
// exportaudit:keep — reachable from an exported signature
type DebugCategory uint32

const (
	// DebugDuplicates reports two shapes sharing one ID.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugDuplicates DebugCategory = 1 << iota
	// DebugMissingIDs reports a focusable, scrollable, or
	// OnMouseLeave-bearing shape with no ID, whose behaviour silently
	// does not work.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugMissingIDs
	// DebugUnconsumed reports a callback that acted on an event without
	// consuming it while an ancestor also received it.
	// exportaudit:keep — const name collides with the debugUnconsumed helper
	DebugUnconsumed
	// DebugListBoxNoHeight reports virtualization that degraded: a
	// scrollable listbox resolved to height 0, so every row builds each
	// frame; a variable-height list whose item count passed the
	// per-item storage cap, so it fell back to a uniform row height; or
	// a virtual list that widens frame after frame because its rows
	// demand the width they were handed.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugListBoxNoHeight
	// DebugGradientResampled reports a fill gradient with more stops
	// than the GPU shader uniforms can carry, silently resampled down
	// to the limit on GPU backends.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugGradientResampled
	// DebugWrapOverflow reports a container that sets both Wrap and
	// Overflow: the two are contradictory strategies for the same
	// condition, wrap wins, and overflow is ignored.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugWrapOverflow

	// DebugCallbacks reports a user-visible action the frame pass could
	// not carry out: deferred callbacks that kept re-queueing
	// themselves round after round, so the frame bounded the loop and
	// dropped the rest rather than spin forever; and a link activation
	// that resolved to nothing (an unknown anchor, a relative link with
	// no base URI, or a platform opener that failed).
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugCallbacks

	// DebugWindowDegraded reports a window-level feature the platform
	// could not deliver, so the window opened without it: a
	// WindowCfg.Transparent window that found no ARGB visual, or one
	// running with no compositing manager to blend it against; or a
	// Window.SetWindowOpacity the platform refused, which on Windows
	// is what a Transparent window gets.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugWindowDegraded

	// DebugUnscopedIDs reports a focusable or scrollable shape whose ID
	// resolves to itself — no ID-bearing ancestor above it — so its
	// identity competes in the window-global namespace and cannot be
	// reused elsewhere in the window.
	//
	// Advisory, not a defect: a top-level widget legitimately has no
	// scope, which is why this is the one category [Debug] does not turn
	// on. Ask for it explicitly by passing it to [DebugCategories] when
	// auditing a screen for reusability.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugUnscopedIDs

	// DebugUnresolvedKeys reports per-widget state stored under a leaf
	// ID that an ancestor join rewrote, so the widget's state key and
	// its shape's identity are different strings.
	//
	// A factory that closes over the raw cfg.ID and keys StateMap on it
	// works while the widget sits at the top level, where the scope is
	// empty and the leaf is already the identity. Put the same widget
	// under an ID-bearing ancestor and its shape resolves to
	// "panel:leaf" while its state still lives at "leaf". Nothing
	// panics and nothing looks wrong; the widget stops responding. The
	// two EffID seams exist to prevent this, and neither is required by
	// the compiler.
	//
	// The finding is raised when a state key names no shape in the
	// window and an ancestor join rewrote a shape of that leaf, which
	// is the signature of a missed resolve rather than of an unrelated
	// key that happens to collide.
	//
	// The finding is latent rather than immediate — a widget that keys
	// both the write and the read on the same unresolved leaf works,
	// and fails only when a second instance of it appears under a
	// different scope with the same cfg.ID — but it is a defect either
	// way, so [Debug] turns it on.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugUnresolvedKeys

	// DebugUnknownFocus reports a window whose focus ID names no
	// focusable shape in the frame, so the keyboard has nowhere to go.
	//
	// [Window.SetFocus] takes the *effective* ID and accepts any
	// string. A leaf spelled without the scope its widget sits under —
	// "nav" where the frame stamped "detail:nav" — is accepted, stored
	// and never matched, which is the same silent shape the resolve
	// checks report from the other side.
	//
	// The check runs from the frame audit rather than from SetFocus,
	// because setting focus on a widget the current frame has not built
	// yet is legitimate and common: a view function may focus a control
	// it is in the middle of returning. Only a frame that finished with
	// nothing focusable under that name is a finding.
	//
	// It also fires when the focused widget leaves the tree — a tab
	// switch, a closed dialog — which is a real dangling focus rather
	// than a false alarm: the keyboard is dead until something takes
	// focus back. [Window.ClearFocus] is how a view drops it on purpose.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugUnknownFocus

	// DebugStampDrift reports a shape whose effective ID is not the one
	// its position in the tree calls for: either it disagrees with the
	// scope the shape was arranged under, or an ID-bearing shape has no
	// stamp at all.
	//
	// Identity is stamped once, during layout generation, by
	// generateViewLayout and appendChildViews. A shape that never went
	// through either — a hand-built Layout spliced into a generated
	// tree — carries no stamp, and every store then keys it on its bare
	// leaf, which works until a second widget of that leaf appears
	// somewhere else in the window.
	//
	// This is the check that keeps the single stamping rule honest, so
	// [Debug] turns it on.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugStampDrift

	// DebugUnknownLookup reports an ID lookup that found nothing while
	// the frame stamped the same leaf under a scope, so the caller
	// spelled a leaf where an effective ID was wanted.
	//
	// [Layout.FindByID], [Window.ScrollVerticalTo] and
	// [Window.ScrollVerticalToPct] all take the *effective* ID and all
	// answer a wrong spelling by doing nothing: FindByID returns
	// (nil, false) into the caller's usual `if !ok { return }`, and the
	// scroll calls store an offset no scrollable ever reads. The
	// failure is latent — a leaf is the identity at the top level, so
	// the call works until the widget is dropped under a panel with an
	// ID, and then the feature stops with no output.
	//
	// The finding is raised only when the frame stamped an identity
	// whose last segment is the leaf that was asked for, which is the
	// signature of a missed scope rather than of a lookup for something
	// that is simply not rendered. Probing for a widget that may or may
	// not be in the frame is a legitimate pattern — rtfResolveAnchor
	// does it — and stays silent, as does a lookup made before the
	// first frame is laid out.
	//
	// Warn-once memory for this category is package-level rather than
	// per-window: [Layout.FindByID] is reached from a Layout, which
	// does not name the window that stamped it. A finding is therefore
	// reported once per process per (call, id) pair, and is asserted
	// through the debug output rather than through
	// [Window.TestFindings].
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugUnknownLookup

	// DebugAll is every category [Debug] turns on. [DebugUnscopedIDs]
	// is deliberately absent: it reports a design property rather than
	// a defect, and fires on widgets that are correct as written.
	// exportaudit:keep — dev-diagnostic API for app authors
	DebugAll = DebugDuplicates | DebugMissingIDs | DebugUnconsumed |
		DebugListBoxNoHeight | DebugGradientResampled | DebugWrapOverflow |
		DebugCallbacks | DebugWindowDegraded | DebugUnresolvedKeys |
		DebugUnknownFocus | DebugStampDrift | DebugUnknownLookup
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
//   - two shapes sharing one effective ID
//   - a focusable shape with no ID (never keyboard-reachable)
//   - a scrollable shape with no ID (scroll offset shared with every
//     other ID-less scrollable in the window)
//   - a shape with an OnMouseLeave and no ID (the callback never fires)
//   - a scrollable listbox that resolved to height 0 (virtualization
//     off, every row builds each frame)
//   - a fill gradient with more stops than the GPU shader uniform limit
//     (silently resampled down to the limit on GPU backends)
//   - a container that sets both Wrap and Overflow (wrap wins, overflow
//     is ignored)
//   - a state key that is a bare leaf while an ancestor join rewrote
//     the shape of that name (the widget never resolved its cfg.ID, or
//     resolved at the wrong time)
//   - a frame that finished with a focus ID no focusable shape claims
//   - a shape whose stamp disagrees with the scope it was arranged
//     under, or an ID-bearing shape with no stamp at all
//   - a window-level feature the platform could not deliver
//
// It also reports, from dispatch rather than from the frame audit, a
// callback that acted on an event without consuming it while an
// ancestor also received it; deferred callbacks that kept re-queueing
// themselves until the frame bounded the loop; and a link activation
// that resolved to nothing. See debug_event.go.
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
// exportaudit:keep — collides with the internal debugMask state var
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
// A zero mask is everything off; [DebugAll] is every category [Debug]
// turns on, which excludes [DebugUnscopedIDs]. Turning a category on
// after it was off clears that category's warn-once memory, so a
// re-enabled category reports the frame in front of it.
// exportaudit:keep — dev-diagnostic API for app authors
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
// exportaudit:keep — dev-diagnostic API for app authors
func DebugEnabled() bool { return debugMask.Load() != 0 }

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
	// effIDAnswers records what (*Window).EffID returned for each leaf
	// this frame, so the audit can compare a resolve against the scope
	// the shape actually landed in. Frame-scoped and filled only while
	// DebugUnresolvedKeys is on; see debug_state_keys.go.
	effIDAnswers map[string]string
	// collect, when non-nil, receives findings instead of debugOut. Set
	// only by TestUnconsumedEvents, which needs the findings as data
	// rather than as text on stderr.
	collect *[]string
	gen     uint64
}

// debugAudit runs the dev-mode checks over one frame's composed
// layout tree. No-op unless an audit category is on.
//
// The tree walk is skipped unless a category that reads the frame is
// on; walkCategories below lists them. The unconsumed check runs from
// dispatch and the listbox check from the view phase, so they gate
// themselves.
//
// Called from updateLayoutLocked after composeLayout, so it sees the
// same tree the renderer does, including floating and overlay layers.
func (w *Window) debugAudit(root *Layout) {
	const walkCategories = DebugDuplicates | DebugMissingIDs |
		DebugUnscopedIDs | DebugUnresolvedKeys | DebugUnknownFocus
	if DebugCategory(debugMask.Load())&walkCategories == 0 {
		return
	}
	// ids is frame-scoped, unlike w.debug.warned.
	ids := &debugIDs{claimed: make(map[string]string)}
	var path []int
	w.debugWalk(root, &path, ids)
	// The state-key audit compares the identities this walk collected
	// against the keys the state maps hold, so it runs after the walk
	// and gates itself.
	w.debugCheckStateKeys(ids)
	// Reads the identities the walk collected, so it runs after it.
	w.debugCheckFocusTarget(ids)
}

// focusableByLeaf returns the identities of the focusable shapes this
// frame stamped for one leaf, sorted so the message is the same on
// every run. It reads what the audit walk collected rather than the
// window's tree, so the finding describes the frame it inspected.
func focusableByLeaf(ids *debugIDs, leaf string) []string {
	var out []string
	for id := range ids.focusable {
		if lastIDSegment(id) == leaf {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// debugCheckFocusTarget reports a focus ID that names no focusable
// shape in the frame.
//
// The message offers the effective IDs the frame did stamp for the
// same leaf, because the usual cause is a leaf spelled without its
// scope and the right answer is one short string away.
func (w *Window) debugCheckFocusTarget(ids *debugIDs) {
	if DebugCategory(debugMask.Load())&DebugUnknownFocus == 0 {
		return
	}
	id := w.viewState.focusID
	if id == "" {
		return
	}
	if _, ok := ids.focusable[id]; ok {
		return
	}
	hint := "no widget of that name is in this frame"
	if _, claimed := ids.claimed[id]; claimed {
		hint = "a shape of that name is in this frame but is not focusable"
	} else if near := focusableByLeaf(ids, lastIDSegment(id)); len(near) > 0 {
		hint = "the frame stamped " + strings.Join(near, ", ") +
			" for that leaf"
	}
	w.debugWarn(debugCheckUnknownFocus, id,
		"focus is set to %q but nothing focusable in the frame claims "+
			"it, so the keyboard has nowhere to go; %s. SetFocus takes "+
			"the effective ID — read it back with (*Window).ResolveID.",
		id, hint)
}

// debugStampsChecked reports whether the stamp check is on. Read once
// per tree rather than per shape: resolveFocusOwners walks every shape
// in the frame and the answer cannot change while it does.
func (w *Window) debugStampsChecked() bool {
	return w != nil &&
		DebugCategory(debugMask.Load())&DebugStampDrift != 0
}

// debugCheckStamp verifies one shape's identity against the scope it
// was arranged under.
//
// It runs from resolveFocusOwners, the one pass that still sees a
// shape next to its scope, rather than from the frame audit: by audit
// time the floats have been lifted into their own layers, so the
// composed tree no longer says what scope a float was written in.
//
// The recomputation here is a check, not a second source. It fires only
// under the debug gate and its answer is never stored.
func (w *Window) debugCheckStamp(s *Shape, scope string) {
	// A shape with no ID has no identity to check, and a diagnostic is
	// never the pass that panics on a tree the pipeline tolerates.
	if s == nil || s.ID == "" {
		return
	}
	want := w.joinLeaf(scope, s.ID)
	if s.effID == want {
		return
	}
	if s.effID == "" {
		w.debugWarn(debugCheckStampDrift, s.ID,
			"shape %q carries no effective ID, so every store keys it "+
				"on the bare leaf while the frame arranged it at %q; it "+
				"was not built through GenerateViewLayout. Build child "+
				"views with appendChildViews or GenerateViewLayout "+
				"rather than appending a hand-built Layout.",
			s.ID, want)
		return
	}
	w.debugWarn(debugCheckStampDrift, s.ID,
		"shape %q was stamped %q but the frame arranged it under scope "+
			"%q, where it resolves to %q; its state, focus and scroll "+
			"slots are keyed on the stamp and nothing else will find "+
			"them.", s.ID, s.effID, scope, want)
}

// debugWalk is the depth-first audit. path is the index chain from
// the root to layout, maintained in place to keep the walk to one
// allocation.
func (w *Window) debugWalk(layout *Layout, path *[]int, ids *debugIDs) {
	if s := layout.Shape; s != nil {
		w.debugCheckShape(s, *path, ids)
	}
	for i := range layout.Children {
		*path = append(*path, i)
		w.debugWalk(&layout.Children[i], path, ids)
		*path = (*path)[:len(*path)-1]
	}
}

// debugIDs is what the audit walk collects about identity, shared by
// the checks that need the whole frame rather than one shape.
type debugIDs struct {
	// claimed maps an effective ID to the path of the shape that
	// claimed it first, so a duplicate can name both sites.
	claimed map[string]string
	// scoped maps a leaf to the effective ID an ancestor join gave it.
	// Only shapes whose identity actually changed under the join
	// appear, so a widget that composes its own absolute ID is absent.
	// The state-key audit reads it; see debug_state_keys.go.
	scoped map[string]string
	// focusable holds the identity of every shape the focus system can
	// land on, so the focus-target check can tell "no such widget" from
	// "a widget of that name exists but cannot take focus".
	focusable map[string]struct{}
}

// noteScoped records a leaf that an ancestor join rewrote. The first
// claim wins, which keeps the finding stable across runs.
func (d *debugIDs) noteScoped(leaf, effID string) {
	if leaf == "" || leaf == effID {
		return
	}
	if _, seen := d.scoped[leaf]; seen {
		return
	}
	if d.scoped == nil {
		d.scoped = make(map[string]string)
	}
	d.scoped[leaf] = effID
}

// debugCheckShape runs the per-shape checks. path is the shape's
// index chain from the root; ids records which path first claimed
// each ID this frame.
func (w *Window) debugCheckShape(s *Shape, path []int, ids *debugIDs) {
	if s.ID != "" {
		// Uniqueness is on the *effective* ID: the same leaf under two
		// different ID-bearing ancestors is two identities and is not a
		// duplicate. The message names the effective path, since that is
		// the string the stores and the public APIs use.
		key := s.idKey()
		ids.noteScoped(s.ID, key)
		if s.Focusable {
			if ids.focusable == nil {
				ids.focusable = make(map[string]struct{})
			}
			ids.focusable[key] = struct{}{}
		}
		if first, dup := ids.claimed[key]; dup {
			w.debugWarn(debugCheckDupID, key,
				"duplicate ID %q at %s, first claimed at %s; ID is the "+
					"identity key for focus, scroll, and per-widget state, so "+
					"the two collapse to one tab stop and one state slot",
				key, debugPath(path), first)
		} else {
			ids.claimed[key] = debugPath(path)
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
	return w.TestFindings(DebugAll)
}

// TestFindings renders the window and returns, as data, every finding
// the given categories report. An empty result means the categories
// found nothing in the frame.
//
// [Window.TestDuplicateIDs] is this call with [DebugAll]. Use this form
// to reach a category that [Debug] leaves off, which is otherwise
// reported only on stderr:
//
//	found := w.TestFindings(gui.DebugAll | gui.DebugUnresolvedKeys)
//
// The mask replaces the process gate for the duration of the call, and
// the previous mask is restored after. That gate is process-wide, so
// this is not safe to call from a parallel test. Like
// [Window.TestDuplicateIDs] it dispatches nothing and fires no
// callbacks, and it covers the window as rendered, not the app: drive
// the app into each interesting state and check again.
// exportaudit:keep — dev-diagnostic API for app authors
func (w *Window) TestFindings(mask DebugCategory) []string {
	var found []string
	// Restore the exact mask, not just on/off: a caller may have a
	// narrower gate installed that this call must not widen for good.
	prevMask := DebugCategory(debugMask.Load())
	// A fresh warn-once map: a sweep should report the window in front
	// of it, not skip what an earlier sweep or a stray frame reported.
	prevWarned := w.debug.warned
	w.debug.warned = nil
	w.debug.collect = &found
	DebugCategories(mask)
	// The generation only moves on an off -> on transition, so widening
	// an already-on gate would keep stale warn-once memory. The nil map
	// above covers that; this keeps the two in step.
	w.debug.gen = debugGen.Load()
	defer func() {
		DebugCategories(prevMask)
		w.debug.collect = nil
		w.debug.warned = prevWarned
	}()
	// The gate goes on before the render, not after it: a check that
	// records during layout generation — the EffID phase check — sees
	// nothing if the frame it is meant to inspect was drawn with the
	// categories still off.
	root := w.TestRender(nil)
	if root == nil {
		return nil
	}
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

// DebugGradientResampled reports a fill gradient whose stops exceeded
// the GPU shader uniform limit and were resampled down to it, which
// costs some fidelity even with error-driven placement. Called by the
// GPU backends' draw pass; x, y is the gradient rect origin, used as
// the warn-once discriminator.
func (w *Window) DebugGradientResampled(x, y float32, kept, total int) {
	// NaN never equals itself, so a NaN map key could never match and
	// the warn-once memory would grow a key every frame. Fold NaN in
	// the discriminator only; the message keeps the true position.
	if x != x {
		x = 0
	}
	if y != y {
		y = 0
	}
	w.debugWarn(debugCheckGradientResampled,
		fmt.Sprintf("gradient %g,%g", x, y),
		"gradient at (%g, %g) has %d stops; resampled to %d "+
			"(GPU shader uniform limit)", x, y, total, kept)
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
