package gui

import "time"

// FrameTimings holds per-frame pipeline stage durations.
// exportaudit:keep — collides with the window's frameTimings state field
type FrameTimings struct {
	ViewGen       time.Duration
	LayoutArrange time.Duration
	RenderBuild   time.Duration
}

// QueueCommand adds a command to the window's atomic command queue.
// Commands execute on the main thread during the next frame update.
// Preferred way to update UI state from other threads.
func (w *Window) QueueCommand(cb func(*Window)) {
	if cb == nil {
		return
	}
	w.queueCommand(queuedCommand{
		kind:     queuedCommandWindowFn,
		windowFn: cb,
	})
	w.wakeMain()
}

// QueueValueCommand queues a value callback for execution on the main thread.
func (w *Window) queueValueCommand(cb func(float32, *Window), value float32) {
	if cb == nil {
		return
	}
	w.queueCommand(queuedCommand{
		kind:    queuedCommandValueFn,
		valueFn: cb,
		value:   value,
	})
	w.wakeMain()
}

// QueueAnimateCommand queues an Animate callback for execution on the main thread.
func (w *Window) queueAnimateCommand(cb func(*Animate, *Window), a *Animate) {
	if cb == nil {
		return
	}
	w.queueCommand(queuedCommand{
		kind:      queuedCommandAnimateFn,
		animateFn: cb,
		animate:   a,
	})
	w.wakeMain()
}

func (w *Window) queueCommand(cmd queuedCommand) {
	w.commandsMu.Lock()
	w.reclaimCommandScratch()
	w.commands = append(w.commands, cmd)
	w.commandsMu.Unlock()
}

func (w *Window) queueCommandsBatch(cmds []queuedCommand) {
	if len(cmds) == 0 {
		return
	}
	w.commandsMu.Lock()
	w.reclaimCommandScratch()
	w.commands = append(w.commands, cmds...)
	w.commandsMu.Unlock()
}

// reclaimCommandScratch reclaims the scratch buffer when the main
// command slice is nil. Caller must hold commandsMu.
func (w *Window) reclaimCommandScratch() {
	if w.commands == nil && cap(w.commandScratch) > 0 {
		w.commands = w.commandScratch[:0]
		w.commandScratch = nil
	}
}

// flushCommands executes all pending commands. Called by the main
// loop at frame start.
func (w *Window) flushCommands() {
	w.commandsMu.Lock()
	if len(w.commands) == 0 {
		w.commandsMu.Unlock()
		return
	}
	// Swap to avoid holding lock during execution. toRun's backing
	// array must NOT be recycled into commandScratch yet: writers
	// reclaim the scratch buffer (reclaimCommandScratch) and would
	// append into the array while the loop below still reads it.
	toRun := w.commands
	w.commands = w.commandScratch[:0]
	w.commandScratch = nil
	w.commandsMu.Unlock()

	for i := range toRun {
		cmd := toRun[i]
		switch cmd.kind {
		case queuedCommandWindowFn:
			cmd.windowFn(w)
		case queuedCommandValueFn:
			cmd.valueFn(cmd.value, w)
		case queuedCommandAnimateFn:
			cmd.animateFn(cmd.animate, w)
		}
	}

	// Iteration done — toRun's backing is safe to hand out for
	// reuse now (unless a nested flush already installed one).
	w.commandsMu.Lock()
	if w.commandScratch == nil {
		w.commandScratch = toRun[:0]
	}
	w.commandsMu.Unlock()
}

// markLayoutRefresh requests a full layout rebuild next frame.
// Overrides any pending render-only refresh.
//
// Setting the flag is not the whole request: a public entry point must also
// call wakeMain, because backends block indefinitely when FrameFn reports
// nothing to draw and a flag alone is only read the next time something else
// wakes the loop. Frame-thread sites (svg, command queue, testing hooks)
// correctly do not wake — the loop is already running.
func (w *Window) markLayoutRefresh() {
	w.refreshLayout = true
	w.refreshRenderOnly = false
}

// markRenderOnlyRefresh requests a renderer-only rebuild from the
// existing layout tree. No-op if a full layout refresh is pending.
func (w *Window) markRenderOnlyRefresh() {
	if !w.refreshLayout {
		w.refreshRenderOnly = true
	}
}

// UpdateWindow marks the window as needing a full layout update.
//
// Safe to call from any goroutine: it wakes the backend's idle loop, so a
// refresh requested off the frame thread is painted promptly rather than
// whenever the next input event happens to arrive. Callers mutating window
// state alongside it still need the window lock; this only schedules the
// frame.
func (w *Window) UpdateWindow() {
	w.markLayoutRefresh()
	w.wakeMain()
}

// requestRenderOnly marks the window for render-only refresh.
func (w *Window) requestRenderOnly() {
	w.markRenderOnlyRefresh()
}

