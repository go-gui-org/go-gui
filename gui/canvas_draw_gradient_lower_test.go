package gui

import (
	"math"
	"testing"
)

// A concentric radial fill is handed to the backend as one shader quad
// instead of a ring mesh. These tests cover the two halves of that: the
// conditions under which the swap happens at all, and the ordering that
// keeps a lowered fill painting where the mesh used to.

func lowerStops() []GradientStop {
	return []GradientStop{
		{Color: RGBA(255, 255, 255, 255), Pos: 0},
		{Color: RGBA(255, 200, 80, 128), Pos: 0.5},
		{Color: RGBA(255, 200, 80, 0), Pos: 1},
	}
}

// TestFilledCircleGradientLowersConcentric pins the substitution and
// the geometry that makes it exact: a square around the circle, with a
// corner radius that rounds it back down to that circle.
func TestFilledCircleGradientLowersConcentric(t *testing.T) {
	dc := NewDrawContext(200, 200, nil)
	stops := lowerStops()
	dc.FilledCircleGradient(50, 60, 20,
		&CanvasGradient{Radial: true, Stops: stops})

	if n := len(dc.Batches()); n != 0 {
		t.Errorf("got %d triangle batches, want none", n)
	}
	grads := dc.Gradients()
	if len(grads) != 1 {
		t.Fatalf("got %d gradients, want 1", len(grads))
	}
	e := grads[0]
	if e.X != 30 || e.Y != 40 || e.W != 40 || e.H != 40 {
		t.Errorf("quad = (%v,%v %vx%v), want (30,40 40x40)",
			e.X, e.Y, e.W, e.H)
	}
	if e.Def.Type != GradientRadial {
		t.Errorf("gradient type = %v, want radial", e.Def.Type)
	}
	if e.afterBatch != 0 {
		t.Errorf("afterBatch = %d, want 0 for the first fill",
			e.afterBatch)
	}
	if len(e.Def.Stops) != len(stops) {
		t.Fatalf("got %d stops, want %d", len(e.Def.Stops), len(stops))
	}
	for i, s := range stops {
		if e.Def.Stops[i] != s {
			t.Errorf("stop %d = %v, want %v", i, e.Def.Stops[i], s)
		}
	}
}

// TestFilledCircleGradientKeepsMeshWhenResampleWouldShow covers the
// one fidelity line the lowering will not cross. Being over the
// uniform slots is not itself disqualifying — a smooth ramp resamples
// to within a rounding step and is lowered anyway. A ramp that
// alternates every stop has no faithful short form, so it keeps the
// mesh, which reproduces every stop.
func TestFilledCircleGradientKeepsMeshWhenResampleWouldShow(t *testing.T) {
	stops := make([]GradientStop, 2*gradientShaderStopLimit+1)
	for i := range stops {
		c := RGBA(255, 255, 255, 255)
		if i%2 == 1 {
			c = RGBA(0, 0, 0, 255)
		}
		stops[i] = GradientStop{
			Color: c,
			Pos:   float32(i) / float32(len(stops)-1),
		}
	}
	dc := NewDrawContext(200, 200, nil)
	dc.FilledCircleGradient(50, 60, 20,
		&CanvasGradient{Radial: true, Stops: stops})

	if n := len(dc.Gradients()); n != 0 {
		t.Errorf("got %d lowered gradients, want none for a ramp the "+
			"uniforms cannot carry faithfully", n)
	}
	if len(dc.Batches()) == 0 {
		t.Error("no mesh emitted; the fill went nowhere")
	}
}

// TestFilledCircleGradientLowersTheInTreeHalo is the motivating case,
// pinned so a limit or threshold change cannot quietly drop it back to
// the mesh. It is the ten-stop accumulated-ring glow from
// examples/solar_system, which every frame of that example draws at
// least once.
func TestFilledCircleGradientLowersTheInTreeHalo(t *testing.T) {
	// alpha = 1 - exp(-k*(1-u)^4), flat across the body it surrounds:
	// two stops for the flat part, eight across the falloff.
	const inFrac = 700.0 / 1190.0
	k := float64(140*0.46) / 4
	at := func(u float64) Color {
		a := 1 - math.Exp(-k*math.Pow(1-u, 4))
		return RGB(255, 186, 78).WithOpacity(float32(a))
	}
	stops := []GradientStop{
		{Color: at(0), Pos: 0},
		{Color: at(0), Pos: inFrac},
	}
	for i := 1; i <= 8; i++ {
		u := float64(i) / 8
		stops = append(stops, GradientStop{
			Color: at(u), Pos: float32(inFrac + (1-inFrac)*u)})
	}

	dc := NewDrawContext(3000, 3000, nil)
	dc.FilledCircleGradient(1200, 1200, 1190,
		&CanvasGradient{Radial: true, Stops: stops})

	if n := len(dc.Gradients()); n != 1 {
		t.Fatalf("got %d lowered gradients, want 1: the halo this "+
			"whole path exists for went back to the mesh", n)
	}
	if n := len(dc.Batches()); n != 0 {
		t.Errorf("got %d triangle batches alongside it, want none", n)
	}
}

