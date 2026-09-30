package gui

import "strings"

// debugAutoState is the focused auto key and the fingerprint of its
// shape at the last audit.
type debugAutoState struct {
	focusID string
	fp      uint64
}

// debugCheckAutoIDShift reports a focused auto key that a different
// widget claims than in the last frame (#881).
//
// An auto key is a position: the count of earlier widgets of one kind
// in one scope. A widget of the same kind that appears above the
// focused one takes its key, and focus follows the key, so the next
// keystroke goes into the wrong field. Nothing else notices: from the
// stores' view the key is still there.
//
// The check compares a fingerprint of the focused shape — its a11y
// role and label — with the last audit. The value text is left out on
// purpose: it changes on every keystroke. A shape with no label has
// only its role in the fingerprint, so a shift between two unlabelled
// widgets of one kind is not seen.
//
// Only the focused key is checked. A shifted scroll offset or selection
// is visual; a shifted focus corrupts data.
func (w *Window) debugCheckAutoIDShift(root *Layout) {
	if DebugCategory(debugMask.Load())&DebugAutoIDs == 0 {
		return
	}
	id := w.FocusID()
	if id == "" || !hasAutoSegment(id) {
		w.debug.auto.focusID = ""
		return
	}
	ly, ok := root.findByID(id)
	if !ok || ly.Shape == nil {
		// DebugUnknownFocus owns a focus ID nothing claims.
		w.debug.auto.focusID = ""
		return
	}
	fp := autoIDFingerprint(ly.Shape)
	prev := w.debug.auto
	w.debug.auto = debugAutoState{focusID: id, fp: fp}
	if prev.focusID != id || prev.fp == fp {
		return
	}
	w.debugWarn(debugCheckAutoIDShift, id,
		"auto identity shifted: focus key %q now belongs to a different "+
			"widget than in the last frame, because a widget of the same "+
			"kind appeared or went before it, so the keyboard moved to "+
			"another field; give this widget or its container an ID",
		id)
}

// autoIDFingerprint hashes what tells two widgets of one kind apart
// without changing while the user edits one: the a11y role and label.
// FNV-1a, written out so the debug audit adds no allocation.
func autoIDFingerprint(s *Shape) uint64 {
	const (
		offset = 14695981039346656037
		prime  = 1099511628211
	)
	h := uint64(offset)
	h ^= uint64(s.A11YRole)
	h *= prime
	if s.a11Y != nil {
		for i := 0; i < len(s.a11Y.Label); i++ {
			h ^= uint64(s.a11Y.Label[i])
			h *= prime
		}
	}
	return h
}

// debugCheckAutoIDReserved reports an app ID whose last segment uses
// the prefix reserved for generated leaves. Such an ID can equal a
// generated one and take its focus, scroll and state slots. key is the
// shape's effective ID; path locates it in the frame.
func (w *Window) debugCheckAutoIDReserved(s *Shape, key string, path []int) {
	leaf := lastIDSegment(s.ID)
	if !strings.HasPrefix(leaf, autoIDPrefix) || w.isGeneratedLeaf(leaf) {
		return
	}
	w.debugWarn(debugCheckAutoIDReserved, key,
		"ID %q at %s starts with the reserved prefix %q, which "+
			"marks the IDs gui generates for widgets without one; "+
			"rename it so it cannot collide with a generated ID",
		key, debugPath(path), autoIDPrefix)
}
