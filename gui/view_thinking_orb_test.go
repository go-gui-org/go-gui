package gui

import (
	"math"
	"testing"
	"time"
)

func TestThinkingOrbDefaultLayout(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID: "orb1",
	}), w)
	if layout.Shape.Width != 64 {
		t.Errorf("default width = %v, want 64", layout.Shape.Width)
	}
	if layout.Shape.Height != 64 {
		t.Errorf("default height = %v, want 64", layout.Shape.Height)
	}
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
	cv := layout.Children[0]
	if cv.Shape.shapeType != shapeDrawCanvas {
		t.Errorf("child shapeType = %d, want DrawCanvas", cv.Shape.shapeType)
	}
	if cv.Shape.events == nil || cv.Shape.events.OnDraw == nil {
		t.Error("canvas OnDraw not set")
	}
}

func TestThinkingOrbSmallSize(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:   "orb-small",
		Size: ThinkingOrbSmall,
	}), w)
	if layout.Shape.Width != 20 {
		t.Errorf("small width = %v, want 20", layout.Shape.Width)
	}
}

func TestThinkingOrbInvalidDesignClamps(t *testing.T) {
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:     "orb-bad",
		Design: ThinkingOrbDesign(99),
	}), w)
	if layout.Shape.Width != 64 {
		t.Errorf("width = %v, want 64", layout.Shape.Width)
	}
}

func TestThinkingOrbWidthFallback(t *testing.T) {
	// Unset, negative, and non-finite widths floor onto the
	// tuned box side; a set width survives. The sanitize helper
	// itself is covered by TestMathSpinnerPositive.
	for _, width := range []float32{0, -5, float32(math.NaN()),
		float32(math.Inf(1))} {
		w := &Window{}
		layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
			ID:    "orb-width",
			Width: width,
		}), w)
		if layout.Shape.Width != 64 {
			t.Errorf("width %v laid out %v, want 64",
				width, layout.Shape.Width)
		}
	}
	w := &Window{}
	layout := generateViewLayout(ThinkingOrb(ThinkingOrbCfg{
		ID:    "orb-width-set",
		Width: 100,
	}), w)
	if layout.Shape.Width != 100 {
		t.Errorf("width 100 laid out %v, want 100",
			layout.Shape.Width)
	}
}

func TestThinkingOrbDurationClamps(t *testing.T) {
	if thinkingOrbDuration(1e9) != 50*time.Millisecond {
		t.Errorf("huge speed duration = %v, want 50ms",
			thinkingOrbDuration(1e9))
	}
	if thinkingOrbDuration(1e-9) != time.Hour {
		t.Errorf("tiny speed duration = %v, want 1h",
			thinkingOrbDuration(1e-9))
	}
}

func TestThinkingOrbInkMirror(t *testing.T) {
	// Near-black ink on light reads near 0; mirrored on dark it
	// reads near 255.
	if got := orbInkByte(0.1, false); got > 30 {
		t.Errorf("light ink = %d, want near 26", got)
	}
	if got := orbInkByte(0.1, true); got < 225 {
		t.Errorf("dark ink = %d, want near 230", got)
	}
	if orbAlphaByte(0) != 0 || orbAlphaByte(1) != 255 {
		t.Error("alpha ends must map exactly")
	}
}

func TestThinkingOrbPausedRegistersNoAnimation(t *testing.T) {
	w := newOrbTestWindow(t)
	w.TestRender(func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{
			ID:     "orb-paused",
			Design: ThinkingOrbSearching,
			Paused: true,
		})
	})
	if w.HasAnimation(ScopeID("orb-paused", "orb")) {
		t.Error("paused orb must not register an animation")
	}
}

func TestThinkingOrbLiveRegistersAnimation(t *testing.T) {
	w := newOrbTestWindow(t)
	w.TestRender(func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{
			ID:     "orb-live",
			Design: ThinkingOrbWorking,
		})
	})
	if !w.HasAnimation(ScopeID("orb-live", "orb")) {
		t.Error("live orb must register a view-bound animation")
	}
	found := w.TestDuplicateIDs()
	if len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}

func TestThinkingOrbScopedIDsStayUnique(t *testing.T) {
	w := newOrbTestWindow(t)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{
				Column(ContainerCfg{
					ID:     "panel-a",
					Sizing: FitFit,
					Content: []View{
						ThinkingOrb(ThinkingOrbCfg{
							ID:     "orb",
							Design: ThinkingOrbSearching,
						}),
					},
				}),
				Column(ContainerCfg{
					ID:     "panel-b",
					Sizing: FitFit,
					Content: []View{
						ThinkingOrb(ThinkingOrbCfg{
							ID:     "orb",
							Design: ThinkingOrbComposing,
						}),
					},
				}),
			},
		})
	})
	if found := w.TestDuplicateIDs(); len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}