// RequestRedraw is an alias for RequestRenderOnly. Safe to call
// from OnHover/OnMouseLeave callbacks, and from any goroutine — like
// UpdateWindow it wakes the backend's idle loop.
func (w *Window) RequestRedraw() {
	w.markRenderOnlyRefresh()
	w.wakeMain()
}

// UpdateView sets the view generator and triggers a full refresh.
func (w *Window) UpdateView(gen func(*Window) View) {
	w.lockForAPI("UpdateView")
	defer w.mu.Unlock()
	w.viewState.registry.Clear()
	w.viewGenerator = gen
	w.markLayoutRefresh()
	// Under w.mu, which is deliberate: a wake only posts to the platform's
	// event queue and takes no window lock, so it cannot re-enter.
	w.wakeMain()
}

// FrameFn is called by the backend each frame. It flushes
// queued commands and rebuilds layout/renderers as needed.
// Returns true when the renderers changed — rebuilt, or patched in
// place by the caret blink — and the backend should call renderFrame.
func (w *Window) FrameFn() bool {
	w.frameCount++
	// Before anything reads theme state this frame: make this window's
	// theme the installed one. Also covers the backend's clear-color
	// read, which happens right after FrameFn on the same thread.
	w.installTheme()
	w.flushCommands()
	var rebuilt bool
	// Two passes, not one: Update ends by running the callbacks the
	// frame pass deferred (window_deferred.go). A blur commit that
	// writes state or moves focus changes what the frame must show,
	// and neither marks a refresh flag, so the flush's own report is
	// what re-arms the loop — the pass re-runs whenever any callback
	// ran. A callback-driven re-run is a full Update, never a
	// render-only pass: the callback may have changed state the layout
	// tree encodes. ran doubles as the re-run's trigger: it holds the
	// previous pass's flush report until the pass overwrites it.
	// Bounded at two — a view that dirties itself every pass is a bug
	// the frame loop must not amplify into a spin; the next FrameFn
	// picks it up anyway.
	ran := false
	for pass := 0; pass < 2 &&
		(ran || w.refreshLayout || w.refreshRenderOnly); pass++ {
		if w.refreshLayout || ran {
			ran = w.Update()
		} else {
			ran = w.updateRenderOnly()
		}
		rebuilt = true
	}
	// A blink tick can patch the caret renderer in place (issue
	// #404); the list changed without a rebuild, but the backend
	// must still present. A pass that ran subsumes the patch.
	rebuilt = rebuilt || w.renderersDirty
	w.renderersDirty = false
	w.initA11y()
	w.syncA11y()
	return rebuilt
}

// PumpFrame runs one frame from a *nested* platform runloop — a modal
// dialog, menu tracking, live resize — where the backend's own event loop
// is blocked and would otherwise deliver no frames at all. Same work as
// FrameFn (flush queued commands, rebuild layout/renderers) and the same
// return value: true when the backend should follow with renderFrame.
//
// Two guards make it safe to call from that re-entrant position:
//
//   - w.pumping rejects a pump nested inside another pump.
//   - The TryLock probe rejects the frame when some caller further up the
//     stack already owns w.mu — e.g. an event handler that opened the
//     modal dialog from inside Update. Blocking there would deadlock the
//     main thread, and skipping costs nothing: the next timer tick retries.
//
// Main-thread only, like FrameFn.
func (w *Window) PumpFrame() bool {
	if w.pumping {
		return false
	}
	// Probe only — the lock is released immediately. On the main thread the
	// sole possible holder is our own call stack, so a successful TryLock
	// means FrameFn's own w.mu.Lock cannot deadlock.
	if !w.mu.TryLock() {
		return false
	}
	w.mu.Unlock()

	w.pumping = true
	defer func() { w.pumping = false }()
	return w.FrameFn()
}

// Update performs a full layout rebuild and re-renders, then runs the
// app callbacks the pass deferred. It reports whether any deferred
// callback ran.
//
// The split is load-bearing, not cosmetic: updateLocked holds w.mu
// across layoutArrange and buildRenderers, and both reach app code.
// Running that code inside the lock deadlocks the main thread the
// moment it calls back into the window (issue #394), so library code
// raises it with deferCallback and it runs here, with nothing held.
// See window_deferred.go.
//
// The bool matters because those callbacks run after the renderers were
// built: a callback that writes state or moves focus is absent from
// this pass's output, so the caller must re-run (FrameFn re-checks
// after each pass) or the frame stays stale until the next event.
func (w *Window) Update() bool {
	w.updateLocked()
	return w.flushDeferredCallbacks()
}

