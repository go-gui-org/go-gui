package gui

import (
	"math"
	"testing"
	"time"
)

func TestSpringTo(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(10, 100)
	if sp.state.position != 10 {
		t.Errorf("position = %f", sp.state.position)
	}
	if sp.state.target != 100 {
		t.Errorf("target = %f", sp.state.target)
	}
}

func TestSpringRetarget(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0, 50)
	sp.state.velocity = 5
	sp.retarget(200)
	if sp.state.target != 200 {
		t.Error("target not updated")
	}
	if sp.state.velocity != 5 {
		t.Error("velocity should be preserved")
	}
}

func TestSpringSettles(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringStiff
	sp.SpringTo(100, 100) // already at target
	sp.start = time.Now().Add(-time.Second)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	runQueuedCommands(deferred)
	if got != 100 {
		t.Errorf("expected 100, got %f", got)
	}
	if !sp.stopped {
		t.Error("should be at rest")
	}
}

func TestSpringStoppedSkips(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.stopped = true
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := updateSpring(sp, 0.016, &ac)
	if ok {
		t.Error("stopped spring should return false")
	}
}

func TestSpringID(t *testing.T) {
	sp := NewSpringAnimation("myspring", func(float32, *Window) {})
	if sp.ID() != "myspring" {
		t.Errorf("ID = %q, want myspring", sp.ID())
	}
}

func TestSpringRefreshKind(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	if sp.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %d, want %d", sp.RefreshKind(), AnimationRefreshLayout)
	}
}

func TestSpringIsStopped(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	if sp.IsStopped() {
		t.Error("new spring should not be stopped")
	}
	sp.stopped = true
	if !sp.IsStopped() {
		t.Error("should report stopped")
	}
}

func TestSpringSetStart(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	now := time.Now()
	sp.SetStart(now)
	if sp.start != now {
		t.Error("SetStart should update start time")
	}
}

func TestSpringUpdateInterface(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringStiff
	sp.SpringTo(100, 100) // already at target
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	ok := sp.Update(nil, 0.016, &ac)
	if !ok {
		t.Error("Update should return true")
	}
	runQueuedCommands(deferred)
	if got != 100 {
		t.Errorf("got %f, want 100", got)
	}
}

func TestSpringThresholdUsesUpdatedPosition(t *testing.T) {
	var got float32
	sp := NewSpringAnimation("s", func(v float32, _ *Window) { got = v })
	sp.Config = SpringCfg{Stiffness: 100, Damping: 10, Mass: 1, Threshold: 0.01}
	// Position very close to target: pre-update displacement is tiny,
	// but velocity will carry position past threshold after integration.
	sp.state.position = 100.005
	sp.state.velocity = -5
	sp.state.target = 100
	sp.state.atRest = false
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	updateSpring(sp, 0.016, &ac)
	// The spring should NOT be at rest because displacement is recomputed
	// after the position update.
	if sp.stopped && got == sp.state.target {
		// If stopped with exact target, the old (stale) displacement
		// was used. The fix ensures the post-update displacement is checked.
		t.Error("threshold should use post-update displacement")
	}
}

// tickSpringN advances sp n fixed 16ms steps.
func tickSpringN(sp *SpringAnimation, n int) {
	for range n {
		deferred := make([]queuedCommand, 0, 4)
		ac := newAnimationCommands(&deferred)
		updateSpring(sp, 0.016, &ac)
		runQueuedCommands(deferred)
	}
}

// Re-adding a spring under the ID of a running spring keeps the running
// spring's position and velocity (#1015). Before the fix SpringTo zeroed
// the velocity, so a toggle mid-motion stopped dead and restarted from rest.
func TestAnimationAddSpringKeepsMotion(t *testing.T) {
	w := &Window{}
	first := NewSpringAnimation("s", func(float32, *Window) {})
	first.SpringTo(0, 100)
	w.AnimationAdd(first)
	tickSpringN(first, 5)
	pos, vel := first.state.position, first.state.velocity
	if vel <= 0 {
		t.Fatalf("setup: velocity = %f, want > 0", vel)
	}

	// The caller passes its own idea of "from"; the running spring wins.
	second := NewSpringAnimation("s", func(float32, *Window) {})
	second.SpringTo(0, -100)
	w.AnimationAdd(second)

	if second.state.position != pos {
		t.Errorf("position = %f, want %f", second.state.position, pos)
	}
	if second.state.velocity != vel {
		t.Errorf("velocity = %f, want %f", second.state.velocity, vel)
	}
	if second.state.target != -100 {
		t.Errorf("target = %f, want -100", second.state.target)
	}
}

