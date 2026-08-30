// The solar_system example is an interactive orrery: eight planets on
// tilted elliptical orbits around a glowing sun, over a twinkling
// starfield. Hovering a planet glows it and shows a tooltip that
// follows the cursor; clicking one zooms and pans the camera to follow
// it and opens an info panel. Arrow keys, nav dots, scroll, pinch, and
// +/- all drive the same state.
//
// It exists to show DrawCanvas, Animate, the float system, and
// gesture/scroll/key input working together in one app.
package main

import (
	"math/rand/v2"
	"time"

	"flag"
	"log"
	"os"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	windowW = 1100
	windowH = 760

	tickAnim  = "solar_tick"
	tickDelay = 16 * time.Millisecond

	// tickSecs is how much *simulated* time one tick advances, and it
	// is deliberately not tickDelay. The animation still fires every
	// 16 ms of wall clock, but advances 3.04 ms of simulation, so
	// Earth's 11.4 s period maps to 60 s wall clock — one Earth year
	// per minute. Orbits, spins and star twinkle slow together because
	// they are functions of App.Time; camera tween stays on wall time
	// so zoom/pan do not get sluggish with the orrery.
	tickSecs = float32(0.00304)

	// wallTickSecs is the wall-clock delta advanceCamera steps by. Keep
	// it separate from tickSecs so slowing the orrery does not slow the
	// camera.
	wallTickSecs = float32(0.016)

	canvasID = "solar_canvas"

	// starCount is the fixed starfield size. starAlphaFloor is the
	// dimmest a star may go: the quantized field this replaced put its
	// darkest bucket at the bucket's midpoint rather than at zero, and
	// keeping that floor is what stops the faintest stars blinking out
	// entirely at the bottom of the twinkle.
	starCount      = 220
	starAlphaFloor = 0.0625

	// keyZoomStep is the multiplier one +/- press applies.
	keyZoomStep = 1.3
)

// Star is one background point. Position is in normalized [0,1) canvas
// coordinates so the field survives a resize, and is screen-fixed: it
// does not move with the camera, which keeps stars from clumping into
// the center when zoomed in.
type Star struct {
	X, Y  float32
	Size  float32
	Phase float32
	Speed float32
	Base  float32 // baseline brightness in [0,1]
	Amp   float32 // twinkle amplitude
}

// App is the window's single state slot.
type App struct {
	Time    float32 // seconds since start
	Version uint64  // bumped per tick; invalidates the DrawCanvas cache

	Stars []Star

	// glowStops and haloStops back the two radial glow fills. They are
	// reused across frames because the draw runs on every tick and a
	// fresh slice per glow would allocate for the whole session.
	glowStops []gui.GradientStop
	haloStops []gui.GradientStop

	// stars is drawStars' mesh scratch, on the same footing as the two
	// below.
	stars bodyMesh

	// body is drawBody's mesh scratch, reused for the same reason: nine
	// planets a tick, each rebuilding a few thousand vertices. corona
	// is drawCorona's, kept separate so the two capacities settle
	// independently rather than sawing against each other.
	body   bodyMesh
	corona bodyMesh

	// granules is drawGranulation's, on the same footing. It is filled
	// and flushed twice per frame, once per cell polarity.
	granules bodyMesh

	// ringPts is drawRings' polyline scratch: one ring half at a time,
	// six halves a frame. Reused for the same reason the meshes are.
	ringPts []float32

	// belt is drawBelt's mesh scratch, dial is the calendar ring's
	// (ticks + labels + marker in one batch). Separate so capacities
	// settle independently, the reason documented for
	// body/corona/granules.
	belt bodyMesh
	dial bodyMesh

	// Selected and Hovered are a planet index, selSun for the sun, or
	// -1 for the full-system view / nothing under the cursor.
	Selected int
	Hovered  int

	// Cursor position in canvas-content space, from OnMouseMove.
	MouseX, MouseY float32
	MouseIn        bool

	// Camera. Cam* are live; From* are the values the current
	// transition started from.
	CamX, CamY, CamZoom    float32
	FromX, FromY, FromZoom float32
	TweenT                 float32

	// UserZoom is the manual scroll/pinch/button multiplier, kept
	// separate so it composes with the selection zoom.
	UserZoom float32

	// PinchPrev is the previous cumulative PinchScale, or 0 when no
	// pinch is in progress. GesturePinch reports a cumulative scale and
	// the phase constants are unexported, so tracking the previous
	// value is how a per-event delta is recovered.
	PinchPrev float32

	CanvasW, CanvasH float32 // stashed from OnDraw

	// Cached per-planet screen geometry, refreshed once per tick.
	ScreenX [len(planets)]float32
	ScreenY [len(planets)]float32
	ScreenR [len(planets)]float32

	// The world positions those came from, kept because the shading
	// works in world space and the vertical squash above is not
	// invertible from ScreenY alone. Draw-path reads only — see
	// lightVecAt for why lightVec does not use them.
	WorldX [len(planets)]float32
	WorldY [len(planets)]float32

	// The sun's, kept the same way and for the same reason: hit-testing
	// and painting must agree on where it is.
	SunX, SunY, SunR float32

	// Text styles resolved at generation time, where the theme read is
	// correct, and read back inside OnDraw, which runs after it.
	TipStyle   gui.TextStyle
	LabelStyle gui.TextStyle
}

