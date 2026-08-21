package gui

// window_deferred.go — running app callbacks outside the frame lock.
//
// layoutArrange and buildRenderers run under w.mu (window_update.go),
// and both reach code that fires app callbacks: an AmendLayout hook
// detecting blur, a render pass reporting a caret rect. w.mu is a
// plain sync.Mutex, so a callback that calls back into SetFocus,
// UpdateView or any other w.mu-taking API deadlocks the main thread —
// a frozen window with no panic and no CPU burn (issue #394).
//
// The invariant this file restores: no app callback runs while the
// frame lock is held. Library code raises the callback with
// deferCallback and the frame pass runs it afterwards, with nothing
// held. Anything that must touch the live *Layout stays inline — the
// tree is rebuilt from pooled arenas every frame, so a captured
// *Layout dangles by the time the deferred call runs.

// deferCallback queues fn to run after the current frame pass releases
// w.mu. Nil-safe in both directions: a nil fn is dropped, and a call
// from outside a frame pass still works — flushDeferredCallbacks is
// what drains it.
//
// Main-thread only. The frame pass is the sole producer, and it runs
// on the goroutine that owns w.mu, so the append needs no lock. Use
// QueueCommand from any other goroutine.
func (w *Window) deferCallback(fn func(*Window)) {
	if fn == nil {
		return
	}
	w.deferredCallbacks = append(w.deferredCallbacks, fn)
}

// flushDeferredCallbacks runs everything deferCallback collected during
// the frame pass and reports whether any ran. Called with no lock held,
// so a callback is free to call SetFocus, UpdateWindow or any other
// window API.
//
// The bool is the "the frame is stale" signal: a deferred callback runs
// after the renderers were built, so anything it changes (state, focus)
// is absent from this frame's output. FrameFn re-runs the pass when the
// flush reports true — see the two-pass loop in window_update.go.
//
// The slice is swapped out before the loop so a callback that defers
// another callback (an OnBlur that moves focus into a second field,
// which blurs a third) lands in the next batch rather than mutating the
// slice being ranged. The batch loop is bounded: a pair of callbacks
// that re-raise each other would otherwise spin the frame forever.
func (w *Window) flushDeferredCallbacks() bool {
	ran := false
	for batch := 0; len(w.deferredCallbacks) > 0; batch++ {
		if batch >= maxDeferredCallbackBatches {
			w.debugWarn(debugCheckDeferredLoop, "flush",
				"deferred callbacks still re-queueing after %d rounds; "+
					"dropping %d — an app callback is cycling",
				maxDeferredCallbackBatches, len(w.deferredCallbacks))
			w.deferredCallbacks = w.deferredCallbacks[:0]
			// Earlier batches already ran, so the frame is stale.
			return true
		}
		pending := w.deferredCallbacks
		w.deferredCallbacks = nil
		for _, fn := range pending {
			fn(w)
			ran = true
		}
		// Reuse the batch's backing array when nothing re-queued, so a
		// steady stream of blur commits does not allocate per frame.
		if w.deferredCallbacks == nil {
			w.deferredCallbacks = pending[:0]
		}
	}
	return ran
}

// maxDeferredCallbackBatches bounds flushDeferredCallbacks. Two rounds
// cover the legitimate case (a callback that moves focus, blurring a
// second widget); beyond that the app is cycling.
const maxDeferredCallbackBatches = 8

// lockForAPI acquires w.mu for an exported window API, naming the
// caller if the acquisition would deadlock.
//
// The probe is TryLock, the same technique PumpFrame uses: on success
// nothing was held and the caller owns the lock. On failure the lock is
// held by someone, and if that someone is a frame pass the overwhelmingly
// likely explanation is this very goroutine — an app callback reached
// from an AmendLayout hook or the render pass calling back into the
// window. Blocking there never returns, so panic with the API's name
// instead. A crash that says what to fix beats a freeze that says
// nothing.
//
// Calling these APIs from a goroutine other than the frame thread is
// already outside the contract (QueueCommand is the documented route),
// so the message covers both readings rather than trying to tell them
// apart — which TryLock cannot do.
func (w *Window) lockForAPI(api string) {
	if w.mu.TryLock() {
		return
	}
	if w.inFramePass.Load() {
		panic("gui: " + api + " called while the frame lock is held. " +
			"An AmendLayout hook, or an app callback reached from one " +
			"(OnTextCommit with InputCommitBlur, OnBlur, OnTextChanged), " +
			"must not call window APIs directly — use " +
			"ctx.Window.QueueCommand. If this call is from another " +
			"goroutine, use QueueCommand there too.")
	}
	w.mu.Lock()
}
