package gui

import "time"

// SpringCfg controls spring physics behavior.
type SpringCfg struct {
	// Stiffness controls spring force. Values >= ~15600 (with
	// Mass=1) will diverge at the 16ms fixed timestep.
	Stiffness float32
	Damping   float32
	Mass      float32
	Threshold float32
}

// Spring presets.
var (
	SpringDefault = SpringCfg{Stiffness: 100, Damping: 10, Mass: 1.0, Threshold: 0.01}
	SpringGentle  = SpringCfg{Stiffness: 50, Damping: 8, Mass: 1.0, Threshold: 0.01}
	SpringBouncy  = SpringCfg{Stiffness: 300, Damping: 15, Mass: 1.0, Threshold: 0.01}
	SpringStiff   = SpringCfg{Stiffness: 500, Damping: 30, Mass: 1.0, Threshold: 0.01}
)

// springState tracks current spring physics.
type springState struct {
	position float32
	velocity float32
	target   float32
	atRest   bool
}

// SpringAnimation uses spring physics for natural motion.
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
func (s *SpringAnimation) Retarget(to float32) {
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
	if cfg.Mass <= 0 {
		cfg.Mass = SpringDefault.Mass
	}
	if cfg.Threshold <= 0 {
		cfg.Threshold = SpringDefault.Threshold
	}
	displacement := sp.state.position - sp.state.target
	springForce := -cfg.Stiffness * displacement
	dampingForce := -cfg.Damping * sp.state.velocity
	acceleration := (springForce + dampingForce) / cfg.Mass

	sp.state.velocity += acceleration * dt
	sp.state.position += sp.state.velocity * dt
	displacement = sp.state.position - sp.state.target

	if f32Abs(sp.state.velocity) < cfg.Threshold && f32Abs(displacement) < cfg.Threshold {
		sp.state.position = sp.state.target
		sp.state.velocity = 0
		sp.state.atRest = true
		ac.AppendOnValue(sp.OnValue, sp.state.target)
		ac.AppendOnDone(sp.OnDone)
		sp.stopped = true
		return true
	}

	ac.AppendOnValue(sp.OnValue, sp.state.position)
	return true
}
