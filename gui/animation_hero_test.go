package gui

import (
	"testing"
	"time"
)

func TestCaptureHeroSnapshots(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "hero1", Hero: true, X: 10, Y: 20, Width: 100, Height: 50}},
			{Shape: &Shape{ID: "nothero", X: 5, Y: 5, Width: 10, Height: 10}},
			{Shape: &Shape{ID: "hero2", Hero: true, X: 30, Y: 40, Width: 200, Height: 100}},
		},
	}
	snaps := captureHeroSnapshots(layout)
	if len(snaps) != 2 {
		t.Errorf("got %d snapshots, want 2", len(snaps))
	}
	if s, ok := snaps["hero1"]; !ok || s.x != 10 {
		t.Error("hero1 snapshot wrong")
	}
}

func TestHeroTransitionUpdate(t *testing.T) {
	ht := NewHeroTransition(HeroTransitionCfg{})
	ht.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateTransition(&ht.transitionBase, &ac)
	if !ok {
		t.Error("should update")
	}
	if !ht.stopped {
		t.Error("should be stopped after duration")
	}
	if ht.progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", ht.progress)
	}
}

func TestApplyHeroRecursive(t *testing.T) {
	layout := Layout{
		Shape: &Shape{ID: "h", Hero: true, X: 100, Y: 100, Width: 200, Height: 200, Opacity: 1},
	}
	outgoing := map[string]posSnapshot{
		"h": {x: 0, y: 0, width: 100, height: 100},
	}
	incoming := map[string]posSnapshot{
		"h": {x: 100, y: 100, width: 200, height: 200},
	}
	// progress=0 → morphProgress=0 → should be at outgoing position
	applyHeroRecursive(&layout, 0, outgoing, incoming, 0, 0)
	if layout.Shape.X != 0 {
		t.Errorf("X = %f, want 0", layout.Shape.X)
	}
}

// heroShiftTree is a hero card that morphs from (0,0,100x100) to
// (100,100,200x200) with an ID-less label inset 10pt from the card's
// final corner. At progress 0.5 morphProgress is 1, so build the tree
// per test and drive progress explicitly.
func heroShiftTree() (*Layout, map[string]posSnapshot, map[string]posSnapshot) {
	layout := &Layout{
		Shape: &Shape{ID: "card", Hero: true, X: 100, Y: 100, Width: 200, Height: 200, Opacity: 1},
		Children: []Layout{{
			Shape: &Shape{X: 110, Y: 110, Width: 50, Height: 20, Opacity: 1},
			Children: []Layout{{
				Shape: &Shape{X: 120, Y: 120, Width: 10, Height: 10, Opacity: 1},
			}},
		}},
	}
	outgoing := map[string]posSnapshot{"card": {x: 0, y: 0, width: 100, height: 100}}
	incoming := map[string]posSnapshot{"card": {x: 100, y: 100, width: 200, height: 200}}
	return layout, outgoing, incoming
}

func TestApplyHeroShiftsIDLessChildren(t *testing.T) {
	// progress 0.25 → morphProgress 0.5 → the card is halfway, so its
	// contents must be halfway too, keeping their inset.
	layout, outgoing, incoming := heroShiftTree()
	applyHeroRecursive(layout, 0.25, outgoing, incoming, 0, 0)
	checkShape(t, "card", layout.Shape, 50, 50, 150, 150)
	checkShape(t, "label", layout.Children[0].Shape, 60, 60, 50, 20)
	checkShape(t, "label child", layout.Children[0].Children[0].Shape, 70, 70, 10, 10)
}

func TestApplyHeroNoShiftWhenUnmatched(t *testing.T) {
	// No incoming entry: the card only fades, so nothing moves.
	layout, outgoing, _ := heroShiftTree()
	applyHeroRecursive(layout, 0.25, outgoing, map[string]posSnapshot{}, 0, 0)
	checkShape(t, "card", layout.Shape, 100, 100, 200, 200)
	checkShape(t, "label", layout.Children[0].Shape, 110, 110, 50, 20)
}

func TestApplyHeroOwnSnapshotReplacesShift(t *testing.T) {
	// A hero child with its own snapshot must not double-count the
	// parent's morph — the snapshot is absolute and already accounts
	// for the ancestor.
	layout, outgoing, incoming := heroShiftTree()
	layout.Children[0].Shape.ID = "label"
	layout.Children[0].Shape.Hero = true
	outgoing["label"] = posSnapshot{x: 10, y: 10, width: 50, height: 20}
	incoming["label"] = posSnapshot{x: 110, y: 110, width: 50, height: 20}
	applyHeroRecursive(layout, 0.25, outgoing, incoming, 0, 0)
	// lerp(10, 110, 0.5) = 60 — the same place the carried shift would
	// have put it, but derived from the label's own snapshot.
	checkShape(t, "card", layout.Shape, 50, 50, 150, 150)
	checkShape(t, "label", layout.Children[0].Shape, 60, 60, 50, 20)
	checkShape(t, "label child", layout.Children[0].Children[0].Shape, 70, 70, 10, 10)
}

func TestHeroTransitionID(t *testing.T) {
	ht := NewHeroTransition(HeroTransitionCfg{})
	if ht.ID() != heroTransitionID {
		t.Errorf("ID = %q, want %q", ht.ID(), heroTransitionID)
	}
}

func TestHeroTransitionRefreshKind(t *testing.T) {
	ht := NewHeroTransition(HeroTransitionCfg{})
	if ht.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %d, want %d", ht.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestHeroTransitionUpdateInterface(t *testing.T) {
	ht := NewHeroTransition(HeroTransitionCfg{})
	ht.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := ht.Update(nil, 0, &ac)
	if !ok {
		t.Error("Update should return true on completion")
	}
	if !ht.stopped {
		t.Error("should be stopped")
	}
}

func TestPropagateOpacity(t *testing.T) {
	layout := Layout{
		Shape: &Shape{Opacity: 1},
		Children: []Layout{
			{Shape: &Shape{Opacity: 1}},
		},
	}
	propagateOpacity(&layout, 0.5)
	if layout.Shape.Opacity != 0.5 {
		t.Errorf("parent opacity = %f, want 0.5", layout.Shape.Opacity)
	}
	if layout.Children[0].Shape.Opacity != 0.5 {
		t.Errorf("child opacity = %f, want 0.5", layout.Children[0].Shape.Opacity)
	}
}

func TestHeroTransitionOnDone(t *testing.T) {
	done := false
	ht := NewHeroTransition(HeroTransitionCfg{
		OnDone: func(*Window) { done = true },
	})
	ht.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateTransition(&ht.transitionBase, &ac)
	runQueuedCommands(deferred)
	if !done {
		t.Error("OnDone not called")
	}
}
