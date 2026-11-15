package gui

import (
	"slices"
	"testing"
	"time"
)

// stepFrame is one display frame at 60 Hz, the step a soak run uses.
const stepFrame = time.Second / 60

// stepFor calls TestStep n times with stepFrame.
func stepFor(w *Window, n int) {
	for range n {
		w.TestStep(stepFrame)
	}
}

// A tween runs on the manual clock: it ends after its duration in
// steps, with no wall time spent, and is half way at half the steps.
func TestStepAdvancesTween(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
	var got float32
	done := false
	tw := NewTweenAnimation("tw", 0, 100, func(v float32, _ *Window) { got = v })
	tw.Duration = 300 * time.Millisecond
	tw.Easing = EaseLinear
	tw.OnDone = func(*Window) { done = true }

	w.TestStep(0) // switch to the manual clock before the tween starts
	w.AnimationAdd(tw)
	stepFor(w, 9) // 150 ms
	if got < 45 || got > 55 {
		t.Fatalf("value after 9 of 18 steps = %v, want about 50", got)
	}
	if done {
		t.Fatal("tween finished half way")
	}
	// stepFrame rounds 1/60 s down, so 18 steps end 12 ns short of
	// 300 ms; the 19th is past it.
	stepFor(w, 10)
	if !done || got != 100 {
		t.Fatalf("after 19 steps: done = %v, value = %v; want true, 100", done, got)
	}
	if w.HasAnimation("tw") {
		t.Fatal("finished tween still registered")
	}
}

// A one-shot Animate fires on the first step past its delay, not
// before. stepFrame is 1/60 s rounded down to 16_666_666 ns, so 30
// steps sum to 499_999_980 ns: still under a 500 ms delay. Step 31 is
// past it.
func TestStepAnimateDelayBoundary(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
	fired := 0
	w.TestStep(0)
	w.AnimationAdd(&Animate{
		AnimID:   "a",
		Delay:    500 * time.Millisecond,
		Callback: func(*Animate, *Window) { fired++ },
	})
	stepFor(w, 30) // 499_999_980 ns: not past the delay
	if fired != 0 {
		t.Fatalf("fired %d times before the delay", fired)
	}
	w.TestStep(stepFrame)
	if fired != 1 {
		t.Fatalf("fired %d times one step past the delay, want 1", fired)
	}
}

// The caret blink toggles on the manual clock and parks after the idle
// timeout, all without the animation goroutine.
func TestStepBlinkCaretTogglesAndParks(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestStep(0)
	// A caret blinks only in a window the OS has focused; a backend
	// reports that with EventFocused.
	w.EventFn(&Event{Type: EventFocused})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Content: []View{Input(InputCfg{ID: "in"})}})
	})
	if err := w.TestFocus("in"); err != nil {
		t.Fatal(err)
	}
	// The frame registers the blink (syncIMECaretState); a focus move
	// alone marks no refresh, so run one.
	w.TestRender(nil)
	if !w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("focused input registered no caret blink")
	}
	on := w.viewState.inputCursorOn.Load()
	toggled := false
	for range int(blinkCursorAnimationDelay/stepFrame) + 2 {
		w.TestStep(stepFrame)
		if w.viewState.inputCursorOn.Load() != on {
			toggled = true
			break
		}
	}
	if !toggled {
		t.Fatal("caret did not toggle within one blink delay of steps")
	}
	stepFor(w, int(blinkCursorIdleTimeout/stepFrame)+2)
	if w.HasAnimation(blinkCursorAnimationID) {
		t.Fatal("caret blink did not park after the idle timeout")
	}
	if w.animationStarted {
		t.Fatal("the animation goroutine started under TestStep")
	}
}

