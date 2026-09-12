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
	// outgoing holds the hero geometry of the frame the transition was
	// registered on — the "before" side of the morph. AnimationAdd
	// fills it; a transition never registered through AnimationAdd has
	// no before side and every hero simply fades in.
	//
	// There is deliberately no incoming map. The apply walk runs over
	// the incoming tree, so every hero it reaches is on the incoming
	// side by construction, and the morph target is the shape's live
	// post-layout geometry rather than a recorded value.
	outgoing map[string]posSnapshot
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
	if dur <= 0 {
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

// captureHeroSnapshots finds all hero-marked elements. Takes a pointer:
// Layout is a large struct and this runs on a frame's root.
func captureHeroSnapshots(layout *Layout) map[string]posSnapshot {
	snapshots := make(map[string]posSnapshot)
	captureSnapshots(layout, snapshots, true, 0)
	return snapshots
}

// applyHeroTransition modifies layout during render for hero effect.
// Called from layoutPipeline under w.mu.
func applyHeroTransition(layout *Layout, w *Window) {
	progress, outgoing, ok := w.getHeroTransition()
	if !ok {
		return
	}
	applyHeroRecursiveDepth(layout, progress, outgoing, 0, 0, 0)
}

// getHeroTransition returns the running hero transition's progress and
// its outgoing snapshots. Everything is copied out under w.animMu for
// the reason getLayoutTransition spells out: progress and stopped move
// every tick under the animation goroutine, so the frame takes one
// stable value instead of dereferencing the animation after unlocking.
// The map is write-once before the transition is published.
func (w *Window) getHeroTransition() (float32, map[string]posSnapshot, bool) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	a, ok := w.animations[heroTransitionID]
	if !ok {
		return 0, nil, false
	}
	ht, isHero := a.(*HeroTransition)
	if !isHero || ht.stopped {
		return 0, nil, false
	}
	return ht.progress, ht.outgoing, true
}

func propagateOpacityDepth(layout *Layout, opacity float32, depth int) {
	if overMaxDepth(depth) {
		return
	}
	layout.Shape.Opacity = opacity
	for i := range layout.Children {
		propagateOpacityDepth(&layout.Children[i], opacity, depth+1)
	}
}

// applyHeroRecursiveDepth morphs each hero that has an outgoing snapshot
// toward its live post-layout geometry. A hero with no snapshot is new
// on this side of the transition and fades in over the second half.
// Fading is one-directional: a hero that LEFT the tree is not in the
// walk at all, so it cannot fade out.
//
// dx/dy carry the nearest morphed ancestor's position shift,
// the same rule the layout transition follows: layout coordinates are
// absolute, so a morphing card's label — no ID, so no snapshot — would
// otherwise stay at its final position while the card travelled. A hero
// with its own snapshot replaces the carried shift instead of adding to
// it, because the snapshot is an absolute position.
func applyHeroRecursiveDepth(layout *Layout, progress float32, outgoing map[string]posSnapshot, dx, dy float32, depth int) {
	if overMaxDepth(depth) {
		return
	}
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
			finalX, finalY := layout.Shape.X, layout.Shape.Y
			layout.Shape.X = lerp(out.x, finalX, morphProgress)
			layout.Shape.Y = lerp(out.y, finalY, morphProgress)
			layout.Shape.Width = lerp(out.width, layout.Shape.Width, morphProgress)
			layout.Shape.Height = lerp(out.height, layout.Shape.Height, morphProgress)
			dx, dy = layout.Shape.X-finalX, layout.Shape.Y-finalY
			shifted = true
		} else {
			propagateOpacityDepth(layout, fadeProgress, depth)
		}
	}
	if !shifted {
		layout.Shape.X += dx
		layout.Shape.Y += dy
	}
	for i := range layout.Children {
		applyHeroRecursiveDepth(&layout.Children[i], progress, outgoing, dx, dy, depth+1)
	}
}
