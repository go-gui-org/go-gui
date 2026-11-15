package soak

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// fixtureApp is the state of the small window the tests soak.
type fixtureApp struct {
	clicks int
	text   string
	leak   [][]byte // grows each tick when leakEveryTick is set
	ticks  int
}

// fixtureWindow builds a window with a few inputs and buttons, enough
// for every scripted action to find a target. tick, when set, runs on
// every animation tick (a repeating Animate with no delay).
func fixtureWindow(t *testing.T, tick func(*fixtureApp)) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{State: &fixtureApp{}, Width: 640, Height: 480})
	w.SetView(func(w *gui.Window) gui.View {
		app := gui.State[fixtureApp](w)
		rows := make([]gui.View, 0, 12)
		for i := range 8 {
			rows = append(rows, gui.Button(gui.ButtonCfg{
				ID:      gui.ScopeIDN("btn", "", i),
				Content: []gui.View{gui.Text(gui.TextCfg{Text: "Button"})},
				OnClick: func(ctx gui.EventCtx) {
					gui.State[fixtureApp](ctx.Window).clicks++
					ctx.Consume()
				},
			}))
		}
		rows = append(rows,
			gui.Input(gui.InputCfg{
				ID: "name", Text: app.text,
				OnTextChanged: func(s string, ctx gui.EventCtx) {
					gui.State[fixtureApp](ctx.Window).text = s
				},
			}),
		)
		return gui.Column(gui.ContainerCfg{
			ID: "list", Sizing: gui.FillFill, Scrollable: true, Content: rows,
		})
	})
	if tick != nil {
		// Switch to the manual clock first: an animation added before it
		// starts the wall-clock goroutine, whose ticks would land in the
		// run's first frames by timing, not by seed.
		w.TestStep(0)
		w.AnimationAdd(&gui.Animate{
			AnimID: "fixture-tick",
			Repeat: true,
			Callback: func(_ *gui.Animate, w *gui.Window) {
				tick(gui.State[fixtureApp](w))
			},
		})
	}
	return w
}

func shortCfg(seed uint64) Config {
	return Config{Seed: seed, Duration: 20 * time.Second, Warmup: 2 * time.Second, RenderEvery: 120}
}

// The same seed takes the same actions on the same frames: a failure
// seen once can be replayed.
func TestRunSeedReplays(t *testing.T) {
	trace := func() []string {
		var log []string
		cfg := shortCfg(42)
		cfg.OnAction = func(frame int, a string) {
			log = append(log, strconv.Itoa(frame)+" "+a)
		}
		if _, err := Run(fixtureWindow(t, nil), cfg); err != nil {
			t.Fatal(err)
		}
		return log
	}
	a, b := trace(), trace()
	if len(a) < 10 {
		t.Fatalf("only %d actions in 20 s; the run did not act", len(a))
	}
	if !slices.Equal(a, b) {
		for i := range min(len(a), len(b)) {
			if a[i] != b[i] {
				t.Fatalf("runs differ at action %d:\n  %s\n  %s", i, a[i], b[i])
			}
		}
		t.Fatalf("runs differ in length: %d vs %d", len(a), len(b))
	}
}

// A scripted run reaches the widgets: buttons are clicked and text
// lands in the input.
func TestRunScriptedActsOnWidgets(t *testing.T) {
	w := fixtureWindow(t, nil)
	rep, err := Run(w, shortCfg(7))
	if err != nil {
		t.Fatal(err)
	}
	app := gui.State[fixtureApp](w)
	if app.clicks == 0 {
		t.Errorf("no button click landed in %d actions", rep.Actions)
	}
	if app.text == "" {
		t.Errorf("no text landed in the input in %d actions", rep.Actions)
	}
	if rep.Measured == 0 || rep.HeapSamples < 2 {
		t.Errorf("measured %d frames, %d heap samples; want both > 0", rep.Measured, rep.HeapSamples)
	}
	if rep.Frames != int(20*time.Second/FrameDt) {
		t.Errorf("frames = %d, want %d", rep.Frames, int(20*time.Second/FrameDt))
	}
}

