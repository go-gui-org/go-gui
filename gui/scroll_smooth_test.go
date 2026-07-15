package gui

import (
	"math"
	"testing"
)

// driveScrollSmooth runs the smoother's Update/apply cycle until it
// stops or maxTicks is reached, mimicking the animation loop +
// main-goroutine flush. Returns the number of ticks executed.
func driveScrollSmooth(w *Window, maxTicks int) int {
	ss := w.scrollSmooth
	ticks := 0
	for ticks < maxTicks {
		deferred := make([]queuedCommand, 0, 2)
		ac := newAnimationCommands(&deferred)
		cont := ss.Update(w, 0.016, &ac)
		for i := range deferred {
			if deferred[i].windowFn != nil {
				deferred[i].windowFn(w)
			}
		}
		ticks++
		if !cont {
			break
		}
	}
	return ticks
}

func TestScrollSmoothTargetSetDisplayUnchangedFrame0(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("1", 100, 100, 100, 300)

	ok := scrollSmoothBy(w, layout, scrollAxisY, -50)
	if !ok {
		t.Fatal("expected scrollSmoothBy to report change")
	}

	// Displayed offset must NOT have moved yet — no apply has run.
	sy := w.scrollY()
	if v, present := sy.Get("1"); present && v != 0 {
		t.Errorf("displayed offset changed on frame 0: got %v", v)
	}

	e := w.scrollSmooth.findEntry("1", scrollAxisY)
	if e == nil || !e.active {
		t.Fatal("expected an active entry")
	}
	if e.target != -50 {
		t.Errorf("target = %v, want -50", e.target)
	}
	if e.current != 0 {
		t.Errorf("current = %v, want 0", e.current)
	}
}

func TestScrollSmoothConvergesAndStops(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("2", 100, 100, 100, 300)
	scrollSmoothBy(w, layout, scrollAxisY, -50)

	prev := float32(0)
	applied := func() float32 { v, _ := w.scrollY().Get("2"); return v }

	// Drive one tick at a time, asserting monotonic approach (never
	// overshoots past the -50 target).
	ss := w.scrollSmooth
	ticks := 0
	for ticks < 100 {
		deferred := make([]queuedCommand, 0, 2)
		ac := newAnimationCommands(&deferred)
		cont := ss.Update(w, 0.016, &ac)
		for i := range deferred {
			if deferred[i].windowFn != nil {
				deferred[i].windowFn(w)
			}
		}
		cur := applied()
		if cur < -50.0001 {
			t.Fatalf("overshoot: applied %v past target -50", cur)
		}
		if cur > prev {
			t.Fatalf("non-monotonic: %v then %v", prev, cur)
		}
		prev = cur
		ticks++
		if !cont {
			break
		}
	}

	if !ss.IsStopped() {
		t.Error("expected smoother to stop after converging")
	}
	if v := applied(); v != -50 {
		t.Errorf("final offset = %v, want -50", v)
	}
	if ticks < 3 {
		t.Errorf("converged too fast (%d ticks) — not actually easing", ticks)
	}
	if ticks > 40 {
		t.Errorf("took %d ticks to settle — too sluggish", ticks)
	}
}

func TestScrollSmoothClampsAtBounds(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("3", 100, 100, 100, 300) // maxOffset -200

	// Huge delta clamps target to the max offset.
	scrollSmoothBy(w, layout, scrollAxisY, -9999)
	e := w.scrollSmooth.findEntry("3", scrollAxisY)
	if e.target != -200 {
		t.Fatalf("target = %v, want -200 (clamped)", e.target)
	}
	driveScrollSmooth(w, 200)
	if v, _ := w.scrollY().Get("3"); v != -200 {
		t.Errorf("settled offset = %v, want -200", v)
	}

	// Already at the bottom: another downward notch is a no-op.
	if scrollSmoothBy(w, layout, scrollAxisY, -10) {
		t.Error("expected false scrolling past max bound")
	}
	// At the top, an upward notch is a no-op.
	w.scrollY().Set("3", 0)
	scrollSmoothCancel(w, "3", scrollAxisY)
	if scrollSmoothBy(w, layout, scrollAxisY, 10) {
		t.Error("expected false scrolling past min bound")
	}
}

