package gui

import (
	"math"
	"time"
)

// SpringCfg controls spring physics behavior.
// exportaudit:keep — caller-facing config (issue #372)
type SpringCfg struct {
	// Stiffness controls spring force. The motion is computed exactly
	// (#1017), so any stiffness is stable at any tick rate.
	// exportaudit:keep — caller-facing config (issue #372)
	Stiffness float32
	// exportaudit:keep — caller-facing config (issue #372)
	Damping float32
	// exportaudit:keep — caller-facing config (issue #372)
	Mass      float32
	Threshold float32
}

// Spring presets.
var (
	// SpringDefault is a neutral spring (100/10/1).
	// exportaudit:keep — preset values (issue #372)
	SpringDefault = SpringCfg{Stiffness: 100, Damping: 10, Mass: 1.0, Threshold: 0.01}
	// SpringGentle is a soft, slow spring (50/8/1).
	// exportaudit:keep — preset values (issue #372)
	SpringGentle = SpringCfg{Stiffness: 50, Damping: 8, Mass: 1.0, Threshold: 0.01}
	// SpringBouncy is a lively underdamped spring (300/15/1).
	// exportaudit:keep — reachable from an exported signature
	SpringBouncy = SpringCfg{Stiffness: 300, Damping: 15, Mass: 1.0, Threshold: 0.01}
	// SpringStiff is a snappy spring (500/30/1).
	// exportaudit:keep — preset values (issue #372)
	SpringStiff = SpringCfg{Stiffness: 500, Damping: 30, Mass: 1.0, Threshold: 0.01}
)

// springState tracks current spring physics.
type springState struct {
	position float32
	velocity float32
	target   float32
	atRest   bool
}

// SpringAnimation uses spring physics for natural motion.
// exportaudit:keep — reachable from an exported signature
type SpringAnimation struct {
	start time.Time
	// sampled is when state was last brought up to date. Update moves the
	// spring by the time since then, so a late or dropped tick does not
	// slow the motion (#1017).
	sampled time.Time
	OnValue func(float32, *Window)
	OnDone  func(*Window)
	AnimID  string
	Config  SpringCfg
	// Refresh is the frame each tick asks for. Zero means a full layout.
	// AnimationRefreshRenderOnly skips the view function and layout; set
	// it only when OnValue changes what is drawn, not the widget tree
	// (#1016).
	Refresh AnimationRefreshKind
	state   springState
	stopped bool
}

// ID implements Animation.
func (s *SpringAnimation) ID() string { return s.AnimID }

// RefreshKind implements Animation.
func (s *SpringAnimation) RefreshKind() AnimationRefreshKind {
	return valueAnimRefreshKind(s.Refresh)
}

// IsStopped implements Animation.
func (s *SpringAnimation) IsStopped() bool { return s.stopped }

// SetStart implements Animation.
func (s *SpringAnimation) SetStart(now time.Time) {
	s.start = now
	s.sampled = now
}

// Update implements Animation. It ignores the loop's fixed tick and
// moves the spring by the time that really passed since the last sample:
// time.Ticker drops ticks when the loop is late, and a fixed step per tick
// would then play the spring in slow motion (#1017).
//
// A spring that never went through SetStart (a custom loop that calls
// Update directly) has no sample time. Its first step uses the dt it was
// given; a step from the zero time.Time would be 2000 years long and snap
// the spring to its target.
func (s *SpringAnimation) Update(_ *Window, dt float32, ac *AnimationCommands) bool {
	now := time.Now()
	if !s.sampled.IsZero() {
		dt = float32(now.Sub(s.sampled).Seconds())
	}
	s.sampled = now
	return updateSpring(s, dt, ac)
}

// NewSpringAnimation creates a SpringAnimation with defaults.
func NewSpringAnimation(id string, onValue func(float32, *Window)) *SpringAnimation {
	return &SpringAnimation{
		AnimID:  id,
		Config:  SpringDefault,
		OnValue: onValue,
	}
}

// SpringTo sets the spring to start at from targeting to. When this
// spring is a new object and AnimationAdd uses it to replace a moving
// spring with the same ID, the moving spring's position and velocity take
// the place of from. Do not call SpringTo on a spring that is still
// registered: the animation loop reads its state, so the call races a
// tick. Build a new spring instead.
func (s *SpringAnimation) SpringTo(from, to float32) {
	s.state.position = from
	s.state.velocity = 0
	s.state.target = to
	s.state.atRest = false
	s.stopped = false
}

// Retarget changes the target while preserving position/velocity.
func (s *SpringAnimation) retarget(to float32) {
	s.state.target = to
	s.state.atRest = false
	s.stopped = false
}

