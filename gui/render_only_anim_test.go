package gui

import (
	"bytes"
	"log"
	"strings"
	"testing"
	"time"
)

// Repaint-only animation frames.
//
// A looping loading animation (ProgressBar, Skeleton, MathSpinner,
// ThinkingOrb) changes only how its widget is drawn. Its tick asks for
// AnimationRefreshRenderOnly, so the window rebuilds the render
// commands from the tree it already laid out and skips the view
// function and layout. These tests pin the two halves of that contract:
//
//  1. the tick asks for a render-only frame, and
//  2. a render-only frame after the tick draws exactly what a full
//     frame would draw at the same animation value.
//
// The second half is what makes the first one safe. A widget whose
// render-only frame differs from its full frame shows a stale picture
// until something else forces a layout.

// renderOnlyAnimCases are the built-in looping widgets that repaint
// without a layout.
func renderOnlyAnimCases() []struct {
	name  string
	build func(*Window) View
} {
	return []struct {
		name  string
		build func(*Window) View
	}{
		{"progress_bar", func(*Window) View {
			return ProgressBar(ProgressBarCfg{
				ID: "pb", Indefinite: true, Width: 200, Height: 8,
			})
		}},
		{"progress_bar_vertical", func(*Window) View {
			return ProgressBar(ProgressBarCfg{
				ID: "pbv", Indefinite: true, Vertical: true,
				Width: 8, Height: 120,
			})
		}},
		{"skeleton", func(*Window) View {
			return Skeleton(SkeletonCfg{ID: "sk", Width: 200, Height: 20})
		}},
		{"math_spinner", func(w *Window) View {
			return MathSpinner(MathSpinnerCfg{ID: "ms", Rotate: true}, w)
		}},
		{"thinking_orb", func(*Window) View {
			return ThinkingOrb(ThinkingOrbCfg{ID: "orb"})
		}},
	}
}

// newRenderOnlyAnimWindow lays out build once, which registers the
// widget's animations, then stops the animation goroutine. The tests
// drive the animations by hand: a live tick would write wall-clock
// values between the frames being compared.
func newRenderOnlyAnimWindow(t *testing.T, build func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{
		State:  new(int),
		Width:  goldenWidth,
		Height: goldenHeight,
	})
	w.SetView(wrapGoldenRoot(build))
	w.refreshLayout.Store(true)
	w.FrameFn()
	w.stopAnimationLoop()
	return w
}

// renderOnlyAnimKeyframes returns the keyframe animations the window
// holds.
func renderOnlyAnimKeyframes(w *Window) []*KeyframeAnimation {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	var out []*KeyframeAnimation
	for _, a := range w.animations {
		if kf, ok := a.(*KeyframeAnimation); ok {
			out = append(out, kf)
		}
	}
	return out
}

// setRenderOnlyAnimValue delivers v to every keyframe animation, the
// way a tick does.
func setRenderOnlyAnimValue(t *testing.T, w *Window, v float32) {
	t.Helper()
	kfs := renderOnlyAnimKeyframes(w)
	if len(kfs) == 0 {
		t.Fatal("widget registered no keyframe animation")
	}
	for _, kf := range kfs {
		kf.OnValue(v, w)
	}
}

func TestRenderOnlyAnimTickAsksForRenderOnly(t *testing.T) {
	for _, c := range renderOnlyAnimCases() {
		t.Run(c.name, func(t *testing.T) {
			w := newRenderOnlyAnimWindow(t, c.build)
			kfs := renderOnlyAnimKeyframes(w)
			if len(kfs) == 0 {
				t.Fatal("widget registered no keyframe animation")
			}
			for _, kf := range kfs {
				if got := kf.RefreshKind(); got != AnimationRefreshRenderOnly {
					t.Errorf("%s: RefreshKind = %d, want render-only",
						kf.AnimID, got)
				}
			}
		})
	}
}

func TestRenderOnlyAnimFrameMatchesFullFrame(t *testing.T) {
	for _, c := range renderOnlyAnimCases() {
		t.Run(c.name, func(t *testing.T) {
			w := newRenderOnlyAnimWindow(t, c.build)
			setRenderOnlyAnimValue(t, w, 0.2)
			w.refreshLayout.Store(true)
			w.FrameFn()
			before := serializeCmds(w.renderers)

			setRenderOnlyAnimValue(t, w, 0.6)
			w.renderOnlyLocked()
			got := serializeCmds(w.renderers)

			w.refreshLayout.Store(true)
			w.FrameFn()
			want := serializeCmds(w.renderers)

			if want == before {
				t.Fatal("the animation value does not change the " +
					"picture; the case checks nothing")
			}
			if got != want {
				t.Errorf("render-only frame differs from the full "+
					"frame:\n%s", goldenDiff(want, got))
			}
		})
	}
}

