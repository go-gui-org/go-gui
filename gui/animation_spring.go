package gui

import "time"

// SpringCfg controls spring physics behavior.
// exportaudit:keep — caller-facing config (issue #372)
type SpringCfg struct {
	// Stiffness controls spring force. Values >= ~15600 (with
	// Mass=1) diverge at the 16ms fixed timestep; a diverged spring
	// snaps to its target and retires rather than emitting NaN.
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
	start   time.Time
	OnValue func(float32, *Window)
	OnDone  func(*Window)
	AnimID  string
	Config  SpringCfg
	state   springState
	stopped bool
}

// ID implements Animation.
func (s *SpringAnimation) ID() string { return s.AnimID }

// RefreshKind implements Animation.
func (s *SpringAnimation) RefreshKind() AnimationRefreshKind { return AnimationRefreshLayout }

// IsStopped implements Animation.
func (s *SpringAnimation) IsStopped() bool { return s.stopped }

// SetStart implements Animation.
func (s *SpringAnimation) SetStart(now time.Time) { s.start = now }

// Update implements Animation.
func (s *SpringAnimation) Update(_ *Window, dt float32, ac *AnimationCommands) bool {
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

// SpringTo sets the spring to start at from targeting to.
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
	// negative or non-finite damping injects energy: either diverges
	// into the snap path below. Fall back to the default instead of
	// surprising the caller with an instant arrival.
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
	displacement := sp.state.position - sp.state.target
	springForce := -cfg.Stiffness * displacement
	dampingForce := -cfg.Damping * sp.state.velocity
	acceleration := (springForce + dampingForce) / cfg.Mass

	sp.state.velocity += acceleration * dt
	sp.state.position += sp.state.velocity * dt
	displacement = sp.state.position - sp.state.target

	// Explicit Euler at the fixed timestep amplifies a spring too stiff
	// for the step (see SpringCfg.Stiffness): velocity reaches +Inf and
	// position becomes NaN. NaN fails every comparison, so the rest
	// check below would never fire — the spring would live forever,
	// hand NaN to OnValue and force a layout refresh each tick. Treat
	// divergence as arrival: snap to the target and retire, so the
	// caller's OnDone still runs and no non-finite value escapes.
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
