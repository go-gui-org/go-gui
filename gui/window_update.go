package gui

import "time"

// FrameTimings holds per-frame pipeline stage durations.
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
func (w *Window) QueueValueCommand(cb func(float32, *Window), value float32) {
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
func (w *Window) QueueAnimateCommand(cb func(*Animate, *Window), a *Animate) {
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
	// Swap to avoid holding lock during execution.
	toRun := w.commands
	w.commands = w.commandScratch[:0]
	w.commandScratch = toRun[:0]
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
}

// markLayoutRefresh requests a full layout rebuild next frame.
// Overrides any pending render-only refresh.
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
func (w *Window) UpdateWindow() {
	w.markLayoutRefresh()
}

// RequestRenderOnly marks the window for render-only refresh.
func (w *Window) RequestRenderOnly() {
	w.markRenderOnlyRefresh()
}

// RequestRedraw is an alias for RequestRenderOnly. Safe to call
// from OnHover/OnMouseLeave callbacks.
func (w *Window) RequestRedraw() {
	w.markRenderOnlyRefresh()
}

// UpdateView sets the view generator and triggers a full refresh.
func (w *Window) UpdateView(gen func(*Window) View) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.viewState.registry.Clear()
	w.viewGenerator = gen
	w.markLayoutRefresh()
}

// FrameFn is called by the backend each frame. It flushes
// queued commands and rebuilds layout/renderers as needed.
// Returns true when renderers were rebuilt and the backend
// should call renderFrame.
func (w *Window) FrameFn() bool {
	w.frameCount++
	w.flushCommands()
	var rebuilt bool
	if w.refreshLayout {
		w.Update()
		rebuilt = true
	} else if w.refreshRenderOnly {
		w.UpdateRenderOnly()
		rebuilt = true
	}
	w.initA11y()
	w.syncA11y()
	return rebuilt
}

// Update performs a full layout rebuild and re-renders.
func (w *Window) Update() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.refreshLayout = false
	w.refreshRenderOnly = false

	if w.viewGenerator == nil {
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
	view := w.viewGenerator(w)
	rootLayout := generateViewLayout(view, w)
	if t {
		t1 = time.Now()
	}

	layers := layoutArrange(&rootLayout, w)
	if t {
		t2 = time.Now()
	}

	w.layout = composeLayout(layers, w)
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

// UpdateRenderOnly rebuilds renderers from the existing layout.
func (w *Window) UpdateRenderOnly() {
	w.mu.Lock()
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
	w.scratch.resetRenderPools()
	renderLayout(&w.layout, bgColor, clip, w)
	if inspectorSupported && w.inspectorEnabled {
		inspectorInjectWireframe(w)
	}
}