// A settled spring carries nothing: its velocity is zero, so the caller's
// from must win, as it did before #1015.
func TestAnimationAddSpringAfterRestUsesFrom(t *testing.T) {
	w := &Window{}
	first := NewSpringAnimation("s", func(float32, *Window) {})
	first.SpringTo(50, 50)
	w.AnimationAdd(first)
	tickSpringN(first, 1)
	if !first.stopped {
		t.Fatal("setup: spring should have settled")
	}

	second := NewSpringAnimation("s", func(float32, *Window) {})
	second.SpringTo(10, 100)
	w.AnimationAdd(second)
	if second.state.position != 10 || second.state.velocity != 0 {
		t.Errorf("pos/vel = %f/%f, want 10/0",
			second.state.position, second.state.velocity)
	}
}

// AnimationRemove before AnimationAdd is the documented hard reset.
func TestAnimationAddSpringAfterRemoveUsesFrom(t *testing.T) {
	w := &Window{}
	first := NewSpringAnimation("s", func(float32, *Window) {})
	first.SpringTo(0, 100)
	w.AnimationAdd(first)
	tickSpringN(first, 5)
	w.AnimationRemove("s")

	second := NewSpringAnimation("s", func(float32, *Window) {})
	second.SpringTo(0, -100)
	w.AnimationAdd(second)
	if second.state.position != 0 || second.state.velocity != 0 {
		t.Errorf("pos/vel = %f/%f, want 0/0",
			second.state.position, second.state.velocity)
	}
}

// linearEase keeps a tween's velocity constant, so a test can check the
// carried velocity without depending on when the sample is taken.
func linearEase(t float32) float32 { return t }

// A spring that replaces a running tween starts at the tween's value and
// velocity (#1015), so a sidebar toggled during its tween turns smoothly.
func TestAnimationAddSpringOverTweenKeepsMotion(t *testing.T) {
	w := &Window{}
	// A 100s tween makes the wall time that passes between the backdate
	// and the add (a few ms, more on a loaded runner) too small to see.
	tw := &TweenAnimation{
		AnimID: "s", From: 0, To: 100, Duration: 100 * time.Second,
		Easing: linearEase, OnValue: func(float32, *Window) {},
	}
	w.AnimationAdd(tw)
	tw.start = time.Now().Add(-50 * time.Second) // halfway

	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0, -100)
	w.AnimationAdd(sp)

	if f32Abs(sp.state.position-50) > 0.1 {
		t.Errorf("position = %f, want 50", sp.state.position)
	}
	if f32Abs(sp.state.velocity-1) > 0.01 {
		t.Errorf("velocity = %f, want 1 (100 units / 100s)", sp.state.velocity)
	}
}

// A tween that has run its course carries nothing: the new spring
// starts at its own from, at rest.
func TestAnimationAddSpringOverFinishedTweenUsesFrom(t *testing.T) {
	w := &Window{}
	tw := &TweenAnimation{
		AnimID: "s", From: 0, To: 1, Duration: 300 * time.Millisecond,
		OnValue: func(float32, *Window) {},
	}
	w.AnimationAdd(tw)
	tw.start = time.Now().Add(-time.Second)

	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0.25, 1)
	w.AnimationAdd(sp)
	if sp.state.position != 0.25 || sp.state.velocity != 0 {
		t.Errorf("pos/vel = %f/%f, want 0.25/0",
			sp.state.position, sp.state.velocity)
	}
}

// tweenMotion rejects what it cannot sample: a stopped tween, and an
// easing hook that returns NaN.
func TestTweenMotionRejectsDegenerate(t *testing.T) {
	tw := &TweenAnimation{From: 0, To: 1, Duration: time.Second,
		Easing: linearEase, start: time.Now()}
	if _, _, ok := tweenMotion(tw); !ok {
		t.Fatal("setup: running tween should sample")
	}
	tw.stopped = true
	if _, _, ok := tweenMotion(tw); ok {
		t.Error("stopped tween sampled")
	}
	tw.stopped = false
	tw.Easing = func(float32) float32 { return float32(math.NaN()) }
	if _, _, ok := tweenMotion(tw); ok {
		t.Error("NaN easing sampled")
	}
}

