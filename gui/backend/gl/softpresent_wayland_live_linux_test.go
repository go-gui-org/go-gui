//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"math"
	"runtime"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

// Live tests of the Wayland soft present path (#1265 step 4). They need a
// compositor and skip without one, like the other Wayland window tests;
// scripts/wayland/test.sh runs them under sway, weston and mutter.

// newSoftWayland opens a soft window through New with GOGUI_RENDERER=soft
// and runs OnInit, as the loop would.
func newSoftWayland(t *testing.T, cfg gui.WindowCfg) (*Backend, *gui.Window) {
	t.Helper()
	requireWayland(t)
	t.Setenv("GOGUI_RENDERER", "soft")
	b := newWaylandTestBackend(t, cfg)
	t.Cleanup(b.Destroy)
	if !b.plat.softMode || b.plat.wl.soft == nil {
		t.Fatal("GOGUI_RENDERER=soft did not select the Wayland soft path")
	}
	w := b.plat.w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	return b, w
}

// softFrames runs the loop's frame step for d: FrameFn, then present when
// there is something to draw, dispatching between passes so the frame
// callbacks and buffer releases arrive. It returns how many buffers were
// committed.
func softFrames(t *testing.T, b *Backend, d time.Duration) int {
	t.Helper()
	ww := b.plat.wl
	ww.dirty = true
	commits := 0
	for end := time.Now().Add(d); time.Now().Before(end); {
		now := time.Now()
		if pending, _ := ww.framePending(now); !pending {
			if b.plat.w.FrameFn() || ww.dirty {
				if b.presentWayland(b.plat.w, now) {
					ww.dirty = false
					commits++
				} else {
					ww.dirty = true
				}
			}
		}
		if err := ww.d.conn.Dispatch(5 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	return commits
}

// animateView invalidates every frame, so the loop draws as often as the
// frame callbacks allow and the buffers are reused through their releases.
func animateView(w *gui.Window) gui.View {
	w.InvalidateLayout()
	return softTextView(w)
}

// TestSoftWaylandNeverTouchesEGL drives frames, a resize and a scale change
// on a named-soft window, and checks that no EGL state exists: the display
// loaded none, and a GL call on this path would panic on the uninitialized
// GL table when the test runs alone. It also checks that buffers come back:
// more commits than wlSoftMaxSlots with no more slots than that means the
// compositor's releases reached the ring.
func TestSoftWaylandNeverTouchesEGL(t *testing.T) {
	b, w := newSoftWayland(t, gui.WindowCfg{Width: 240, Height: 160,
		OnInit: func(w *gui.Window) { w.SetView(animateView) }})
	ww := b.plat.wl

	commits := softFrames(t, b, 500*time.Millisecond)
	ww.resize(300, 200, ww.scale120)
	commits += softFrames(t, b, 200*time.Millisecond)
	ww.resize(ww.logW, ww.logH, 240)
	commits += softFrames(t, b, 200*time.Millisecond)
	ww.resize(ww.logW, ww.logH, 120)
	commits += softFrames(t, b, 200*time.Millisecond)

	if d := ww.d; d.eglDpy != 0 || !d.soft {
		t.Fatalf("display: soft=%v eglDpy=%#x, want soft with no EGL display", d.soft, d.eglDpy)
	}
	p := &b.plat
	if p.eglDpy != 0 || p.eglSurface != 0 || p.eglContext != 0 || ww.eglWin.Ptr() != 0 {
		t.Fatalf("soft window created EGL state: dpy=%#x surface=%#x context=%#x",
			p.eglDpy, p.eglSurface, p.eglContext)
	}
	if commits <= wlSoftMaxSlots {
		t.Fatalf("%d commits in 1.1 s of animation; the frame callbacks or the "+
			"buffer releases never came", commits)
	}
	// A release in softFrames' last dispatch frees an old-size slot that
	// no acquire has dropped yet; one more acquire, as the next frame
	// would make, does that before the check.
	if _, err := ww.soft.ring.acquire(b.physW, b.physH); err != nil {
		t.Fatal(err)
	}
	live := 0
	for _, s := range ww.soft.ring.slots {
		if s != nil {
			live++
			if s.w != b.physW || s.h != b.physH {
				// A free slot of an old size is dropped on the next
				// acquire; a held one may remain.
				if !s.busy {
					t.Errorf("free slot of an old size %dx%d kept (phys %dx%d)",
						s.w, s.h, b.physW, b.physH)
				}
			}
		}
	}
	if live == 0 {
		t.Fatal("no slot after the frames")
	}
	if w.TextMeasurer() == nil {
		t.Fatal("the soft renderer installed no text measurer")
	}
}

// TestSoftWaylandPixelsMatchRenderToImage: the bytes committed to the
// compositor are exactly what soft.RenderToImage draws for the same view, in
// B, G, R order (XRGB8888 in memory). The compositor's side of the buffer is
// not readable from a client without a screencopy protocol, so this checks
// the shared memory the compositor maps, which covers the swizzle and the
// stride.
func TestSoftWaylandPixelsMatchRenderToImage(t *testing.T) {
	const lw, lh = 320, 200
	cfg := gui.WindowCfg{Width: lw, Height: lh,
		OnInit: func(w *gui.Window) { w.SetView(softTextView) }}
	b, _ := newSoftWayland(t, cfg)
	if softFrames(t, b, 300*time.Millisecond) == 0 {
		t.Fatal("no frame committed")
	}
	sp := b.plat.wl.soft
	if sp.last == nil {
		t.Fatal("no last slot after a commit")
	}
	pw, ph := int(sp.last.w), int(sp.last.h)

	ref := gui.NewWindow(cfg)
	want, err := soft.RenderToImage(ref, b.dpiScale)
	if err != nil {
		t.Fatalf("RenderToImage: %v", err)
	}
	t.Cleanup(func() { soft.Release(ref) })
	if want.Rect.Dx() != pw || want.Rect.Dy() != ph {
		t.Skipf("window is %dx%d, reference %dx%d: the compositor resized it",
			pw, ph, want.Rect.Dx(), want.Rect.Dy())
	}
	got := sp.last.mem
	diff, first := 0, -1
	for i := range pw * ph {
		g, r := got[i*4:i*4+3], want.Pix[i*4:i*4+3]
		if g[0] != r[2] || g[1] != r[1] || g[2] != r[0] {
			if first < 0 {
				first = i
			}
			diff++
		}
	}
	if diff > 0 {
		t.Fatalf("%d of %d pixels differ; first at (%d,%d): buffer BGR %v, want RGB %v",
			diff, pw*ph, first%pw, first/pw, got[first*4:first*4+3], want.Pix[first*4:first*4+3])
	}
}

// TestSoftWaylandAutoPick: with GOGUI_RENDERER unset, a compositor whose EGL
// hands clients a software rasterizer (the harness, a Cinnamon Wayland
// session) gets a soft window on native Wayland, with the EGL display
// terminated. On a compositor with hardware GL it must stay GL; which one
// this machine has is read from the result, since only the probe knows.
func TestSoftWaylandAutoPick(t *testing.T) {
	requireWayland(t)
	// Not newWaylandTestBackend: its requireWayland would pin gl again.
	t.Setenv("GOGUI_RENDERER", "")
	t.Setenv("GOGUI_WAYLAND", "1")
	b, err := New(waylandTestWindow(gui.WindowCfg{Width: 200, Height: 120}))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(b.Destroy)
	if b.plat.wl == nil {
		t.Fatal("New fell back to X11 with a compositor present")
	}
	d := b.plat.wl.d
	if !d.soft {
		if b.plat.softMode || b.plat.wl.soft != nil || d.eglDpy == 0 {
			t.Fatalf("GL choice with soft state: softMode=%v soft=%v eglDpy=%#x",
				b.plat.softMode, b.plat.wl.soft != nil, d.eglDpy)
		}
		t.Skip("hardware GL on this compositor: auto-pick kept GL")
	}
	if !b.plat.softMode || b.plat.wl.soft == nil {
		t.Fatal("display chose soft but the window is not on the soft path")
	}
	if d.eglDpy != 0 || b.plat.eglDpy != 0 {
		t.Fatalf("soft auto-pick kept the EGL display: %#x / %#x", d.eglDpy, b.plat.eglDpy)
	}
	if softFrames(t, b, 200*time.Millisecond) == 0 {
		t.Fatal("auto-picked soft window committed no frame")
	}
}

// TestSoftWaylandPresentAllocs pins a steady-state soft frame's present at 0
// allocations (#1265 step 5): draw into the renderer's reused buffer, copy
// into a mapped wl_shm buffer, then frame, attach, damage and commit, each a
// libwayland call through wl's call. Only present is counted. Dispatching
// the frame callback and the buffer release in between goes through
// purego's callback path, which allocates per event (ebitengine/purego#567)
// and is shared with GL (TestWaylandFrameAllocs).
func TestSoftWaylandPresentAllocs(t *testing.T) {
	if raceEnabled {
		t.Skip("allocation counts include the race runtime's own under -race")
	}
	b, w := newSoftWayland(t, gui.WindowCfg{Width: 240, Height: 160,
		OnInit: func(w *gui.Window) { w.SetView(softTextView) }})
	// Warm: the slots exist, the atlas holds every glyph, the frame
	// callback's decoder is built.
	softFrames(t, b, 300*time.Millisecond)

	// MemStats counts every goroutine's mallocs, and New starts go-glyph's
	// background font-coverage warm, which can land a stray allocation in
	// a round; the best of a few rounds is present's own cost, as in
	// TestWaylandFrameAllocs.
	best := math.Inf(1)
	for round := 0; round < wlFrameAllocRounds && best > 0; round++ {
		best = min(best, softPresentMallocs(t, b, w, 30))
	}
	if best > 0 {
		t.Errorf("a soft Wayland present allocates %.2f times per frame, want 0", best)
	}
}

// softPresentMallocs presents frames frames and returns the mallocs per
// frame counted around presentWayland alone.
func softPresentMallocs(t *testing.T, b *Backend, w *gui.Window, frames int) float64 {
	t.Helper()
	ww := b.plat.wl
	var before, after runtime.MemStats
	var mallocs uint64
	for i := range frames {
		// Wait, uncounted, for what the loop waits for: the last frame's
		// callback and a free buffer.
		deadline := time.Now().Add(3 * time.Second)
		for ww.frameCb.Valid() || !hasFreeSlot(ww.soft) {
			if time.Now().After(deadline) {
				t.Fatalf("frame %d: no frame callback or buffer release within 3s", i)
			}
			if err := ww.d.conn.Dispatch(20 * time.Millisecond); err != nil {
				t.Fatal(err)
			}
		}
		runtime.ReadMemStats(&before)
		ok := b.presentWayland(w, time.Now())
		runtime.ReadMemStats(&after)
		if !ok {
			t.Fatalf("frame %d: present committed nothing", i)
		}
		mallocs += after.Mallocs - before.Mallocs
	}
	return float64(mallocs) / float64(frames)
}

// hasFreeSlot reports whether acquire would return a slot instead of
// skipping the frame: a free slot, or room for a new one. The size does not
// matter here: the window does not resize during the test.
func hasFreeSlot(sp *wlSoftPresent) bool {
	for _, s := range sp.ring.slots {
		if s == nil || !s.busy {
			return true
		}
	}
	return false
}
