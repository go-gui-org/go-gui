package gui

import (
	"slices"
	"strings"
)

// Effective IDs — per-scope uniqueness.
//
// Shape.ID is a *leaf* name. The identity a store or a match keys on is
// the leaf joined to the IDs of its ID-bearing ancestors:
//
//	Panel{ID: "settings"} -> Input{ID: "name"}   // effID settings:name
//	Panel{ID: "profile"}  -> Input{ID: "name"}   // effID profile:name
//
// so the same leaf may appear under two different scopes without the
// two collapsing onto one focus slot, one scroll offset, one state
// entry. Only explicit IDs contribute — never tree position, never a
// child index — so identity survives a reorder and changes only when an
// app changes an ancestor's ID.
//
// A leaf that already contains IDSep is *absolute*: it is the whole
// identity and no further join happens. That is what today's
// ScopeID(cfg.ID, part) composites produce, which is why they keep
// working unchanged.
//
// Two paths compute the join, and they must not drift, so both go
// through resolveLeaf:
//
//   - resolveShapeIDs stamps Shape.effID over a built tree, from
//     layoutArrange, before the layout passes run. It serves everything
//     that happens after layout generation: focus traversal, hover,
//     scroll, hero, the duplicate audit.
//   - (*Window).EffID serves a widget that reads its own state *during*
//     GenerateLayout — a combobox whose open/closed flag decides the
//     subtree it returns cannot wait for a pass that runs afterwards.
//     generateViewLayout maintains the scope for it.
//
// See docs/specs/widget-id-per-scope-uniqueness.md.

// resolveLeaf joins one leaf to the enclosing scope. It is the single
// definition of the rule; both the resolve pass and the generation-time
// scope call it.
//
// An empty leaf stays empty: an ID-less shape has no identity and adds
// no scope. An absolute leaf (one containing IDSep) passes through
// untouched.
func resolveLeaf(scope, leaf string) string {
	if leaf == "" || scope == "" {
		return leaf
	}
	if strings.Contains(leaf, IDSep) {
		return leaf
	}
	return ScopeID(scope, leaf)
}

// idJoinKey memoizes one join. The value depends on nothing but these
// two strings, so a hit is always correct and an eviction only costs a
// recomputation — the objection to a *positional* cache does not apply.
type idJoinKey struct {
	scope string
	leaf  string
}

// joinLeaf is resolveLeaf with a cross-frame memo.
//
// The join is one allocation, and it runs once per ID-bearing widget
// per frame on both paths — generation and the resolve pass — which
// measured as +1 alloc per row on BenchmarkViewFrame. Scope and leaf
// repeat frame to frame (a row's leaf is a constant or a stable key,
// and the scope is the parent's memoized result), so the memo turns
// that back into a map hit.
//
// Bounded, and correctness never depends on a hit: a frame with more
// distinct identities than the cache holds recomputes some of them.
//
// SAFETY: main-goroutine only, like the other per-window caches. Every
// caller is in the view phase, the layout pipeline, or event dispatch,
// all of which run there (see the StateRegistry doc).
func (w *Window) joinLeaf(scope, leaf string) string {
	// Everything the rule answers without allocating — an empty side, an
	// absolute leaf — is answered by the rule itself, and never reaches
	// the map. Only the joining case is worth a lookup.
	if w == nil || leaf == "" || scope == "" ||
		strings.Contains(leaf, IDSep) {
		return resolveLeaf(scope, leaf)
	}
	key := idJoinKey{scope: scope, leaf: leaf}
	m := lazyBoundedMap(&w.idJoinCache, capIDJoin)
	if joined, ok := m.Get(key); ok {
		return joined
	}
	joined := ScopeID(scope, leaf)
	m.Set(key, joined)
	return joined
}

// EffID resolves a leaf ID against the scope currently being generated.
//
// Call it in a widget factory that keys per-widget state on cfg.ID and
// reads that state inside GenerateLayout, for both the read and the
// write, so the key matches the effID the resolve pass will stamp on
// the same shape:
//
//	key := w.EffID(cfg.ID)
//	open := StateReadOr(w, nsCombobox, key, false)
//
// A widget with several keys resolves once instead, over its own copy
// of the cfg, and everything downstream — state slots, focus checks,
// inner IDs composed with ScopeID, the shape's own ID — reads the
// resolved value:
//
//	cfg.ID = w.EffID(cfg.ID)
//
// That is idempotent: the result contains IDSep under an ID-bearing
// ancestor, so resolving it again returns it unchanged, and the resolve
// pass treats the shape's ID as absolute and leaves it exactly as
// computed here.
//
// Handlers may close over the resolved key: it stays valid for as long
// as the ancestor IDs above the widget do, and when one of those
// changes the widget's identity has changed by design.
//
// Outside layout generation the scope is empty, so EffID returns the
// leaf unchanged.
// exportaudit:keep — public seam for widget code (see CLAUDE.md)
func (w *Window) EffID(leaf string) string {
	if w == nil {
		return leaf
	}
	return w.joinLeaf(w.viewState.idScope, leaf)
}