func TestScrollSmoothAccumulatesTarget(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("4", 100, 100, 100, 300)

	// Two rapid notches before any tick: targets accumulate.
	scrollSmoothBy(w, layout, scrollAxisY, -30)
	scrollSmoothBy(w, layout, scrollAxisY, -30)

	e := w.scrollSmooth.findEntry("4", scrollAxisY)
	if e.target != -60 {
		t.Errorf("target = %v, want -60 (accumulated)", e.target)
	}
	if e.current != 0 {
		t.Errorf("current = %v, want 0 (not yet eased)", e.current)
	}
}

func TestScrollSmoothCanceledByInstantScroll(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("5", 100, 100, 100, 300)

	scrollSmoothBy(w, layout, scrollAxisY, -50)
	if e := w.scrollSmooth.findEntry("5", scrollAxisY); e == nil || !e.active {
		t.Fatal("expected active entry before instant scroll")
	}

	// An instant (precise/keyboard) scroll cancels the in-flight ease.
	scrollVertical(layout, -10, w)

	e := w.scrollSmooth.findEntry("5", scrollAxisY)
	if e.active {
		t.Error("instant scroll should deactivate the smoother entry")
	}
	if v, _ := w.scrollY().Get("5"); v != -10 {
		t.Errorf("instant offset = %v, want -10", v)
	}
}

func TestScrollSmoothHorizontalAxis(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	w := &Window{}
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Width: 400, Height: 50}}
	layout := Layout{
		Shape: &Shape{
			shapeType:  shapeRectangle,
			Scrollable: true,
			ID:         "6",
			Width:      100,
			Height:     50,
			Axis:       AxisLeftToRight,
		},
		Children: []Layout{child},
	}
	w.layout = Layout{
		Shape:    &Shape{shapeType: shapeRectangle},
		Children: []Layout{layout},
	}
	ly := &w.layout.Children[0]

	scrollSmoothBy(w, ly, scrollAxisX, -50)
	driveScrollSmooth(w, 200)
	if v, _ := w.scrollX().Get("6"); v != -50 {
		t.Errorf("horizontal settled = %v, want -50", v)
	}
}

func TestScrollSmoothFiresOnScroll(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("7", 100, 100, 100, 300)
	w.layout.Children[0].Parent = &w.layout
	fired := 0
	layout.Shape.events = &eventHandlers{
		OnScroll: func(_ *Layout, _ *Window) { fired++ },
	}
	scrollSmoothBy(w, layout, scrollAxisY, -50)
	driveScrollSmooth(w, 200)
	if fired == 0 {
		t.Error("OnScroll never fired during easing")
	}
}

func TestScrollSmoothResetClearsEntries(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("8", 100, 100, 100, 300)
	scrollSmoothBy(w, layout, scrollAxisY, -50)
	w.scrollSmoothReset()
	if len(w.scrollSmooth.entries) != 0 {
		t.Errorf("entries not cleared: %d remain", len(w.scrollSmooth.entries))
	}
	if !w.scrollSmooth.stopped {
		t.Error("expected stopped after reset")
	}
}

// TestScrollSmoothNoAllocSteadyState guards the per-event allocation
// budget: once an entry exists, accumulating deltas must not allocate.
func TestScrollSmoothNoAllocSteadyState(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("9", 100, 100, 100, 900) // maxOffset -800
	// Warm up: create the entry and animations map.
	scrollSmoothBy(w, layout, scrollAxisY, -5)

	i := 0
	allocs := testing.AllocsPerRun(200, func() {
		// Oscillate within bounds so target keeps changing (full path).
		if i%2 == 0 {
			scrollSmoothBy(w, layout, scrollAxisY, -5)
		} else {
			scrollSmoothBy(w, layout, scrollAxisY, 5)
		}
		i++
	})
	if allocs != 0 {
		t.Errorf("scrollSmoothBy allocated %v times/op in steady state", allocs)
	}
}

