package gui

import (
	"math"
	"testing"
	"time"
)

// Shader containers in these tests are sbW x sbH logical pixels.
const sbW, sbH = 100, 50

// --- packing ---------------------------------------------------------

// packTest packs a block in logical pixels, then converts it the way a
// backend reads it, at scale.
func packTest(f *shaderFrameState, scale float32) [16]float32 {
	var dst [16]float32
	packShaderBuiltins(&dst, f, sbW, sbH, false)
	r := RenderCmd{shaderBuiltins: &dst}
	return r.ShaderBuiltinsAt(scale)
}

func TestShaderBuiltinsResolution(t *testing.T) {
	var f shaderFrameState
	f.advance(time.Unix(0, 0), 1)
	got := packTest(&f, 2)
	if got[shaderBuiltinResX] != 200 || got[shaderBuiltinResY] != 100 ||
		got[shaderBuiltinResZ] != 1 {
		t.Errorf("iResolution = %v, want (200,100,1)",
			got[shaderBuiltinResX:shaderBuiltinResZ+1])
	}
	for i := 10; i < 16; i++ {
		if got[i] != 0 {
			t.Errorf("reserved slot %d = %v, want 0", i, got[i])
		}
	}
}

func TestShaderBuiltinsClock(t *testing.T) {
	var f shaderFrameState
	t0 := time.Unix(100, 0)
	f.advance(t0, 1)
	got := packTest(&f, 1)
	if got[shaderBuiltinTime] != 0 || got[shaderBuiltinTimeDelta] != 0 ||
		got[shaderBuiltinFrame] != 0 {
		t.Errorf("first frame time/delta/frame = %v/%v/%v, want 0/0/0",
			got[shaderBuiltinTime], got[shaderBuiltinTimeDelta],
			got[shaderBuiltinFrame])
	}
	f.advance(t0.Add(100*time.Millisecond), 2)
	f.advance(t0.Add(150*time.Millisecond), 3)
	got = packTest(&f, 1)
	if got[shaderBuiltinTime] != 0.15 || got[shaderBuiltinTimeDelta] != 0.05 ||
		got[shaderBuiltinFrame] != 2 {
		t.Errorf("third frame time/delta/frame = %v/%v/%v, want 0.15/0.05/2",
			got[shaderBuiltinTime], got[shaderBuiltinTimeDelta],
			got[shaderBuiltinFrame])
	}
}

// After a pause, iTimeDelta is capped so a delta-driven animation does
// not jump by the whole pause; iTime still reports the real time.
func TestShaderBuiltinsTimeDeltaCapped(t *testing.T) {
	var f shaderFrameState
	t0 := time.Unix(100, 0)
	f.advance(t0, 1)
	f.advance(t0.Add(3*time.Second), 2)
	got := packTest(&f, 1)
	if got[shaderBuiltinTimeDelta] != shaderMaxTimeDelta {
		t.Errorf("iTimeDelta after a 3 s pause = %v, want %v",
			got[shaderBuiltinTimeDelta], shaderMaxTimeDelta)
	}
	if got[shaderBuiltinTime] != 3 {
		t.Errorf("iTime after a 3 s pause = %v, want 3 (not capped)",
			got[shaderBuiltinTime])
	}
}

func TestShaderBuiltinsHeadlessZeroesClock(t *testing.T) {
	var f shaderFrameState
	f.advance(time.Unix(0, 0), 1)
	f.advance(time.Unix(3, 0), 2)
	var got [16]float32
	packShaderBuiltins(&got, &f, sbW, sbH, true)
	if got[shaderBuiltinTime] != 0 || got[shaderBuiltinTimeDelta] != 0 ||
		got[shaderBuiltinFrame] != 0 {
		t.Error("headless pack should zero time, delta and frame")
	}
	if got[shaderBuiltinResX] != sbW {
		t.Error("headless pack should keep the resolution")
	}
}

// ShaderBuiltinsAt scales the resolution and leaves time and frame
// alone.
func TestShaderBuiltinsAtScales(t *testing.T) {
	block := [16]float32{
		shaderBuiltinResX: 100, shaderBuiltinResY: 50, shaderBuiltinResZ: 1,
		shaderBuiltinTime: 3, shaderBuiltinTimeDelta: 0.125,
		shaderBuiltinFrame: 7,
	}
	r := RenderCmd{shaderBuiltins: &block}
	got := r.ShaderBuiltinsAt(1.25)
	want := block
	want[shaderBuiltinResX], want[shaderBuiltinResY] = 125, 62.5
	if got != want {
		t.Errorf("at 1.25:\n got %v\nwant %v", got, want)
	}
	if block[shaderBuiltinResX] != 100 {
		t.Error("ShaderBuiltinsAt must not change the stored block")
	}
}

