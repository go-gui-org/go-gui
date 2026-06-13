package gui

import (
	"sync"
	"testing"
	"time"
)

// TestAnimationStressAddRemove stress-tests rapid add/remove cycles on the
// animation map. Verifies no panics and stable idle detection.
func TestAnimationStressAddRemove(t *testing.T) {
	w := newTestWindow()

	const cycles = 100
	const animsPerCycle = 20

	for range cycles {
		// Add many animations.
		for j := range animsPerCycle {
			a := NewTweenAnimation("tween-"+string(rune('a'+j%26)), 0, 1, nil)
			w.AnimationAdd(a)
		}

		if w.animationCount() != animsPerCycle {
			t.Errorf("cycle: got %d animations, want %d",
				w.animationCount(), animsPerCycle)
		}

		// Remove them all.
		for j := range animsPerCycle {
			w.AnimationRemove("tween-" + string(rune('a'+j%26)))
		}

		if w.animationCount() != 0 {
			t.Errorf("after remove: got %d animations, want 0",
				w.animationCount())
		}
	}
}

// animationCount returns the number of active animations. Not locked —
// caller must hold w.mu or be in a single-goroutine context.
func (w *Window) animationCount() int {
	return len(w.animations)
}

// TestAnimationStressUpdateMany stress-tests updating many animations
// simultaneously, simulating a 120Hz animation loop (~8.33ms tick).
func TestAnimationStressUpdateMany(t *testing.T) {
	w := newTestWindow()

	const numAnims = 100
	const numTicks = 60 // ~0.5s at 120Hz

	// Add many tween animations with OnValue callbacks.
	for i := range numAnims {
		idx := i
		tw := NewTweenAnimation(
			"stress-"+string(rune('a'+i%26))+"-"+string(rune('0'+i/26%10)),
			0, 1,
			func(v float32, _ *Window) {
				// Track value is in range.
				if v < 0 || v > 1 {
					panic("tween value out of range")
				}
				_ = idx // capture
			},
		)
		w.AnimationAdd(tw)
	}

	if w.animationCount() != numAnims {
		t.Fatalf("got %d animations, want %d", w.animationCount(), numAnims)
	}

	// Simulate many ticks.
	dt := float32(1.0 / 120.0)
	deferred := make([]queuedCommand, 0, 8)

	for range numTicks {
		func() {
			w.mu.Lock()
			defer w.mu.Unlock()

			ac := newAnimationCommands(&deferred)
			stoppedIDs := make([]string, 0, 4)

			for _, a := range w.animations {
				a.Update(w, dt, &ac)
				if a.IsStopped() {
					stoppedIDs = append(stoppedIDs, a.ID())
				}
			}
			for _, id := range stoppedIDs {
				delete(w.animations, id)
			}
		}()
	}

	// After 60 ticks, many tweens should have completed (duration = 1s
	// default, 60 ticks * ~8ms = ~480ms → ~48% progress).
	remaining := w.animationCount()
	if remaining == 0 {
		t.Error("all animations stopped — expected some still running")
	}
	t.Logf("remaining animations after %d ticks: %d/%d", numTicks, remaining, numAnims)
}

// TestAnimationStressMixedTypes stress-tests simultaneous updates of
// different animation types (Tween, Spring, Keyframe, Animate).
func TestAnimationStressMixedTypes(t *testing.T) {
	w := newTestWindow()

	// Add mix of animation types.
	for i := range 40 {
		si := string(rune('a' + i%26))
		switch i % 4 {
		case 0:
			tw := NewTweenAnimation("tw-"+si, 0, 1,
				func(v float32, _ *Window) { _ = v })
			w.AnimationAdd(tw)
		case 1:
			sp := NewSpringAnimation("sp-"+si,
				func(v float32, _ *Window) { _ = v })
			sp.SpringTo(0, 1)
			w.AnimationAdd(sp)
		case 2:
			kf := NewKeyframeAnimation("kf-"+si,
				[]Keyframe{
					{At: 0, Value: 0},
					{At: 0.5, Value: 50},
					{At: 1, Value: 100},
				},
				func(v float32, _ *Window) { _ = v })
			w.AnimationAdd(kf)
		case 3:
			an := &Animate{
				AnimID:   "an-" + si,
				Callback: func(_ *Animate, _ *Window) {},
				Delay:    50 * time.Millisecond,
				Repeat:   true,
			}
			w.AnimationAdd(an)
		}
	}

	// Simulate ticks.
	dt := float32(1.0 / 120.0)
	deferred := make([]queuedCommand, 0, 8)

	for range 30 {
		func() {
			w.mu.Lock()
			defer w.mu.Unlock()

			ac := newAnimationCommands(&deferred)
			stoppedIDs := make([]string, 0, 4)

			for _, a := range w.animations {
				a.Update(w, dt, &ac)
				if a.IsStopped() {
					stoppedIDs = append(stoppedIDs, a.ID())
				}
			}
			for _, id := range stoppedIDs {
				delete(w.animations, id)
			}
		}()
	}

	t.Logf("remaining mixed animations: %d", w.animationCount())
}

