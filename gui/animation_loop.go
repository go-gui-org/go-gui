package gui

import "time"

const animationCycle = 16 * time.Millisecond

// animViewBoundStale is the heartbeat threshold for view-bound animations.
// An animation not touched for this duration is cancelled automatically.
const animViewBoundStale = 2 * int64(time.Second)

// viewBoundNow is the clock for view-bound heartbeats. It is deliberately
// time.Now() and not w.Now(): w.Now() follows the time-travel scrub pin,
// which jumps backwards on restore and forwards again on resume. A
// heartbeat stamped while the clock was pinned to a past instant, then
// compared against live time after the resume, looks arbitrarily stale —
// so every visible widget's animation would be cancelled on the first tick
// after a scrub ends. The heartbeat measures liveness and is never shown to
// a user, so it has no reason to be scrubbable.
func viewBoundNow() int64 { return time.Now().UnixNano() }

// AnimationAdd registers a new animation. If an animation with the
// same ID exists, it is replaced.
//
// The ID must be non-empty: animations are keyed by ID, so every
// unnamed animation would collide on "" and replace the previous one.
// Empty IDs panic, matching State[T]'s treatment of programmer error.
//
// A HeroTransition is snapshotted here, the way AnimateLayout captures
// before its caller changes the view: the "before" geometry only exists
// until the next arrange, and the documented call sequence is
// AnimationAdd followed by UpdateView. Re-adding a transition mid-flight
// replaces it outright (the ID is fixed), so the new morph starts from
// wherever the tree is at that moment.
func (w *Window) AnimationAdd(a Animation) {
	// Capture before taking animMu: the walk is O(tree) and would
	// otherwise stall the animation goroutine for its duration.
	if ht, ok := a.(*HeroTransition); ok && ht.outgoing == nil {
		ht.outgoing = captureHeroSnapshots(&w.layout)
	}
	w.animMu.Lock()
	defer w.animMu.Unlock()
	w.animationAddLocked(a)
}

// animationAddLocked is the lock-free core of AnimationAdd. Callers
// must already hold w.animMu (e.g. syncBlinkCursor).
func (w *Window) animationAddLocked(a Animation) {
	if a.ID() == "" {
		panic("gui: AnimationAdd requires a non-empty ID")
	}
	a.SetStart(time.Now())
	if w.animations == nil {
		w.animations = make(map[string]Animation)
	}
	wasEmpty := len(w.animations) == 0
	w.animations[a.ID()] = a
	if wasEmpty {
		w.ensureAnimationLoop()
		w.animationResume()
	}
}

// ensureAnimationLoop starts the animation goroutine on first use.
// No-op for windows without lifecycle channels (unit-test stubs).
func (w *Window) ensureAnimationLoop() {
	if w.animationStop == nil {
		return
	}
	w.animationStartOnce.Do(func() {
		w.animationStarted = true
		go w.animationLoop()
	})
}

// animationResume signals the animation loop to restart its
// ticker. Safe to call when already running (buffered channel).
func (w *Window) animationResume() {
	select {
	case w.animationResumeCh <- struct{}{}:
	default:
	}
}

// animationAddViewBound registers an animation and marks it as view-bound.
// View-bound animations auto-cancel when their widget leaves the view tree.
// Called from View functions; acquires w.animMu internally.
func (w *Window) animationAddViewBound(a Animation) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	w.animationAddLocked(a)
	if w.animViewBound == nil {
		w.animViewBound = make(map[string]int64)
	}
	w.animViewBound[a.ID()] = viewBoundNow()
}

// touchViewBoundAnimation updates the heartbeat for a view-bound animation
// and reports whether the animation exists. Called each frame the widget is
// visible. Called from View functions; acquires w.animMu internally.
func (w *Window) touchViewBoundAnimation(id string) bool {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if !w.hasAnimationLocked(id) {
		return false
	}
	if _, ok := w.animViewBound[id]; ok {
		w.animViewBound[id] = viewBoundNow()
	}
	return true
}

// AnimationRemove stops and removes an animation by ID.
func (w *Window) AnimationRemove(id string) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	delete(w.animations, id)
	delete(w.animViewBound, id)
}

func (w *Window) hasAnimationLocked(id string) bool {
	_, ok := w.animations[id]
	return ok
}

// HasAnimation returns true if an animation with the given ID is
// currently active.
func (w *Window) HasAnimation(id string) bool {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	return w.hasAnimationLocked(id)
}