// childScopeID returns the scope a shape's children generate and
// resolve under. Only an ID-bearing shape opens a scope; an ID-less one
// passes its own through, which keeps its children flat and keeps their
// collisions as loud as they are today.
//
// A float is not a boundary. It is written inside the tree, so it keeps
// the scope of the panel it was written in — two panels may each hold a
// combobox with the same leaf and get distinct dropdowns. That works
// because layoutArrange resolves before it extracts the floats; an
// *injected* overlay (toast, dialog, inspector) is generated outside
// the tree and so starts from an empty scope either way.
func childScopeID(w *Window, scope string, s *Shape) string {
	if s == nil || s.ID == "" {
		return scope
	}
	return w.joinLeaf(scope, s.ID)
}

// idFrame is one ID-bearing ancestor on the resolve stack, holding the
// leaf it was written as and the identity it resolved to. focusOwner
// names an ancestor by its leaf, so resolving that reference needs both.
type idFrame struct {
	leaf string
	eff  string
}

// resolveShapeIDs stamps effID on every shape in one tree.
//
// Called from layoutArrange: once on the whole main tree before the
// floats are extracted, and once on each injected overlay. Both start
// from an empty scope. Missing a root would leave a whole layer keyed
// on leaves.
func resolveShapeIDs(layout *Layout, w *Window) {
	if layout == nil {
		return
	}
	var frames []idFrame
	if w != nil {
		// Generation is over by the time this runs, so the view phase's
		// scope must be back at "". Clear it rather than assume: a view
		// function that panics past the restore in generateViewLayout
		// would otherwise poison every later frame's keys with a stale
		// prefix, which is silent and window-wide.
		w.viewState.idScope = ""
		// Reuse the backing array across frames: the stack is at most as
		// deep as the ID-bearing nesting, so it stops growing after the
		// first few frames.
		frames = w.idScopeStack[:0]
	}
	frames = resolveShapeIDsWalk(w, layout, "", frames)
	if w != nil {
		// Drop a backing array one pathologically deep frame grew, so a
		// transient tree cannot pin memory for the window's lifetime.
		if cap(frames) > maxIDScopeStackKeep {
			frames = nil
		}
		w.idScopeStack = frames[:0]
	}
}

// maxIDScopeStackKeep bounds the ancestor stack retained between
// frames. Depth beyond this is pathological, not a layout worth
// optimising for, so its buffer is released instead of pinned.
const maxIDScopeStackKeep = 256

// resolveShapeIDsWalk resolves one node and its children, returning the
// frame stack so a grown backing array survives to the next sibling.
func resolveShapeIDsWalk(
	w *Window, layout *Layout, scope string, frames []idFrame,
) []idFrame {
	s := layout.Shape
	if s == nil {
		for i := range layout.Children {
			frames = resolveShapeIDsWalk(w, &layout.Children[i], scope, frames)
		}
		return frames
	}

	s.effID = w.joinLeaf(scope, s.ID)
	if s.focusOwner != "" {
		// In place: after this pass focusOwner is the owner's effID,
		// which is what focusKey hands to the stores. See Shape.
		s.focusOwner = resolveOwnerID(w, frames, scope, s.focusOwner)
	}

	childScope := childScopeID(w, scope, s)
	pushed := false
	if s.ID != "" {
		frames = append(frames, idFrame{leaf: s.ID, eff: s.effID})
		pushed = true
	}
	for i := range layout.Children {
		frames = resolveShapeIDsWalk(w, &layout.Children[i], childScope, frames)
	}
	if pushed {
		frames = frames[:len(frames)-1]
	}
	return frames
}

// resolveOwnerID resolves a focusOwner reference to the owner's effID.
//
// focusOwner holds an ancestor's leaf, so the innermost frame carrying
// that leaf is the owner. A reference that names no ancestor — a
// composite pointing at a widget elsewhere in the tree — falls back to
// the same join a leaf in this position would get.
func resolveOwnerID(
	w *Window, frames []idFrame, scope, owner string,
) string {
	for _, f := range slices.Backward(frames) {
		if f.leaf == owner {
			return f.eff
		}
	}
	return w.joinLeaf(scope, owner)
}