// A registered spring with a NaN position (SpringTo(NaN, …) before its first
// tick) must not hand NaN to its replacement: the new spring keeps its from.
func TestAnimationAddSpringOverNaNSpringUsesFrom(t *testing.T) {
	w := &Window{}
	bad := NewSpringAnimation("s", func(float32, *Window) {})
	bad.SpringTo(float32(math.NaN()), 100)
	w.AnimationAdd(bad)

	next := NewSpringAnimation("s", func(float32, *Window) {})
	next.SpringTo(5, 0)
	w.AnimationAdd(next)
	if next.state.position != 5 || next.state.velocity != 0 {
		t.Errorf("pos/vel = %f/%f, want 5/0",
			next.state.position, next.state.velocity)
	}
}

// A zero-duration tween is finished the moment it starts, so it has no
// motion to give and no division by its duration happens.
func TestTweenMotionZeroDurationRejected(t *testing.T) {
	tw := &TweenAnimation{From: 0, To: 1, start: time.Now()}
	if _, _, ok := tweenMotion(tw); ok {
		t.Error("zero-duration tween sampled")
	}
}

// Re-adding the same spring object after SpringTo is a restart: SpringTo
// already zeroed its velocity and there is no other spring to copy from.
// AnimationAdd's doc names this case.
func TestAnimationAddSameSpringRestartsFromRest(t *testing.T) {
	w := &Window{}
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0, 100)
	w.AnimationAdd(sp)
	tickSpringN(sp, 5)

	sp.SpringTo(10, -100)
	w.AnimationAdd(sp)
	if sp.state.position != 10 || sp.state.velocity != 0 {
		t.Errorf("pos/vel = %f/%f, want 10/0",
			sp.state.position, sp.state.velocity)
	}
}

// stepSpringBy advances sp through the given tick sizes (seconds) and
// returns its position.
func stepSpringBy(sp *SpringAnimation, dts ...float32) float32 {
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	for _, dt := range dts {
		deferred = deferred[:0]
		updateSpring(sp, dt, &ac)
	}
	return sp.state.position
}

// repeatDt returns n copies of dt.
func repeatDt(dt float32, n int) []float32 {
	out := make([]float32, n)
	for i := range out {
		out[i] = dt
	}
	return out
}

// The value at time t must not depend on how the time was cut into ticks
// (#1017). Before, each tick took one explicit Euler step, so 4 ms ticks,
// 16 ms ticks and a dropped tick each gave a different value at the same t.
func TestSpringValueIndependentOfTickRate(t *testing.T) {
	newSpring := func() *SpringAnimation {
		sp := NewSpringAnimation("s", func(float32, *Window) {})
		sp.Config = SpringBouncy
		sp.SpringTo(0, 100)
		return sp
	}
	// 0.192 s in three shapes: 12 ticks of 16 ms, 48 ticks of 4 ms, and
	// 16 ms ticks with two dropped (one 48 ms tick in their place).
	at16 := stepSpringBy(newSpring(), repeatDt(0.016, 12)...)
	at4 := stepSpringBy(newSpring(), repeatDt(0.004, 48)...)
	dropped := append(repeatDt(0.016, 5), 0.048)
	dropped = append(dropped, repeatDt(0.016, 4)...)
	atDropped := stepSpringBy(newSpring(), dropped...)

	if f32Abs(at16-at4) > 0.01 {
		t.Errorf("16 ms ticks = %f, 4 ms ticks = %f, want equal", at16, at4)
	}
	if f32Abs(at16-atDropped) > 0.01 {
		t.Errorf("16 ms ticks = %f, dropped ticks = %f, want equal", at16, atDropped)
	}
}

// Update moves the spring by the time that really passed since the last
// sample, not by the fixed tick the loop passes in (#1017). A late tick
// must not leave the spring behind.
func TestSpringUpdateUsesElapsedTime(t *testing.T) {
	late := NewSpringAnimation("s", func(float32, *Window) {})
	late.SpringTo(0, 100)
	late.SetStart(time.Now().Add(-100 * time.Millisecond))
	before := late.sampled
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	late.Update(nil, 0.016, &ac)
	// Wall time passes between SetStart and the clock read in Update, more
	// under -race or on a loaded runner. Bracket the step instead of
	// guessing it: at least 100 ms, at most the time measured now. The
	// spring still rises over this range (its first peak is at ~0.36 s).
	elapsed := float32(time.Since(before).Seconds())

	stepFor := func(dt float32) float32 {
		ref := NewSpringAnimation("s", func(float32, *Window) {})
		ref.SpringTo(0, 100)
		return stepSpringBy(ref, dt)
	}
	lo, hi := stepFor(0.1), stepFor(elapsed)
	got := late.state.position
	if got < lo-1e-3 || got > hi+1e-3 {
		t.Errorf("position = %f, want in [%f, %f] (100 ms of motion or more)",
			got, lo, hi)
	}
}

