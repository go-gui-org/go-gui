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
	applyHeroRecursive(layout, ht.progress, ht.outgoing, ht.incoming)
}

func propagateOpacity(layout *Layout, opacity float32) {
	layout.Shape.Opacity = opacity
	for i := range layout.Children {
		propagateOpacity(&layout.Children[i], opacity)
	}
}

func applyHeroRecursive(layout *Layout, progress float32, outgoing, incoming map[string]posSnapshot) {
	if layout.Shape.Hero && layout.Shape.ID != "" {
		id := layout.Shape.ID
		morphProgress := f32Min(1, progress*2)
		fadeProgress := f32Max(0, (progress-0.5)*2)

		if out, hasOut := outgoing[id]; hasOut {
			if _, hasIn := incoming[id]; hasIn {
				layout.Shape.X = Lerp(out.x, layout.Shape.X, morphProgress)
				layout.Shape.Y = Lerp(out.y, layout.Shape.Y, morphProgress)
				layout.Shape.Width = Lerp(out.width, layout.Shape.Width, morphProgress)
				layout.Shape.Height = Lerp(out.height, layout.Shape.Height, morphProgress)
			}
		} else {
			propagateOpacity(layout, fadeProgress)
		}
	}
	for i := range layout.Children {
		applyHeroRecursive(&layout.Children[i], progress, outgoing, incoming)
	}
}