// A window that keeps 1 KiB more each frame fails the heap slope gate.
func TestRunDetectsLeak(t *testing.T) {
	w := fixtureWindow(t, func(app *fixtureApp) {
		app.leak = append(app.leak, make([]byte, 1024))
	})
	rep, err := Run(w, shortCfg(3))
	if err != nil {
		t.Fatal(err)
	}
	// 60 KiB per simulated second is 3.5 MiB per minute.
	if rep.HeapSlope < 1<<20 {
		t.Fatalf("slope %.0f B/min for a 3.5 MiB/min leak\n%s", rep.HeapSlope, rep)
	}
	err = rep.Check(Gates{MaxHeapSlope: 256 << 10})
	if !errors.Is(err, ErrGate) {
		t.Fatalf("Check = %v, want ErrGate", err)
	}
	if !strings.Contains(err.Error(), "seed 3") {
		t.Errorf("gate error does not name the seed: %v", err)
	}
}

// A panic on a frame stops the run and is reported with its frame and
// the actions before it.
func TestRunReportsPanic(t *testing.T) {
	w := fixtureWindow(t, func(app *fixtureApp) {
		app.ticks++
		if app.ticks == 300 {
			panic("boom")
		}
	})
	rep, err := Run(w, shortCfg(5))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Panic == nil {
		t.Fatal("no panic reported")
	}
	if !strings.Contains(rep.Panic.Value, "boom") {
		t.Errorf("panic value = %q", rep.Panic.Value)
	}
	// Tick 300 runs in the 300th step, frame index 299: the frame
	// counter goes up after the step returns. Exact, because the
	// fixture ticks only on the manual clock.
	if rep.Panic.Frame != 299 {
		t.Errorf("panic at frame %d, want 299", rep.Panic.Frame)
	}
	if len(rep.Panic.LastActions) == 0 {
		t.Error("panic report has no actions")
	}
	if rep.Frames >= int(20*time.Second/FrameDt) {
		t.Error("run went on after the panic")
	}
	if rep.Check(Gates{}) == nil {
		t.Error("Check passed a run that panicked")
	}
}

// Random mode sends raw events, including degenerate resizes, and
// survives them on a plain window.
func TestRunRandomMode(t *testing.T) {
	cfg := shortCfg(11)
	cfg.Mode = ModeRandom
	rep, err := Run(fixtureWindow(t, nil), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Panic != nil {
		t.Fatalf("random mode panicked:\n%s", rep)
	}
	if rep.Actions < 100 {
		t.Errorf("only %d random events", rep.Actions)
	}
}

func TestParseMode(t *testing.T) {
	for _, m := range []Mode{ModeScripted, ModeRandom} {
		got, err := ParseMode(m.String())
		if err != nil || got != m {
			t.Errorf("ParseMode(%q) = %v, %v", m, got, err)
		}
	}
	if _, err := ParseMode("bogus"); err == nil {
		t.Error("ParseMode accepted bogus")
	}
}

func TestStats(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	if got := slope(xs, []float64{5, 7, 9, 11}); math.Abs(got-2) > 1e-9 {
		t.Errorf("slope = %v, want 2", got)
	}
	if got := slope([]float64{1, 1}, []float64{1, 2}); got != 0 {
		t.Errorf("slope over one x = %v, want 0", got)
	}
	if got := slope(nil, nil); got != 0 {
		t.Errorf("slope of nothing = %v", got)
	}
	vals := make([]uint64, 100)
	for i := range vals {
		vals[i] = uint64(i + 1)
	}
	if got := percentile(vals, 0.99); got != 99 {
		t.Errorf("p99 = %v, want 99", got)
	}
	if got := mean([]uint64{1, 2, 3}); got != 2 {
		t.Errorf("mean = %v, want 2", got)
	}
	if mean(nil) != 0 || percentile(nil, 0.5) != 0 {
		t.Error("empty stats not zero")
	}
	// A flat floor with peaks on top has no floor slope; the raw slope
	// is fooled by a late peak.
	ts := []float64{0, 1, 2, 3, 4, 5, 6, 7}
	hs := []float64{10, 50, 10, 10, 10, 50, 10, 90}
	if got := slope(windowMinima(ts, hs, 4)); got != 0 {
		t.Errorf("floor slope of a flat floor = %v, want 0", got)
	}
	if slope(ts, hs) <= 0 {
		t.Error("precondition: raw slope should see the late peak")
	}
	mx, my := windowMinima(ts, []float64{9, 8, 7, 6, 5, 4, 3, 2}, 3)
	if !slices.Equal(mx, []float64{1, 4, 7}) || !slices.Equal(my, []float64{8, 5, 2}) {
		t.Errorf("windowMinima = %v %v", mx, my)
	}
	if x, y := windowMinima(nil, nil, 4); x != nil || y != nil {
		t.Error("windowMinima of nothing not nil")
	}
}