// TestScrollSmoothRejectsNonFiniteDelta guards against NaN/Inf wheel
// deltas poisoning the ease target into a never-settling animation
// (60fps layout-refresh DoS).
func TestScrollSmoothRejectsNonFiniteDelta(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	nan := float32(math.NaN())
	layout, w := makeScrollLayout("10", 100, 100, 100, 300)

	if scrollSmoothBy(w, layout, scrollAxisY, nan) {
		t.Error("NaN delta must be rejected")
	}
	if e := w.scrollSmooth.findEntry("10", scrollAxisY); e != nil && e.active {
		t.Error("NaN delta must not arm an entry")
	}

	// -Inf clamps to maxOffset (finite) and is accepted like any
	// large delta; the settled offset must stay in bounds.
	if !scrollSmoothBy(w, layout, scrollAxisY, float32(math.Inf(-1))) {
		t.Fatal("expected -Inf delta to clamp and arm")
	}
	driveScrollSmooth(w, 200)
	if v, _ := w.scrollY().Get("10"); v != -200 {
		t.Errorf("settled offset = %v, want -200", v)
	}
}

// TestScrollSmoothRetiresPoisonedEntry verifies Update deactivates an
// entry whose target became non-finite instead of ticking forever.
func TestScrollSmoothRetiresPoisonedEntry(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("11", 100, 100, 100, 300)
	scrollSmoothBy(w, layout, scrollAxisY, -50)
	e := w.scrollSmooth.findEntry("11", scrollAxisY)
	e.target = float32(math.NaN())

	ticks := driveScrollSmooth(w, 10)
	if ticks >= 10 {
		t.Fatal("poisoned entry never retired")
	}
	if e.active {
		t.Error("poisoned entry still active")
	}
	if v, _ := w.scrollY().Get("11"); v != 0 {
		t.Errorf("offset moved to %v despite poisoned target", v)
	}
}

func TestScrollSmoothByNoScrollID(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("0", 100, 100, 100, 300)
	layout.Shape.Scrollable = false
	layout.Shape.ID = ""
	if scrollSmoothBy(w, layout, scrollAxisY, -50) {
		t.Error("expected false for non-scrollable")
	}
}

func TestScrollSmoothByRespectsScrollMode(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("12", 100, 100, 100, 300)

	layout.Shape.ScrollMode = ScrollHorizontalOnly
	if scrollSmoothBy(w, layout, scrollAxisY, -50) {
		t.Error("vertical ease must be rejected for ScrollHorizontalOnly")
	}
	layout.Shape.ScrollMode = ScrollVerticalOnly
	if scrollSmoothBy(w, layout, scrollAxisX, -50) {
		t.Error("horizontal ease must be rejected for ScrollVerticalOnly")
	}
}

func TestScrollSmoothCanceledByScrollToPct(t *testing.T) {
	guiTheme.ScrollMultiplier = 1
	layout, w := makeScrollLayout("13", 100, 100, 100, 300)
	scrollSmoothBy(w, layout, scrollAxisY, -50)

	w.ScrollVerticalToPct("13", 1)

	e := w.scrollSmooth.findEntry("13", scrollAxisY)
	if e.active {
		t.Error("programmatic scroll must cancel the in-flight ease")
	}
	if v, _ := w.scrollY().Get("13"); v != -200 {
		t.Errorf("offset = %v, want -200", v)
	}
}

func TestCommandApplyScrollSmoothNilSafe(t *testing.T) {
	w := &Window{}
	commandApplyScrollSmooth(w) // must not panic
}
