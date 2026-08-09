package gui

import "time"

// LayoutTransitionCfg configures layout animation.
type LayoutTransitionCfg struct {
	Easing   EasingFn // nil → EaseOutCubic
	OnDone   func(*Window)
	Duration time.Duration
}

const layoutTransitionID = "__layout_transition__"

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
	applyTransitionRecursive(layout, lt)
}

func applyTransitionRecursive(layout *Layout, lt *layoutTransition) {
	if layout.Shape.ID != "" {
		if old, ok := lt.snapshots[layout.Shape.idKey()]; ok {
			t := lt.progress
			layout.Shape.X = lerp(old.x, layout.Shape.X, t)
			layout.Shape.Y = lerp(old.y, layout.Shape.Y, t)
			layout.Shape.Width = lerp(old.width, layout.Shape.Width, t)
			layout.Shape.Height = lerp(old.height, layout.Shape.Height, t)
		}
	}
	for i := range layout.Children {
		applyTransitionRecursive(&layout.Children[i], lt)
	}
}