// A scale that is not a positive finite number counts as 1, and a
// command without a block reads zeros.
func TestShaderBuiltinsAtEdgeCases(t *testing.T) {
	block := [16]float32{shaderBuiltinResX: 100}
	r := RenderCmd{shaderBuiltins: &block}
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	for _, s := range []float32{0, -2, nan, inf} {
		if got := r.ShaderBuiltinsAt(s); got[shaderBuiltinResX] != 100 {
			t.Errorf("scale %v: res %v, want 100", s, got[shaderBuiltinResX])
		}
	}
	var none RenderCmd
	if got := none.ShaderBuiltinsAt(2); got != [16]float32{} {
		t.Errorf("no block: %v, want zeros", got)
	}
}

// --- emission and frames ----------------------------------------------

// shaderBox is a fixed-size shader container.
func shaderBox(id string, content ...View) View {
	return Column(ContainerCfg{
		ID: id, Width: sbW, Height: sbH, Sizing: FixedFixed,
		Padding: PaddingNone, SizeBorder: NoBorder,
		Shader:  &Shader{Metal: "m", GLSL: "g"},
		Content: content,
	})
}

// shaderRoot lays the views from content out in a column at the
// window's top-left corner. content runs every frame.
func shaderRoot(content func() []View) func(*Window) View {
	return func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, Spacing: SpacingPx(0),
			Content: content(),
		})
	}
}

func shaderWindow(t *testing.T, view func(*Window) View) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.SetView(view)
	shaderFrameStep(w)
	return w
}

// shaderFrameStep draws one frame the way the backend loop does:
// FrameFn advances the frame counter, then rebuilds the view, the
// layout and the render commands.
func shaderFrameStep(w *Window) {
	w.markLayoutRefresh(refreshTest)
	w.FrameFn()
}

// A shader container's command carries the built-in block, and it
// points at the shape's own storage. The block is in logical pixels
// whatever BackingScale says: that field lags a frame behind.
func TestRenderLayoutShaderEmitsBuiltins(t *testing.T) {
	w := makeWindow()
	w.BackingScale = 2
	shape := &Shape{
		shapeType: shapeRectangle,
		Color:     ColorTransparent,
		X:         10, Y: 20, Width: sbW, Height: sbH,
		fx: &shapeEffects{Shader: &Shader{Metal: "m", GLSL: "g"}},
	}
	renderLayout(&Layout{Shape: shape}, ColorTransparent,
		makeClip(0, 0, 200, 200), w)

	for _, r := range w.renderers {
		if r.Kind != RenderCustomShader {
			continue
		}
		if r.shaderBuiltins != &shape.fx.shaderBuiltins {
			t.Fatal("shaderBuiltins should point at the shape's storage")
		}
		if r.shaderBuiltins[shaderBuiltinResX] != sbW ||
			r.shaderBuiltins[shaderBuiltinResY] != sbH {
			t.Errorf("stored iResolution = %v,%v, want logical %v,%v",
				r.shaderBuiltins[shaderBuiltinResX],
				r.shaderBuiltins[shaderBuiltinResY], sbW, sbH)
		}
		return
	}
	t.Fatal("missing RenderCustomShader")
}

// iFrame counts frames, not render passes: a frame FrameFn rebuilds
// twice advances it once.
func TestShaderBuiltinsFrameSpansRenderPasses(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{shaderBox("sh")}
	}))
	frameOf := func() float32 {
		return mustShape(t, w, "sh").fx.shaderBuiltins[shaderBuiltinFrame]
	}
	start := frameOf()
	w.frameCount++
	for pass := 1; pass <= 2; pass++ {
		w.markLayoutRefresh(refreshTest)
		w.Update()
		if got := frameOf(); got != start+1 {
			t.Errorf("pass %d: iFrame = %v, want %v", pass, got, start+1)
		}
	}
	shaderFrameStep(w)
	if got := frameOf(); got != start+2 {
		t.Errorf("next frame: iFrame = %v, want %v", got, start+2)
	}
}

// iMouse is not tracked: its slots stay zero, Shadertoy's value for a
// canvas never clicked, even under a press and drag on the shader.
func TestShaderBuiltinsMouseStaysZero(t *testing.T) {
	w := shaderWindow(t, shaderRoot(func() []View {
		return []View{shaderBox("sh")}
	}))
	s := mustShape(t, w, "sh")
	x, y := s.X+30, s.Y+20
	for _, e := range []Event{
		{Type: EventMouseDown, MouseButton: MouseLeft, MouseX: x, MouseY: y},
		{Type: EventMouseMove, MouseButton: MouseLeft, MouseX: x + 10, MouseY: y},
	} {
		w.EventFn(&e)
	}
	shaderFrameStep(w)
	b := mustShape(t, w, "sh").fx.shaderBuiltins
	if got := [4]float32(b[shaderBuiltinMouse : shaderBuiltinMouse+4]); got != [4]float32{} {
		t.Errorf("iMouse during a drag = %v, want zero", got)
	}
}