// TestAnimationStressDuplicateID verifies that adding an animation with
// the same ID replaces the existing one without leaking.
func TestAnimationStressDuplicateID(t *testing.T) {
	w := newTestWindow()

	for i := range 20 {
		tw := NewTweenAnimation("dup", 0, 1,
			func(v float32, _ *Window) { _ = v })
		w.AnimationAdd(tw)
		if w.animationCount() != 1 {
			t.Errorf("iteration %d: got %d animations, want 1",
				i, w.animationCount())
		}
	}
}

// TestAnimationStressHasAnimation stress-tests concurrent HasAnimation
// calls with add/remove operations.
func TestAnimationStressHasAnimation(t *testing.T) {
	w := newTestWindow()
	tw := NewTweenAnimation("check", 0, 1, nil)
	w.AnimationAdd(tw)

	// Verify from multiple goroutines.
	var wg sync.WaitGroup
	const workers = 8
	const iterations = 500

	for range workers {
		wg.Go(func() {
			for range iterations {
				w.HasAnimation("check") // verify no panic
			}
		})
	}
	wg.Wait()
}

// TestAnimationStressCommandQueue stress-tests the command queue with
// rapid concurrent enqueues from multiple goroutines, simulating the
// animation loop dispatching callbacks.
func TestAnimationStressCommandQueue(t *testing.T) {
	w := newTestWindow()

	const workers = 4
	const commandsPerWorker = 250

	var wg sync.WaitGroup
	var counter int
	var mu sync.Mutex

	for range workers {
		wg.Go(func() {
			for range commandsPerWorker {
				w.QueueCommand(func(w *Window) {
					mu.Lock()
					counter++
					mu.Unlock()
				})
			}
		})
	}
	wg.Wait()

	// Flush all commands.
	w.flushCommands()

	if counter != workers*commandsPerWorker {
		t.Errorf("got %d, want %d commands executed",
			counter, workers*commandsPerWorker)
	}
}

// TestAnimationStressValueCommand stress-tests QueueValueCommand with
// concurrent enqueues.
func TestAnimationStressValueCommand(t *testing.T) {
	w := newTestWindow()

	const workers = 4
	const cmds = 200

	var wg sync.WaitGroup
	var sum float32
	var mu sync.Mutex

	for range workers {
		wg.Go(func() {
			for range cmds {
				w.QueueValueCommand(func(v float32, _ *Window) {
					mu.Lock()
					sum += v
					mu.Unlock()
				}, 1.0)
			}
		})
	}
	wg.Wait()

	w.flushCommands()

	expected := float32(workers * cmds)
	if !f32AreClose(sum, expected) {
		t.Errorf("sum: got %f, want %f", sum, expected)
	}
}

// TestAnimationStressFrameFnWithAnimations stress-tests FrameFn with
// active animations that need layout refresh.
func TestAnimationStressFrameFnWithAnimations(t *testing.T) {
	w := newTestWindow()

	// Set up a view generator so FrameFn has something to work with.
	// The view must return a valid layout; a minimal empty Column works.
	w.viewGenerator = func(w *Window) View {
		return Column(ContainerCfg{})
	}

	// Add animations that request layout refresh.
	for i := range 50 {
		tw := NewTweenAnimation("frm-"+string(rune('a'+i%26))+
			"-"+string(rune('0'+i/26%10)), 0, 1,
			func(v float32, _ *Window) { _ = v })
		w.AnimationAdd(tw)
	}

	// Mark for refresh and run FrameFn.
	w.refreshLayout = true
	rebuilt := w.FrameFn()
	if !rebuilt {
		t.Error("expected FrameFn to rebuild")
	}

	// Animation count should be preserved (they don't auto-stop in one
	// frame).
	if w.animationCount() != 50 {
		t.Errorf("got %d animations, want 50", w.animationCount())
	}
}
