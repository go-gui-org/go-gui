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
// Generation owns the join. generateViewLayout stamps Shape.effID from
// the scope it built the shape under, and appendChildViews stamps a
// parent on its way to pushing that parent's scope. Nothing derives an
// identity a second time afterwards, so there is nothing to drift
// against: what every later pass reads — focus traversal, hover,
// scroll, hero, the duplicate audit — is what generation wrote.
//
// (*Window).EffID answers the same question for a widget that reads its
// own state *during* GenerateLayout — a combobox whose open/closed flag
// decides the subtree it returns cannot wait for its own shape to be
// stamped. It joins against the same ambient scope through the same
// resolveLeaf, so the two agree by construction.
//
// One job is left for arrange. resolveFocusOwners rewrites a
// Shape.focusOwner reference, which names an ancestor by its leaf and
// so needs the ancestor stack that a single downward scope string
// cannot carry.
//
// See docs/specs/widget-id-per-scope-uniqueness.md.

// resolveLeaf joins one leaf to the enclosing scope. It is the single
// definition of the rule; the stamp and (*Window).EffID both reach it,
// and the drift check recomputes through it.
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
// per frame — measured as +1 alloc per row on BenchmarkViewFrame back
// when it ran on two paths. It still pays with generation as the only
// stamper: removing the memo measured +25% allocs/op. Scope and leaf
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
// write, so the key matches the effID generation stamps on the same
// shape:
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
// Call it during layout generation and nowhere else. The ID scope is
// only live while the framework descends the View tree, so a call from
// a widget factory body, an event handler or app code has no scope to
// join and returns the leaf. That answer is also the correct answer for
// a top-level widget, so the mistake is invisible until the widget is
// put inside a panel — which is why [DebugUnresolvedKeys] reports the
// call rather than letting it pass. Handlers have [EventCtx.EffID],
// which cannot be called at the wrong time.
// exportaudit:keep — public seam for widget code (see CLAUDE.md)
func (w *Window) EffID(leaf string) string {
	if w == nil {
		return leaf
	}
	if w.viewState.genDepth == 0 {
		w.debugWarn(debugCheckEffIDPhase, leaf,
			"EffID(%q) was called outside layout generation, where the "+
				"ID scope is empty, so it returned the leaf unchanged. "+
				"That is the right answer only for a top-level widget. "+
				"Move the call into GenerateLayout — a factory that "+
				"builds eagerly must defer — or use ctx.EffID in a "+
				"handler.", leaf)
	}
	effID := w.joinLeaf(w.viewState.idScope, leaf)
	// A call from inside the tree has a non-zero depth and still misses
	// the scope when the factory body runs before the descent that
	// opens it. Depth cannot see that; the frame audit can, by
	// comparing this answer with where the shape resolved.
	w.debugNoteEffID(leaf, effID)
	return effID
}

// stampEffID resolves a shape's leaf against the scope it was
// generated under and records the answer on the shape. It is the only
// writer of effID.
//
// A shape with no ID has no identity, so it is left alone rather than
// stamped with the empty string the rule would return: skipping it
// keeps the join off the path for the majority of shapes in a frame.
func stampEffID(w *Window, s *Shape, scope string) string {
	if s == nil || s.ID == "" {
		return ""
	}
	s.effID = w.joinLeaf(scope, s.ID)
	return s.effID
}

// childScopeID stamps a shape and returns the scope its children
// generate under. Only an ID-bearing shape opens a scope; an ID-less
// one passes its own through, which keeps its children flat and keeps
// their collisions as loud as they are today.
//
// A float is not a boundary. It is written inside the tree and stamped
// where it was written, so it keeps the scope of the panel it was
// written in — two panels may each hold a combobox with the same leaf
// and get distinct dropdowns — and float extraction, which happens
// much later, can no longer affect that. An *injected* overlay (toast,
// dialog, inspector) is generated outside the tree and so starts from
// an empty scope.
func childScopeID(w *Window, scope string, s *Shape) string {
	if s == nil || s.ID == "" {
		return scope
	}
	return stampEffID(w, s, scope)
}

// idFrame is one ID-bearing ancestor on the resolve stack, holding the
// leaf it was written as and the identity it resolved to. focusOwner
// names an ancestor by its leaf, so resolving that reference needs both.
type idFrame struct {
	leaf string
	eff  string
}

// resolveFocusOwners rewrites every focusOwner reference in one tree.
//
// Identity itself was stamped at generation, so what is left here is
// the one question a downward scope string cannot answer: focusOwner
// names an ancestor by its *leaf*, and finding that ancestor needs the
// stack of ID-bearing shapes above the reference.
//
// Called from layoutArrange: once on the whole main tree, and once on
// each injected overlay. The walk joins nothing on the common path —
// the scope it carries is read back off the stamps — so it costs the
// pointer walk and no allocation.
func resolveFocusOwners(layout *Layout, w *Window) {
	if layout == nil {
		return
	}
	var frames []idFrame
	if w != nil {
		// Reuse the backing array across frames: the stack is at most as
		// deep as the ID-bearing nesting, so it stops growing after the
		// first few frames.
		frames = w.idScopeStack[:0]
	}
	// Read once per tree, not once per shape: the walk visits every
	// shape in the frame and the gate cannot move while it does.
	frames = resolveFocusOwnersWalk(
		w, layout, "", frames, w.debugStampsChecked(), 0)
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

// resolveFocusOwnersWalk resolves one node and its children, returning
// the frame stack so a grown backing array survives to the next
// sibling.
func resolveFocusOwnersWalk(
	w *Window, layout *Layout, scope string, frames []idFrame,
	checkStamps bool, depth int,
) []idFrame {
	if overMaxDepth(depth) {
		return frames
	}
	s := layout.Shape
	if s == nil {
		for i := range layout.Children {
			frames = resolveFocusOwnersWalk(
				w, &layout.Children[i], scope, frames, checkStamps, depth+1)
		}
		return frames
	}

	if checkStamps {
		w.debugCheckStamp(s, scope)
	}

	if s.focusOwner != "" {
		// In place: after this pass focusOwner is the owner's effID,
		// which is what focusKey hands to the stores. See Shape.
		s.focusOwner = resolveOwnerID(w, frames, scope, s.focusOwner)
	}

	// Read back rather than re-joined: the child scope of an ID-bearing
	// shape is the identity generation already stamped on it. An
	// unstamped shape (a hand-built Layout that skipped generation)
	// carries no stamp, so fall back to the join there: otherwise its
	// children would inherit the grandparent scope and both the drift
	// check and focusOwner resolution would answer for the wrong
	// position. Common path costs nothing — the join runs only for a
	// shape generation never stamped.
	childScope := scope
	pushed := false
	if s.ID != "" {
		childScope = s.effID
		if childScope == "" {
			childScope = w.joinLeaf(scope, s.ID)
		}
		// The frame carries the recovered identity too, not the empty
		// stamp: resolveOwnerID answers with a frame's eff verbatim, so
		// an unstamped ancestor would otherwise blank out every
		// focusOwner beneath it rather than merely misplace it.
		frames = append(frames, idFrame{leaf: s.ID, eff: childScope})
		pushed = true
	}
	for i := range layout.Children {
		frames = resolveFocusOwnersWalk(
			w, &layout.Children[i], childScope, frames, checkStamps, depth+1)
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
