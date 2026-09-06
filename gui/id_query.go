package gui

import "strings"

// Reading effective IDs back out of a frame.
//
// Every public API that addresses a widget — SetFocus, FindByID,
// ScrollVerticalTo, the Test* helpers — takes the *effective* ID, and
// an app author writes only the leaf. The join rule is documented and
// deterministic, but spelling the result by hand means reading one's
// own View tree for the ID-bearing ancestors, and a wrong spelling
// fails in silence: SetFocus on an unknown ID focuses nothing and
// ScrollVerticalTo writes an offset no scrollable reads.
//
// These two functions answer from the frame instead. They read the
// arranged tree, which carries the identity generation stamped, so the
// answer is what the addressing APIs expect rather than a second
// implementation of the rule. See issue #521.

// EffectiveIDs returns every effective ID in the last laid-out frame,
// in tree order, with duplicates kept.
//
// Use it to see what a window is addressable by. Duplicates are kept
// because two shapes sharing an identity is a defect the addressing
// APIs cannot express — [Window.TestDuplicateIDs] reports it — and
// hiding it here would make this the one view that looks clean.
//
// The list is empty before the first frame is laid out: identities are
// stamped during generation, so a window that has never rendered has
// none.
// exportaudit:keep — dev-diagnostic API for app authors (issue #521)
func (w *Window) EffectiveIDs() []string {
	if w == nil {
		return nil
	}
	var ids []string
	collectEffectiveIDs(&w.layout, &ids)
	return ids
}

// ResolveID returns the effective IDs the last laid-out frame stamped
// for a widget written with the given leaf.
//
// It is the direct answer to "I wrote ID: \"nav\", what do I pass to
// SetFocus?". A leaf under a panel with ID "detail" answers
// ["detail:nav"]; a top-level one answers ["nav"].
//
// A shape matches when its identity is the leaf itself, or when the
// last segment of its identity is the leaf. The second form is needed
// because a widget that resolves its own ID — the datagrid, and every
// factory that writes cfg.ID = w.EffID(cfg.ID) — puts the already
// joined string on the shape, so there is no bare leaf left to compare
// against.
//
// More than one answer means the leaf is used under more than one
// scope, which is legal, and the caller must pick the scope it meant.
// No answer means no widget of that name is in the current frame:
// either the spelling is wrong or the widget is not rendered.
// exportaudit:keep — dev-diagnostic API for app authors (issue #521)
func (w *Window) ResolveID(leaf string) []string {
	if w == nil || leaf == "" {
		return nil
	}
	var out []string
	for _, id := range w.EffectiveIDs() {
		if id == leaf || lastIDSegment(id) == leaf {
			out = append(out, id)
		}
	}
	return out
}

// lastIDSegment returns the part of an effective ID after the final
// IDSep, which is the leaf its Cfg was written with.
func lastIDSegment(id string) string {
	if i := strings.LastIndex(id, IDSep); i >= 0 {
		return id[i+len(IDSep):]
	}
	return id
}

// collectEffectiveIDs appends the identity of every ID-bearing shape
// in one tree. An ID-less shape has no identity and is skipped, but
// its children are still walked: an ID-less container adds no scope
// and does not hide what is under it.
func collectEffectiveIDs(layout *Layout, out *[]string) {
	if layout == nil {
		return
	}
	if layout.Shape != nil {
		if key := layout.Shape.idKey(); key != "" {
			*out = append(*out, key)
		}
	}
	for i := range layout.Children {
		collectEffectiveIDs(&layout.Children[i], out)
	}
}