// A render-only frame never runs the view function, which is where a
// widget normally refreshes its view-bound heartbeat. The render pass
// has to refresh it instead, or the loop retires its animation after
// animViewBoundStale and the widget freezes.
func TestRenderOnlyAnimKeepsAnimationAlive(t *testing.T) {
	for _, c := range renderOnlyAnimCases() {
		t.Run(c.name, func(t *testing.T) {
			w := newRenderOnlyAnimWindow(t, c.build)
			stale := viewBoundNow().Add(-animViewBoundStale + 100*time.Millisecond)
			w.animMu.Lock()
			if len(w.animViewBound) == 0 {
				w.animMu.Unlock()
				t.Fatal("widget registered no view-bound animation")
			}
			for id := range w.animViewBound {
				w.animViewBound[id] = stale
			}
			w.animMu.Unlock()

			w.renderOnlyLocked()

			w.animMu.Lock()
			defer w.animMu.Unlock()
			for id, seen := range w.animViewBound {
				if !seen.After(stale) {
					t.Errorf("%s: heartbeat not refreshed by a "+
						"render-only frame", id)
				}
			}
		})
	}
}

// A render-only frame must not refresh the heartbeat of a widget that
// left the tree: the tree it walks is the one from the last layout,
// and a widget missing from it is not drawn.
func TestRenderOnlyAnimHeartbeatNeedsTheWidget(t *testing.T) {
	w := newRenderOnlyAnimWindow(t, func(*Window) View {
		return ProgressBar(ProgressBarCfg{ID: "pb", Indefinite: true})
	})
	w.SetView(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	w.refreshLayout.Store(true)
	w.FrameFn()
	stale := viewBoundNow().Add(-time.Second)
	w.animMu.Lock()
	for id := range w.animViewBound {
		w.animViewBound[id] = stale
	}
	w.animMu.Unlock()

	w.renderOnlyLocked()

	w.animMu.Lock()
	defer w.animMu.Unlock()
	for id, seen := range w.animViewBound {
		if seen.After(stale) {
			t.Errorf("%s: heartbeat refreshed for a widget no longer "+
				"in the tree", id)
		}
	}
}

// DrawCanvasCfg.VersionFn is read at render time, so a render-only
// frame redraws a canvas whose version moved between frames.
func TestDrawCanvasVersionFnRedrawsOnRenderOnly(t *testing.T) {
	var version uint64 = 1
	draws := 0
	w := newRenderOnlyAnimWindow(t, func(*Window) View {
		return DrawCanvas(DrawCanvasCfg{
			ID: "cv", Width: 40, Height: 40,
			Version:   99, // ignored while VersionFn is set
			VersionFn: func() uint64 { return version },
			OnDraw: func(dc *DrawContext) {
				draws++
				dc.FilledRect(0, 0, 10, 10, RGB(255, 0, 0))
			},
		})
	})
	if draws != 1 {
		t.Fatalf("first frame: draws = %d, want 1", draws)
	}

	w.renderOnlyLocked()
	if draws != 1 {
		t.Errorf("unchanged version: draws = %d, want 1 (cache hit)", draws)
	}

	version = 2
	w.renderOnlyLocked()
	if draws != 2 {
		t.Errorf("new version under render-only: draws = %d, want 2", draws)
	}
}

// A panicking VersionFn is app code failing inside the render pass. The
// canvas falls back to Version and the frame goes on.
func TestDrawCanvasVersionFnPanicFallsBackToVersion(t *testing.T) {
	draws := 0
	w := newRenderOnlyAnimWindow(t, func(*Window) View {
		return DrawCanvas(DrawCanvasCfg{
			ID: "cv", Width: 40, Height: 40,
			Version:   7,
			VersionFn: func() uint64 { panic("boom") },
			OnDraw:    func(*DrawContext) { draws++ },
		})
	})
	w.renderOnlyLocked()
	if draws != 1 {
		t.Errorf("draws = %d, want 1: Version 7 should still hit the cache", draws)
	}
}

// A tick of a looping loading widget costs a render-only frame, and the
// frame itself allocates nothing. Before the switch each tick ran the
// view function and layout for the whole window: about 1600 allocations
// on a 200-button screen.
//
// The orb is the one case above zero, and the allocations are its canvas
// redraw, not the frame: its ink alpha changes every frame, so the
// number of color batches changes, and DrawContext.takeBatch allocates
// whenever a redraw needs more batches than the last one left in the
// pool. A full frame paid the same redraw cost before this change.
func TestRenderOnlyAnimTickAllocs(t *testing.T) {
	budget := map[string]float64{"thinking_orb": 10}
	for _, c := range renderOnlyAnimCases() {
		t.Run(c.name, func(t *testing.T) {
			w := newRenderOnlyAnimWindow(t, c.build)
			kfs := renderOnlyAnimKeyframes(w)
			v := float32(0)
			tick := func() {
				v += 0.01
				if v > 1 {
					v = 0
				}
				for _, kf := range kfs {
					kf.OnValue(v, w)
				}
				w.renderOnlyLocked()
			}
			// Warm the canvas buffer pool and the state maps.
			for range 4 {
				tick()
			}
			if got, want := testing.AllocsPerRun(50, tick), budget[c.name]; got > want {
				t.Errorf("tick allocs = %v, want <= %v", got, want)
			}
		})
	}
}

// BenchmarkRenderOnlyAnimTick times one indefinite ProgressBar tick on
// a 200-button screen, the frame a looping loading widget now costs.
func BenchmarkRenderOnlyAnimTick(b *testing.B) {
	w := NewTestWindow(b, WindowCfg{State: new(int), Width: 800, Height: 600})
	w.SetView(func(*Window) View {
		content := make([]View, 0, 201)
		content = append(content, ProgressBar(ProgressBarCfg{
			ID: "pb", Indefinite: true, Width: 200, Height: 8,
		}))
		for i := range 200 {
			content = append(content, Button(ButtonCfg{
				ID:      ScopeIDN("", "b", i),
				Content: []View{Text(TextCfg{Text: "Button"})},
			}))
		}
		return Column(ContainerCfg{Sizing: FillFill, Content: content})
	})
	w.refreshLayout.Store(true)
	w.FrameFn()
	w.stopAnimationLoop()
	kfs := renderOnlyAnimKeyframes(w)
	b.ReportAllocs()
	v := float32(0)
	for b.Loop() {
		v += 0.01
		if v > 1 {
			v = 0
		}
		for _, kf := range kfs {
			kf.OnValue(v, w)
		}
		w.renderOnlyLocked()
	}
}

// The VersionFn and OnDraw panic warnings are separate: a panicking
// VersionFn must not use up the one-shot warning a later OnDraw panic
// needs.
func TestDrawCanvasVersionFnPanicKeepsOnDrawWarning(t *testing.T) {
	var logged bytes.Buffer
	prev := log.Writer()
	log.SetOutput(&logged)
	t.Cleanup(func() { log.SetOutput(prev) })
	newRenderOnlyAnimWindow(t, func(*Window) View {
		return Row(ContainerCfg{Content: []View{
			DrawCanvas(DrawCanvasCfg{
				ID: "a", Width: 10, Height: 10,
				VersionFn: func() uint64 { panic("version") },
				OnDraw:    func(*DrawContext) {},
			}),
			DrawCanvas(DrawCanvasCfg{
				ID: "b", Width: 10, Height: 10,
				OnDraw: func(*DrawContext) { panic("draw") },
			}),
		}})
	})
	out := logged.String()
	if !strings.Contains(out, "VersionFn panicked") {
		t.Errorf("VersionFn panic not logged; log:\n%s", out)
	}
	if !strings.Contains(out, "OnDraw panicked") {
		t.Errorf("OnDraw panic not logged after a VersionFn panic; log:\n%s", out)
	}
}

// amendOnRender re-runs a hook on render-only frames only: a full
// frame runs AmendLayout once (layoutAmend), not a second time from the
// render walk, and a hook without the flag never runs on a render-only
// frame.
func TestAmendOnRenderRunsOnlyOnRenderOnlyFrames(t *testing.T) {
	for _, flag := range []bool{false, true} {
		calls := 0
		w := newRenderOnlyAnimWindow(t, func(*Window) View {
			return Row(ContainerCfg{
				ID:            "amend",
				amendOnRender: flag,
				AmendLayout:   func(EventCtx) { calls++ },
			})
		})
		if calls != 1 {
			t.Fatalf("amendOnRender=%v: full frame ran hook %d times, want 1",
				flag, calls)
		}
		w.renderOnlyLocked()
		want := 1
		if flag {
			want = 2
		}
		if calls != want {
			t.Fatalf("amendOnRender=%v: after render-only frame %d calls, want %d",
				flag, calls, want)
		}
	}
}