func state(w *gui.Window) *App { return gui.State[App](w) }

func newApp() *App {
	a := &App{
		Selected: -1,
		Hovered:  -1,
		UserZoom: 1,
		CamZoom:  1,
		Stars:    makeStars(),
	}
	// Start settled in the full-system view rather than tweening in
	// from an arbitrary origin.
	a.TweenT = 1
	return a
}

// makeStars builds the field once with a fixed seed, so a run and a
// test see the same sky.
func makeStars() []Star {
	rng := rand.New(rand.NewPCG(0x50144, 0x5751E4))
	stars := make([]Star, starCount)
	for i := range stars {
		stars[i] = Star{
			X:     rng.Float32(),
			Y:     rng.Float32(),
			Size:  0.5 + rng.Float32()*1.3,
			Phase: rng.Float32() * 6.283,
			Speed: 0.8 + rng.Float32()*2.4,
			Base:  0.30 + rng.Float32()*0.35,
			Amp:   0.10 + rng.Float32()*0.30,
		}
	}
	return stars
}

func main() {
	screenshot := flag.String("screenshot", "", "write screenshot and exit")
	flag.Parse()

	gui.SetTheme(gui.ThemeDark)

	w := gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Title:  "Solar System",
		Width:  windowW,
		Height: windowH,
		OnInit: func(w *gui.Window) {
			w.UpdateView(mainView)
			w.AnimationAdd(&gui.Animate{
				AnimID: tickAnim,
				Delay:  tickDelay,
				Repeat: true,
				// The simulation moves what the canvas paints, never
				// what the widget tree contains: the info panel and
				// the nav dots read a.Selected, which only an event
				// can change. So a tick rebuilds renderers from the
				// layout already in hand and skips the view pass
				// entirely. selectBody asks for the layout refresh
				// when it does move the selection.
				Refresh: gui.AnimationRefreshRenderOnly,
				Callback: func(_ *gui.Animate, w *gui.Window) {
					tick(state(w))
				},
			})
		},
		OnEvent: handleEvent,
	})

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	backend.Run(w)
}

// tick advances the simulation one frame. Version++ is mandatory:
// renderDrawCanvas only re-runs OnDraw when Version, size, or backing
// scale changed, so without it the canvas would freeze on frame one.
func tick(a *App) {
	a.Time += tickSecs
	a.advanceCamera(wallTickSecs)
	a.recompute()
	if a.MouseIn {
		a.Hovered = a.hitTest(a.MouseX, a.MouseY)
	} else {
		a.Hovered = -1
	}
	a.Version++
}

// selectBody focuses a planet by index or the sun with selSun, and
// clears the selection with -1. Re-selecting what is already selected
// is a no-op so a repeated click does not restart the tween.
func selectBody(a *App, w *gui.Window, i int) {
	if i == a.Selected {
		return
	}
	a.beginTransition()
	a.Selected = i
	// The only App field the view reads. Under a render-only tick
	// nothing re-runs mainView on its own, so the info panel and the
	// nav dots would keep showing the previous body without this.
	if w != nil {
		w.UpdateWindow()
	}
	// A manual zoom belongs to the view the user was in; carrying it
	// into the next selection is what makes zoom "fight" the camera.
	a.UserZoom = 1
}

// stepSelection moves selection by delta with wraparound over the sun
// followed by the planets sun-outward. With nothing selected, Right
// starts at the sun and Left at Neptune.
//
// The walk runs over a *rank* rather than over Selected directly,
// because selSun is deliberately outside the planet index range and so
// is not adjacent to Mercury in arithmetic.
func stepSelection(a *App, w *gui.Window, delta int) {
	n := len(planets) + 1 // the sun takes rank 0

	rank := 0
	switch {
	case a.Selected == selSun:
		rank = 0
	case a.Selected >= 0:
		rank = a.Selected + 1
	case delta > 0:
		selectBody(a, w, selSun)
		return
	default:
		selectBody(a, w, len(planets)-1)
		return
	}

	rank = ((rank+delta)%n + n) % n
	if rank == 0 {
		selectBody(a, w, selSun)
		return
	}
	selectBody(a, w, rank-1)
}

// handleEvent takes the window-level keys, which must work without the
// canvas holding focus.
func handleEvent(e *gui.Event, w *gui.Window) {
	if e.Type != gui.EventKeyDown {
		return
	}
	a := state(w)
	switch e.KeyCode {
	case gui.KeyLeft:
		stepSelection(a, w, -1)
		e.IsHandled = true
	case gui.KeyRight:
		stepSelection(a, w, 1)
		e.IsHandled = true
	case gui.KeyEscape:
		if a.Selected != -1 {
			selectBody(a, w, -1)
			e.IsHandled = true
		}
	case gui.KeyEqual, gui.KeyKPAdd:
		a.applyUserZoom(keyZoomStep)
		e.IsHandled = true
	case gui.KeyMinus, gui.KeyKPSubtract:
		a.applyUserZoom(1 / keyZoomStep)
		e.IsHandled = true
	}
}
