package gui

import "time"

// Keyframe represents a single animation waypoint.
type Keyframe struct {
	Easing EasingFn // easing TO this keyframe
	At     float32  // position 0.0-1.0
	Value  float32
}

// KeyframeAnimation interpolates through multiple waypoints
// with per-segment easing.
type KeyframeAnimation struct {
	start     time.Time
	OnValue   func(float32, *Window)
	OnDone    func(*Window)
	AnimID    string
	Keyframes []Keyframe
	Duration  time.Duration
	Repeat    bool
	stopped   bool
}

const keyframeDefaultDuration = 500 * time.Millisecond

// ID implements Animation.
func (k *KeyframeAnimation) ID() string { return k.AnimID }

// RefreshKind implements Animation.
func (k *KeyframeAnimation) RefreshKind() AnimationRefreshKind { return AnimationRefreshLayout }

// IsStopped implements Animation.
func (k *KeyframeAnimation) IsStopped() bool { return k.stopped }

// SetStart implements Animation.
func (k *KeyframeAnimation) SetStart(now time.Time) { k.start = now }

// Update implements Animation.
func (k *KeyframeAnimation) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	return updateKeyframe(k, ac)
}

// NewKeyframeAnimation creates a KeyframeAnimation with defaults.
func NewKeyframeAnimation(id string, keyframes []Keyframe, onValue func(float32, *Window)) *KeyframeAnimation {
	return &KeyframeAnimation{
		AnimID:    id,
		Duration:  keyframeDefaultDuration,
		Keyframes: keyframes,
		OnValue:   onValue,
	}
}

func updateKeyframe(kf *KeyframeAnimation, ac *AnimationCommands) bool {
	if kf.stopped {
		return false
	}
	if kf.OnValue == nil {
		kf.stopped = true
		return false
	}
	progress, done := durationProgress(kf.start, kf.Duration)
	if done {
		if len(kf.Keyframes) > 0 {
			ac.AppendOnValue(kf.OnValue, kf.Keyframes[len(kf.Keyframes)-1].Value)
		}
		if kf.Repeat {
			kf.start = kf.start.Add(kf.Duration)
			return true
		}
		ac.AppendOnDone(kf.OnDone)
		kf.stopped = true
		return true
	}
	ac.AppendOnValue(kf.OnValue, interpolateKeyframes(kf.Keyframes, progress))
	return true
}

func interpolateKeyframes(keyframes []Keyframe, progress float32) float32 {
	if len(keyframes) < 2 {
		if len(keyframes) == 1 {
			return keyframes[0].Value
		}
		return 0
	}
	// Binary search for the first keyframe with At >= progress.
	lo, hi := 0, len(keyframes)-1
	for lo < hi {
		mid := (lo + hi) / 2
		if keyframes[mid].At < progress {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == 0 {
		return keyframes[0].Value
	}
	prev := keyframes[lo-1]
	curr := keyframes[lo]
	segLen := curr.At - prev.At
	if segLen <= 0 {
		return curr.Value
	}
	local := (progress - prev.At) / segLen
	easing := curr.Easing
	if easing == nil {
		easing = EaseLinear
	}
	return Lerp(prev.Value, curr.Value, easing(local))
}