// animationLoop runs in a goroutine, updating all animations each
// tick and dispatching deferred callbacks via the command queue.
// The ticker starts paused and resumes when animationAdd signals
// via animationResumeCh. It pauses again when all animations stop.
func (w *Window) animationLoop() {
	if w.animationDone != nil {
		defer close(w.animationDone)
	}

	dt := float32(animationCycle) / float32(time.Second)
	deferred := make([]queuedCommand, 0, 8)
	stoppedIDs := make([]string, 0, 4)

	var ticker *time.Ticker
	var tickCh <-chan time.Time

	for {
		select {
		case <-tickCh:
		case <-w.animationResumeCh:
			if ticker == nil {
				ticker = time.NewTicker(animationCycle)
				tickCh = ticker.C
			}
			continue
		case <-w.animationStop:
			if ticker != nil {
				ticker.Stop()
			}
			return
		}

		refreshKind := animationRefreshNone
		deferred = deferred[:0]
		stoppedIDs = stoppedIDs[:0]

		w.animMu.Lock()
		ac := newAnimationCommands(&deferred)
		for _, a := range w.animations {
			updated := a.Update(w, dt, &ac)
			if updated {
				refreshKind = maxAnimationRefreshKind(
					refreshKind, a.RefreshKind())
			}
			if a.IsStopped() {
				stoppedIDs = append(stoppedIDs, a.ID())
			}
		}
		// Auto-cancel view-bound animations whose widget left the view tree.
		now := viewBoundNow()
		for id, seen := range w.animViewBound {
			if now-seen > animViewBoundStale {
				stoppedIDs = append(stoppedIDs, id)
			}
		}
		for _, id := range stoppedIDs {
			delete(w.animations, id)
			delete(w.animViewBound, id)
		}
		idle := len(w.animations) == 0
		w.animMu.Unlock()

		if idle && ticker != nil {
			ticker.Stop()
			ticker = nil
			tickCh = nil
		}

		switch refreshKind {
		case AnimationRefreshRenderOnly:
			deferred = append(deferred, queuedCommand{
				kind:     queuedCommandWindowFn,
				windowFn: commandMarkRenderOnlyRefresh,
			})
		case AnimationRefreshLayout:
			deferred = append(deferred, queuedCommand{
				kind:     queuedCommandWindowFn,
				windowFn: commandMarkLayoutRefresh,
			})
		}
		w.queueCommandsBatch(deferred)
		if len(deferred) > 0 {
			w.wakeMain()
		}
	}
}

// wakeMain calls the backend's wake function to unblock the
// main event loop from WaitEventTimeout. Nil-safe.
func (w *Window) wakeMain() {
	if fn := w.wakeMainFn; fn != nil {
		fn()
	}
}

func (w *Window) stopAnimationLoop() {
	if w.animationStop == nil || !w.animationStarted {
		return
	}
	w.animationStopOnce.Do(func() {
		close(w.animationStop)
		if w.animationDone != nil {
			<-w.animationDone
		}
	})
}

func updateAnimate(a *Animate, ac *AnimationCommands) bool {
	if a.stopped {
		return false
	}
	if a.Callback == nil {
		a.stopped = true
		return false
	}
	now := time.Now()
	if now.Sub(a.start) <= a.Delay {
		return false
	}
	ac.appendAnimate(a.Callback, a)
	if !a.Repeat {
		a.stopped = true
		return true
	}
	// Zero delay with repeat fires every tick (~16ms). Advancing by
	// exactly one Delay keeps a longer cadence drift-free while ticks
	// arrive on time.
	a.start = a.start.Add(a.Delay)
	a.start = resyncAfterStall(a.start, now, a.Delay)
	return true
}

// resyncAfterStall drops the backlog a stalled animation accumulated.
// A minimized window, a debugger break or one very long frame leaves
// start many delays in the past; stepping one Delay per tick would then
// drain that backlog at one callback every ~16ms — a burst of catch-up
// fires long after the interval each belonged to. Once start is more
// than one further delay behind, give up the missed intervals and
// resync to now. Returns start unchanged on a tick that arrived on
// time, so the drift-free cadence survives.
func resyncAfterStall(start, now time.Time, delay time.Duration) time.Time {
	if now.Sub(start) > delay {
		return now
	}
	return start
}

func updateBlinkCursor(b *BlinkCursorAnimation, w *Window, ac *AnimationCommands) bool {
	if b.stopped {
		return false
	}
	now := time.Now()
	if now.Sub(b.start) > blinkCursorAnimationDelay {
		// Store(!Load()) is safe because all writers hold animMu:
		// this (via animation goroutine) and resetBlinkCursorVisible
		// (via main thread). If animMu is ever removed from either
		// path, switch to CompareAndSwap.
		w.viewState.inputCursorOn.Store(!w.viewState.inputCursorOn.Load())
		// Toggle the caret renderer on the main thread during the
		// next frame's command flush — it lives in the render list
		// and needs no tree rebuild (issue #404). Pulsar's own
		// Animate still promotes the tick to a layout refresh.
		ac.appendOnDone(commandToggleCaretBlink)
		b.start = b.start.Add(blinkCursorAnimationDelay)
		// Without this the caret strobes after any stall, catching up
		// one toggle per tick until it reaches the present.
		b.start = resyncAfterStall(b.start, now, blinkCursorAnimationDelay)
		return true
	}
	return false
}
