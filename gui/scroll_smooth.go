package gui

import "time"

// Discrete mouse-wheel scrolling eases toward a target offset instead
// of jumping instantly. This is a cheap exponential lerp — not a
// spring or momentum simulation. Each tick moves the displayed offset
// a fixed fraction of the remaining distance, so it decelerates
// naturally and settles in ~100ms. Trackpad/precise scrolling already
// carries OS momentum and is NOT routed here (see scrollVertical).
const (
	scrollSmoothAnimationID = "___scroll_smooth___"
	// scrollSmoothFactor is the fraction of remaining distance moved
	// per 16ms tick (~87% closed in ~7 frames / ~112ms).
	scrollSmoothFactor = float32(0.25)
	// scrollSmoothSnap: within this many px of target, snap and stop.
	scrollSmoothSnap = float32(0.5)
)

// scrollAxis selects which scroll map an entry drives.
type scrollAxis uint8

const (
	scrollAxisY scrollAxis = iota
	scrollAxisX
)

// scrollSmoothEntry tracks one scrollable's eased offset. current is
// what layout reads (pushed to the scrollX/scrollY map each tick);
// target is where the wheel deltas want it.
type scrollSmoothEntry struct {
	id      string
	axis    scrollAxis
	target  float32
	current float32
	active  bool
	settled bool // reached target this tick; retire next tick
}

// scrollApply is a lock-free snapshot of an entry for the apply pass.
type scrollApply struct {
	id   string
	axis scrollAxis
	val  float32
}

// scrollSmoothAnimation is a per-window singleton Animation that eases
// every actively-scrolling container. Its entries are guarded by
// w.animMu: mutated by Update on the animation goroutine (which holds
// animMu) and by scrollSmoothBy/Cancel on the main goroutine.
// Displayed offsets are written only on the main goroutine, via a
// deferred commandApplyScrollSmooth, so the scrollX/scrollY maps stay
// single-goroutine.
type scrollSmoothAnimation struct {
	start   time.Time
	entries []scrollSmoothEntry
	pending []scrollApply // apply-pass scratch, main goroutine only
	stopped bool
}

// ID implements Animation.
func (s *scrollSmoothAnimation) ID() string { return scrollSmoothAnimationID }

// RefreshKind implements Animation. Scroll offset repositions children
// during layout arrange, so a full layout rebuild is required.
func (s *scrollSmoothAnimation) RefreshKind() AnimationRefreshKind {
	return AnimationRefreshLayout
}

// IsStopped implements Animation.
func (s *scrollSmoothAnimation) IsStopped() bool { return s.stopped }

// SetStart implements Animation.
func (s *scrollSmoothAnimation) SetStart(t time.Time) { s.start = t }

// Update implements Animation. Runs on the animation goroutine while
// w.animMu is held (see animationLoop).
func (s *scrollSmoothAnimation) Update(_ *Window, _ float32, ac *AnimationCommands) bool {
	anyActive := false
	for i := range s.entries {
		e := &s.entries[i]
		if !e.active {
			continue
		}
		if e.settled {
			// Final value was applied last tick; retire now.
			e.active = false
			continue
		}
		diff := e.target - e.current
		if !f32IsFinite(diff) {
			// Poisoned entry (NaN/Inf target or current): retire
			// instead of ticking forever without converging.
			e.active = false
			continue
		}
		if f32Abs(diff) < scrollSmoothSnap {
			e.current = e.target
			e.settled = true
		} else {
			e.current += diff * scrollSmoothFactor
		}
		anyActive = true
	}
	if !anyActive {
		s.stopped = true
		return false
	}
	ac.AppendOnDone(commandApplyScrollSmooth)
	return true
}

// entryFor returns the entry for id+axis, creating an inactive
// one if absent. Caller must hold w.animMu.
func (s *scrollSmoothAnimation) entryFor(id string, axis scrollAxis) *scrollSmoothEntry {
	if e := s.findEntry(id, axis); e != nil {
		return e
	}
	s.entries = append(s.entries, scrollSmoothEntry{id: id, axis: axis})
	return &s.entries[len(s.entries)-1]
}

