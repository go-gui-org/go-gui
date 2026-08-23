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

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
)

const (
	windowW = 1100
	windowH = 760

	tickAnim  = "solar_tick"
	tickDelay = 16 * time.Millisecond
	tickSecs  = float32(0.016)

	canvasID = "solar_canvas"

	// starCount is the fixed starfield size, and starAlphaLevels the
	// number of quantized twinkle brightnesses. See drawStars for why
	// the quantization matters.
	starCount       = 220
	starAlphaLevels = 8

	// keyZoomStep is the multiplier one +/- press applies.
	keyZoomStep = 1.15
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
				Callback: func(_ *gui.Animate, w *gui.Window) {
					tick(state(w))
				},
			})
		},
		OnEvent: handleEvent,
	})

	backend.Run(w)
}

// tick advances the simulation one frame. Version++ is mandatory:
// renderDrawCanvas only re-runs OnDraw when Version, size, or backing
// scale changed, so without it the canvas would freeze on frame one.
func tick(a *App) {
	a.Time += tickSecs
	a.advanceCamera(tickSecs)
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
func selectBody(a *App, i int) {
	if i == a.Selected {
		return
	}
	a.beginTransition()
	a.Selected = i
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
func stepSelection(a *App, delta int) {
	n := len(planets) + 1 // the sun takes rank 0

	rank := 0
	switch {
	case a.Selected == selSun:
		rank = 0
	case a.Selected >= 0:
		rank = a.Selected + 1
	case delta > 0:
		selectBody(a, selSun)
		return
	default:
		selectBody(a, len(planets)-1)
		return
	}

	rank = ((rank+delta)%n + n) % n
	if rank == 0 {
		selectBody(a, selSun)
		return
	}
	selectBody(a, rank-1)
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
		stepSelection(a, -1)
		e.IsHandled = true
	case gui.KeyRight:
		stepSelection(a, 1)
		e.IsHandled = true
	case gui.KeyEscape:
		if a.Selected != -1 {
			selectBody(a, -1)
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
