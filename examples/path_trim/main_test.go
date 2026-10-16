package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newWindow(t *testing.T) *gui.Window {
	t.Helper()
	w := gui.NewTestWindow(t, gui.WindowCfg{
		State:  &app{},
		Width:  400,
		Height: 300,
	})
	w.TestRender(mainView)
	return w
}

func TestMainViewNoFindings(t *testing.T) {
	w := newWindow(t)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}

// Replay starts the tween that draws the paths in.
func TestReplayStartsAnimation(t *testing.T) {
	w := newWindow(t)
	if err := w.TestClick("replay"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if !w.HasAnimation(trimAnim) {
		t.Fatal("replay did not start the trim animation")
	}
}

// triangleCount draws the paths at progress p and counts the
// triangle floats the strokes emit.
func triangleCount(p float32) int {
	s := &app{progress: p}
	dc := gui.NewDrawContext(canvasW, canvasH, nil)
	drawPaths(dc, s, gui.ThemeDark)
	n := 0
	for _, b := range dc.Batches() {
		n += len(b.Triangles)
	}
	return n
}

// More progress draws more: at 0 only the ring's track shows, and
// each step adds stroke.
func TestProgressDrawsMore(t *testing.T) {
	prev := triangleCount(0)
	if prev == 0 {
		t.Fatal("the ring's track is missing at progress 0")
	}
	for _, p := range []float32{0.25, 0.5, 1} {
		n := triangleCount(p)
		if n <= prev {
			t.Errorf("progress %v draws %d floats, want more than %d",
				p, n, prev)
		}
		prev = n
	}
}

// A redraw at a new progress allocates nothing once the paths exist.
func TestRedrawDoesNotAllocate(t *testing.T) {
	s := &app{progress: 1}
	dc := gui.NewDrawContext(canvasW, canvasH, nil)
	drawPaths(dc, s, gui.ThemeDark)
	p := float32(0)
	allocs := testing.AllocsPerRun(50, func() {
		p += 0.02
		s.progress = p
		s.mark.TrimTo(&s.trimmed, 0, s.progress)
		s.ring.TrimTo(&s.trimmed, 0, s.progress)
	})
	if allocs != 0 {
		t.Errorf("allocs per trim = %v, want 0", allocs)
	}
}