func (w *Window) updateLocked() {
	// Repeated from FrameFn because tests drive Update directly.
	// Idempotent: a no-op when this window's theme is already installed.
	w.installTheme()
	w.mu.Lock()
	w.inFramePass.Store(true)
	defer w.inFramePass.Store(false)
	w.refreshLayout = false
	w.refreshRenderOnly = false
	// Every full layout rebuild may have changed a11y-visible state
	// (labels, geometry, roles, live values). Mark the tree dirty so
	// syncA11y pushes once the throttle allows (issue #407).
	w.a11y.dirty = true

	if w.viewGenerator == nil {
		w.mu.Unlock()
		return
	}

	if inspectorSupported && w.inspectorEnabled {
		if w.inspectorPropsCache == nil {
			w.inspectorPropsCache = make(map[string]inspectorNodeProps)
		} else {
			clear(w.inspectorPropsCache)
		}
		selected := inspectorSelectedPath(w)
		w.inspectorTreeCache = inspectorBuildTreeNodes(
			w, &w.layout, selected, w.inspectorPropsCache)
	}

	if len(w.layout.Children) > 0 {
		w.scratch.layerLayouts.put(w.layout.Children)
	}

	t := w.Config.Timings
	var t0, t1, t2 time.Time
	if t {
		t0 = time.Now()
	}

	w.scratch.resetViewPools()

	// Release w.mu during View generation so the animation goroutine
	// (which holds w.animMu, not w.mu) can tick. View functions
	// access scratch pools (frame-scoped, single-goroutine), atomic
	// inputCursorOn, and animations (guarded by w.animMu).
	w.mu.Unlock()
	view := w.viewGenerator(w)
	rootLayout := generateViewLayout(view, w)
	w.mu.Lock()
	defer w.mu.Unlock()

	// The root layout has no parent to Fill against, so a FillFill
	// root would otherwise collapse to content size. Pin each Fill
	// axis to the window dimension via min=max so the intuitive
	// spelling (Sizing: FillFill, no Width/Height) fills the window
	// and tracks resize automatically. Constraining min=max (rather
	// than seeding Width/Height) is robust to the fit pass, which
	// accumulates on the main axis, and the subsequent fill pass still
	// distributes the remaining space to any Fill children.
	ensureLayoutShape(&rootLayout)
	if rootLayout.Shape.Sizing.Width == sizingFill {
		rootLayout.Shape.MinWidth = float32(w.windowWidth)
		rootLayout.Shape.MaxWidth = float32(w.windowWidth)
	}
	if rootLayout.Shape.Sizing.Height == sizingFill {
		rootLayout.Shape.MinHeight = float32(w.windowHeight)
		rootLayout.Shape.MaxHeight = float32(w.windowHeight)
	}

	if t {
		t1 = time.Now()
	}

	layers := layoutArrange(&rootLayout, w)
	if t {
		t2 = time.Now()
	}

	w.layout = composeLayout(layers, w)
	// Dev-mode identity checks. One atomic load when the gate is off.
	w.debugAudit(&w.layout)
	w.buildRenderers(w.Config.BgColor, w.windowRect())
	if t {
		t3 := time.Now()
		w.frameTimings = FrameTimings{
			ViewGen:       t1.Sub(t0),
			LayoutArrange: t2.Sub(t1),
			RenderBuild:   t3.Sub(t2),
		}
	}
}

// updateRenderOnly rebuilds renderers from the existing layout, then
// runs whatever the render pass deferred. Reports whether any deferred
// callback ran — same contract as Update. buildRenderers reaches app
// code the same way the full pass does (syncIMEEditContext, the caret
// rect report), so it needs the same treatment — see Update.
func (w *Window) updateRenderOnly() bool {
	w.renderOnlyLocked()
	return w.flushDeferredCallbacks()
}

func (w *Window) renderOnlyLocked() {
	w.mu.Lock()
	w.inFramePass.Store(true)
	defer w.inFramePass.Store(false)
	defer w.mu.Unlock()
	w.refreshRenderOnly = false
	w.buildRenderers(w.Config.BgColor, w.windowRect())
}

// composeLayout wraps layer layouts into a single root.
func composeLayout(layers []Layout, w *Window) Layout {
	return Layout{
		Shape: w.allocShape(Shape{
			Width:  float32(w.windowWidth),
			Height: float32(w.windowHeight),
		}),
		Children: layers,
	}
}

// buildRenderers resets and rebuilds the render command list.
func (w *Window) buildRenderers(bgColor Color, clip drawClip) {
	w.renderers = w.renderers[:0]
	w.renderPass++
	// The caret's renderer index is only valid within the list just
	// built; the blink toggle re-records on every rebuild (issue #404).
	w.caretCmd = caretCmdState{}
	w.scratch.resetRenderPools()
	// The arranged tree is the only place that says whether the
	// focused widget is editable, so the platform input method is
	// switched here rather than at focus time (gui/ime_context.go).
	// The caret-blink animation is gated on the same tree — an ID
	// alone cannot say whether a widget draws a caret (issue #403).
	w.syncIMEEditContext()
	w.syncBlinkCursor()
	renderLayout(&w.layout, bgColor, clip, w)
	if inspectorSupported && w.inspectorEnabled {
		inspectorInjectWireframe(w)
	}
}
