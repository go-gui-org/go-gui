package gui

import (
	"maps"
	"slices"
	"strings"
)

// Dev-mode audit for per-widget state stored under an unresolved ID.
//
// Every other identity mistake in this library has a gate: a duplicate
// effective ID, a focusable or scrollable shape with no ID, and
// hand-rolled ID composition are all reported. A state key that was
// never resolved is the one that was not, and it is the quietest of
// them: the widget renders, the state is written, and the read never
// sees it.
//
// The audit does not instrument the store. It runs once per frame from
// debugAudit, over the identities the layout walk already collected,
// and compares them with the keys the string-keyed state maps hold.

// stringKeyed is the seam onto a BoundedMap whose keys are strings.
// The registry stores maps boxed in `any` with the key type erased, so
// the audit asks through this interface rather than knowing K; a map
// keyed by anything else answers nil.
type stringKeyed interface {
	stringKeys() []string
}

// debugCheckStateKeys reports a state key that is a bare leaf while
// the shape of that name resolved under a scope.
//
// A key is a finding when all three hold:
//
//   - it is a bare leaf: non-empty and free of IDSep, so it was never
//     joined to a scope and is not absolute;
//   - no shape in the window carries it as an identity, so it is not
//     the legitimate key of an unscoped top-level widget;
//   - an ancestor join rewrote a shape of that leaf, which is the
//     shape whose state this key was meant to be.
//
// The third condition is what keeps the audit quiet. It reads
// debugIDs.scoped, which holds only the leaves the resolve pass
// actually changed, so a widget that builds its own absolute ID —
// Form composes "form:login" from cfg.ID "login" — is absent from the
// index and keys its state on cfg.ID without a finding. A cache keyed
// by a file name or a URL satisfies the first two conditions and is
// reported only if a widget in the same window resolved from that
// exact leaf.
func (w *Window) debugCheckStateKeys(ids *debugIDs) {
	if DebugCategory(debugMask.Load())&DebugUnresolvedKeys == 0 {
		return
	}
	w.debugCheckEffIDAnswers(ids)
	if len(ids.scoped) == 0 {
		// No shape in this window was rewritten by a join, so no key
		// can be the unresolved form of one.
		return
	}
	// Namespaces in sorted order, so a window with several findings
	// reports them the same way on every run.
	for _, ns := range slices.Sorted(maps.Keys(w.viewState.registry.maps)) {
		w.debugScanKeys(ns, w.viewState.registry.maps[ns], ids)
	}
	// The hot namespaces are cached fields rather than registry
	// entries, and they are the ones most often keyed by a widget ID.
	w.debugScanKeys(nsDebugScrollX, w.scrollXMap, ids)
	w.debugScanKeys(nsDebugScrollY, w.scrollYMap, ids)
	w.debugScanKeys(nsDebugOverflow, w.overflowMap, ids)
	w.debugScanKeys(nsDebugHoverInside, w.hoverInsideMap, ids)
}

// Names for the hot maps, which have no registry namespace of their
// own but must still be nameable in a finding.
const (
	nsDebugScrollX     = "scrollX"
	nsDebugScrollY     = "scrollY"
	nsDebugOverflow    = "overflow"
	nsDebugHoverInside = "hoverInside"
)

// debugScanKeys reports the findings in one namespace. m is a
// BoundedMap boxed in `any`; a map with a non-string key type, a nil
// map and a value that is not a BoundedMap at all are all skipped.
func (w *Window) debugScanKeys(ns string, m any, ids *debugIDs) {
	sk, ok := m.(stringKeyed)
	if !ok {
		return
	}
	for _, key := range sk.stringKeys() {
		if key == "" || strings.Contains(key, IDSep) {
			continue
		}
		if _, claimed := ids.claimed[key]; claimed {
			// A shape really is named by this key.
			continue
		}
		effID, ok := ids.scoped[key]
		if !ok {
			continue
		}
		w.debugWarn(debugCheckUnresolvedKey, ns+"/"+key,
			"state key %q in namespace %q is an unresolved leaf; "+
				"the shape of that name resolved to %q, so state is "+
				"written and read under different keys. Resolve the "+
				"leaf with w.EffID during GenerateLayout, or with "+
				"ctx.EffID in a handler.",
			key, ns, effID)
	}
}

// maxEffIDAnswers bounds the per-frame record. A window with more
// distinct resolves than this in one frame is past the point where
// naming one more of them helps, and the bound is what keeps a gate
// left on in a long-running app from growing without limit.
const maxEffIDAnswers = 4096

// debugNoteEffID records what EffID answered for a leaf this frame.
// Only the first answer per leaf is kept: a widget that resolves the
// same leaf twice in one frame gets one finding, not two.
func (w *Window) debugNoteEffID(leaf, effID string) {
	if leaf == "" || DebugCategory(debugMask.Load())&DebugUnresolvedKeys == 0 {
		return
	}
	if len(w.debug.effIDAnswers) >= maxEffIDAnswers {
		return
	}
	if w.debug.effIDAnswers == nil {
		w.debug.effIDAnswers = make(map[string]string)
	}
	if _, seen := w.debug.effIDAnswers[leaf]; seen {
		return
	}
	w.debug.effIDAnswers[leaf] = effID
}

// debugCheckEffIDAnswers reports a resolve that returned the bare leaf
// while the shape of that leaf landed under a scope.
//
// This is the general form of the state-key audit, and it catches the
// case the depth check cannot. A factory body called while building a
// parent's Content slice runs at a non-zero generation depth — the
// framework is descending, just not into the container this widget is
// about to sit in — so the scope EffID joins to is the enclosing one,
// not the widget's own. The answer is the bare leaf, the shape resolves
// to "panel:leaf", and the two disagree.
//
// It reports whatever the widget did with the result: a state key, an
// inner ID composed with ScopeID, a focus check, a value simply stored
// in a struct. The state-key scan below sees only what reached a
// scanned map.
func (w *Window) debugCheckEffIDAnswers(ids *debugIDs) {
	// Cleared whether or not anything is reported, so the next frame
	// starts from what that frame actually resolved.
	defer clear(w.debug.effIDAnswers)
	for leaf, answered := range w.debug.effIDAnswers {
		if answered != leaf {
			// The resolve picked up a scope. Whether it picked up the
			// right one is the duplicate-ID check's business.
			continue
		}
		effID, ok := ids.scoped[leaf]
		if !ok {
			// No shape of this leaf was rewritten by a join, so the
			// bare answer is the identity.
			continue
		}
		w.debugWarn(debugCheckEffIDPhase, leaf,
			"EffID(%q) returned the bare leaf, but the shape of that "+
				"name resolved to %q. The call ran before the descent "+
				"into the scope it belongs to — a widget factory that "+
				"builds eagerly resolves against the enclosing scope, "+
				"not its own. Defer the build to GenerateLayout.",
			leaf, effID)
	}
}