// TestFilledCircleGradientKeepsMeshWhenOffCenter covers the shape the
// shader cannot express at all: GradientDef carries no center of its
// own, so a ramp that is not concentric with its circle has to be
// tessellated.
func TestFilledCircleGradientKeepsMeshWhenOffCenter(t *testing.T) {
	for _, tc := range []struct {
		name string
		g    CanvasGradient
	}{
		{"center moved", CanvasGradient{
			Radial: true, R: 20, CX: 55, CY: 60, Stops: lowerStops()}},
		{"focal offset", CanvasGradient{
			Radial: true, R: 20, CX: 50, CY: 60, FX: 44, FY: 60,
			Stops: lowerStops()}},
		{"repeating spread", CanvasGradient{
			Radial: true, Spread: SpreadRepeat, Stops: lowerStops()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dc := NewDrawContext(200, 200, nil)
			dc.FilledCircleGradient(50, 60, 20, &tc.g)
			if n := len(dc.Gradients()); n != 0 {
				t.Errorf("got %d lowered gradients, want none", n)
			}
			if len(dc.Batches()) == 0 {
				t.Error("no mesh emitted; the fill went nowhere")
			}
		})
	}
}

// TestFilledCircleGradientRecorderKeepsGeometry holds the recorder
// contract: a recorder is promised the raw primitive, so a fill it is
// watching must never turn into a quad it cannot read.
func TestFilledCircleGradientRecorderKeepsGeometry(t *testing.T) {
	dc := NewDrawContext(200, 200, nil)
	dc.SetRecorder(nopRecorder{})
	dc.FilledCircleGradient(50, 60, 20,
		&CanvasGradient{Radial: true, Stops: lowerStops()})

	if n := len(dc.Gradients()); n != 0 {
		t.Errorf("got %d lowered gradients, want none under a recorder", n)
	}
}

// TestRenderDrawCanvasInterleavesGradients is the ordering gate. A halo
// is drawn before the body it surrounds, so a lowered halo that emitted
// after every triangle batch would paint over the body instead of
// behind it.
func TestRenderDrawCanvasInterleavesGradients(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		Width:     200, Height: 200,
		Color: ColorTransparent,
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.FilledRect(0, 0, 10, 10, Blue)
				dc.FilledCircleGradient(50, 60, 20,
					&CanvasGradient{Radial: true, Stops: lowerStops()})
				// A different color, so the run-length merge in
				// getBatch cannot fold this into the first batch and
				// hide the ordering.
				dc.FilledRect(20, 0, 10, 10, Red)
			},
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 300, 300), w)

	var kinds []renderKind
	for i := range w.renderers {
		switch w.renderers[i].Kind {
		case RenderSvg, RenderGradient:
			kinds = append(kinds, w.renderers[i].Kind)
		}
	}
	want := []renderKind{RenderSvg, RenderGradient, RenderSvg}
	if len(kinds) != len(want) {
		t.Fatalf("emitted %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("emitted %v, want %v", kinds, want)
		}
	}
}

// TestRenderDrawCanvasGradientOnlyNotSkipped covers the early-out: a
// canvas whose whole content lowered to gradients has no batches, no
// texts and no images, and must not be mistaken for an empty one.
func TestRenderDrawCanvasGradientOnlyNotSkipped(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		Width:     200, Height: 200,
		Color: ColorTransparent,
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.FilledCircleGradient(50, 60, 20,
					&CanvasGradient{Radial: true, Stops: lowerStops()})
			},
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 300, 300), w)

	found := false
	for i := range w.renderers {
		if w.renderers[i].Kind == RenderGradient {
			found = true
		}
	}
	if !found {
		t.Error("no gradient emitted for a gradient-only canvas")
	}
}

// TestDrawCanvasGradientRedrawReusesStops is the pooling gate, written
// by pointer identity for the reason spelled out on
// TestDrawCanvasRedrawReusesBuffers: allocation counting is too noisy
// over a whole redraw to mean anything.
func TestDrawCanvasGradientRedrawReusesStops(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ID:        "grad-canvas",
		shapeType: shapeDrawCanvas,
		Width:     200, Height: 200,
		Color: ColorTransparent,
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.FilledCircleGradient(50, 60, 20,
					&CanvasGradient{Radial: true, Stops: lowerStops()})
			},
		},
	}
	clip := makeClip(0, 0, 300, 300)
	var version uint64
	frame := func() {
		version++
		shape.Version = version
		w.renderers = w.renderers[:0]
		w.renderPass++
		renderDrawCanvas(shape, clip, w)
	}
	// Three frames to settle: the two entry arrays alternate, so each
	// is grown on a different frame.
	for range 3 {
		frame()
	}

	sm := StateMap[string, drawCanvasCache](w, nsDrawCanvas, capModerate)
	before, ok := sm.Get("grad-canvas")
	if !ok || len(before.Gradients) != 1 {
		t.Fatalf("cache holds %d gradients after redraw, want 1",
			len(before.Gradients))
	}
	was := &before.Gradients[0].Def.Stops[0]

	frame()

	after, _ := sm.Get("grad-canvas")
	if len(after.Gradients) != 1 {
		t.Fatalf("gradient count moved to %d", len(after.Gradients))
	}
	if got := &after.Gradients[0].Def.Stops[0]; got != was {
		t.Error("stop buffer re-allocated; the redraw is not recycling")
	}
}
