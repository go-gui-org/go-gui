package gui

import (
	"testing"
	"time"
)

func TestCaptureLayoutSnapshots(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "root", X: 0, Y: 0, Width: 800, Height: 600},
		Children: []Layout{
			{Shape: &Shape{ID: "a", X: 10, Y: 10, Width: 100, Height: 50}},
			{Shape: &Shape{X: 20, Y: 20, Width: 30, Height: 30}}, // no ID
		},
	}
	snaps := captureLayoutSnapshots(layout)
	if len(snaps) != 2 {
		t.Errorf("got %d snapshots, want 2 (root + a)", len(snaps))
	}
	if s, ok := snaps["a"]; !ok || s.x != 10 {
		t.Error("snapshot 'a' wrong")
	}
}

func TestLayoutTransitionUpdate(t *testing.T) {
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(&lt.transitionBase, &ac)
	if !ok {
		t.Error("should update")
	}
	if !lt.stopped {
		t.Error("should be stopped after duration")
	}
}

func TestApplyTransitionRecursive(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "box", X: 100, Y: 100, Width: 200, Height: 200},
	}
	lt := &layoutTransition{
		transitionBase: transitionBase{progress: 0.5},
		snapshots: map[string]posSnapshot{
			"box": {x: 0, y: 0, width: 100, height: 100},
		},
	}
	applyTransitionRecursive(&layout, lt, 0, 0, 0)
	// At progress=0.5: Lerp(0, 100, 0.5) = 50
	if layout.Shape.X != 50 {
		t.Errorf("X = %f, want 50", layout.Shape.X)
	}
}

// snapTestTree builds a two-level tree whose every shape moves from
// (0,0,100x100) to (100,100,200x200), so a lerped channel reads 50 at
// progress 0.5 and a snapped one keeps its final value.
func snapTestTree(rootSnap, childSnap AnimFlags) (*Layout, *layoutTransition) {
	layout := &Layout{
		Shape: &Shape{ID: "root", X: 100, Y: 100, Width: 200, Height: 200, AnimSnap: rootSnap},
		Children: []Layout{
			{Shape: &Shape{ID: "child", X: 100, Y: 100, Width: 200, Height: 200, AnimSnap: childSnap}},
			{Shape: &Shape{ID: "sibling", X: 100, Y: 100, Width: 200, Height: 200}},
		},
	}
	snap := posSnapshot{x: 0, y: 0, width: 100, height: 100}
	lt := &layoutTransition{
		transitionBase: transitionBase{progress: 0.5},
		snapshots: map[string]posSnapshot{
			"root":    snap,
			"child":   snap,
			"sibling": snap,
		},
	}
	return layout, lt
}

func checkShape(t *testing.T, name string, s *Shape, x, y, w, h float32) {
	t.Helper()
	if s.X != x || s.Y != y || s.Width != w || s.Height != h {
		t.Errorf("%s = (%g,%g,%gx%g), want (%g,%g,%gx%g)",
			name, s.X, s.Y, s.Width, s.Height, x, y, w, h)
	}
}

func TestApplyTransitionSnapChannels(t *testing.T) {
	tests := []struct {
		name       string
		snap       AnimFlags
		x, y, w, h float32
	}{
		// Zero value must reproduce pre-mask behaviour exactly.
		{"zero animates everything", 0, 50, 50, 150, 150},
		{"snap pos", AnimSnapPos, 100, 100, 150, 150},
		{"snap size", AnimSnapSize, 50, 50, 200, 200},
		{"snap all", AnimSnapAll, 100, 100, 200, 200},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			layout, lt := snapTestTree(0, tc.snap)
			applyTransitionRecursive(layout, lt, 0, 0, 0)
			checkShape(t, "child", layout.Children[0].Shape, tc.x, tc.y, tc.w, tc.h)
			// The mask is per-shape: an unflagged sibling is untouched.
			checkShape(t, "sibling", layout.Children[1].Shape, 50, 50, 150, 150)
		})
	}
}

func TestApplyTransitionSnapInherits(t *testing.T) {
	// A snapped parent snaps its whole subtree even though no child
	// sets the flag.
	layout, lt := snapTestTree(AnimSnapAll, 0)
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "root", layout.Shape, 100, 100, 200, 200)
	checkShape(t, "child", layout.Children[0].Shape, 100, 100, 200, 200)
	checkShape(t, "sibling", layout.Children[1].Shape, 100, 100, 200, 200)
}

func TestApplyTransitionSnapDoesNotInheritUpward(t *testing.T) {
	// A snapped child leaves its parent and its siblings animating.
	layout, lt := snapTestTree(0, AnimSnapAll)
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "root", layout.Shape, 50, 50, 150, 150)
	checkShape(t, "child", layout.Children[0].Shape, 100, 100, 200, 200)
	checkShape(t, "sibling", layout.Children[1].Shape, 50, 50, 150, 150)
}