// A spring driven by Update without SetStart has no sample time. Its
// first step must use the given dt, not the time since the zero
// time.Time, which would snap it to the target.
func TestSpringUpdateWithoutSetStartUsesDt(t *testing.T) {
	sp := NewSpringAnimation("s", func(float32, *Window) {})
	sp.SpringTo(0, 100)
	deferred := make([]queuedCommand, 0, 4)
	ac := newAnimationCommands(&deferred)
	sp.Update(nil, 0.016, &ac)
	if sp.stopped {
		t.Fatal("spring snapped to its target on the first Update")
	}
	ref := NewSpringAnimation("s", func(float32, *Window) {})
	ref.SpringTo(0, 100)
	if want := stepSpringBy(ref, 0.016); f32Abs(sp.state.position-want) > 1e-4 {
		t.Errorf("position = %f, want %f (one 16 ms step)", sp.state.position, want)
	}
}

// The step is exact, so the preset springs follow the true spring motion.
// A fine Euler integration is the reference: at a 0.1 ms step its error is
// far below the tolerance.
func TestSpringPresetsMatchFineEuler(t *testing.T) {
	presets := map[string]SpringCfg{
		"default": SpringDefault, "gentle": SpringGentle,
		"bouncy": SpringBouncy, "stiff": SpringStiff,
	}
	for name, cfg := range presets {
		t.Run(name, func(t *testing.T) {
			// Semi-implicit Euler, as the old code did, in float64.
			pos, vel := 0.0, 0.0
			k, c, m := float64(cfg.Stiffness), float64(cfg.Damping), float64(cfg.Mass)
			const fine = 1e-4
			for range 3200 { // 0.32 s
				vel += (-k*(pos-1) - c*vel) / m * fine
				pos += vel * fine
			}
			sp := NewSpringAnimation("s", func(float32, *Window) {})
			sp.Config = cfg
			sp.SpringTo(0, 1)
			got := stepSpringBy(sp, repeatDt(0.016, 20)...)
			if math.Abs(float64(got)-pos) > 2e-3 {
				t.Errorf("position at 0.32 s = %f, want %f", got, pos)
			}
		})
	}
}

// Each spring regime (under, critically and overdamped, and no stiffness)
// starts at its origin with its velocity, and settles at the target.
func TestSpringStepRegimes(t *testing.T) {
	cases := map[string]SpringCfg{
		"underdamped": {Stiffness: 100, Damping: 4, Mass: 1},
		"critical":    {Stiffness: 100, Damping: 20, Mass: 1},
		"overdamped":  {Stiffness: 100, Damping: 60, Mass: 1},
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			k, c, m := float64(cfg.Stiffness), float64(cfg.Damping), float64(cfg.Mass)
			d, v := springStep(2, -3, k, c, m, 0)
			if math.Abs(d-2) > 1e-9 || math.Abs(v+3) > 1e-9 {
				t.Errorf("t=0: d/v = %f/%f, want 2/-3", d, v)
			}
			// The velocity is the derivative of the displacement.
			const h = 1e-6
			d1, v1 := springStep(2, -3, k, c, m, 0.1)
			d2, _ := springStep(2, -3, k, c, m, 0.1+h)
			if slope := (d2 - d1) / h; math.Abs(slope-v1) > 1e-3 {
				t.Errorf("velocity = %f, slope = %f", v1, slope)
			}
			d, v = springStep(2, -3, k, c, m, 10)
			if math.Abs(d) > 1e-6 || math.Abs(v) > 1e-6 {
				t.Errorf("t=10: d/v = %g/%g, want 0/0", d, v)
			}
		})
	}
	// No stiffness: damping alone slows the velocity, nothing pulls back.
	d, v := springStep(0, 2, 0, 1, 1, 1)
	if math.Abs(v-2*math.Exp(-1)) > 1e-9 || math.Abs(d-2*(1-math.Exp(-1))) > 1e-9 {
		t.Errorf("k=0: d/v = %f/%f", d, v)
	}
	d, v = springStep(1, 2, 0, 0, 1, 1)
	if d != 3 || v != 2 {
		t.Errorf("k=0, c=0: d/v = %f/%f, want 3/2", d, v)
	}
}