func updateSpring(sp *SpringAnimation, dt float32, ac *AnimationCommands) bool {
	if sp.stopped || sp.state.atRest {
		return false
	}
	if sp.OnValue == nil {
		sp.stopped = true
		return false
	}
	cfg := sp.Config
	if !f32IsFinite(cfg.Mass) || cfg.Mass <= 0 {
		cfg.Mass = SpringDefault.Mass
	}
	if !f32IsFinite(cfg.Threshold) || cfg.Threshold <= 0 {
		cfg.Threshold = SpringDefault.Threshold
	}
	// A negative or non-finite stiffness repels from the target and a
	// negative or non-finite damping injects energy: the motion grows
	// without bound. Fall back to the default instead.
	if !f32IsFinite(cfg.Stiffness) || cfg.Stiffness < 0 {
		cfg.Stiffness = SpringDefault.Stiffness
	}
	if !f32IsFinite(cfg.Damping) || cfg.Damping < 0 {
		cfg.Damping = SpringDefault.Damping
	}
	// A non-finite target (via SpringTo/retarget) has nowhere to snap
	// to: retire without emitting so NaN never reaches OnValue.
	if !f32IsFinite(sp.state.target) {
		sp.state.atRest = true
		ac.appendOnDone(sp.OnDone)
		sp.stopped = true
		return true
	}
	// The exact solution moves the spring by dt in one step. Exact steps
	// add up: one step of 48 ms gives the same state as three of 16 ms,
	// so the tick rate does not change the motion. Calculate in float64:
	// the exponentials and the overdamped roots lose precision in float32.
	d, v := springStep(
		float64(sp.state.position-sp.state.target),
		float64(sp.state.velocity),
		float64(cfg.Stiffness), float64(cfg.Damping), float64(cfg.Mass),
		float64(max(dt, 0)))
	sp.state.position = sp.state.target + float32(d)
	sp.state.velocity = float32(v)
	displacement := sp.state.position - sp.state.target

	// The exact step cannot diverge, but a non-finite start (SpringTo(NaN,
	// …)) or a value past the float32 range still gives NaN or Inf. NaN
	// fails every comparison, so the rest check below would never fire —
	// the spring would live forever, hand NaN to OnValue and force a
	// layout refresh each tick. Snap to the target and retire instead, so
	// the caller's OnDone still runs and no non-finite value escapes.
	diverged := !f32AllFinite2(sp.state.position, sp.state.velocity)

	if diverged ||
		(f32Abs(sp.state.velocity) < cfg.Threshold && f32Abs(displacement) < cfg.Threshold) {
		sp.state.position = sp.state.target
		sp.state.velocity = 0
		sp.state.atRest = true
		ac.appendOnValue(sp.OnValue, sp.state.target)
		ac.appendOnDone(sp.OnDone)
		sp.stopped = true
		return true
	}

	ac.appendOnValue(sp.OnValue, sp.state.position)
	return true
}

// springCriticalBand is how near the damping ratio must be to 1 for
// springStep to use the critically damped solution. The overdamped one
// divides by the gap between its two roots, which goes to zero at 1.
const springCriticalBand = 1e-4

// springStep returns the displacement from the target and the velocity of
// a damped spring after time t, from displacement d0 and velocity v0. It
// is the exact (closed-form) solution of m·x” + c·x' + k·x = 0, so it is
// stable for any stiffness and any t (#1017).
func springStep(d0, v0, k, c, m, t float64) (d, v float64) {
	if k <= 0 {
		// No spring force: damping alone slows the motion, and nothing
		// pulls it back to the target.
		if c <= 0 {
			return d0 + v0*t, v0
		}
		a := c / m
		e := math.Exp(-a * t)
		return d0 + v0*(1-e)/a, v0 * e
	}
	w0 := math.Sqrt(k / m)           // natural frequency
	zeta := c / (2 * math.Sqrt(k*m)) // damping ratio
	switch {
	case math.Abs(zeta-1) < springCriticalBand:
		// Critically damped: x = e^(-w0·t)·(d0 + b·t).
		b := v0 + w0*d0
		e := math.Exp(-w0 * t)
		return e * (d0 + b*t), e * (v0 - w0*b*t)
	case zeta < 1:
		// Underdamped: a decaying oscillation at the damped frequency wd.
		a := zeta * w0
		wd := w0 * math.Sqrt(1-zeta*zeta)
		b := (v0 + a*d0) / wd
		e := math.Exp(-a * t)
		sin, cos := math.Sincos(wd * t)
		return e * (d0*cos + b*sin),
			e * (v0*cos - (w0*w0*d0+a*v0)/wd*sin)
	default:
		// Overdamped: the sum of two decaying exponentials. r1 comes from
		// r1·r2 = w0², not from w0·(s - zeta): that subtraction loses
		// precision when zeta is large.
		r2 := -w0 * (zeta + math.Sqrt(zeta*zeta-1))
		r1 := w0 * w0 / r2
		c1 := (v0 - r2*d0) / (r1 - r2)
		c2 := d0 - c1
		e1 := math.Exp(r1 * t)
		e2 := math.Exp(r2 * t)
		return c1*e1 + c2*e2, r1*c1*e1 + r2*c2*e2
	}
}