// A tooltip appears after its delay of steps: the hover start and the
// delay check both read the manual clock.
func TestStepTooltipDelay(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestStep(0)
	delay := 200 * time.Millisecond
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone,
			Content: []View{
				WithTooltip(w, WithTooltipCfg{
					ID: "tip", Text: "hi", Delay: delay,
					Content: []View{Row(ContainerCfg{ID: "box", Width: 100, Height: 40, Sizing: FixedFixed})},
				}),
			},
		})
	})
	cx, cy := xfCenter(mustShape(t, w, "box"))
	w.EventFn(&Event{Type: EventMouseMove, MouseX: cx, MouseY: cy})
	w.TestStep(0)
	if w.viewState.tooltip.hoverID != "tip" {
		t.Fatalf("hoverID = %q, want tip", w.viewState.tooltip.hoverID)
	}
	stepFor(w, int(delay/stepFrame)-1)
	if w.viewState.tooltip.id != "" {
		t.Fatal("tooltip shown before its delay")
	}
	stepFor(w, 3)
	if w.viewState.tooltip.id != "tip" {
		t.Fatalf("tooltip id = %q after the delay, want tip", w.viewState.tooltip.id)
	}
}

// The manual clock moves only with TestStep, and w.Now follows it.
func TestStepClockAdvancesByDt(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestStep(0)
	t0 := w.clockNow()
	time.Sleep(5 * time.Millisecond)
	if !w.clockNow().Equal(t0) {
		t.Fatal("manual clock moved with wall time")
	}
	w.TestStep(10 * time.Minute)
	if got := w.clockNow().Sub(t0); got != 10*time.Minute {
		t.Fatalf("clock advanced %v, want 10m", got)
	}
	if got := w.Now().Sub(t0); got != 10*time.Minute {
		t.Fatalf("w.Now advanced %v, want 10m", got)
	}
}

// The time-travel pin still wins over the manual clock in w.Now, and
// clockNow ignores the pin.
func TestStepTimeTravelPinWins(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestStep(time.Second)
	pin := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
	w.setVirtualNow(&pin)
	if !w.Now().Equal(pin) {
		t.Fatalf("w.Now = %v, want the pin", w.Now())
	}
	if w.clockNow().Equal(pin) {
		t.Fatal("clockNow followed the scrub pin")
	}
}

// AnimationAdd after the first step starts no goroutine; an animation
// added before it has its goroutine stopped.
func TestStepStopsAnimationGoroutine(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.AnimationAdd(&Animate{AnimID: "r", Delay: time.Hour, Callback: func(*Animate, *Window) {}})
	if !w.animationStarted {
		t.Fatal("precondition: AnimationAdd started no loop")
	}
	w.TestStep(0)
	if !animationLoopStopped(w) {
		t.Fatal("first TestStep left the animation goroutine running")
	}

	w2 := NewTestWindow(t, WindowCfg{})
	w2.TestStep(0)
	w2.AnimationAdd(&Animate{AnimID: "r", Delay: time.Hour, Callback: func(*Animate, *Window) {}})
	if w2.animationStarted {
		t.Fatal("AnimationAdd started a goroutine under TestStep")
	}
}

// A step on a quiet window allocates nothing: the soak harness counts
// mallocs per stepFrame, and the stepper must not add its own.
func TestStepIdleAllocatesNothing(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
	w.TestStep(0)
	w.AnimationAdd(&Animate{AnimID: "r", Delay: time.Hour, Callback: func(*Animate, *Window) {}})
	stepFor(w, 3)
	if n := testing.AllocsPerRun(100, func() { w.TestStep(stepFrame) }); n != 0 {
		t.Fatalf("TestStep allocates %v per call on a quiet window, want 0", n)
	}
}

func TestStepNegativePanics(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	defer func() {
		if recover() == nil {
			t.Fatal("TestStep(-1) did not panic")
		}
	}()
	w.TestStep(-1)
}

// A time-travel snapshot is stamped with the window clock, so a scrub
// to it pins w.Now() to the manual instant the app saw, not to a wall
// clock instant minutes away from it.
func TestStepSnapshotStampsManualClock(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{State: &testState{}})
	w.history = newSnapshotRing(1 << 20)
	w.TestStep(10 * time.Minute)
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	n := len(w.history.entries)
	if n == 0 {
		t.Fatal("precondition: no snapshot recorded")
	}
	if got, want := w.history.entries[n-1].when, w.clockNow(); !got.Equal(want) {
		t.Fatalf("snapshot when = %v, want the manual clock %v", got, want)
	}
}

