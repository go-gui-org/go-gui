package gui

import (
	"strconv"
	"strings"
)

// Auto widget identity (#881, docs/specs/auto-widget-identity.md).
//
// A widget whose Cfg leaves ID empty gets a generated leaf during
// layout generation. The leaf is made from three parts:
//
//  1. the scope: the nearest ancestor with an explicit ID, which the
//     current join already supplies;
//  2. the kind: the widget factory, so a Text or a spinner that
//     appears does not shift the keys of the inputs next to it;
//  3. the count of earlier widgets of that kind in the same scope, in
//     pre-order.
//
// The leaf text is "~<kind><count>", for example "~input3". It holds
// no IDSep, so it joins like any other leaf, and from that point the
// stamp, the join cache, idKey and the stores need no change.
//
// Two rules keep the position key from leaking into explicit identity:
//
//   - An auto leaf does not open a scope (childScopeID). An explicit ID
//     below an ID-less scroll container resolves exactly as it would
//     with the container absent, so app code can still name it.
//   - An explicit ID on a container does open a scope, and its children
//     count from zero there. That is the firewall: a widget inserted
//     outside the container cannot shift the keys inside it.
//
// What is left is a same-kind insert inside one scope, which moves a
// key to the widget after it. DebugAutoIDs reports that for the
// focused widget, where it does the most damage: keystrokes go into
// the wrong field.

// autoIDPrefix starts every generated leaf. App IDs must not start
// with it; DebugAutoIDs reports one that does.
const autoIDPrefix = "~"

// autoCountKey counts one kind of widget in one scope.
type autoCountKey struct {
	scope string
	kind  string
}

// autoLeafKey names one generated leaf. The scope is not part of it:
// the join adds the scope later, through the join cache.
type autoLeafKey struct {
	kind string
	n    uint32
}

// capAutoLeaves bounds the leaf cache. A frame with more auto widgets
// of one kind than this still works; the leaves past the cap are built
// each frame instead of read back.
const capAutoLeaves = 4096

// autoLeaf returns the next generated leaf of kind in the scope being
// generated. Call it only during layout generation, from the deferred
// branch of a factory whose cfg.ID is empty:
//
//	if cfg.ID == "" {
//		return ViewFunc(func(w *Window) View {
//			cfg.ID = w.autoLeaf("slider")
//			return Slider(cfg)
//		})
//	}
//
// kind must be a constant. Its string is part of the counter key and
// the leaf-cache key, so a constant keeps both free of allocation.
func (w *Window) autoLeaf(kind string) string {
	if w == nil {
		return autoIDPrefix + kind + "0"
	}
	return w.autoLeafIn(w.viewState.idScope, kind)
}

// autoWindowScope is the counter scope of autoLeafWindow. It holds a
// NUL, which no effective ID can, so it never equals a real scope.
const autoWindowScope = "\x00window"

// autoLeafWindow is autoLeaf with one count for the whole window
// instead of one per scope. Use it for a widget whose own registry is
// keyed by the bare ID, window-wide: Form keys its field registry and
// FormSummary on it and builds the absolute layout ID "form:<id>", so
// two ID-less forms in two panels would both be "~form0" and share
// one registry. The price is a weaker firewall: a form inserted
// anywhere earlier in the window shifts the key of the forms after it.
func (w *Window) autoLeafWindow(kind string) string {
	if w == nil {
		return autoIDPrefix + kind + "0"
	}
	return w.autoLeafIn(autoWindowScope, kind)
}

// autoLeafIn counts kind in the given counter scope and returns the
// leaf for that count.
func (w *Window) autoLeafIn(scope, kind string) string {
	vs := &w.viewState
	if vs.autoCounts == nil {
		vs.autoCounts = make(map[autoCountKey]uint32)
	}
	ck := autoCountKey{scope: scope, kind: kind}
	n := vs.autoCounts[ck]
	vs.autoCounts[ck] = n + 1

	lk := autoLeafKey{kind: kind, n: n}
	if leaf, ok := vs.autoLeaves[lk]; ok {
		return leaf
	}
	leaf := autoIDPrefix + kind + strconv.FormatUint(uint64(n), 10)
	if vs.autoLeaves == nil {
		vs.autoLeaves = make(map[autoLeafKey]string)
	}
	if len(vs.autoLeaves) < capAutoLeaves {
		vs.autoLeaves[lk] = leaf
	}
	return leaf
}

// resetAutoIDs starts the counters of a new frame. Called once per
// frame, before the root view is generated, and not again for the
// overlays that arrange injects later: those continue the counters of
// the root scope, so a dialog and the main tree cannot both claim
// "~button0" at the top level. clear keeps the map's buckets, so a
// frame after the first allocates nothing here.
func (w *Window) resetAutoIDs() {
	clear(w.viewState.autoCounts)
}

// isAutoID reports whether the last segment of an ID is a generated
// leaf. That is the test for "this shape got an auto ID", whether its
// ID is the bare leaf or the leaf joined to its scope by w.EffID.
func isAutoID(id string) bool {
	return strings.HasPrefix(lastIDSegment(id), autoIDPrefix)
}

// hasAutoSegment reports whether any segment of an ID is a generated
// leaf. A composite with an auto ID composes its inner IDs from it
// ("~combobox0:list"), and those are as position-dependent as the
// composite itself.
func hasAutoSegment(id string) bool {
	return strings.HasPrefix(id, autoIDPrefix) ||
		strings.Contains(id, IDSep+autoIDPrefix)
}

// isGeneratedLeaf reports whether leaf is one autoLeaf produced, as
// opposed to an app ID that happens to use the reserved prefix. Only
// leaves in the cache can answer yes, so a frame past capAutoLeaves
// can report a generated leaf as reserved; the cap is far above any
// real count of one kind.
func (w *Window) isGeneratedLeaf(leaf string) bool {
	if w == nil {
		return false
	}
	kind, digits, ok := splitAutoLeaf(leaf)
	if !ok {
		return false
	}
	n, err := strconv.ParseUint(digits, 10, 32)
	if err != nil {
		return false
	}
	got, ok := w.viewState.autoLeaves[autoLeafKey{kind: kind, n: uint32(n)}]
	return ok && got == leaf
}

// splitAutoLeaf splits "~input12" into "input" and "12".
func splitAutoLeaf(leaf string) (kind, digits string, ok bool) {
	body, found := strings.CutPrefix(leaf, autoIDPrefix)
	if !found {
		return "", "", false
	}
	i := len(body)
	for i > 0 && body[i-1] >= '0' && body[i-1] <= '9' {
		i--
	}
	if i == 0 || i == len(body) {
		return "", "", false
	}
	return body[:i], body[i:], true
}
