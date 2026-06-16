package gui

import "time"

const animationCycle = 16 * time.Millisecond

// animViewBoundStale is the heartbeat threshold for view-bound animations.
// An animation not touched for this duration is cancelled automatically.
const animViewBoundStale = 2 * int64(time.Second)

// AnimationAdd registers a new animation. If an animation with the
// same ID exists, it is replaced.
func (w *Window) AnimationAdd(a Animation) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	w.animationAddLocked(a)
}

// animationAddLocked is the lock-free core of AnimationAdd. Callers
// must already hold w.animMu (e.g. setIDFocusLocked).
func (w *Window) animationAddLocked(a Animation) {
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
	w.animViewBound[a.ID()] = w.Now().UnixNano()
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
		w.animViewBound[id] = w.Now().UnixNano()
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

		refreshKind := AnimationRefreshNone
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
		now := w.Now().UnixNano()
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
	if time.Since(a.start) > a.Delay {
		ac.appendAnimate(a.Callback, a)
		if a.Repeat {
			// Zero delay with repeat fires every tick (~16ms).
			a.start = a.start.Add(a.Delay)
		} else {
			a.stopped = true
		}
		return true
	}
	return false
}

func updateBlinkCursor(b *BlinkCursorAnimation, w *Window) bool {
	if b.stopped {
		return false
	}
	if time.Since(b.start) > blinkCursorAnimationDelay {
		// Store(!Load()) is safe because all writers hold animMu:
		// this (via animation goroutine) and resetBlinkCursorVisible
		// (via main thread). If animMu is ever removed from either
		// path, switch to CompareAndSwap.
		w.viewState.inputCursorOn.Store(!w.viewState.inputCursorOn.Load())
		b.start = b.start.Add(blinkCursorAnimationDelay)
		return true
	}
	return false
}
