package gui

import "time"

// HeroTransitionCfg configures hero transition.
type HeroTransitionCfg struct {
	Easing   EasingFn // nil → EaseOutCubic
	OnDone   func(*Window)
	Duration time.Duration
}

const heroTransitionID = "__hero_transition__"

// HeroTransition animates elements between views. Only one
// HeroTransition can be active at a time (fixed internal ID).
// exportaudit:keep — reachable from an exported signature
type HeroTransition struct {
	outgoing map[string]posSnapshot
	incoming map[string]posSnapshot
	transitionBase
}

// ID implements Animation.
func (h *HeroTransition) ID() string { return heroTransitionID }

// RefreshKind implements Animation.
func (h *HeroTransition) RefreshKind() AnimationRefreshKind { return AnimationRefreshLayout }

// Update implements Animation.
func (h *HeroTransition) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	return updateTransition(&h.transitionBase, ac)
}

// NewHeroTransition creates a HeroTransition with defaults.
func NewHeroTransition(cfg HeroTransitionCfg) *HeroTransition {
	dur := cfg.Duration
	if dur == 0 {
		dur = 300 * time.Millisecond
	}
	eas := cfg.Easing
	if eas == nil {
		eas = EaseOutCubic
	}
	return &HeroTransition{
		transitionBase: transitionBase{
			duration: dur,
			easing:   eas,
			OnDone:   cfg.OnDone,
		},
	}
}

// captureHeroSnapshots finds all hero-marked elements.
func captureHeroSnapshots(layout Layout) map[string]posSnapshot {
	snapshots := make(map[string]posSnapshot)
	captureSnapshots(&layout, snapshots, true)
	return snapshots
}

// applyHeroTransition modifies layout during render for hero effect.
// Called from layoutPipeline under w.mu. Acquires w.animMu to safely
// read w.animations (the animation goroutine may concurrently delete).
func applyHeroTransition(layout *Layout, w *Window) {
	w.animMu.Lock()
	a, ok := w.animations[heroTransitionID]
	w.animMu.Unlock()
	if !ok {
		return
	}
	ht, ok := a.(*HeroTransition)
	if !ok || ht.stopped {
		return
	}
	applyHeroRecursive(layout, ht.progress, ht.outgoing, ht.incoming, 0, 0)
}

func propagateOpacity(layout *Layout, opacity float32) {
	layout.Shape.Opacity = opacity
	for i := range layout.Children {
		propagateOpacity(&layout.Children[i], opacity)
	}
}

// applyHeroRecursive morphs each matched hero toward its incoming
// geometry. dx/dy carry the nearest morphed ancestor's position shift,
// the same rule applyTransitionRecursive follows: layout coordinates are
// absolute, so a morphing card's label — no ID, so no snapshot — would
// otherwise stay at its final position while the card travelled. A hero
// with its own snapshot replaces the carried shift instead of adding to
// it, because the snapshot is an absolute position.
func applyHeroRecursive(layout *Layout, progress float32, outgoing, incoming map[string]posSnapshot, dx, dy float32) {
	shifted := false
	if layout.Shape.Hero && layout.Shape.ID != "" {
		// Hero matching is identity-keyed: the same leaf under two
		// different ID-bearing ancestors is two widgets and does not
		// morph across a transition. A shared hero needs the same
		// effective path on both sides.
		id := layout.Shape.idKey()
		morphProgress := f32Min(1, progress*2)
		fadeProgress := f32Max(0, (progress-0.5)*2)

		if out, hasOut := outgoing[id]; hasOut {
			if _, hasIn := incoming[id]; hasIn {
				finalX, finalY := layout.Shape.X, layout.Shape.Y
				layout.Shape.X = lerp(out.x, finalX, morphProgress)
				layout.Shape.Y = lerp(out.y, finalY, morphProgress)
				layout.Shape.Width = lerp(out.width, layout.Shape.Width, morphProgress)
				layout.Shape.Height = lerp(out.height, layout.Shape.Height, morphProgress)
				dx, dy = layout.Shape.X-finalX, layout.Shape.Y-finalY
				shifted = true
			}
		} else {
			propagateOpacity(layout, fadeProgress)
		}
	}
	if !shifted {
		layout.Shape.X += dx
		layout.Shape.Y += dy
	}
	for i := range layout.Children {
		applyHeroRecursive(&layout.Children[i], progress, outgoing, incoming, dx, dy)
	}
}
