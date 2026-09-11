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
	snaps := captureHeroSnapshots(&layout)
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
	// progress=0 → morphProgress=0 → should be at outgoing position
	applyHeroRecursiveDepth(&layout, 0, outgoing, 0, 0, 0)
	if layout.Shape.X != 0 {
		t.Errorf("X = %f, want 0", layout.Shape.X)
	}
}

// heroShiftTree is a hero card that morphs from (0,0,100x100) to
// (100,100,200x200) with an ID-less label inset 10pt from the card's
// final corner. At progress 0.5 morphProgress is 1, so build the tree
// per test and drive progress explicitly.
func heroShiftTree() (*Layout, map[string]posSnapshot) {
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
	return layout, outgoing
}

func TestApplyHeroShiftsIDLessChildren(t *testing.T) {
	// progress 0.25 → morphProgress 0.5 → the card is halfway, so its
	// contents must be halfway too, keeping their inset.
	layout, outgoing := heroShiftTree()
	applyHeroRecursiveDepth(layout, 0.25, outgoing, 0, 0, 0)
	checkShape(t, "card", layout.Shape, 50, 50, 150, 150)
	checkShape(t, "label", layout.Children[0].Shape, 60, 60, 50, 20)
	checkShape(t, "label child", layout.Children[0].Children[0].Shape, 70, 70, 10, 10)
}

func TestApplyHeroFadesInHeroWithNoSnapshot(t *testing.T) {
	// A hero the outgoing side never had is new: it holds its final
	// geometry and fades in over the second half of the transition.
	layout, _ := heroShiftTree()
	applyHeroRecursiveDepth(layout, 0.75, map[string]posSnapshot{}, 0, 0, 0)
	checkShape(t, "card", layout.Shape, 100, 100, 200, 200)
	checkShape(t, "label", layout.Children[0].Shape, 110, 110, 50, 20)
	// fadeProgress = (0.75-0.5)*2 = 0.5, propagated to the subtree.
	if got := layout.Shape.Opacity; got != 0.5 {
		t.Errorf("card opacity = %v, want 0.5", got)
	}
	if got := layout.Children[0].Shape.Opacity; got != 0.5 {
		t.Errorf("label opacity = %v, want 0.5", got)
	}
}

func TestApplyHeroOwnSnapshotReplacesShift(t *testing.T) {
	// A hero child with its own snapshot must not double-count the
	// parent's morph — the snapshot is absolute and already accounts
	// for the ancestor.
	layout, outgoing := heroShiftTree()
	layout.Children[0].Shape.ID = "label"
	layout.Children[0].Shape.Hero = true
	outgoing["label"] = posSnapshot{x: 10, y: 10, width: 50, height: 20}
	applyHeroRecursiveDepth(layout, 0.25, outgoing, 0, 0, 0)
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
	propagateOpacityDepth(&layout, 0.5, 0)
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

// TestAnimationAddCapturesHeroSnapshots is the wiring regression: the
// documented sequence is AnimationAdd then SetView, so AnimationAdd
// is the last moment the outgoing geometry exists. Before this was
// wired, outgoing stayed nil and no hero ever morphed — every one of
// them only faded in.
func TestAnimationAddCapturesHeroSnapshots(t *testing.T) {
	w := &Window{}
	w.layout = Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "card", Hero: true, X: 0, Y: 0, Width: 100, Height: 100}},
			{Shape: &Shape{ID: "plain", X: 5, Y: 5, Width: 10, Height: 10}},
		},
	}

	ht := NewHeroTransition(HeroTransitionCfg{Duration: time.Second})
	w.AnimationAdd(ht)

	if len(ht.outgoing) != 1 {
		t.Fatalf("captured %d hero snapshots, want 1 (non-hero shapes excluded)", len(ht.outgoing))
	}
	if out, ok := ht.outgoing["card"]; !ok || out.width != 100 {
		t.Fatalf("card snapshot = %+v, ok=%v", out, ok)
	}

	// The view has changed and the new tree is arranged: the card now
	// sits at (200,200) at twice the size. progress 0.25 doubles to a
	// morphProgress of 0.5, so the card must be halfway.
	ht.progress = 0.25
	after := Layout{
		Shape: &Shape{ID: "root"},
		Children: []Layout{
			{Shape: &Shape{ID: "card", Hero: true, X: 200, Y: 200, Width: 200, Height: 200}},
		},
	}
	applyHeroTransition(&after, w)
	checkShape(t, "card", after.Children[0].Shape, 100, 100, 150, 150)
}
