package gui

import "time"

// TweenAnimation interpolates a value from A to B over a fixed
// duration with easing.
type TweenAnimation struct {
	start time.Time
	// Easing shapes progress before interpolation. Nil takes
	// EaseOutCubic, matching NewTweenAnimation and the transitions.
	Easing   EasingFn
	OnValue  func(float32, *Window)
	OnDone   func(*Window)
	AnimID   string
	Duration time.Duration
	From     float32
	To       float32
	stopped  bool
}

const tweenDefaultDuration = 300 * time.Millisecond

// ID implements Animation.
func (t *TweenAnimation) ID() string { return t.AnimID }

// RefreshKind implements Animation.
func (t *TweenAnimation) RefreshKind() AnimationRefreshKind { return AnimationRefreshLayout }

// IsStopped implements Animation.
func (t *TweenAnimation) IsStopped() bool { return t.stopped }

// SetStart implements Animation.
func (t *TweenAnimation) SetStart(now time.Time) { t.start = now }

// Update implements Animation.
func (t *TweenAnimation) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	return updateTween(t, ac)
}

// NewTweenAnimation creates a TweenAnimation with defaults.
func NewTweenAnimation(id string, from, to float32, onValue func(float32, *Window)) *TweenAnimation {
	return &TweenAnimation{
		AnimID:   id,
		Duration: tweenDefaultDuration,
		Easing:   EaseOutCubic,
		From:     from,
		To:       to,
		OnValue:  onValue,
	}
}

func updateTween(tw *TweenAnimation, ac *AnimationCommands) bool {
	if tw.stopped {
		return false
	}
	if tw.OnValue == nil {
		tw.stopped = true
		return false
	}
	progress, done := durationProgress(tw.start, tw.Duration)
	if done {
		// A non-finite To would poison layout geometry: skip the
		// value but still run OnDone so the caller can clean up.
		if f32IsFinite(tw.To) {
			ac.appendOnValue(tw.OnValue, tw.To)
		}
		ac.appendOnDone(tw.OnDone)
		tw.stopped = true
		return true
	}
	// Non-finite endpoints can never produce a paintable value.
	if !f32IsFinite(tw.From) || !f32IsFinite(tw.To) {
		return false
	}
	easing := tw.Easing
	if easing == nil {
		easing = EaseOutCubic
	}
	eased := easing(progress)
	v := lerp(tw.From, tw.To, eased)
	// A custom easing hook may return NaN/Inf: drop the frame rather
	// than hand it to layout. Nothing changed, so no refresh.
	if !f32IsFinite(v) {
		return false
	}
	ac.appendOnValue(tw.OnValue, v)
	return true
}