// A date picker opens on the month of the window clock.
func TestStepDatePickerOpensOnClockMonth(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestStep(400 * 24 * time.Hour)
	s := datePickerGetState(w, &DatePickerCfg{ID: "dp"})
	now := w.clockNow()
	if s.ViewYear != now.Year() || s.ViewMonth != int(now.Month()) {
		t.Fatalf("view = %d-%02d, want %d-%02d",
			s.ViewYear, s.ViewMonth, now.Year(), now.Month())
	}
}

// The a11y sync throttle runs on the window clock: under TestStep a
// dirty tree syncs once the steps pass a11ySyncInterval, not when wall
// time does (review of #1262).
func TestStepA11ySyncThrottleUsesManualClock(t *testing.T) {
	w := newA11yWindow()
	w.initA11y()
	w.layout = Layout{Shape: &Shape{A11YRole: AccessRoleButton}}
	w.startManualClock()
	mp := w.nativePlatform.(*mockA11yPlatform)

	w.a11y.dirty = true
	w.syncA11y()
	if mp.calls != 1 {
		t.Fatalf("first sync: %d calls, want 1", mp.calls)
	}
	// Inside the interval on the manual clock: throttled.
	w.manualOffset.Add(int64(a11ySyncInterval / 2))
	w.a11y.dirty = true
	w.syncA11y()
	if mp.calls != 1 {
		t.Fatalf("inside interval: %d calls, want 1", mp.calls)
	}
	// Past the interval on the manual clock, with almost no wall time.
	w.manualOffset.Add(int64(a11ySyncInterval))
	w.syncA11y()
	if mp.calls != 2 {
		t.Fatalf("past interval: %d calls, want 2", mp.calls)
	}
}

// An AnimationAdd on another goroutine can read manualClock as false
// just before the first TestStep and reach animationStartOnce after
// it. The first TestStep uses up the Once, so that late start runs
// nothing (review of #1262).
func TestStepBlocksLateAnimationStart(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	w.TestStep(0)
	started := false
	// The late arrival of the racing ensureAnimationLoop.
	w.animationStartOnce.Do(func() { started = true })
	if started {
		t.Fatal("animation goroutine can still start after TestStep")
	}
}

// A roller without a SelectedDate opens on the window clock's date, as
// the date picker does (review of #1262).
func TestStepDatePickerRollerOpensOnClockDate(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	w.TestStep(400 * 24 * time.Hour)
	rv := DatePickerRoller(DatePickerRollerCfg{ID: "roller"}).(*datePickerRollerView)
	rv.GenerateLayout(w)
	got, want := rv.cfg.SelectedDate, w.clockNow()
	if got.Year() != want.Year() || got.YearDay() != want.YearDay() {
		t.Fatalf("selected = %s, want %s",
			got.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

// Animations that finish on the same tick queue their callbacks in ID
// order, not in map order, so a TestStep replay is the same every run
// (PR #1267 review).
func TestStepSameTickCallbacksInIDOrder(t *testing.T) {
	ids := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	for run := range 20 {
		w := NewTestWindow(t, WindowCfg{})
		w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
		w.TestStep(0)
		var order []string
		for _, id := range ids {
			tw := NewTweenAnimation(id, 0, 1, func(float32, *Window) {})
			tw.Duration = 100 * time.Millisecond
			tw.OnDone = func(*Window) { order = append(order, id) }
			w.AnimationAdd(tw)
		}
		w.TestStep(200 * time.Millisecond)
		if !slices.Equal(order, ids) {
			t.Fatalf("run %d: callback order %v, want %v", run, order, ids)
		}
	}
}

// A tick instant before an animation's start gives progress 0, not a
// negative value the easing would extrapolate (PR #1267 review).
func TestDurationProgressBeforeStartIsZero(t *testing.T) {
	start := time.Now()
	p, done := durationProgress(start, start.Add(-time.Millisecond), time.Second)
	if p != 0 || done {
		t.Fatalf("progress = %v, done = %v; want 0, false", p, done)
	}
}
