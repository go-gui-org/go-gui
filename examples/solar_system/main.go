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
	"flag"
	"log"
	"math/rand/v2"
	"os"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

const (
	windowW = 1100
	windowH = 760

	tickAnim  = "solar_tick"
	tickDelay = 16 * time.Millisecond

	// tickSecs is how much *simulated* time one nominal tick advances,
	// and it is deliberately not tickDelay. The animation fires about
	// every 16 ms of wall clock, but advances 3.04 ms of simulation, so
	// Earth's 11.4 s period maps to 60 s wall clock — one Earth year
	// per minute at 1x speed. Orbits, spins and star twinkle slow
	// together because they are functions of App.Time; camera tween
	// stays on wall time so zoom/pan do not get sluggish with the
	// orrery.
	tickSecs = float32(0.00304)

	// wallTickSecs is the nominal wall-clock gap between ticks. It is
	// the step for the first tick, before there is a previous tick to
	// measure from. simPerWall is the simulated seconds per wall
	// second at 1x, derived from the pair so the two cannot drift.
	wallTickSecs = float32(0.016)
	simPerWall   = tickSecs / wallTickSecs

	// maxWallStep caps one tick's wall-clock step. A stalled frame or a
	// laptop wake would otherwise jump every planet forward by the
	// whole gap in one frame. Under the cap, a slow frame advances by
	// the real time it took, so the orrery keeps wall-clock pace when
	// frames drop instead of slowing with them.
	maxWallStep = float32(0.1)

	// speedMin and speedMax bound the orrery speed multiplier that the
	// Up and Down keys step by speedStep. The camera ignores it.
	//
	// All three are written as floats on purpose: an untyped integer 2
	// makes 1/speedStep integer division, which is 0.
	speedMin  = 0.25
	speedMax  = 16.0
	speedStep = 2.0

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
	// Time is simulated seconds since start. It is float64 because it
	// grows without bound: as float32 its step passes half a tick after
	// about a day and the motion stutters. Consumers reduce it with
	// turns before any float32 math.
	Time    float64
	Version uint64 // bumped per tick; invalidates the DrawCanvas cache

	// Speed multiplies how fast simulated time runs. Paused freezes it.
	// Neither touches the camera, which runs on wall time.
	Speed  float32
	Paused bool

	// Muted is the ambience toggle. It is App state, not only an audio
	// call, so the button label reads it back.
	Muted bool

	// lastTick is the wall time of the previous tick, zero before the
	// first. wallStep measures each step from it.
	lastTick time.Time

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

	// DotHover is the nav dot under the pointer, in the same encoding,
	// or -1. drawTooltip names that body. A frame runs events, then
	// layout, then render: the canvas's OnMouseMove clears DotHover
	// (the canvas sees every move, dots included, because a dot has no
	// OnMouseMove to claim it), the dot's OnHover sets it again in the
	// layout pass while the pointer is still on it, and OnDraw reads
	// the result. So a pointer leaving a dot clears it with no leave
	// event, which a Button does not have.
	DotHover int

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
		DotHover: -1,
		Speed:    1,
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
	w := newWindow()

	if *screenshot != "" {
		if err := soft.RenderToPNG(w, 2, *screenshot); err != nil {
			log.Fatalf("screenshot: %v", err)
		}
		os.Exit(0)
	}
	// Audio starts here, on the interactive path only. In OnInit it
	// also ran for -screenshot, because RenderToPNG runs OnInit: the
	// track started, then os.Exit cut it off, and headless CI logged
	// an audio error for an image render.
	startAmbience()
	backend.Run(w)
}

// ambienceStarts counts startAmbience calls, so a test can prove that
// building and rendering the window starts no audio.
var ambienceStarts int

// newWindow builds the window. It starts no audio; main does that.
func newWindow() *gui.Window {
	return gui.NewWindow(gui.WindowCfg{
		State:  newApp(),
		Title:  "Solar System",
		Width:  windowW,
		Height: windowH,
		OnInit: func(w *gui.Window) {
			w.SetView(mainView)
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
					a := state(w)
					tick(a, a.wallStep(time.Now()))
				},
			})
		},
		OnEvent: handleEvent,
	})
}

// wallStep returns the wall-clock seconds since the previous tick,
// clamped to [0, maxWallStep], and records now as the previous tick.
// The first call has nothing to measure from and returns the nominal
// wallTickSecs.
//
// Measuring is the point. A fixed step per callback ties the orrery's
// pace to the frame rate: a frame that takes 50 ms would still advance
// only 16 ms, so the planets slow down whenever the machine is busy.
func (a *App) wallStep(now time.Time) float32 {
	prev := a.lastTick
	a.lastTick = now
	if prev.IsZero() {
		return wallTickSecs
	}
	return clamp32(float32(now.Sub(prev).Seconds()), 0, maxWallStep)
}

// tick advances the simulation by wallDt seconds of wall clock.
// Version++ is mandatory: renderDrawCanvas only re-runs OnDraw when
// Version, size, or backing scale changed, so without it the canvas
// would freeze on frame one.
func tick(a *App, wallDt float32) {
	if !a.Paused {
		a.Time += float64(wallDt * simPerWall * a.Speed)
	}
	a.advanceCamera(wallDt)
	a.recompute()
	switch {
	case a.DotHover != -1:
		// The pointer is on a nav dot; a planet behind it is not the
		// thing being pointed at.
		a.Hovered = -1
	case a.MouseIn:
		a.Hovered = a.hitTest(a.MouseX, a.MouseY)
	default:
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
		w.InvalidateLayout()
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

// togglePause freezes or resumes simulated time. The camera keeps
// running, so a paused orrery can still be zoomed and panned.
func togglePause(a *App, w *gui.Window) {
	a.Paused = !a.Paused
	refreshView(w)
}

// stepSpeed multiplies the orrery speed by factor, clamped to
// [speedMin, speedMax].
func stepSpeed(a *App, w *gui.Window, factor float32) {
	a.Speed = clamp32(a.Speed*factor, speedMin, speedMax)
	refreshView(w)
}

// toggleMute silences or restores the ambience track.
func toggleMute(a *App, w *gui.Window) {
	a.Muted = !a.Muted
	setAmbienceMuted(a.Muted)
	refreshView(w)
}

// refreshView asks for a view pass. The tick refreshes renderers only,
// so state the controls show (pause, speed, mute) needs this to reach
// the screen. w is nil in tests that drive App directly.
func refreshView(w *gui.Window) {
	if w != nil {
		w.InvalidateLayout()
	}
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
	case gui.KeySpace:
		// Only reaches here when no focused button took Space first.
		togglePause(a, w)
		e.IsHandled = true
	case gui.KeyDown:
		stepSpeed(a, w, 1/speedStep)
		e.IsHandled = true
	case gui.KeyUp:
		stepSpeed(a, w, speedStep)
		e.IsHandled = true
	case gui.KeyM:
		toggleMute(a, w)
		e.IsHandled = true
	}
}
