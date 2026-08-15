package gui

import "time"

// LayoutTransitionCfg configures layout animation.
type LayoutTransitionCfg struct {
	Easing   EasingFn // nil → EaseOutCubic
	OnDone   func(*Window)
	Duration time.Duration
}

const layoutTransitionID = "__layout_transition__"

// AnimFlags marks the layout-transition channels that SNAP instead of
// easing. It is an opt-out mask: the zero value animates every channel
// the active transition covers, so a caller that never sets it sees the
// behaviour it always had.
//
// go-gui animates only when AnimateLayout / AnimationAdd is called, so
// "don't animate this" is already spelled "don't start the animation".
// The mask exists for the narrower case — a transition that must run,
// with one channel or one subtree held still.
//
// exportaudit:keep — reachable from an exported field (ContainerCfg.AnimSnap)
type AnimFlags uint8

const (
	// AnimSnapPos skips the X/Y lerp: the shape jumps to its new
	// position while its size still eases.
	//
	// exportaudit:keep — one member of a public mask; the set ships whole
	AnimSnapPos AnimFlags = 1 << iota

	// AnimSnapSize skips the Width/Height lerp: the shape jumps to its
	// new size while its position still eases. This is the "slide, don't
	// stretch" case for card and grid reflow.
	AnimSnapSize

	// AnimSnapAll holds a shape (and, by inheritance, its subtree)
	// completely still for the duration of a transition.
	//
	// exportaudit:keep — one member of a public mask; the set ships whole
	AnimSnapAll = AnimSnapPos | AnimSnapSize
)

// LayoutTransition animates layout changes (resize, reorder, add,
// remove) using FLIP-style animation.
type layoutTransition struct {
	snapshots map[string]posSnapshot
	transitionBase
}

// ID implements Animation.
func (l *layoutTransition) ID() string { return layoutTransitionID }

// RefreshKind implements Animation.
func (l *layoutTransition) RefreshKind() AnimationRefreshKind { return AnimationRefreshLayout }

// Update implements Animation.
func (l *layoutTransition) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	return updateTransition(&l.transitionBase, ac)
}

// AnimateLayout triggers layout transition animation. Call BEFORE
// making layout changes to capture current positions.
func (w *Window) AnimateLayout(cfg LayoutTransitionCfg) {
	dur := cfg.Duration
	if dur == 0 {
		dur = 200 * time.Millisecond
	}
	eas := cfg.Easing
	if eas == nil {
		eas = EaseOutCubic
	}
	lt := &layoutTransition{
		transitionBase: transitionBase{
			duration: dur,
			easing:   eas,
			OnDone:   cfg.OnDone,
		},
		snapshots: captureLayoutSnapshots(w.layout),
	}
	w.AnimationAdd(lt)
}

// captureLayoutSnapshots recursively captures all element positions.
func captureLayoutSnapshots(layout Layout) map[string]posSnapshot {
	snapshots := make(map[string]posSnapshot)
	captureSnapshots(&layout, snapshots, false)
	return snapshots
}

func captureSnapshots(layout *Layout, snapshots map[string]posSnapshot, heroOnly bool) {
	// Snapshots key on the effective ID, so a widget keeps its identity
	// across a transition only while its ID-bearing ancestors do — the
	// same rule the stores use.
	if layout.Shape.ID != "" && (!heroOnly || layout.Shape.Hero) {
		snapshots[layout.Shape.idKey()] = posSnapshot{
			x:      layout.Shape.X,
			y:      layout.Shape.Y,
			width:  layout.Shape.Width,
			height: layout.Shape.Height,
		}
	}
	for i := range layout.Children {
		captureSnapshots(&layout.Children[i], snapshots, heroOnly)
	}
}

// getLayoutTransition returns the active layout transition, if any.
// Acquires w.animMu to safely read w.animations (the animation
// goroutine may concurrently delete stopped animations).
func (w *Window) getLayoutTransition() *layoutTransition {
	w.animMu.Lock()
	a, ok := w.animations[layoutTransitionID]
	w.animMu.Unlock()
	if !ok {
		return nil
	}
	lt, ok := a.(*layoutTransition)
	if !ok {
		return nil
	}
	return lt
}

// applyLayoutTransition interpolates positions during amend phase.
func applyLayoutTransition(layout *Layout, w *Window) {
	lt := w.getLayoutTransition()
	if lt == nil || lt.stopped {
		return
	}
	applyTransitionRecursive(layout, lt, 0, 0, 0)
}

// applyTransitionRecursive lerps each covered channel toward the shape's
// current (post-layout) value. snap is the mask inherited from the
// ancestors; the walk already carries state, so inheritance is a
// parameter rather than a separate cascade pass.
//
// dx/dy carry the nearest interpolated ancestor's position shift. Layout
// coordinates are absolute, so moving a container does not move its
// children — a Text has no ID, so it has no snapshot of its own, and
// without the carried shift it would sit at its final position while its
// container slid underneath it. A shape that has its own snapshot
// replaces the shift rather than adding to it: the snapshot recorded an
// absolute position, so it already accounts for wherever its ancestors
// were.
func applyTransitionRecursive(layout *Layout, lt *layoutTransition, snap AnimFlags, dx, dy float32) {
	// OR-inherit: a snapped parent snaps its whole subtree. With
	// opt-out semantics there is deliberately no way for a child to
	// escape an ancestor's mask — that escape hatch is where the
	// prior art (go-shirei) spends most of its complexity.
	snap |= layout.Shape.AnimSnap

	// A shape whose position snaps sits at its final coordinates, and so
	// does everything under it.
	if snap&AnimSnapPos != 0 {
		dx, dy = 0, 0
	}

	shifted := false
	if layout.Shape.ID != "" && snap != AnimSnapAll {
		if old, ok := lt.snapshots[layout.Shape.idKey()]; ok {
			t := lt.progress
			if snap&AnimSnapPos == 0 {
				finalX, finalY := layout.Shape.X, layout.Shape.Y
				layout.Shape.X = lerp(old.x, finalX, t)
				layout.Shape.Y = lerp(old.y, finalY, t)
				dx, dy = layout.Shape.X-finalX, layout.Shape.Y-finalY
				shifted = true
			}
			if snap&AnimSnapSize == 0 {
				layout.Shape.Width = lerp(old.width, layout.Shape.Width, t)
				layout.Shape.Height = lerp(old.height, layout.Shape.Height, t)
			}
		}
	}
	if !shifted {
		// No snapshot of its own (an ID-less child, or one that first
		// appeared this frame): ride the ancestor's shift.
		layout.Shape.X += dx
		layout.Shape.Y += dy
	}

	for i := range layout.Children {
		applyTransitionRecursive(&layout.Children[i], lt, snap, dx, dy)
	}
}