func TestThinkingOrbLabelStructure(t *testing.T) {
	w := newOrbTestWindow(t)
	root := w.TestRender(func(*Window) View {
		return ThinkingOrbLabel(ThinkingOrbLabelCfg{
			ID:     "orb-label",
			Text:   "Searching the web…",
			Design: ThinkingOrbSearching,
		})
	})
	outer, ok := root.FindByID("orb-label")
	if !ok {
		t.Fatal("label outer not found")
	}
	if len(outer.Children) != 2 {
		t.Fatalf("label children = %d, want orb + text",
			len(outer.Children))
	}
	if found := w.TestDuplicateIDs(); len(found) != 0 {
		t.Errorf("duplicate effective IDs: %v", found)
	}
}

// newOrbTestWindow returns a test window whose animation loop stops
// when the test ends. A live orb keeps ticking otherwise, and its
// goroutine adds mallocs to the alloc gates of later tests.
func newOrbTestWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{State: new(int)})
	t.Cleanup(w.stopAnimationLoop)
	return w
}

// thinkingOrbAnim returns the orb's registered keyframe tick, or
// nil when none runs.
func thinkingOrbAnim(w *Window, id string) *KeyframeAnimation {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	kf, _ := w.animations[ScopeID(id, "orb")].(*KeyframeAnimation)
	return kf
}

func TestThinkingOrbClockCrossesLoopWithoutJump(t *testing.T) {
	// The keyframe progress wraps 1 → 0 every tick period. The
	// geometry clock must keep counting up across the wrap, because
	// no design repeats at the tick period: a clock that snaps back
	// to 0 makes the orb jump once per loop.
	w := newOrbTestWindow(t)
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-wrap"})
	}
	w.TestRender(view)
	kf := thinkingOrbAnim(w, "orb-wrap")
	if kf == nil {
		t.Fatal("live orb registered no tick")
	}
	for _, val := range []float32{0.5, 0.99, 1, 0.01} {
		kf.OnValue(val, w)
	}
	clk := StateReadOr(w, nsThinkingOrb, "orb-wrap", orbClock{})
	want := 1.01 * orbCycleLen
	if math.Abs(clk.t-want) > 1e-4 {
		t.Errorf("clock after wrap = %v, want %v", clk.t, want)
	}
}

func TestThinkingOrbPauseStopsTick(t *testing.T) {
	// Pausing a live orb must remove its tick at once. A tick left
	// to the 2 s view-bound expiry keeps moving the paused orb.
	w := newOrbTestWindow(t)
	paused := false
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-p", Paused: paused})
	}
	w.TestRender(view)
	if thinkingOrbAnim(w, "orb-p") == nil {
		t.Fatal("live orb registered no tick")
	}
	paused = true
	w.TestRender(nil)
	if w.HasAnimation(ScopeID("orb-p", "orb")) {
		t.Error("paused orb kept its tick")
	}
}

func TestThinkingOrbResumeContinuesClock(t *testing.T) {
	// A resumed orb restarts its tick from progress 0. The clock
	// must carry on from where it paused, not jump by the stale
	// progress of the old tick.
	w := newOrbTestWindow(t)
	paused := false
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-r", Paused: paused})
	}
	w.TestRender(view)
	old := thinkingOrbAnim(w, "orb-r")
	old.OnValue(0.7, w)
	paused = true
	w.TestRender(nil)
	paused = false
	w.TestRender(nil)
	// A value the old tick queued before it was removed must not
	// move the clock.
	old.OnValue(0.9, w)
	thinkingOrbAnim(w, "orb-r").OnValue(0.1, w)
	clk := StateReadOr(w, nsThinkingOrb, "orb-r", orbClock{})
	want := 0.8 * orbCycleLen
	if math.Abs(clk.t-want) > 1e-4 {
		t.Errorf("clock after resume = %v, want %v", clk.t, want)
	}
}

func TestThinkingOrbSpeedFollowsDesignChange(t *testing.T) {
	// Each design has its own tuned speed. A design change under
	// the same ID must rebuild the tick at the new design's speed.
	w := newOrbTestWindow(t)
	design := ThinkingOrbWorking
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-d", Design: design})
	}
	w.TestRender(view)
	design = ThinkingOrbConnecting
	w.TestRender(nil)
	kf := thinkingOrbAnim(w, "orb-d")
	if kf == nil {
		t.Fatal("no tick after design change")
	}
	want := thinkingOrbDuration(orbResolve(ThinkingOrbConnecting,
		ThinkingOrbRegular).speed)
	if kf.Duration != want {
		t.Errorf("duration = %v, want %v", kf.Duration, want)
	}
}