func TestApplyTransitionSnapPartialInheritCombines(t *testing.T) {
	// Inheritance is a union: parent snaps position, child snaps size,
	// so the child ends up fully still while the parent still resizes.
	layout, lt := snapTestTree(AnimSnapPos, AnimSnapSize)
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "root", layout.Shape, 100, 100, 150, 150)
	checkShape(t, "child", layout.Children[0].Shape, 100, 100, 200, 200)
	checkShape(t, "sibling", layout.Children[1].Shape, 100, 100, 150, 150)
}

// shiftTestTree is a snapshotted container that slides 0→100 with an
// ID-less label inside it, positioned 10pt in from the container's final
// corner. The label must ride the container, not jump ahead of it.
func shiftTestTree(containerSnap AnimFlags) (*Layout, *layoutTransition) {
	layout := &Layout{
		Shape: &Shape{ID: "panel", X: 100, Y: 100, Width: 200, Height: 200, AnimSnap: containerSnap},
		Children: []Layout{{
			Shape: &Shape{X: 110, Y: 110, Width: 50, Height: 20},
			Children: []Layout{{
				Shape: &Shape{X: 120, Y: 120, Width: 10, Height: 10},
			}},
		}},
	}
	lt := &layoutTransition{
		transitionBase: transitionBase{progress: 0.5},
		snapshots: map[string]posSnapshot{
			"panel": {x: 0, y: 0, width: 100, height: 100},
		},
	}
	return layout, lt
}

func TestApplyTransitionShiftsIDLessChildren(t *testing.T) {
	// Layout coords are absolute, so without the carried shift the
	// label would sit at its final position while the panel slid.
	layout, lt := shiftTestTree(0)
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "panel", layout.Shape, 50, 50, 150, 150)
	// Panel moved -50; the label and its own child follow by -50 and
	// keep their offsets inside the panel (10 and 20).
	checkShape(t, "label", layout.Children[0].Shape, 60, 60, 50, 20)
	checkShape(t, "label child", layout.Children[0].Children[0].Shape, 70, 70, 10, 10)
}

func TestApplyTransitionSnapPosDoesNotShiftChildren(t *testing.T) {
	// The panel holds its final position, so nothing under it moves.
	layout, lt := shiftTestTree(AnimSnapPos)
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "panel", layout.Shape, 100, 100, 150, 150)
	checkShape(t, "label", layout.Children[0].Shape, 110, 110, 50, 20)
}

func TestApplyTransitionOwnSnapshotReplacesShift(t *testing.T) {
	// A child with its own snapshot must not double-count: the
	// snapshot is an absolute position and already accounts for where
	// the ancestor was.
	layout, lt := shiftTestTree(0)
	layout.Children[0].Shape.ID = "label"
	lt.snapshots["label"] = posSnapshot{x: 10, y: 10, width: 50, height: 20}
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	// lerp(10, 110, 0.5) = 60 — the same place the carried shift would
	// have put it, but derived from the label's own snapshot.
	checkShape(t, "label", layout.Children[0].Shape, 60, 60, 50, 20)
	checkShape(t, "label child", layout.Children[0].Children[0].Shape, 70, 70, 10, 10)
}

func TestApplyTransitionNewShapeRidesAncestorShift(t *testing.T) {
	// A shape that first appeared this frame has an ID but no
	// snapshot, so it must still ride the nearest interpolated
	// ancestor — it appears attached to the panel, not stranded at
	// its final coordinates.
	layout, lt := shiftTestTree(0)
	layout.Children[0].Shape.ID = "new-label"
	applyTransitionRecursive(layout, lt, 0, 0, 0)
	checkShape(t, "panel", layout.Shape, 50, 50, 150, 150)
	checkShape(t, "new label", layout.Children[0].Shape, 60, 60, 50, 20)
}

func TestLayoutTransitionID(t *testing.T) {
	lt := &layoutTransition{}
	if lt.ID() != layoutTransitionID {
		t.Errorf("ID = %q, want %q", lt.ID(), layoutTransitionID)
	}
}

func TestLayoutTransitionRefreshKind(t *testing.T) {
	lt := &layoutTransition{}
	if lt.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %d, want %d", lt.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestLayoutTransitionUpdateInterface(t *testing.T) {
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := lt.Update(nil, 0, &ac)
	if !ok {
		t.Error("Update should return true")
	}
	if !lt.stopped {
		t.Error("should be stopped")
	}
}

func TestLayoutTransitionOnDone(t *testing.T) {
	done := false
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: 200 * time.Millisecond,
			easing:   EaseOutCubic,
			OnDone:   func(*Window) { done = true },
		},
		snapshots: make(map[string]posSnapshot),
	}
	lt.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTransition(&lt.transitionBase, &ac)
	runQueuedCommands(deferred)
	if !done {
		t.Error("OnDone not called")
	}
}
