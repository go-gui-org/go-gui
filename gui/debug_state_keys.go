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