func TestThinkingOrbStillVersionTracksInputs(t *testing.T) {
	// A still orb's canvas redraws only when Version changes, so
	// every input the drawing reads must feed Version: the frame
	// time, the design, the size, the color and the ground.
	version := func(cfg ThinkingOrbCfg) uint64 {
		cfg.ID = "orb-v"
		cfg.Paused = true
		w := &Window{}
		return generateViewLayout(ThinkingOrb(cfg), w).
			Children[0].Shape.Version
	}
	base := version(ThinkingOrbCfg{})
	if version(ThinkingOrbCfg{Color: RGB(200, 0, 0)}) == base {
		t.Error("Color change kept the canvas version")
	}
	if version(ThinkingOrbCfg{Color: RGB(200, 0, 0)}) ==
		version(ThinkingOrbCfg{Color: RGB(0, 200, 0)}) {
		t.Error("color RGB change kept the canvas version")
	}
	if version(ThinkingOrbCfg{Design: ThinkingOrbShaping}) == base {
		t.Error("Design change kept the canvas version")
	}
	light := thinkingOrbVersion(orbStillT, ThinkingOrbWorking,
		ThinkingOrbRegular, false, false, Color{})
	dark := thinkingOrbVersion(orbStillT, ThinkingOrbWorking,
		ThinkingOrbRegular, true, false, Color{})
	if light == dark {
		t.Error("ground polarity change kept the canvas version")
	}
}

func TestThinkingOrbInkColor(t *testing.T) {
	// A caller color tints dots and lines alike and keeps its own
	// alpha as a multiplier on the mark alpha.
	base := RGBA(255, 0, 0, 128)
	got := orbInk(0.4, 1, false, true, base)
	if got.R != 255 || got.G != 0 || got.B != 0 || got.A != 128 {
		t.Errorf("custom ink = %+v, want red at alpha 128", got)
	}
	got = orbInk(0.4, 0.5, false, true, base)
	if got.A != 64 {
		t.Errorf("custom ink alpha = %d, want 64", got.A)
	}
	gray := orbInk(0.4, 1, false, false, Color{})
	if gray.R != gray.G || gray.G != gray.B || gray.A != 255 {
		t.Errorf("unset ink = %+v, want opaque gray", gray)
	}
}

func TestThinkingOrbLabelPausedHoldsText(t *testing.T) {
	// A paused label must not shimmer: a still orb beside moving
	// text reads as busy.
	textAnim := func(paused bool) *textAnimRender {
		w := newOrbTestWindow(t)
		root := w.TestRender(func(*Window) View {
			return ThinkingOrbLabel(ThinkingOrbLabelCfg{
				ID: "lbl", Text: "Waiting…", Paused: paused,
			})
		})
		outer, ok := root.FindByID("lbl")
		if !ok {
			t.Fatal("label not found")
		}
		return outer.Children[1].Shape.TC.anim
	}
	if textAnim(false) == nil {
		t.Fatal("live label text has no shimmer; probe is wrong")
	}
	if textAnim(true) != nil {
		t.Error("paused label text still shimmers")
	}
}

func TestThinkingOrbLabelA11YFallsBackToDesign(t *testing.T) {
	// With no Text and no caller label, the label reads the design
	// name, as a bare orb does. The inner orb has no node of its
	// own, so an empty name here leaves the element unnamed.
	w := &Window{}
	layout := generateViewLayout(ThinkingOrbLabel(ThinkingOrbLabelCfg{
		ID: "lbl", Design: ThinkingOrbSearching,
	}), w)
	if layout.Shape.a11Y == nil ||
		layout.Shape.a11Y.Label != ThinkingOrbSearching.A11YLabel() {
		t.Errorf("label a11y = %+v, want %q", layout.Shape.a11Y,
			ThinkingOrbSearching.A11YLabel())
	}
}

func TestThinkingOrbPauseDropsQueuedTick(t *testing.T) {
	// A value the tick queued just before the pause lands after it.
	// It must not move the paused frame.
	w := newOrbTestWindow(t)
	paused := false
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-q", Paused: paused})
	}
	w.TestRender(view)
	old := thinkingOrbAnim(w, "orb-q")
	old.OnValue(0.3, w)
	paused = true
	w.TestRender(nil)
	before := StateReadOr(w, nsThinkingOrb, "orb-q", orbClock{}).t
	old.OnValue(0.6, w)
	after := StateReadOr(w, nsThinkingOrb, "orb-q", orbClock{}).t
	if after != before {
		t.Errorf("paused clock moved %v → %v", before, after)
	}
}

func TestThinkingOrbStillAmendNoAlloc(t *testing.T) {
	// A still orb runs its AmendLayout every frame. After the frame
	// that stopped the tick, that pass must not allocate.
	w := newOrbTestWindow(t)
	paused := false
	view := func(*Window) View {
		return ThinkingOrb(ThinkingOrbCfg{ID: "orb-s", Paused: paused})
	}
	w.TestRender(view)
	paused = true
	w.TestRender(nil)
	allocs := testing.AllocsPerRun(20, func() {
		thinkingOrbAmendLayout(nil, w, ThinkingOrbWorking,
			ThinkingOrbRegular, 1, true, "orb-s")
	})
	if allocs != 0 {
		t.Errorf("still AmendLayout allocates %v times", allocs)
	}
}