// findEntry returns the entry for id+axis, or nil. Caller must
// hold w.animMu.
func (s *scrollSmoothAnimation) findEntry(id string, axis scrollAxis) *scrollSmoothEntry {
	for i := range s.entries {
		if s.entries[i].id == id && s.entries[i].axis == axis {
			return &s.entries[i]
		}
	}
	return nil
}

// scrollSmoothBy accumulates a discrete-wheel delta into the eased
// target for layout's scrollable and (re)arms the smoother. Mirrors
// scrollVertical/Horizontal's clamp/ScrollMultiplier math but defers
// the displayed offset to the animation. Returns true if a repaint is
// warranted. Main goroutine only.
func scrollSmoothBy(w *Window, layout *Layout, axis scrollAxis, delta float32) bool {
	id := layout.Shape.ID
	if !layout.Shape.Scrollable || id == "" {
		return false
	}
	if axis == scrollAxisY && layout.Shape.ScrollMode == ScrollHorizontalOnly {
		return false
	}
	if axis == scrollAxisX && layout.Shape.ScrollMode == ScrollVerticalOnly {
		return false
	}

	var maxOffset, displayed float32
	if axis == scrollAxisY {
		maxOffset = scrollMaxOffsetY(layout)
		displayed, _ = w.scrollY().Get(id)
	} else {
		maxOffset = scrollMaxOffsetX(layout)
		displayed, _ = w.scrollX().Get(id)
	}
	increment := delta * guiTheme.ScrollMultiplier

	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		w.scrollSmooth = &scrollSmoothAnimation{}
	}
	ss := w.scrollSmooth
	e := ss.entryFor(id, axis)

	base := displayed
	if e.active {
		base = e.target
	}
	newTarget := f32Clamp(base+increment, maxOffset, 0)
	if !f32IsFinite(newTarget) {
		// NaN delta, multiplier, or displayed offset would poison the
		// ease into a never-settling animation. Reject the event.
		return false
	}
	if newTarget == base {
		return false // already at/heading to this offset
	}
	if !e.active {
		e.current = displayed
	}
	e.target = newTarget
	e.active = true
	e.settled = false
	ss.stopped = false
	w.animationAddLocked(ss)
	return true
}

// scrollSmoothCancel deactivates any eased scroll for id+axis so
// a direct offset write (trackpad, keyboard, scrollbar drag,
// programmatic) is not overwritten by an in-flight ease. No-op if none
// active. Main goroutine only.
func scrollSmoothCancel(w *Window, id string, axis scrollAxis) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		return
	}
	if e := w.scrollSmooth.findEntry(id, axis); e != nil {
		e.active = false
		e.settled = false
	}
}

// scrollSmoothReset drops all eased-scroll state. Called when the view
// tree is rebuilt (clearHotMaps), since scroll id keys may change.
func (w *Window) scrollSmoothReset() {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth != nil {
		w.scrollSmooth.stopped = true
		w.scrollSmooth.entries = w.scrollSmooth.entries[:0]
	}
}

// commandApplyScrollSmooth writes each active entry's eased offset to
// the scroll map and fires OnScroll. Runs on the main goroutine
// (enqueued via AppendOnDone). Snapshots under animMu, then applies
// outside it so OnScroll callbacks may safely re-enter animMu.
func commandApplyScrollSmooth(w *Window) {
	ss := w.scrollSmooth
	if ss == nil {
		return
	}
	w.animMu.Lock()
	ss.pending = ss.pending[:0]
	for i := range ss.entries {
		e := &ss.entries[i]
		if !e.active {
			continue
		}
		ss.pending = append(ss.pending, scrollApply{
			id: e.id, axis: e.axis, val: e.current,
		})
	}
	w.animMu.Unlock()

	for _, p := range ss.pending {
		if p.axis == scrollAxisY {
			w.scrollY().Set(p.id, p.val)
		} else {
			w.scrollX().Set(p.id, p.val)
		}
		if ly, ok := findScrollLayout(w, p.id); ok {
			fireOnScroll(ly, w)
		}
	}
}
