package gui

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Dev-mode diagnostic for an ID lookup that missed.
//
// The addressing APIs take the effective ID and an app author writes
// only the leaf, so "nav" under a panel with ID "detail" reaches
// nothing: FindByID answers (nil, false) and the scroll calls write an
// offset no scrollable reads. Every neighbouring API is already
// guarded from the other side — SetFocus by DebugUnknownFocus, state
// keys by DebugUnresolvedKeys, generation stamps by DebugStampDrift —
// and a lookup was the remaining hole. See issue #536.
//
// Two properties keep the finding honest:
//
// Near miss only. The report needs an identity in the frame whose last
// segment is the leaf that was asked for. A lookup for a name the
// frame stamped nowhere is a probe, not a misspelling: rtfResolveAnchor
// tries a scoped anchor and falls back to a bare slug, and a scroll
// offset set before the first frame names a widget that does not exist
// yet. Both stay silent.
//
// Package-level warn-once. Layout does not name the window that
// stamped it, and FindByID is a Layout method, so this category cannot
// use the per-window memory in debugWarn. It is keyed by the calling
// API and the id instead, and cleared when the gate's generation
// moves so re-enabling Debug after a fix reports the current state.

// debugLookupKey is the warn-once key: one report per calling API per
// id, for the life of one generation of the gate.
type debugLookupKey struct {
	api string
	id  string
}

var (
	lookupWarnMu  sync.Mutex
	lookupWarned  map[debugLookupKey]struct{}
	lookupWarnGen uint64
)

// debugLookupMiss reports a failed lookup that names a leaf the frame
// stamped under a scope. layout is the tree the caller searched; the
// walk climbs to the frame root first, so a lookup made against a
// subtree still sees the identity it should have used.
//
// Called only from a miss path, so a lookup that finds its target
// costs nothing beyond the branch it was already taking.
func debugLookupMiss(layout *Layout, api, effectiveID string) {
	if effectiveID == "" || layout == nil {
		return
	}
	if DebugCategory(debugMask.Load())&DebugUnknownLookup == 0 {
		return
	}
	// Peek before the walk. A miss inside a view function repeats at
	// the frame rate, and the finding is printed once, so the second
	// frame onwards must not pay for a tree walk whose result is
	// already known.
	if lookupWarnSeen(api, effectiveID, false) {
		return
	}
	root := layout
	for root.Parent != nil {
		root = root.Parent
	}
	near := lookupNearMisses(root, effectiveID)
	if len(near) == 0 {
		// Nothing to report *yet*: the widget may be built by a later
		// frame, so the pair stays unmarked and can still report then.
		return
	}
	if lookupWarnSeen(api, effectiveID, true) {
		return
	}
	// Diagnostics are best-effort; a failed write to stderr is not
	// something a GUI frame can act on.
	_, _ = fmt.Fprintf(debugOut,
		"gui: %s(%q) found nothing, but the frame stamped %s for that "+
			"leaf; %s takes the effective ID — read it back with "+
			"(*Window).ResolveID.\n",
		api, effectiveID, quoteJoin(near), api)
}

// lookupNearMisses returns the identities in one frame whose last
// segment is the leaf of effectiveID, excluding effectiveID itself.
// Sorted and deduplicated so the message is the same on every run.
func lookupNearMisses(root *Layout, effectiveID string) []string {
	leaf := lastIDSegment(effectiveID)
	var ids []string
	collectEffectiveIDs(root, &ids)
	out := ids[:0]
	for _, got := range ids {
		if got != effectiveID && lastIDSegment(got) == leaf {
			out = append(out, got)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// lookupWarnSeen reports whether this (api, effectiveID) pair has
// already been reported under the current generation of the gate.
// When mark is set and the pair is new, it is remembered before
// returning, so the next call answers true.
//
// Split from the report itself so debugLookupMiss can ask the cheap
// question (peek) before the expensive one (walk the frame for near
// misses) and mark only what it actually printed.
func lookupWarnSeen(api, effectiveID string, mark bool) bool {
	lookupWarnMu.Lock()
	defer lookupWarnMu.Unlock()
	// Discard memory built under an older generation, so Debug(false)
	// then Debug(true) re-reports rather than staying silent.
	if gen := debugGen.Load(); gen != lookupWarnGen {
		lookupWarnGen = gen
		lookupWarned = nil
	}
	key := debugLookupKey{api: api, id: effectiveID}
	if _, seen := lookupWarned[key]; seen {
		return true
	}
	if !mark {
		return false
	}
	if lookupWarned == nil {
		lookupWarned = make(map[debugLookupKey]struct{})
	}
	if len(lookupWarned) >= maxLookupWarned {
		// Bound the memory a gate left on in a long-running app can
		// hold: dynamic IDs would otherwise grow it without limit.
		// The finding is suppressed rather than re-reported every
		// frame, which is the quieter failure for a diagnostic.
		return true
	}
	lookupWarned[key] = struct{}{}
	return false
}

// lookupCandidateLimit bounds how many identities one finding names.
// A frame may stamp the same leaf under hundreds of scopes — one row
// ID per row of a long list — and a diagnostic that prints all of them
// buries the message it is trying to deliver. The first few are enough
// to show the shape of the scope that was missed.
const lookupCandidateLimit = 5

// maxLookupWarned bounds the warn-once memory. Dynamic IDs in a
// long-running app would otherwise grow it without limit; the bound is
// what keeps a gate left on from leaking. Mirrors maxEffIDAnswers in
// debug_state_keys.go.
const maxLookupWarned = 4096

// quoteJoin renders the candidate identities as a quoted, comma
// separated list, capped at lookupCandidateLimit with a count of what
// was left out.
func quoteJoin(ids []string) string {
	var b strings.Builder
	shown := min(len(ids), lookupCandidateLimit)
	for i, id := range ids[:shown] {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(strconv.Quote(id))
	}
	if rest := len(ids) - shown; rest > 0 {
		fmt.Fprintf(&b, " (and %d more)", rest)
	}
	return b.String()
}
