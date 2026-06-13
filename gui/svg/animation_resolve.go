package svg

import (
	"slices"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// beginSpec is one activation time for an animation: either an
// absolute offset (targetID=="") or a reference to another
// animation's begin/end plus an offset.
type beginSpec struct {
	targetID string
	isEnd    bool
	offset   float32
}

// parseBeginSpecs parses the "begin" attribute of an <animate>
// element into an ordered spec list. Returns nil when the
// attribute is absent, empty, or contains no syncbase references
// (no post-pass resolution needed). Malformed entries are
// skipped; the caller falls back to parseBeginLiteral. Caps at
// maxKeyframes entries to bound allocation.
func parseBeginSpecs(elem string) []beginSpec {
	s, ok := findAttr(elem, "begin")
	if !ok || s == "" {
		return nil
	}
	if !strings.Contains(s, ".begin") && !strings.Contains(s, ".end") {
		return nil
	}
	parts := strings.Split(s, ";")
	if len(parts) > maxKeyframes {
		parts = parts[:maxKeyframes]
	}
	out := make([]beginSpec, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		sp, ok := parseOneBeginSpec(p)
		if ok {
			out = append(out, sp)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// parseOneBeginSpec parses a single begin-list entry. An entry
// is either a time value (literal) or "id.begin[+-]offset" /
// "id.end[+-]offset". Uses LastIndex so ids containing dots
// are preserved intact.
func parseOneBeginSpec(p string) (beginSpec, bool) {
	idxBegin := strings.LastIndex(p, ".begin")
	idxEnd := strings.LastIndex(p, ".end")
	if idxBegin < 0 && idxEnd < 0 {
		return beginSpec{offset: parseTimeValue(p)}, true
	}
	dot := idxBegin
	tokLen := len(".begin")
	isEnd := false
	if idxEnd >= 0 && idxEnd > idxBegin {
		dot = idxEnd
		tokLen = len(".end")
		isEnd = true
	}
	if dot == 0 {
		return beginSpec{}, false
	}
	targetID := strings.TrimSpace(p[:dot])
	if targetID == "" {
		return beginSpec{}, false
	}
	rest := strings.TrimSpace(p[dot+tokLen:])
	var offset float32
	if rest != "" {
		sign := float32(1)
		switch rest[0] {
		case '+':
			rest = rest[1:]
		case '-':
			sign = -1
			rest = rest[1:]
		}
		offset = sign * parseTimeValue(strings.TrimSpace(rest))
	}
	return beginSpec{
		targetID: targetID,
		isEnd:    isEnd,
		offset:   offset,
	}, true
}

// registerAnimation records post-parse bookkeeping for an
// animation just appended to state.animations at position idx:
// self-id → index, plus begin-spec list when syncbase refs are
// present.
func registerAnimation(state *parseState, elem string, idx int) {
	if id, ok := findAttr(elem, "id"); ok && id != "" {
		if state.animIDIndex == nil {
			state.animIDIndex = make(map[string]int)
		}
		state.animIDIndex[id] = idx
	}
	specs := parseBeginSpecs(elem)
	if len(specs) == 0 {
		return
	}
	if state.animBeginSpecs == nil {
		state.animBeginSpecs = make(map[int][]beginSpec)
	}
	state.animBeginSpecs[idx] = specs
}

// resolveBegins walks recorded syncbase specs and writes each
// animation's final BeginSec, plus a per-animation Cycle when the
// begin list defines a chain-restart (multiple resolvable begins
// imply a periodic re-fire). After per-animation resolution, the
// largest derived cycle is propagated to every animation that
// participates in the chain (syncbase begin or BeginSec > 0) so
// freeze-chained sequences re-fire as one global loop. Animations
// with no specs and no repeatCount keep their parse-time defaults.
func resolveBegins(
	anims []gui.SvgAnimation,
	specs map[int][]beginSpec,
	ids map[string]int,
) {
	if len(specs) > 0 {
		resolveBeginsCore(anims, specs, ids)
	}
	propagateGlobalCycle(anims, specs)
}

// resolveBeginsCore resolves first-match BeginSec and derives a
// per-animation Cycle from any second resolvable begin entry. A
// "second begin" indicates the animation re-fires after the first
// activation; the cycle period is its offset from the first.
func resolveBeginsCore(
	anims []gui.SvgAnimation,
	specs map[int][]beginSpec,
	ids map[string]int,
) {
	resolvedFirst := make([]bool, len(anims))
	for i := range anims {
		if _, has := specs[i]; !has {
			resolvedFirst[i] = true
		}
	}
	var resolveFirst func(i int, stack []int) bool
	resolveFirst = func(i int, stack []int) bool {
		if resolvedFirst[i] {
			return true
		}
		if slices.Contains(stack, i) {
			return false
		}
		stack = append(stack, i)
		for _, sp := range specs[i] {
			t, ok := resolveSpec(sp, anims, ids, stack, resolveFirst)
			if !ok {
				continue
			}
			anims[i].BeginSec = t
			resolvedFirst[i] = true
			return true
		}
		resolvedFirst[i] = true
		return false
	}
	for i := range anims {
		if !resolvedFirst[i] {
			resolveFirst(i, nil)
		}
	}
	// Second pass: derive Cycle from a second resolvable begin spec.
	for i, list := range specs {
		if anims[i].Cycle > 0 || len(list) < 2 {
			continue
		}
		seen := false
		var first float32
		for _, sp := range list {
			t, ok := resolveSpec(sp, anims, ids, nil, resolveFirst)
			if !ok {
				continue
			}
			if !seen {
				first = t
				seen = true
				continue
			}
			if t > first {
				anims[i].Cycle = t - first
				break
			}
		}
	}
}

// resolveSpec evaluates a single begin entry to an absolute time.
// stack and recurse may be nil for non-recursive read-only resolves
// (used by the cycle pass after first-pass resolution is complete).
func resolveSpec(
	sp beginSpec, anims []gui.SvgAnimation,
	ids map[string]int, stack []int,
	recurse func(i int, stack []int) bool,
) (float32, bool) {
	if sp.targetID == "" {
		return sp.offset, true
	}
	tgt, ok := ids[sp.targetID]
	if !ok {
		return 0, false
	}
	if recurse != nil && !recurse(tgt, stack) {
		return 0, false
	}
	base := anims[tgt].BeginSec
	if sp.isEnd {
		base += anims[tgt].DurSec
	}
	return base + sp.offset, true
}

// propagateGlobalCycle picks the largest per-animation cycle and
// applies it to chain-participating animations whose own cycle is
// still 0. Chain participation is approximated by "has a syncbase
// begin spec" or "has a non-zero BeginSec" — both indicate the
// animation depends on a chain that must, by design, restart
// periodically. Animations with neither marker (e.g. a one-shot
// fade with no begin and no repeatCount) keep Cycle=0 and play
// once. When no animation has any explicit cycle, this is a no-op.
func propagateGlobalCycle(
	anims []gui.SvgAnimation,
	specs map[int][]beginSpec,
) {
	var global float32
	for i := range anims {
		if anims[i].Cycle > global {
			global = anims[i].Cycle
		}
	}
	if global <= 0 {
		return
	}
	for i := range anims {
		if anims[i].Cycle > 0 {
			continue
		}
		_, hasSpec := specs[i]
		if hasSpec || anims[i].BeginSec > 0 {
			anims[i].Cycle = global
		}
	}
}
